import { fileURLToPath } from 'node:url'
import { setTimeout as sleep } from 'node:timers/promises'
import pino, { type Logger } from 'pino'
import { loadConfigFromEnv, type WorkerConfig } from './config.js'
import { claimSession, postError, postQr, postResult, type ClaimedSession, type WorkerErrorPayload } from './backend.js'
import { runBrowserFlow, type BrowserPaidResult, type BrowserQrResult } from './browser-flow.js'
import { runMailPoller } from './mail-poller.js'
import { redactValue } from './redact.js'

export interface WorkerRuntimeDependencies {
  claimSession: typeof claimSession
  runBrowserFlow: typeof runBrowserFlow
  postQr: typeof postQr
  postResult: typeof postResult
  postError: typeof postError
  runMailPoller: typeof runMailPoller
  logger: LoggerLike
}

export interface WorkerRuntime {
  config: WorkerConfig
  logger: LoggerLike
  signal: AbortSignal
  shutdown: () => Promise<void>
  startMailPoller: () => Promise<void>
  dependencies: WorkerRuntimeDependencies
  activeSessions: Set<Promise<void>>
}

interface LoggerLike {
  info: (obj: unknown, msg?: string) => void
  warn: (obj: unknown, msg?: string) => void
  error: (obj: unknown, msg?: string) => void
  debug: (obj: unknown, msg?: string) => void
  child: (bindings: Record<string, unknown>) => LoggerLike
}

const defaultDependencies: Omit<WorkerRuntimeDependencies, 'logger'> = {
  claimSession,
  runBrowserFlow,
  postQr,
  postResult,
  postError,
  runMailPoller,
}

export function createWorkerRuntime(
  config: WorkerConfig = loadConfigFromEnv(),
  dependencies: Partial<WorkerRuntimeDependencies> = {},
): WorkerRuntime {
  const baseLogger = dependencies.logger ?? createLogger(config)
  const logger = baseLogger.child({
    workerId: redactValue(config.workerId),
    backendBaseUrl: config.backendBaseUrl,
  })

  const mergedDependencies: WorkerRuntimeDependencies = {
    claimSession: dependencies.claimSession ?? defaultDependencies.claimSession,
    runBrowserFlow: dependencies.runBrowserFlow ?? defaultDependencies.runBrowserFlow,
    postQr: dependencies.postQr ?? defaultDependencies.postQr,
    postResult: dependencies.postResult ?? defaultDependencies.postResult,
    postError: dependencies.postError ?? defaultDependencies.postError,
    runMailPoller: dependencies.runMailPoller ?? defaultDependencies.runMailPoller,
    logger,
  }

  const controller = new AbortController()
  const activeSessions = new Set<Promise<void>>()
  let mailPollerStarted = false

  return {
    config,
    logger,
    signal: controller.signal,
    activeSessions,
    dependencies: mergedDependencies,
    async shutdown() {
      controller.abort()
      if (mailPollerStarted) {
        logger.info({ activeSessions: activeSessions.size }, 'shutdown requested')
      }
      await Promise.allSettled([...activeSessions])
    },
    async startMailPoller() {
      if (mailPollerStarted) {
        return
      }
      mailPollerStarted = true
      if (!hasMailConfig(config)) {
        logger.warn({ imapConfigured: false }, 'mail poller disabled because IMAP config is incomplete')
        return
      }
      void mergedDependencies.runMailPoller(config, controller.signal).catch((error) => {
        logger.error({ err: sanitizeError(error) }, 'mail poller exited unexpectedly')
      })
    },
  }
}

export async function runClaimLoopOnce(runtime: WorkerRuntime): Promise<number> {
  let started = 0
  while (!runtime.signal.aborted && runtime.activeSessions.size < runtime.config.maxConcurrentSessions) {
    let session: ClaimedSession | null
    try {
      session = await runtime.dependencies.claimSession(runtime.config)
    } catch (error) {
      runtime.logger.warn({ err: sanitizeError(error) }, 'claim session request failed')
      break
    }
    if (!session) {
      break
    }
    started += 1
    trackSession(runtime, processClaimedSession(runtime, session))
  }
  return started
}

export async function runClaimLoop(runtime: WorkerRuntime): Promise<void> {
  await runtime.startMailPoller()

  while (!runtime.signal.aborted) {
    try {
      await runClaimLoopOnce(runtime)
    } catch (error) {
      runtime.logger.error({ err: sanitizeError(error) }, 'claim loop iteration failed')
    }

    try {
      await sleep(runtime.config.pollIntervalMs, undefined, { signal: runtime.signal })
    } catch (error) {
      if (runtime.signal.aborted || isAbortError(error)) {
        break
      }
      throw error
    }
  }

  await runtime.shutdown()
}

export async function processClaimedSession(runtime: WorkerRuntime, session: ClaimedSession): Promise<void> {
  const sessionLogger = runtime.logger.child({
    sessionId: redactValue(session.session_id),
    amount: session.amount,
    money: session.money,
    productUrl: redactValue(session.product_url),
  })

  sessionLogger.info({ sessionId: redactValue(session.session_id) }, 'processing claimed session')

  try {
    const result = await runtime.dependencies.runBrowserFlow(
      {
        sessionId: session.session_id,
        productUrl: session.product_url,
        contactEmail: session.contact_email,
        expectedAmount: session.money,
        expectedProductName: session.product_name,
      },
      {
        onQr: async (qr) => {
          await runtime.dependencies.postQr(runtime.config, session.session_id, qr)
          sessionLogger.info({ workerOrderNo: redactValue(qr.worker_order_no) }, 'qr posted')
        },
      },
      runtime.config,
    )

    await runtime.dependencies.postResult(runtime.config, session.session_id, result)
    sessionLogger.info({ workerOrderNo: redactValue(result.worker_order_no) }, 'result posted')
  } catch (error) {
    const payload = buildErrorPayload(error, runtime.config)
    await runtime.dependencies.postError(runtime.config, session.session_id, payload)
    sessionLogger.error({ err: sanitizeError(error), errorCode: payload.error_code }, 'session processing failed')
  }
}

export function buildErrorPayload(error: unknown, config: WorkerConfig): WorkerErrorPayload {
  const message = sanitizeError(error)
  const snapshotPath = extractSnapshotPath(message, config.debugSnapshotDir)
  return {
    error_code: 'worker_flow_failed',
    error_message: truncateMessage(message, 500),
    ...(snapshotPath ? { snapshot_path: snapshotPath } : {}),
  }
}

function createLogger(config: WorkerConfig): LoggerLike {
  return pino({
    level: process.env.LOG_LEVEL ?? 'info',
    redact: {
      paths: [
        'workerToken',
        'worker_token',
        'imapPassword',
        'imap_password',
        'qr_code',
        'card_key',
        'worker_card_key',
      ],
      remove: true,
    },
  }).child({
    backendBaseUrl: config.backendBaseUrl,
    workerId: redactValue(config.workerId),
  })
}

function trackSession(runtime: WorkerRuntime, promise: Promise<void>): void {
  runtime.activeSessions.add(promise)
  promise
    .catch((error) => {
      runtime.logger.error({ err: sanitizeError(error) }, 'unhandled session error')
    })
    .finally(() => {
      runtime.activeSessions.delete(promise)
    })
}

function sanitizeError(error: unknown): string {
  const raw = error instanceof Error ? `${error.name}: ${error.message}` : String(error)
  return raw
    .replace(/data:image\/[A-Za-z0-9.+-]+;base64,[A-Za-z0-9+/=]+/g, '[redacted]')
    .replace(/\b(worker_token|token|password|secret|authorization|card_key|qr_code)\b(?:\s*[:=]\s*|\s+)[^"',\s}]+/gi, '[redacted]')
}

function truncateMessage(message: string, maxLength: number): string {
  if (message.length <= maxLength) {
    return message
  }
  return `${message.slice(0, maxLength - 1)}…`
}

function extractSnapshotPath(message: string, fallbackDir: string): string | undefined {
  const match = message.match(/snapshot=([^:]+?)(?::\s*|$)/i)
  if (match?.[1]) {
    return match[1].trim()
  }
  if (message.includes(fallbackDir)) {
    return fallbackDir
  }
  return undefined
}

function hasMailConfig(config: WorkerConfig): boolean {
  return Boolean(config.imapHost && config.imapUser && config.imapPassword)
}

function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

async function main(): Promise<void> {
  const runtime = createWorkerRuntime()
  runtime.logger.info({ pollIntervalMs: runtime.config.pollIntervalMs, concurrency: runtime.config.maxConcurrentSessions }, 'worker starting')

  const handleSignal = (signal: NodeJS.Signals) => {
    runtime.logger.warn({ signal }, 'shutdown signal received')
    void runtime.shutdown()
  }

  process.once('SIGINT', handleSignal)
  process.once('SIGTERM', handleSignal)

  try {
    await runClaimLoop(runtime)
  } finally {
    process.off('SIGINT', handleSignal)
    process.off('SIGTERM', handleSignal)
  }
}

const isMainModule = fileURLToPath(import.meta.url) === process.argv[1]
if (isMainModule) {
  void main().catch((error) => {
    const logger = pino()
    logger.error({ err: sanitizeError(error) }, 'worker exited with fatal error')
    process.exitCode = 1
  })
}
