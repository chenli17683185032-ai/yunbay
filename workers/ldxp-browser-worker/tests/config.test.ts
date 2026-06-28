import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { loadConfigFromEnv } from '../src/config.js'

test('loadConfigFromEnv requires backend base url and token', () => {
  assert.throws(() => loadConfigFromEnv({}), /BACKEND_BASE_URL/)
})

test('loadConfigFromEnv requires worker token without exposing secret contents', () => {
  assert.throws(
    () => loadConfigFromEnv({ BACKEND_BASE_URL: 'https://backend.example' }),
    (error) => {
      assert.match(String(error), /LDXP_WORKER_TOKEN/)
      assert.doesNotMatch(String(error), /secret/)
      return true
    },
  )
})

test('loadConfigFromEnv reads token file, trims whitespace, and parses numeric values', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'ldxp-worker-config-'))
  const tokenFile = join(dir, 'token')
  await writeFile(tokenFile, 'file-secret-token\n')

  const config = loadConfigFromEnv({
    BACKEND_BASE_URL: 'https://backend.example/api/',
    LDXP_WORKER_TOKEN_FILE: tokenFile,
    LDXP_WORKER_ID: 'worker-a',
    LDXP_POLL_INTERVAL_MS: '2500',
    LDXP_CLAIM_INTERVAL_MS: '3000',
    LDXP_MAX_CONCURRENT_SESSIONS: '5',
    LDXP_PRODUCT_LOAD_TIMEOUT_MS: '45000',
    LDXP_QR_TIMEOUT_MS: '65000',
    LDXP_PAYMENT_TIMEOUT_MS: '900000',
    LDXP_RESULT_TIMEOUT_MS: '150000',
    LDXP_DEBUG_SNAPSHOT_DIR: '/tmp/snapshots',
  })

  assert.equal(config.backendBaseUrl, 'https://backend.example/api')
  assert.equal(config.workerToken, 'file-secret-token')
  assert.equal(config.workerId, 'worker-a')
  assert.equal(config.pollIntervalMs, 2500)
  assert.equal(config.claimIntervalMs, 3000)
  assert.equal(config.maxConcurrentSessions, 5)
  assert.equal(config.productLoadTimeoutMs, 45000)
  assert.equal(config.qrTimeoutMs, 65000)
  assert.equal(config.paymentTimeoutMs, 900000)
  assert.equal(config.resultTimeoutMs, 150000)
  assert.equal(config.debugSnapshotDir, '/tmp/snapshots')
})

test('loadConfigFromEnv rejects invalid numeric values', () => {
  assert.throws(
    () =>
      loadConfigFromEnv({
        BACKEND_BASE_URL: 'https://backend.example',
        LDXP_WORKER_TOKEN: 'token',
        LDXP_MAX_CONCURRENT_SESSIONS: 'not-a-number',
      }),
    /LDXP_MAX_CONCURRENT_SESSIONS/,
  )
})
