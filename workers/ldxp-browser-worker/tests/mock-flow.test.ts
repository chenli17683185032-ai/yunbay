import test from 'node:test'
import assert from 'node:assert/strict'
import { buildMockFlowArtifacts } from '../src/mock-flow.js'
import type { WorkerConfig } from '../src/config.js'
import type { ClaimedSession } from '../src/backend.js'

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
    debugSnapshotDir: '/app/snapshots',
    mockMode: true,
    mockCardKey: 'mock-card-key-1234',
    ...overrides,
  }
}

function session(overrides: Partial<ClaimedSession> = {}): ClaimedSession {
  return {
    session_id: 'sess-mock-1',
    amount: 100,
    money: 10.5,
    product_url: 'https://example.test/product',
    product_name: 'Mock Product',
    contact_email: 'buyer@example.test',
    ...overrides,
  }
}

test('buildMockFlowArtifacts is deterministic and keeps qr, result, and mail payloads aligned', () => {
  const first = buildMockFlowArtifacts(session(), config())
  const second = buildMockFlowArtifacts(session(), config())

  assert.deepEqual(first, second)
  assert.match(first.qr.worker_order_no, /^LDMOCK[A-Z0-9]{8,58}$/)
  assert.ok(first.qr.qr_code.startsWith('data:image/png;base64,'))
  assert.equal(first.qr.worker_order_no, first.result.worker_order_no)
  assert.equal(first.qr.worker_order_no, first.mailEvent.order_no)
  assert.equal(first.qr.worker_amount, 10.5)
  assert.equal(first.result.worker_amount, 10.5)
  assert.equal(first.mailEvent.amount, 10.5)
  assert.equal(first.qr.worker_product_name, 'Mock Product')
  assert.equal(first.result.worker_product_name, 'Mock Product')
  assert.equal(first.mailEvent.product_name, 'Mock Product')
  assert.equal(first.result.worker_card_key, 'mock-card-key-1234')
  assert.equal(first.mailEvent.card_key, 'mock-card-key-1234')
  assert.match(first.mailEvent.message_id ?? '', /^<ldxp-mock-[a-z0-9]+@example\.test>$/)
  assert.match(first.mailEvent.imap_uid ?? '', /^\d+$/)
  assert.match(first.mailEvent.raw_hash ?? '', /^[a-f0-9]{64}$/)
  assert.equal(first.mailEvent.received_time, first.mailEvent.paid_time)
  assert.doesNotMatch(first.mailEvent.body_excerpt ?? '', /mock-card-key-1234/)
})

test('buildMockFlowArtifacts uses session money instead of cloud quota amount', () => {
  const artifacts = buildMockFlowArtifacts(session({ amount: 1000, money: 0.1 }), config())

  assert.equal(artifacts.qr.worker_amount, 0.1)
  assert.equal(artifacts.result.worker_amount, 0.1)
  assert.equal(artifacts.mailEvent.amount, 0.1)
})
