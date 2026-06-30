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

test('loadConfigFromEnv defaults mock mode to false without requiring a mock card key', () => {
  const config = loadConfigFromEnv({
    BACKEND_BASE_URL: 'https://backend.example',
    LDXP_WORKER_TOKEN: 'token',
  })

  assert.equal(config.mockMode, false)
  assert.equal(config.mockCardKey, undefined)
})

test('loadConfigFromEnv enables mock mode with a trimmed mock card key', () => {
  const config = loadConfigFromEnv({
    BACKEND_BASE_URL: 'https://backend.example',
    LDXP_WORKER_TOKEN: 'token',
    LDXP_WORKER_MOCK_MODE: '1',
    LDXP_WORKER_MOCK_CARD_KEY: ' mock-card-key-1234 ',
  })

  assert.equal(config.mockMode, true)
  assert.equal(config.mockCardKey, 'mock-card-key-1234')
})

test('loadConfigFromEnv requires mock card key when mock mode is enabled without exposing secret contents', () => {
  assert.throws(
    () =>
      loadConfigFromEnv({
        BACKEND_BASE_URL: 'https://backend.example',
        LDXP_WORKER_TOKEN: 'token',
        LDXP_WORKER_MOCK_MODE: 'true',
      }),
    (error) => {
      assert.match(String(error), /LDXP_WORKER_MOCK_CARD_KEY/)
      assert.doesNotMatch(String(error), /mock-card-key-1234/)
      return true
    },
  )
})

test('loadConfigFromEnv rejects invalid mock mode boolean values', () => {
  assert.throws(
    () =>
      loadConfigFromEnv({
        BACKEND_BASE_URL: 'https://backend.example',
        LDXP_WORKER_TOKEN: 'token',
        LDXP_WORKER_MOCK_MODE: 'maybe',
        LDXP_WORKER_MOCK_CARD_KEY: 'mock-card-key-1234',
      }),
    /LDXP_WORKER_MOCK_MODE/,
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
    LDXP_WORKER_POLL_INTERVAL_MS: '2500',
    LDXP_WORKER_CONCURRENCY: '5',
    LDXP_PRODUCT_LOAD_TIMEOUT_MS: '45000',
    LDXP_QR_TIMEOUT_MS: '65000',
    LDXP_PAYMENT_TIMEOUT_MS: '900000',
    LDXP_RESULT_TIMEOUT_MS: '150000',
    LDXP_BROWSER_SNAPSHOT_DIR: '/tmp/snapshots',
    QQ_IMAP_HOST: 'imap.qq.com',
    QQ_IMAP_PORT: '993',
    QQ_IMAP_USER: '10256345@qq.com',
    QQ_IMAP_PASSWORD_FILE: tokenFile,
  })

  assert.equal(config.backendBaseUrl, 'https://backend.example/api')
  assert.equal(config.workerToken, 'file-secret-token')
  assert.equal(config.workerId, 'worker-a')
  assert.equal(config.pollIntervalMs, 2500)
  assert.equal(config.claimIntervalMs, 2500)
  assert.equal(config.maxConcurrentSessions, 5)
  assert.equal(config.productLoadTimeoutMs, 45000)
  assert.equal(config.qrTimeoutMs, 65000)
  assert.equal(config.paymentTimeoutMs, 900000)
  assert.equal(config.resultTimeoutMs, 150000)
  assert.equal(config.debugSnapshotDir, '/tmp/snapshots')
  assert.equal(config.imapHost, 'imap.qq.com')
  assert.equal(config.imapPort, 993)
  assert.equal(config.imapUser, '10256345@qq.com')
  assert.equal(config.imapPassword, 'file-secret-token')
})

test('loadConfigFromEnv rejects invalid numeric values', () => {
  assert.throws(
    () =>
      loadConfigFromEnv({
        BACKEND_BASE_URL: 'https://backend.example',
        LDXP_WORKER_TOKEN: 'token',
        LDXP_WORKER_CONCURRENCY: 'not-a-number',
      }),
    /LDXP_WORKER_CONCURRENCY/,
  )
})

test('loadConfigFromEnv reports QQ IMAP password file alias when unreadable', () => {
  assert.throws(
    () =>
      loadConfigFromEnv({
        BACKEND_BASE_URL: 'https://backend.example',
        LDXP_WORKER_TOKEN: 'token',
        QQ_IMAP_PASSWORD_FILE: '/tmp/ldxp-missing-secret-file',
      }),
    /LDXP_IMAP_PASSWORD_FILE or QQ_IMAP_PASSWORD_FILE/,
  )
})

test('loadConfigFromEnv supports legacy env aliases', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'ldxp-worker-config-legacy-'))
  const workerTokenFile = join(dir, 'worker-token')
  const imapPasswordFile = join(dir, 'imap-password')
  await writeFile(workerTokenFile, 'worker-token-secret\n')
  await writeFile(imapPasswordFile, 'imap-password-secret\n')

  const config = loadConfigFromEnv({
    BACKEND_BASE_URL: 'https://backend.example',
    LDXP_WORKER_TOKEN_FILE: workerTokenFile,
    LDXP_POLL_INTERVAL_MS: '3000',
    LDXP_MAX_CONCURRENT_SESSIONS: '4',
    LDXP_DEBUG_SNAPSHOT_DIR: '/legacy/snapshots',
    LDXP_IMAP_HOST: 'imap.example',
    LDXP_IMAP_PORT: '995',
    LDXP_IMAP_USER: 'legacy@example.test',
    LDXP_IMAP_PASSWORD_FILE: imapPasswordFile,
  })

  assert.equal(config.pollIntervalMs, 3000)
  assert.equal(config.maxConcurrentSessions, 4)
  assert.equal(config.debugSnapshotDir, '/legacy/snapshots')
  assert.equal(config.imapHost, 'imap.example')
  assert.equal(config.imapPort, 995)
  assert.equal(config.imapUser, 'legacy@example.test')
  assert.equal(config.imapPassword, 'imap-password-secret')
})
