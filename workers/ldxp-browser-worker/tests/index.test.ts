import test from 'node:test'
import assert from 'node:assert/strict'
import {
  createWorkerRuntime,
  buildErrorPayload,
  processClaimedSession,
  runClaimLoopOnce,
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
    debugSnapshotDir: '/app/snapshots',
    ...overrides,
  }
}

test('runClaimLoopOnce claims only available slots', async () => {
  const claimed: string[] = []
  const deps: WorkerRuntimeDependencies = {
    claimSession: async (_config) => {
      const next = ['sess-1', 'sess-2', 'sess-3'][claimed.length]
      if (!next) {
        return null
      }
      claimed.push(next)
      return {
        session_id: next,
        amount: 10,
        money: 10,
        product_url: 'https://example.test/product',
        product_name: 'Product',
        contact_email: 'buyer@example.test',
      } satisfies ClaimedSession
    },
    runBrowserFlow: async () => {
      throw new Error('runBrowserFlow should not be called in claim test')
    },
    postQr: async () => undefined,
    postResult: async () => undefined,
    postError: async () => undefined,
    runMailPoller: async () => undefined,
    logger: createNoopLogger(),
  }

  const runtime = createWorkerRuntime(config({ maxConcurrentSessions: 2 }), deps)
  const started = await runClaimLoopOnce(runtime)

  assert.equal(started, 2)
  assert.deepEqual(claimed, ['sess-1', 'sess-2'])
  await runtime.shutdown()
})

test('processClaimedSession posts qr and result callbacks', async () => {
  const events: string[] = []
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
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

test('processClaimedSession posts sanitized error payload on failure', async () => {
  const payloads: unknown[] = []
  const runtime = createWorkerRuntime(config(), {
    claimSession: async () => null,
    runBrowserFlow: async () => {
      throw new Error('payment failed with token secret and qr data:image/png;base64,AAAA')
    },
    postQr: async () => undefined,
    postResult: async () => undefined,
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
  assert.doesNotMatch(errorPayload.error_message, /token/i)
  assert.doesNotMatch(errorPayload.error_message, /data:image\/png;base64/i)
  await runtime.shutdown()
})

test('buildErrorPayload preserves snapshot path without leaking sensitive data', async () => {
  const payload = buildErrorPayload(
    new Error(
      'Browser flow failed for session sess-1 product https://example.test/product qr=called snapshot=/tmp/snapshots/sess-1-20240628010101.png,/tmp/snapshots/sess-1-20240628010101.html: token secret and qr data:image/png;base64,AAAA',
    ),
    config(),
  )

  assert.equal(payload.error_code, 'worker_flow_failed')
  assert.equal(payload.snapshot_path, '/tmp/snapshots/sess-1-20240628010101.png,/tmp/snapshots/sess-1-20240628010101.html')
  assert.doesNotMatch(payload.error_message, /token/i)
  assert.doesNotMatch(payload.error_message, /data:image\/png;base64/i)
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
