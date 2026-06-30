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
  releaseSessionSlotAfterQr: boolean
  debugSnapshotDir: string
  imapHost?: string
  imapPort?: number
  imapUser?: string
  imapPassword?: string
  mockMode: boolean
  mockCardKey?: string
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
  releaseSessionSlotAfterQr: false,
  debugSnapshotDir: '/app/snapshots',
} as const

export function loadConfigFromEnv(env: Env = process.env): WorkerConfig {
  const backendBaseUrl = requireNonEmpty(env.BACKEND_BASE_URL, 'BACKEND_BASE_URL').replace(/\/+$/, '')
  const workerToken = loadWorkerToken(env)
  const mockMode = parseOptionalBoolean(env.LDXP_WORKER_MOCK_MODE, 'LDXP_WORKER_MOCK_MODE', false)
  const mockCardKey = optionalTrimmed(env.LDXP_WORKER_MOCK_CARD_KEY)
  if (mockMode && !mockCardKey) {
    throw new Error('Missing required environment variable LDXP_WORKER_MOCK_CARD_KEY when LDXP_WORKER_MOCK_MODE is true')
  }
  const pollIntervalMs = parseOptionalPositiveInteger(
    firstDefined(env.LDXP_WORKER_POLL_INTERVAL_MS, env.LDXP_POLL_INTERVAL_MS),
    'LDXP_WORKER_POLL_INTERVAL_MS',
    defaultConfig.pollIntervalMs,
  )

  return {
    backendBaseUrl,
    workerToken,
    workerId: valueOrDefault(env.LDXP_WORKER_ID, defaultConfig.workerId),
    pollIntervalMs,
    claimIntervalMs: parseOptionalPositiveInteger(
      env.LDXP_CLAIM_INTERVAL_MS,
      'LDXP_CLAIM_INTERVAL_MS',
      pollIntervalMs,
    ),
    maxConcurrentSessions: parseOptionalPositiveInteger(
      firstDefined(env.LDXP_WORKER_CONCURRENCY, env.LDXP_MAX_CONCURRENT_SESSIONS),
      'LDXP_WORKER_CONCURRENCY',
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
    releaseSessionSlotAfterQr: parseOptionalBoolean(
      firstDefined(env.LDXP_RELEASE_SLOT_AFTER_QR, env.LDXP_WORKER_RELEASE_SLOT_AFTER_QR),
      'LDXP_RELEASE_SLOT_AFTER_QR',
      defaultConfig.releaseSessionSlotAfterQr,
    ),
    debugSnapshotDir: valueOrDefault(firstDefined(env.LDXP_BROWSER_SNAPSHOT_DIR, env.LDXP_DEBUG_SNAPSHOT_DIR), defaultConfig.debugSnapshotDir),
    imapHost: optionalTrimmed(firstDefined(env.LDXP_IMAP_HOST, env.QQ_IMAP_HOST)),
    imapPort: parseOptionalPositiveInteger(firstDefined(env.LDXP_IMAP_PORT, env.QQ_IMAP_PORT), 'LDXP_IMAP_PORT'),
    imapUser: optionalTrimmed(firstDefined(env.LDXP_IMAP_USER, env.QQ_IMAP_USER)),
    imapPassword: loadOptionalFileSecret(
      firstDefined(env.LDXP_IMAP_PASSWORD, env.QQ_IMAP_PASSWORD),
      firstDefined(env.LDXP_IMAP_PASSWORD_FILE, env.QQ_IMAP_PASSWORD_FILE),
      'LDXP_IMAP_PASSWORD_FILE or QQ_IMAP_PASSWORD_FILE',
    ),
    mockMode,
    mockCardKey,
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

function firstDefined(...values: Array<string | undefined>): string | undefined {
  for (const value of values) {
    const trimmed = optionalTrimmed(value)
    if (trimmed) {
      return trimmed
    }
  }
  return undefined
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

function parseOptionalBoolean(value: string | undefined, variableName: string, fallback: boolean): boolean {
  const trimmed = optionalTrimmed(value)
  if (!trimmed) {
    return fallback
  }

  switch (trimmed.toLowerCase()) {
    case 'true':
    case '1':
    case 'yes':
    case 'on':
      return true
    case 'false':
    case '0':
    case 'no':
    case 'off':
      return false
    default:
      throw new Error(`${variableName} must be a boolean`)
  }
}
