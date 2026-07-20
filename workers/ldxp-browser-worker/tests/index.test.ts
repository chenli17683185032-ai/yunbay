import test from 'node:test'
import assert from 'node:assert/strict'
import {
  createWorkerRuntime,
  buildErrorPayload,
  processClaimedSession,
  runClaimLoop,
  runClaimLoopOnce,
  runPaidWatchLoopOnce,
  type WorkerRuntimeDependencies,
} from '../src/index.js'
import type { WorkerConfig } from '../src/config.js'
import type { ClaimedSession } from '../src/backend.js'
import type { BrowserPaidResult, BrowserQrResult } from '../src/browser-flow.js'

function config(overrides: Partial<WorkerConfig> = {}): WorkerConfig {
  return {
    backendBaseUrl: 'https://backend.example',
    workerToken: 'worker-token-secret',
    workerId: 'worker-a',
    pollIntervalMs: 1000,
    claimIntervalMs: 1000,
    maxConcurrentSessions: 2,
    productLoadTimeoutMs: 30000,
    qrTimeoutMs: 60000,
    paymentTimeoutMs: 900000,
    resultTimeoutMs: 120000,
    releaseSessionSlotAfterQr: false,
    browserPrewarm: false,
    paidWatchPollIntervalMs: 1000,
    debugSnapshotDir: '/app/snapshots',
    mockMode: false,
    ...overrides,
  }
}

test('runClaimLoopOnce tracks active sessions and claims freed slots on the next loop', async () => {
  const claimed: string[] = []
  const browserFlows: Array<Deferred<BrowserPaidResult>> = []
  const deps: WorkerRuntimeDependencies = {
    claimSession: async (_config) => {
      const next = ['sess-1', 'sess-2', 'sess-3'][claimed.length]
      if (!next) {
        return null
      }
      claimed.push(next)
      return claimedSession(next)
    },
    claimPaidWatchSession: async () => null,
    isSessionActive: async () => true,
    runBrowserFlow: async () => {
      const flow = deferred<BrowserPaidResult>()
      browserFlows.push(flow)
      return flow.promise
    },
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async () => undefined,
    runMailPoller: async () => undefined,
    runMockFlow: () => {
      throw new Error('runMockFlow should not be called')
    },
    logger: createNoopLogger(),
  }

  const runtime = createWorkerRuntime(config({ maxConcurrentSessions: 2 }), deps)
  const started = await runClaimLoopOnce(runtime)

  assert.equal(started, 2)
  assert.deepEqual(claimed, ['sess-1', 'sess-2'])
  assert.equal(runtime.activeSessions.size, 2)

  browserFlows[0].resolve(paidResult('order-1'))
  browserFlows[1].resolve(paidResult('order-2'))
  await waitFor(() => runtime.activeSessions.size === 0)

  const startedAfterRelease = await runClaimLoopOnce(runtime)
  assert.equal(startedAfterRelease, 1)
  assert.deepEqual(claimed, ['sess-1', 'sess-2', 'sess-3'])

  browserFlows[2].resolve(paidResult('order-3'))
  await runtime.shutdown()
})

test('runClaimLoopOnce does not start a session that resolves after shutdown', async () => {
  const claim = deferred<ClaimedSession | null>()
  let browserFlowCalls = 0
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => claim.promise,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async () => {
      browserFlowCalls += 1
      return paidResult('late')
    },
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async () => undefined,
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  const loopOnce = runClaimLoopOnce(runtime)
  let shutdownSettled = false
  const shutdown = runtime.shutdown().then(() => {
    shutdownSettled = true
  })
  await new Promise((resolve) => setTimeout(resolve, 20))
  assert.equal(shutdownSettled, false)

  claim.resolve(claimedSession('sess-late'))
  const started = await loopOnce
  await shutdown

  assert.equal(started, 0)
  assert.equal(shutdownSettled, true)
  assert.equal(browserFlowCalls, 0)
  assert.equal(runtime.activeSessions.size, 0)
})

test('buildErrorPayload redacts JSON header and token field forms', async () => {
  const payload = buildErrorPayload(
    new Error('headers={"Authorization":"Bearer bearer_789","X-LDXP-Worker-Token":"head_123","workerToken":"tok_123","access_token":"access_123","api_key":"key_123"}'),
    config(),
  )

  for (const sensitive of ['bearer_789', 'head_123', 'tok_123', 'access_123', 'key_123']) {
    assert.doesNotMatch(payload.error_message, new RegExp(escapeRegExp(sensitive)))
  }
})

test('buildErrorPayload redacts escaped JSON header and token field forms', async () => {
  const payload = buildErrorPayload(
    new Error('headers={\\"Authorization\\":\\"Bearer bearer_789\\",\\"X-LDXP-Worker-Token\\":\\"head_123\\",\\"workerToken\\":\\"tok_123\\",\\"Authorization\\":[\\"Bearer bearer_arr\\"],\\"X-LDXP-Worker-Token\\":[\\"head_arr\\"]}'),
    config(),
  )

  for (const sensitive of ['bearer_789', 'head_123', 'tok_123', 'bearer_arr', 'head_arr']) {
    assert.doesNotMatch(payload.error_message, new RegExp(escapeRegExp(sensitive)))
  }
})

test('processClaimedSession posts qr and result callbacks', async () => {
  const events: string[] = []
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async (_input, callbacks) => {
      const qr: BrowserQrResult = {
        worker_order_no: 'order-1',
        worker_amount: 10,
        worker_product_name: 'Product',
        qr_code: 'data:image/png;base64,AAAA',
        qr_page_url: 'https://example.test/qr',
      }
      await callbacks.onQr(qr)
      return {
        worker_order_no: 'order-1',
        worker_amount: 10,
        worker_product_name: 'Product',
        worker_card_key: 'abcd1234efgh5678',
        worker_status_text: '支付成功',
        worker_success_url: 'https://example.test/success',
      } satisfies BrowserPaidResult
    },
    postQr: async (_config, sessionId, payload) => {
      events.push(`qr:${sessionId}`)
      assert.equal(payload.worker_order_no, 'order-1')
      assert.equal(payload.qr_code, 'data:image/png;base64,AAAA')
    },
    postResult: async (_config, sessionId, payload) => {
      events.push(`result:${sessionId}`)
      assert.equal(payload.worker_card_key, 'abcd1234efgh5678')
    },
    postMailEvent: async () => {
      throw new Error('postMailEvent should not be called outside mock mode')
    },
    postError: async () => {
      throw new Error('postError should not be called for success')
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  await processClaimedSession(runtime, {
    session_id: 'sess-1',
    amount: 10,
    money: 10,
    product_url: 'https://example.test/product',
    product_name: 'Product',
    contact_email: 'buyer@example.test',
  })

  assert.deepEqual(events, ['qr:sess-1', 'result:sess-1'])
  await runtime.shutdown()
})

test('processClaimedSession logs qr timings as soon as qr is posted', async () => {
  const logs: Array<{ obj: unknown; msg?: string }> = []
  const logger = createCapturingLogger(logs)
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async (_input, callbacks, _config, _signal, diagnostics) => {
      diagnostics?.timings.push({ stage: 'click_purchase_to_cashier', ms: 8123 })
      const qr: BrowserQrResult = {
        worker_order_no: 'order-timing',
        worker_amount: 10,
        worker_product_name: 'Product',
        qr_code: 'data:image/png;base64,CCCC',
        qr_page_url: 'https://example.test/qr',
      }
      await callbacks.onQr(qr)
      return paidResult('order-timing')
    },
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async () => {
      throw new Error('postError should not be called for success')
    },
    runMailPoller: async () => undefined,
    logger,
  })

  await processClaimedSession(runtime, claimedSession('sess-timing'))

  const qrLog = logs.find((entry) => entry.msg === 'qr posted')
  assert.ok(qrLog)
  assert.deepEqual((qrLog.obj as { timings?: unknown }).timings, [
    { stage: 'click_purchase_to_cashier', ms: 8123 },
  ])
  await runtime.shutdown()
})

test('processClaimedSession releases the worker slot after posting qr', async () => {
  const events: string[] = []
  const runtime = createWorkerRuntime(config({ releaseSessionSlotAfterQr: true }), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async (_input, callbacks) => {
      const qr: BrowserQrResult = {
        worker_order_no: 'order-qr-only',
        worker_amount: 10,
        worker_product_name: 'Product',
        qr_code: 'data:image/png;base64,BBBB',
        qr_page_url: 'https://example.test/qr',
      }
      await callbacks.onQr(qr)
      return { kind: 'qr_posted', worker_order_no: qr.worker_order_no }
    },
    postQr: async (_config, sessionId, payload) => {
      events.push(`qr:${sessionId}`)
      assert.equal(sessionId, 'sess-qr-only')
      assert.equal(payload.worker_order_no, 'order-qr-only')
    },
    postResult: async () => {
      throw new Error('postResult should not be called before the user pays')
    },
    postMailEvent: async () => undefined,
    postError: async () => {
      throw new Error('postError should not be called after qr was posted')
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  const processing = processClaimedSession(runtime, claimedSession('sess-qr-only'))

  await processing
  assert.deepEqual(events, ['qr:sess-qr-only'])
  await runtime.shutdown()
})

test('runClaimLoopOnce keeps claiming after qr-only sessions release their slots', async () => {
  const claimed: string[] = []
  const events: string[] = []
  const runtime = createWorkerRuntime(config({ maxConcurrentSessions: 2, releaseSessionSlotAfterQr: true }), {
    claimSession: async () => {
      const next = ['sess-1', 'sess-2', 'sess-3'][claimed.length]
      if (!next) {
        return null
      }
      claimed.push(next)
      return claimedSession(next)
    },
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async (input, callbacks) => {
      const qr: BrowserQrResult = {
        worker_order_no: `order-${input.sessionId}`,
        worker_amount: 10,
        worker_product_name: 'Product',
        qr_code: 'data:image/png;base64,BBBB',
        qr_page_url: 'https://example.test/qr',
      }
      await callbacks.onQr(qr)
      return { kind: 'qr_posted', worker_order_no: qr.worker_order_no }
    },
    postQr: async (_config, sessionId) => {
      events.push(`qr:${sessionId}`)
    },
    postResult: async () => {
      throw new Error('postResult should not be called when qr-only mode releases the slot')
    },
    postMailEvent: async () => undefined,
    postError: async () => {
      throw new Error('postError should not be called for qr-only success')
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  const firstPassStarted = await runClaimLoopOnce(runtime)
  await waitFor(() => runtime.activeSessions.size === 0)
  const secondPassStarted = await runClaimLoopOnce(runtime)
  await waitFor(() => runtime.activeSessions.size === 0)

  assert.equal(firstPassStarted, 2)
  assert.equal(secondPassStarted, 1)
  assert.deepEqual(claimed, ['sess-1', 'sess-2', 'sess-3'])
  assert.deepEqual(events, ['qr:sess-1', 'qr:sess-2', 'qr:sess-3'])
  await runtime.shutdown()
})

test('runPaidWatchLoopOnce posts result when watched qr page reaches paid state', async () => {
  const events: string[] = []
  const fakeContext = {
    newPage: async () => ({
      setDefaultTimeout: () => undefined,
      goto: async () => undefined,
      waitForFunction: async () => undefined,
      locator: () => ({
        innerText: async () => '订单详情 - 链动小铺\n已付款\n订单号：LDWATCH001',
      }),
      url: () => 'https://pay.ldxp.cn/order/result/LDWATCH001',
    }),
    close: async () => undefined,
  }
  const runtime = createWorkerRuntime(config({ releaseSessionSlotAfterQr: true, paidWatchPollIntervalMs: 10 }), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => ({
      session_id: 'sess-watch',
      amount: 10,
      money: 0.1,
      worker_order_no: 'LDWATCH001',
      worker_amount: 0.1,
      worker_product_name: '0.1 元测试',
      qr_page_url: 'https://example.test/qr',
      expires_at: 2000,
    }),
    runBrowserFlow: async () => paidResult('unused'),
    postQr: async () => undefined,
    postResult: async (_config, sessionId, payload) => {
      events.push(`result:${sessionId}:${payload.worker_order_no}:${payload.worker_amount}:${payload.worker_card_key}`)
    },
    postMailEvent: async () => undefined,
    postError: async () => undefined,
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })
  runtime.browserManager = {
    getContext: async () => fakeContext as never,
    close: async () => undefined,
    restart: async () => undefined,
  }

  const started = await runPaidWatchLoopOnce(runtime)
  await waitFor(() => runtime.activePaidWatches.size === 0)

  assert.equal(started, 1)
  assert.deepEqual(events, ['result:sess-watch:LDWATCH001:0.1:'])
  await runtime.shutdown()
})

test('processClaimedSession posts mock qr, result, and mail without running browser flow', async () => {
  const events: string[] = []
  let browserFlowCalls = 0
  const runtime = createWorkerRuntime(config({ mockMode: true, mockCardKey: 'mock-card-key-1234' }), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async () => {
      browserFlowCalls += 1
      return paidResult('unexpected')
    },
    runMockFlow: () => ({
      qr: {
        worker_order_no: 'LDMOCKSESS1',
        worker_amount: 0.1,
        worker_product_name: 'Mock Product',
        qr_code: 'data:image/png;base64,AAAA',
        qr_page_url: 'https://example.test/qr',
      },
      result: {
        worker_order_no: 'LDMOCKSESS1',
        worker_amount: 0.1,
        worker_product_name: 'Mock Product',
        worker_card_key: 'mock-card-key-1234',
        worker_status_text: 'mock paid',
        worker_success_url: 'https://example.test/success',
      },
      mailEvent: {
        order_no: 'LDMOCKSESS1',
        amount: 0.1,
        product_name: 'Mock Product',
        card_key: 'mock-card-key-1234',
        raw_hash: 'hash',
      },
    }),
    postQr: async (_config, sessionId, payload) => {
      events.push(`qr:${sessionId}:${payload.worker_order_no}`)
    },
    postResult: async (_config, sessionId, payload) => {
      events.push(`result:${sessionId}:${payload.worker_order_no}:${payload.worker_card_key}`)
    },
    postMailEvent: async (_config, payload) => {
      events.push(`mail:${payload.order_no}:${payload.card_key}`)
    },
    postError: async () => {
      throw new Error('postError should not be called for mock success')
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  await processClaimedSession(runtime, claimedSession('sess-1'))

  assert.equal(browserFlowCalls, 0)
  assert.deepEqual(events, [
    'qr:sess-1:LDMOCKSESS1',
    'result:sess-1:LDMOCKSESS1:mock-card-key-1234',
    'mail:LDMOCKSESS1:mock-card-key-1234',
  ])
  await runtime.shutdown()
})

test('processClaimedSession posts sanitized error payload on failure', async () => {
  const payloads: unknown[] = []
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async () => {
      throw new Error('payment failed with token secret and qr data:image/png;base64,AAAA')
    },
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async (_config, _sessionId, payload) => {
      payloads.push(payload)
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  await processClaimedSession(runtime, {
    session_id: 'sess-1',
    amount: 10,
    money: 10,
    product_url: 'https://example.test/product',
    product_name: 'Product',
    contact_email: 'buyer@example.test',
  })

  assert.equal(payloads.length, 1)
  const errorPayload = payloads[0] as { error_code: string; error_message: string; snapshot_path?: string }
  assert.equal(errorPayload.error_code, 'worker_flow_failed')
  assert.match(errorPayload.error_message, /payment failed/i)
  assert.doesNotMatch(errorPayload.error_message, /secret/i)
  assert.doesNotMatch(errorPayload.error_message, /data:image\/png;base64/i)
  await runtime.shutdown()
})

test('buildErrorPayload classifies paid result parse errors after qr callback', async () => {
  const payload = buildErrorPayload(
    new Error(
      'Browser flow failed for session sess-1 product https://example.test/product qr=called snapshot=/tmp/snapshots/sess-1.html: Unable to extract amount from page text',
    ),
    config(),
  )

  assert.equal(payload.error_code, 'paid_result_parse_failed')
  assert.equal(payload.snapshot_path, 'sess-1.html')
})

test('buildErrorPayload classifies Aliyun ESA challenge failures', () => {
  const payload = buildErrorPayload(
    new Error('Browser flow failed: LDXP WAF challenge blocked product page'),
    config(),
  )

  assert.equal(payload.error_code, 'waf_challenge')
})

test('buildErrorPayload preserves snapshot path without leaking sensitive data', async () => {
  const payload = buildErrorPayload(
    new Error(
      'Browser flow failed for session sess-1 product https://example.test/product qr=called snapshot=/tmp/snapshots/sess-1-20240628010101.png,/tmp/snapshots/sess-1-20240628010101.html: token secret and qr data:image/png;base64,AAAA',
    ),
    config(),
  )

  assert.equal(payload.error_code, 'worker_flow_failed')
  assert.equal(payload.snapshot_path, 'sess-1-20240628010101.png,sess-1-20240628010101.html')
  assert.doesNotMatch(payload.error_message, /secret/i)
  assert.doesNotMatch(payload.error_message, /data:image\/png;base64/i)
})

test('runClaimLoop starts mail poller and exits when runtime is shut down', async () => {
  let runtime!: ReturnType<typeof createWorkerRuntime>
  let mailPollerStarts = 0
  let claimAttempts = 0

  runtime = createWorkerRuntime(config({ pollIntervalMs: 5, imapHost: 'imap.qq.com', imapUser: '10256345@qq.com', imapPassword: 'imap-secret' }), {
    claimSession: async () => {
      claimAttempts += 1
      if (claimAttempts >= 2) {
        queueMicrotask(() => void runtime.shutdown())
      }
      return null
    },
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async () => paidResult('unused'),
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async () => undefined,
    runMailPoller: async (_config, signal) => {
      mailPollerStarts += 1
      await new Promise<void>((resolve) => signal.addEventListener('abort', () => resolve(), { once: true }))
    },
    logger: createNoopLogger(),
  })

  await runClaimLoop(runtime)

  assert.equal(mailPollerStarts, 1)
  assert.ok(claimAttempts >= 2)
  assert.equal(runtime.signal.aborted, true)
})

test('startMailPoller restarts after unexpected poller exit and does not start duplicates', async () => {
  let starts = 0
  const runtime = createWorkerRuntime(config({ pollIntervalMs: 1, imapHost: 'imap.qq.com', imapUser: '10256345@qq.com', imapPassword: 'imap-secret' }), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: async () => paidResult('unused'),
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async () => undefined,
    runMailPoller: async (_config, signal) => {
      starts += 1
      if (starts === 1) {
        throw new Error('temporary mail poller failure')
      }
      await new Promise<void>((resolve) => signal.addEventListener('abort', () => resolve(), { once: true }))
    },
    logger: createNoopLogger(),
  })

  await runtime.startMailPoller()
  await runtime.startMailPoller()
  await new Promise((resolve) => setTimeout(resolve, 50))
  assert.equal(starts, 1)
  await waitFor(() => starts === 2, 1500)
  await runtime.shutdown()

  assert.equal(starts, 2)
})

test('processClaimedSession aborts browser flow when backend says the session was canceled', async () => {
  let receivedSignal: AbortSignal | undefined
  let postErrorCalls = 0
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    isSessionActive: async () => false,
    runBrowserFlow: (async (...args: unknown[]) => {
      receivedSignal = args[3] as AbortSignal | undefined
      await new Promise<never>((_resolve, reject) => {
        receivedSignal?.addEventListener('abort', () => {
          const error = new Error('aborted after user cancel')
          error.name = 'AbortError'
          reject(error)
        }, { once: true })
      })
    }) as unknown as typeof import('../src/browser-flow.js').runBrowserFlow,
    postQr: async () => {
      throw new Error('postQr should not be called after cancellation')
    },
    postResult: async () => {
      throw new Error('postResult should not be called after cancellation')
    },
    postMailEvent: async () => undefined,
    postError: async () => {
      postErrorCalls += 1
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  const processing = processClaimedSession(runtime, claimedSession('sess-canceled'))
  await waitFor(() => receivedSignal !== undefined)
  await processing

  assert.equal(receivedSignal?.aborted, true)
  assert.equal(postErrorCalls, 0)
  assert.equal(runtime.activeSessions.size, 0)
})

test('processClaimedSession does not hang shutdown while browser flow waits', async () => {
  let receivedSignal: AbortSignal | undefined
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
    claimPaidWatchSession: async () => null,
    runBrowserFlow: (async (...args: unknown[]) => {
      receivedSignal = args[3] as AbortSignal | undefined
      await new Promise<never>((_resolve, reject) => {
        receivedSignal?.addEventListener('abort', () => {
          const error = new Error('aborted')
          error.name = 'AbortError'
          reject(error)
        }, { once: true })
      })
    }) as unknown as typeof import('../src/browser-flow.js').runBrowserFlow,
    postQr: async () => undefined,
    postResult: async () => undefined,
    postMailEvent: async () => undefined,
    postError: async () => {
      throw new Error('postError should not be called during shutdown abort')
    },
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  })

  const processing = processClaimedSession(runtime, claimedSession('sess-shutdown'))
  await waitFor(() => receivedSignal !== undefined)
  await runtime.shutdown()
  await processing

  assert.equal(receivedSignal?.aborted, true)
})

test('buildErrorPayload redacts common secret, card, QR, and authorization forms', async () => {
  const payload = buildErrorPayload(
    new Error('workerToken=tok_123 imapPassword: pass_456 Authorization: Bearer bearer_789 X-LDXP-Worker-Token: head_123 token=query_secret&next=1 卡密：card_secret qr_code=data:image/png;base64,AAAA snapshot=/app/snapshots/shot.png,/app/snapshots/shot.html'),
    config(),
  )

  assert.equal(payload.snapshot_path, 'shot.png,shot.html')
  for (const sensitive of ['tok_123', 'pass_456', 'bearer_789', 'head_123', 'query_secret', 'card_secret', 'data:image/png;base64']) {
    assert.doesNotMatch(payload.error_message, new RegExp(escapeRegExp(sensitive)))
  }
})

function createNoopLogger() {
  return {
    info: () => undefined,
    warn: () => undefined,
    error: () => undefined,
    debug: () => undefined,
    child: () => createNoopLogger(),
  }
}

function createCapturingLogger(logs: Array<{ obj: unknown; msg?: string }>) {
  return {
    info: (obj: unknown, msg?: string) => logs.push({ obj, msg }),
    warn: (obj: unknown, msg?: string) => logs.push({ obj, msg }),
    error: (obj: unknown, msg?: string) => logs.push({ obj, msg }),
    debug: (obj: unknown, msg?: string) => logs.push({ obj, msg }),
    child: () => createCapturingLogger(logs),
  }
}

interface Deferred<T> {
  promise: Promise<T>
  resolve: (value: T) => void
  reject: (error: unknown) => void
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function claimedSession(sessionId: string): ClaimedSession {
  return {
    session_id: sessionId,
    amount: 10,
    money: 10,
    product_url: 'https://example.test/product',
    product_name: 'Product',
    contact_email: 'buyer@example.test',
  }
}

function paidResult(orderNo: string): BrowserPaidResult {
  return {
    worker_order_no: orderNo,
    worker_amount: 10,
    worker_product_name: 'Product',
    worker_card_key: `card-${orderNo}`,
    worker_status_text: '支付成功',
    worker_success_url: 'https://example.test/success',
  }
}

async function waitFor(predicate: () => boolean, timeoutMs = 500): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (predicate()) {
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 5))
  }
  assert.equal(predicate(), true)
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
