import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import { once } from 'node:events'
import {
  claimSession,
  claimPaidWatchSession,
  isSessionActive,
  postError,
  postMailEvent,
  postQr,
  postResult,
  type WorkerErrorPayload,
  type WorkerMailEventPayload,
  type WorkerQrPayload,
  type WorkerResultPayload,
} from '../src/backend.js'
import type { WorkerConfig } from '../src/config.js'

interface CapturedRequest {
  method: string | undefined
  url: string | undefined
  token: string | undefined
  body: unknown
}

async function readBody(req: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = []
  for await (const chunk of req) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  }
  const raw = Buffer.concat(chunks).toString('utf8')
  return raw === '' ? null : JSON.parse(raw)
}

async function withServer(
  handler: (req: IncomingMessage, res: ServerResponse) => void | Promise<void>,
): Promise<{ baseUrl: string; close: () => Promise<void> }> {
  const server = createServer((req, res) => {
    void Promise.resolve(handler(req, res)).catch((error) => {
      res.statusCode = 500
      res.end(JSON.stringify({ success: false, message: String(error) }))
    })
  })
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  const address = server.address()
  assert(address && typeof address === 'object')
  const { port } = address as AddressInfo
  return {
    baseUrl: `http://127.0.0.1:${port}`,
    close: () => new Promise((resolve, reject) => server.close((error) => (error ? reject(error) : resolve()))),
  }
}

function config(baseUrl: string): WorkerConfig {
  return {
    backendBaseUrl: baseUrl,
    workerToken: 'worker-token-secret',
    workerId: 'worker-a',
    pollIntervalMs: 1000,
    claimIntervalMs: 1000,
    maxConcurrentSessions: 3,
    productLoadTimeoutMs: 30000,
    qrTimeoutMs: 60000,
    paymentTimeoutMs: 900000,
    resultTimeoutMs: 120000,
    releaseSessionSlotAfterQr: false,
    browserPrewarm: false,
    paidWatchPollIntervalMs: 1000,
    debugSnapshotDir: '/app/snapshots',
    mockMode: false,
  }
}

test('claimSession posts worker id and returns claimed session data', async () => {
  let captured: CapturedRequest | undefined
  const server = await withServer(async (req, res) => {
    captured = {
      method: req.method,
      url: req.url,
      token: req.headers['x-ldxp-worker-token'] as string | undefined,
      body: await readBody(req),
    }
    res.setHeader('content-type', 'application/json')
    res.end(
      JSON.stringify({
        success: true,
        data: {
          session_id: 'sess-1',
          amount: 2,
          money: 19.9,
          product_url: 'https://example.test/product',
          product_name: 'Product',
          contact_email: 'buyer@example.test',
        },
      }),
    )
  })

  try {
    const session = await claimSession(config(server.baseUrl))
    assert.deepEqual(captured, {
      method: 'POST',
      url: '/api/ldxp/worker/sessions/claim',
      token: 'worker-token-secret',
      body: { worker_id: 'worker-a' },
    })
    assert.equal(session?.session_id, 'sess-1')
  } finally {
    await server.close()
  }
})


test('isSessionActive posts worker id and returns backend active flag', async () => {
  let captured: CapturedRequest | undefined
  const server = await withServer(async (req, res) => {
    captured = {
      method: req.method,
      url: req.url,
      token: req.headers['x-ldxp-worker-token'] as string | undefined,
      body: await readBody(req),
    }
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({
      success: true,
      data: {
        session_id: 'sess-active',
        status: 'worker_claimed',
        active: true,
      },
    }))
  })

  try {
    const active = await isSessionActive(config(server.baseUrl), 'sess-active')
    assert.deepEqual(captured, {
      method: 'POST',
      url: '/api/ldxp/worker/sessions/sess-active/active',
      token: 'worker-token-secret',
      body: { worker_id: 'worker-a' },
    })
    assert.equal(active, true)
  } finally {
    await server.close()
  }
})

test('isSessionActive treats missing or canceled sessions as inactive', async () => {
  const server = await withServer(async (_req, res) => {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({
      success: true,
      data: {
        session_id: 'sess-canceled',
        status: 'canceled',
        active: false,
      },
    }))
  })

  try {
    const active = await isSessionActive(config(server.baseUrl), 'sess-canceled')
    assert.equal(active, false)
  } finally {
    await server.close()
  }
})

test('claimPaidWatchSession posts worker id and returns qr-ready watch data', async () => {
  let captured: CapturedRequest | undefined
  const server = await withServer(async (req, res) => {
    captured = {
      method: req.method,
      url: req.url,
      token: req.headers['x-ldxp-worker-token'] as string | undefined,
      body: await readBody(req),
    }
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({
      success: true,
      data: {
        session_id: 'sess-watch',
        amount: 10,
        money: 0.1,
        worker_order_no: 'LDWATCH001',
        worker_amount: 0.1,
        worker_product_name: '0.1 元测试',
        qr_page_url: 'https://example.test/qr',
        expires_at: 2000,
      },
    }))
  })

  try {
    const session = await claimPaidWatchSession(config(server.baseUrl))
    assert.deepEqual(captured, {
      method: 'POST',
      url: '/api/ldxp/worker/sessions/claim-paid-watch',
      token: 'worker-token-secret',
      body: { worker_id: 'worker-a' },
    })
    assert.equal(session?.session_id, 'sess-watch')
    assert.equal(session?.worker_order_no, 'LDWATCH001')
  } finally {
    await server.close()
  }
})

test('claimSession treats a plain-text 404 as no job without parsing JSON', async () => {
  const noJob = await withServer((_req, res) => {
    res.statusCode = 404
    res.end('not found')
  })
  try {
    assert.equal(await claimSession(config(noJob.baseUrl)), null)
  } finally {
    await noJob.close()
  }
})

test('claimSession treats empty success data as no job', async () => {
  const emptyData = await withServer((_req, res) => {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ success: true, data: null }))
  })
  try {
    assert.equal(await claimSession(config(emptyData.baseUrl)), null)
  } finally {
    await emptyData.close()
  }
})

test('claimSession rejects a real backend error', async () => {
  const server = await withServer((_req, res) => {
    res.statusCode = 500
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ success: false, message: 'database unavailable' }))
  })

  try {
    await assert.rejects(() => claimSession(config(server.baseUrl)), /database unavailable/)
  } finally {
    await server.close()
  }
})

test('claimPaidWatchSession treats 404 as no watch job', async () => {
  const noJob = await withServer((_req, res) => {
    res.statusCode = 404
    res.end('not found')
  })

  try {
    assert.equal(await claimPaidWatchSession(config(noJob.baseUrl)), null)
  } finally {
    await noJob.close()
  }
})

test('claimSession forwards AbortSignal to the backend request', async () => {
  let requestCount = 0
  const server = await withServer((_req, res) => {
    requestCount += 1
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ success: true, data: { session_id: 'should-not-return' } }))
  })
  const controller = new AbortController()
  controller.abort()

  try {
    await assert.rejects(() => claimSession(config(server.baseUrl), controller.signal), { name: 'AbortError' })
    assert.equal(requestCount, 0)
  } finally {
    await server.close()
  }
})

test('post worker callbacks send expected routes, token, and payloads', async () => {
  const captured: CapturedRequest[] = []
  const server = await withServer(async (req, res) => {
    captured.push({
      method: req.method,
      url: req.url,
      token: req.headers['x-ldxp-worker-token'] as string | undefined,
      body: await readBody(req),
    })
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ success: true, data: {} }))
  })
  const workerConfig = config(server.baseUrl)

  const qrPayload: WorkerQrPayload = {
    worker_order_no: 'order-1',
    worker_amount: 19.9,
    worker_product_name: 'Product',
    qr_code: 'data:image/png;base64,AAAA',
    qr_page_url: 'https://example.test/qr',
  }
  const resultPayload: WorkerResultPayload = {
    worker_order_no: 'order-1',
    worker_amount: 19.9,
    worker_product_name: 'Product',
    worker_card_key: 'abcd1234efgh5678',
    worker_status_text: 'paid',
    worker_success_url: 'https://example.test/success',
  }
  const errorPayload: WorkerErrorPayload = {
    error_code: 'PAYMENT_TIMEOUT',
    error_message: 'payment timed out',
    snapshot_path: '/app/snapshots/sess-1.png',
  }
  const mailPayload: WorkerMailEventPayload = {
    message_id: '<m1@example.test>',
    imap_uid: '42',
    raw_hash: 'hash',
    from: 'seller@example.test',
    to: 'buyer@example.test',
    subject: 'Paid',
    received_time: 1782604800,
    order_no: 'order-1',
    amount: 19.9,
    product_name: 'Product',
    card_key: 'abcd1234efgh5678',
    paid_time: 1782604800,
    body_excerpt: 'excerpt',
  }

  try {
    await postQr(workerConfig, 'sess-1', qrPayload)
    await postResult(workerConfig, 'sess-1', resultPayload)
    await postError(workerConfig, 'sess-1', errorPayload)
    await postMailEvent(workerConfig, mailPayload)

    assert.deepEqual(captured.map((request) => request.url), [
      '/api/ldxp/worker/sessions/sess-1/qr',
      '/api/ldxp/worker/sessions/sess-1/result',
      '/api/ldxp/worker/sessions/sess-1/error',
      '/api/ldxp/worker/mail-events',
    ])
    assert.deepEqual(captured.map((request) => request.token), [
      'worker-token-secret',
      'worker-token-secret',
      'worker-token-secret',
      'worker-token-secret',
    ])
    assert.deepEqual(captured[0]?.body, { worker_id: 'worker-a', ...qrPayload })
    assert.deepEqual(captured[1]?.body, { worker_id: 'worker-a', ...resultPayload })
    assert.deepEqual(captured[2]?.body, { worker_id: 'worker-a', ...errorPayload })
    assert.deepEqual(captured[3]?.body, mailPayload)
    assert.equal(Object.hasOwn(captured[3]?.body as Record<string, unknown>, 'mail_from'), false)
    assert.equal(Object.hasOwn(captured[3]?.body as Record<string, unknown>, 'mail_to'), false)
  } finally {
    await server.close()
  }
})

test('post worker callback throws on successful HTTP response with API failure payload', async () => {
  const server = await withServer((_req, res) => {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ success: false, message: 'backend rejected callback' }))
  })

  try {
    await assert.rejects(
      () =>
        postError(config(server.baseUrl), 'sess-1', {
          error_code: 'FAILED',
          error_message: 'failed',
        }),
      /backend rejected callback/,
    )
  } finally {
    await server.close()
  }
})

test('post worker callback throws on HTTP 404', async () => {
  const server = await withServer((_req, res) => {
    res.statusCode = 404
    res.end('not found')
  })

  try {
    await assert.rejects(
      () =>
        postError(config(server.baseUrl), 'sess-missing', {
          error_code: 'FAILED',
          error_message: 'failed',
        }),
      /HTTP 404/,
    )
  } finally {
    await server.close()
  }
})
