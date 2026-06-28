import { readFileSync } from 'node:fs'

export interface WorkerConfig {
  backendBaseUrl: string
  workerToken: string
  workerId: string
  pollIntervalMs: number
  claimIntervalMs: number
  maxConcurrentSessions: number
  productLoadTimeoutMs: number
  qrTimeoutMs: number
  paymentTimeoutMs: number
  resultTimeoutMs: number
  debugSnapshotDir: string
  imapHost?: string
  imapPort?: number
  imapUser?: string
  imapPassword?: string
}

type Env = Record<string, string | undefined>

const defaultConfig = {
  workerId: 'ldxp-worker-1',
  pollIntervalMs: 5000,
  claimIntervalMs: 5000,
  maxConcurrentSessions: 3,
  productLoadTimeoutMs: 30000,
  qrTimeoutMs: 60000,
  paymentTimeoutMs: 15 * 60 * 1000,
  resultTimeoutMs: 120000,
  debugSnapshotDir: '/app/snapshots',
} as const

export function loadConfigFromEnv(env: Env = process.env): WorkerConfig {
  const backendBaseUrl = requireNonEmpty(env.BACKEND_BASE_URL, 'BACKEND_BASE_URL').replace(/\/+$/, '')
  const workerToken = loadWorkerToken(env)

  return {
    backendBaseUrl,
    workerToken,
    workerId: valueOrDefault(env.LDXP_WORKER_ID, defaultConfig.workerId),
    pollIntervalMs: parseOptionalPositiveInteger(
      env.LDXP_POLL_INTERVAL_MS,
      'LDXP_POLL_INTERVAL_MS',
      defaultConfig.pollIntervalMs,
    ),
    claimIntervalMs: parseOptionalPositiveInteger(
      env.LDXP_CLAIM_INTERVAL_MS,
      'LDXP_CLAIM_INTERVAL_MS',
      defaultConfig.claimIntervalMs,
    ),
    maxConcurrentSessions: parseOptionalPositiveInteger(
      env.LDXP_MAX_CONCURRENT_SESSIONS,
      'LDXP_MAX_CONCURRENT_SESSIONS',
      defaultConfig.maxConcurrentSessions,
    ),
    productLoadTimeoutMs: parseOptionalPositiveInteger(
      env.LDXP_PRODUCT_LOAD_TIMEOUT_MS,
      'LDXP_PRODUCT_LOAD_TIMEOUT_MS',
      defaultConfig.productLoadTimeoutMs,
    ),
    qrTimeoutMs: parseOptionalPositiveInteger(env.LDXP_QR_TIMEOUT_MS, 'LDXP_QR_TIMEOUT_MS', defaultConfig.qrTimeoutMs),
    paymentTimeoutMs: parseOptionalPositiveInteger(
      env.LDXP_PAYMENT_TIMEOUT_MS,
      'LDXP_PAYMENT_TIMEOUT_MS',
      defaultConfig.paymentTimeoutMs,
    ),
    resultTimeoutMs: parseOptionalPositiveInteger(
      env.LDXP_RESULT_TIMEOUT_MS,
      'LDXP_RESULT_TIMEOUT_MS',
      defaultConfig.resultTimeoutMs,
    ),
    debugSnapshotDir: valueOrDefault(env.LDXP_DEBUG_SNAPSHOT_DIR, defaultConfig.debugSnapshotDir),
    imapHost: optionalTrimmed(env.LDXP_IMAP_HOST),
    imapPort: parseOptionalPositiveInteger(env.LDXP_IMAP_PORT, 'LDXP_IMAP_PORT'),
    imapUser: optionalTrimmed(env.LDXP_IMAP_USER),
    imapPassword: loadOptionalFileSecret(env.LDXP_IMAP_PASSWORD, env.LDXP_IMAP_PASSWORD_FILE, 'LDXP_IMAP_PASSWORD_FILE'),
  }
}

function loadWorkerToken(env: Env): string {
  const token = loadOptionalFileSecret(env.LDXP_WORKER_TOKEN, env.LDXP_WORKER_TOKEN_FILE, 'LDXP_WORKER_TOKEN_FILE')
  if (!token) {
    throw new Error('Missing required environment variable LDXP_WORKER_TOKEN or LDXP_WORKER_TOKEN_FILE')
  }
  return token
}

function loadOptionalFileSecret(value: string | undefined, filePath: string | undefined, fileVariableName: string): string | undefined {
  const trimmedFilePath = optionalTrimmed(filePath)
  if (trimmedFilePath) {
    try {
      const fileValue = readFileSync(trimmedFilePath, 'utf8').trim()
      if (!fileValue) {
        throw new Error('empty file secret')
      }
      return fileValue
    } catch (error) {
      const detail = error instanceof Error ? error.message : 'unknown error'
      throw new Error(`Unable to read secret from ${fileVariableName}: ${detail}`)
    }
  }

  return optionalTrimmed(value)
}

function requireNonEmpty(value: string | undefined, variableName: string): string {
  const trimmed = optionalTrimmed(value)
  if (!trimmed) {
    throw new Error(`Missing required environment variable ${variableName}`)
  }
  return trimmed
}

function valueOrDefault(value: string | undefined, fallback: string): string {
  return optionalTrimmed(value) ?? fallback
}

function optionalTrimmed(value: string | undefined): string | undefined {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}

function parseOptionalPositiveInteger(value: string | undefined, variableName: string, fallback?: number): number
function parseOptionalPositiveInteger(value: string | undefined, variableName: string, fallback: number | undefined = undefined): number | undefined {
  const trimmed = optionalTrimmed(value)
  if (!trimmed) {
    return fallback
  }

  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${variableName} must be a positive integer`)
  }
  return parsed
}
