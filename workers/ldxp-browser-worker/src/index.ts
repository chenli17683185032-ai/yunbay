import { basename } from 'node:path'
import { fileURLToPath } from 'node:url'
import { setTimeout as sleep } from 'node:timers/promises'
import pino, { type Logger } from 'pino'
import { loadConfigFromEnv, type WorkerConfig } from './config.js'
import {
  claimSession,
  claimPaidWatchSession,
  isSessionActive,
  postError,
  postMailEvent,
  postQr,
  postResult,
  type ClaimedSession,
  type PaidWatchSession,
  type WorkerErrorPayload,
} from './backend.js'
import { buildPaidResultFromText, runBrowserFlow, type BrowserFlowDiagnostics, type BrowserFlowResult, type BrowserPaidResult, type BrowserQrResult } from './browser-flow.js'
import { createReusableBrowserManager, type BrowserManager } from './browser-manager.js'
import { runMailPoller } from './mail-poller.js'
import { buildMockFlowArtifacts } from './mock-flow.js'
import { redactValue } from './redact.js'

export interface WorkerRuntimeDependencies {
  claimSession: typeof claimSession
  claimPaidWatchSession: typeof claimPaidWatchSession
  isSessionActive: typeof isSessionActive
  runBrowserFlow: typeof runBrowserFlow
  postQr: typeof postQr
  postResult: typeof postResult
  postMailEvent: typeof postMailEvent
  postError: typeof postError
  runMailPoller: typeof runMailPoller
  runMockFlow: typeof buildMockFlowArtifacts
  logger: LoggerLike
}

export interface WorkerRuntime {
  config: WorkerConfig
  logger: LoggerLike
  signal: AbortSignal
  shutdown: () => Promise<void>
  startMailPoller: () => Promise<void>
  dependencies: WorkerRuntimeDependencies
  browserManager?: BrowserManager
  activeSessions: Set<Promise<void>>
  activeClaims: Set<Promise<void>>
  activePaidWatches: Set<Promise<void>>
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
  claimPaidWatchSession,
  isSessionActive,
  runBrowserFlow,
  postQr,
  postResult,
  postMailEvent,
  postError,
  runMailPoller,
  runMockFlow: buildMockFlowArtifacts,
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
    claimPaidWatchSession: dependencies.claimPaidWatchSession ?? defaultDependencies.claimPaidWatchSession,
    isSessionActive: dependencies.isSessionActive ?? defaultDependencies.isSessionActive,
    runBrowserFlow: dependencies.runBrowserFlow ?? defaultDependencies.runBrowserFlow,
    postQr: dependencies.postQr ?? defaultDependencies.postQr,
    postResult: dependencies.postResult ?? defaultDependencies.postResult,
    postMailEvent: dependencies.postMailEvent ?? defaultDependencies.postMailEvent,
    postError: dependencies.postError ?? defaultDependencies.postError,
    runMailPoller: dependencies.runMailPoller ?? defaultDependencies.runMailPoller,
    runMockFlow: dependencies.runMockFlow ?? defaultDependencies.runMockFlow,
    logger,
  }

  const controller = new AbortController()
  const activeSessions = new Set<Promise<void>>()
  const activeClaims = new Set<Promise<void>>()
  const activePaidWatches = new Set<Promise<void>>()
  const browserManager = config.mockMode ? undefined : createReusableBrowserManager()
  let mailPollerStarted = false
  let mailPollerTask: Promise<void> | undefined
  let shutdownPromise: Promise<void> | undefined

  return {
    config,
    logger,
    signal: controller.signal,
    activeSessions,
    activeClaims,
    activePaidWatches,
    dependencies: mergedDependencies,
    browserManager,
    async shutdown() {
      if (!shutdownPromise) {
        controller.abort()
        if (mailPollerStarted) {
          logger.info({ activeSessions: activeSessions.size }, 'shutdown requested')
        }
        const tasks = [...activeSessions, ...activeClaims, ...activePaidWatches]
        if (mailPollerTask) {
          tasks.push(mailPollerTask)
        }
        shutdownPromise = Promise.allSettled(tasks).then(async () => {
          await browserManager?.close()
        })
      }
      await shutdownPromise
    },
    async startMailPoller() {
      if (mailPollerStarted || controller.signal.aborted) {
        return
      }
      mailPollerStarted = true
      if (config.mockMode) {
        logger.info({ mockMode: true }, 'mail poller disabled in mock mode')
        return
      }
      if (!hasMailConfig(config)) {
        logger.warn({ imapConfigured: false }, 'mail poller disabled because IMAP config is incomplete')
        return
      }
      mailPollerTask = superviseMailPoller(config, controller.signal, mergedDependencies.runMailPoller, logger)
    },
  }
}

function prewarmBrowserIfEnabled(runtime: WorkerRuntime): void {
  if (!runtime.config.browserPrewarm || runtime.config.mockMode || !runtime.browserManager) {
    return
  }
  const marker = runtime as WorkerRuntime & { browserPrewarmStarted?: boolean }
  if (marker.browserPrewarmStarted) {
    return
  }
  marker.browserPrewarmStarted = true
  void runtime.browserManager.restart().catch((error) => {
    runtime.logger.warn({ err: sanitizeError(error) }, 'browser prewarm failed')
  })
}

export async function runClaimLoopOnce(runtime: WorkerRuntime): Promise<number> {
  prewarmBrowserIfEnabled(runtime)
  let started = 0
  while (!runtime.signal.aborted && runtime.activeSessions.size < runtime.config.maxConcurrentSessions) {
    let session: ClaimedSession | null
    try {
      session = await claimSessionWithTracking(runtime)
    } catch (error) {
      if (runtime.signal.aborted || isAbortError(error)) {
        break
      }
      runtime.logger.warn({ err: sanitizeError(error) }, 'claim session request failed')
      break
    }
    if (runtime.signal.aborted || !session) {
      break
    }
    started += 1
    trackSession(runtime, processClaimedSession(runtime, session))
  }
  return started
}

async function claimSessionWithTracking(runtime: WorkerRuntime): Promise<ClaimedSession | null> {
  const claim = runtime.dependencies.claimSession(runtime.config, runtime.signal)
  const tracked = claim.then(
    () => undefined,
    () => undefined,
  )
  runtime.activeClaims.add(tracked)
  try {
    return await claim
  } finally {
    runtime.activeClaims.delete(tracked)
  }
}

export async function runPaidWatchLoopOnce(runtime: WorkerRuntime): Promise<number> {
  if (!runtime.config.releaseSessionSlotAfterQr || runtime.config.mockMode) {
    return 0
  }
  if (runtime.activePaidWatches.size >= runtime.config.maxConcurrentSessions) {
    return 0
  }

  let session: PaidWatchSession | null
  try {
    session = await runtime.dependencies.claimPaidWatchSession(runtime.config, runtime.signal)
  } catch (error) {
    if (runtime.signal.aborted || isAbortError(error)) {
      return 0
    }
    runtime.logger.warn({ err: sanitizeError(error) }, 'claim paid-watch request failed')
    return 0
  }
  if (!session || runtime.signal.aborted) {
    return 0
  }

  trackPaidWatch(runtime, processPaidWatchSession(runtime, session))
  return 1
}

async function processPaidWatchSession(runtime: WorkerRuntime, session: PaidWatchSession): Promise<void> {
  const sessionLogger = runtime.logger.child({
    sessionId: redactValue(session.session_id),
    workerOrderNo: redactValue(session.worker_order_no),
    qrPageUrl: redactValue(session.qr_page_url),
  })
  sessionLogger.info({}, 'watching qr session for paid result')

  try {
    const result = await waitForPaidWatchResult(runtime, session)
    if (!result) {
      sessionLogger.debug({}, 'paid-watch found no paid result yet')
      return
    }
    await runtime.dependencies.postResult(runtime.config, session.session_id, result)
    sessionLogger.info({ workerOrderNo: redactValue(result.worker_order_no) }, 'paid-watch result posted')
  } catch (error) {
    if (runtime.signal.aborted || isAbortError(error)) {
      sessionLogger.warn({ err: sanitizeError(error) }, 'paid-watch aborted during shutdown')
      return
    }
    sessionLogger.warn({ err: sanitizeError(error) }, 'paid-watch iteration failed without marking session failed')
  }
}

async function waitForPaidWatchResult(runtime: WorkerRuntime, session: PaidWatchSession): Promise<BrowserPaidResult | null> {
  if (!runtime.browserManager) {
    return null
  }
  const context = await runtime.browserManager.getContext()
  try {
    const page = await context.newPage()
    const timeout = Math.min(runtime.config.paymentTimeoutMs, runtime.config.paidWatchPollIntervalMs)
    page.setDefaultTimeout(Math.max(runtime.config.resultTimeoutMs, timeout))
    await page.goto(session.qr_page_url, { waitUntil: 'domcontentloaded', timeout: runtime.config.productLoadTimeoutMs })
    try {
      await page.waitForFunction(
        () => {
          const text = document.body?.innerText ?? ''
          if (['未付款', '等待支付', '支付超时', '超时', '已取消', '取消订单', '付款失败'].some((marker) => text.includes(marker))) {
            return false
          }
          return ['已付款', '支付成功', '付款成功', '交易成功', '购买成功'].some((marker) => text.includes(marker))
        },
        undefined,
        { timeout },
      )
    } catch {
      return null
    }
    const text = await page.locator('body').innerText({ timeout: runtime.config.resultTimeoutMs })
    return buildPaidResultFromText({
      resultText: text,
      fallbackOrderNo: session.worker_order_no,
      expectedAmount: session.money,
      workerProductName: session.worker_product_name,
      successUrl: page.url(),
    })
  } finally {
    await context.close().catch(() => undefined)
  }
}

export async function runClaimLoop(runtime: WorkerRuntime): Promise<void> {
  await runtime.startMailPoller()

  while (!runtime.signal.aborted) {
    await runPaidWatchLoopOnce(runtime)
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
  const diagnostics: BrowserFlowDiagnostics = { timings: [] }

  try {
    if (runtime.config.mockMode) {
      await processMockClaimedSession(runtime, session, sessionLogger)
      return
    }

    const sessionSignal = await createSessionAbortSignal(runtime, session.session_id, sessionLogger)
    const result = await (async () => {
      try {
        return await runtime.dependencies.runBrowserFlow(
          {
            sessionId: session.session_id,
            productUrl: session.product_url,
            contactEmail: session.contact_email,
            expectedAmount: session.money,
            expectedProductName: session.product_name,
          },
          {
            onQr: async (qr) => {
              if (!(await isSessionStillActive(runtime, session.session_id, sessionLogger))) {
                sessionSignal.abort()
                throw abortError('ldxp session canceled before qr callback')
              }
              await runtime.dependencies.postQr(runtime.config, session.session_id, qr)
              sessionLogger.info({ workerOrderNo: redactValue(qr.worker_order_no), timings: diagnostics.timings }, 'qr posted')
            },
          },
          runtime.config,
          sessionSignal.signal,
          diagnostics,
          runtime.browserManager,
        )
      } finally {
        sessionSignal.abort()
      }
    })()

    if (isQrPostedResult(result)) {
      sessionLogger.info({ workerOrderNo: redactValue(result.worker_order_no), timings: diagnostics.timings }, 'qr posted; session slot released')
      return
    }

    await runtime.dependencies.postResult(runtime.config, session.session_id, result)
    sessionLogger.info({ workerOrderNo: redactValue(result.worker_order_no), timings: diagnostics.timings }, 'result posted')
  } catch (error) {
    if (runtime.signal.aborted || isAbortError(error)) {
      sessionLogger.warn({ err: sanitizeError(error) }, 'session processing aborted during shutdown')
      return
    }

    const payload = buildErrorPayload(error, runtime.config)
    try {
      await runtime.dependencies.postError(runtime.config, session.session_id, payload)
    } catch (postErrorFailure) {
      sessionLogger.error(
        { err: sanitizeError(postErrorFailure), originalErrorCode: payload.error_code },
        'failed to post session error',
      )
      return
    }
    sessionLogger.error({ err: sanitizeError(error), errorCode: payload.error_code, timings: diagnostics.timings }, 'session processing failed')
  }
}

async function createSessionAbortSignal(runtime: WorkerRuntime, sessionId: string, logger: LoggerLike): Promise<AbortController> {
  const controller = new AbortController()
  const abortFromRuntime = () => controller.abort()
  if (runtime.signal.aborted) {
    controller.abort()
    return controller
  }
  runtime.signal.addEventListener('abort', abortFromRuntime, { once: true })

  const abortIfInactive = async () => {
    if (controller.signal.aborted || runtime.signal.aborted) {
      return
    }
    const active = await isSessionStillActive(runtime, sessionId, logger)
    if (!active) {
      logger.info({}, 'session canceled or inactive; aborting browser flow')
      controller.abort()
    }
  }

  void abortIfInactive()
  const timer = setInterval(() => {
    void abortIfInactive()
  }, Math.max(500, Math.min(runtime.config.pollIntervalMs, 2000)))

  controller.signal.addEventListener('abort', () => {
    clearInterval(timer)
    runtime.signal.removeEventListener('abort', abortFromRuntime)
  }, { once: true })

  return controller
}

async function isSessionStillActive(runtime: WorkerRuntime, sessionId: string, logger: LoggerLike): Promise<boolean> {
  try {
    return await runtime.dependencies.isSessionActive(runtime.config, sessionId, runtime.signal)
  } catch (error) {
    if (runtime.signal.aborted || isAbortError(error)) {
      return false
    }
    logger.warn({ err: sanitizeError(error) }, 'session active check failed; keeping browser flow alive')
    return true
  }
}

function abortError(message = 'ldxp session aborted'): Error {
  const error = new Error(message)
  error.name = 'AbortError'
  return error
}

function isQrPostedResult(result: BrowserFlowResult): result is Extract<BrowserFlowResult, { kind: 'qr_posted' }> {
  return 'kind' in result && result.kind === 'qr_posted'
}

async function processMockClaimedSession(
  runtime: WorkerRuntime,
  session: ClaimedSession,
  sessionLogger: LoggerLike,
): Promise<void> {
  const artifacts = runtime.dependencies.runMockFlow(session, runtime.config)
  await runtime.dependencies.postQr(runtime.config, session.session_id, artifacts.qr)
  sessionLogger.info({ workerOrderNo: redactValue(artifacts.qr.worker_order_no), mockMode: true }, 'mock qr posted')

  await runtime.dependencies.postResult(runtime.config, session.session_id, artifacts.result)
  sessionLogger.info({ workerOrderNo: redactValue(artifacts.result.worker_order_no), mockMode: true }, 'mock result posted')

  await runtime.dependencies.postMailEvent(runtime.config, artifacts.mailEvent)
  sessionLogger.info({ workerOrderNo: redactValue(artifacts.mailEvent.order_no ?? ''), mockMode: true }, 'mock mail event posted')
}

export function buildErrorPayload(error: unknown, config: WorkerConfig): WorkerErrorPayload {
  const message = sanitizeError(error)
  const snapshotPath = extractSnapshotPath(message, config.debugSnapshotDir)
  return {
    error_code: errorCodeFromWorkerFailure(message),
    error_message: truncateMessage(message, 500),
    ...(snapshotPath ? { snapshot_path: snapshotPath } : {}),
  }
}

function errorCodeFromWorkerFailure(message: string): string {
  if (message.includes('LDXP WAF challenge')) {
    return 'waf_challenge'
  }
  if (message.includes('qr=called') && (
    message.includes('Unable to extract amount from page text') ||
    message.includes('Unable to extract card key')
  )) {
    return 'paid_result_parse_failed'
  }
  return 'worker_flow_failed'
}

async function superviseMailPoller(
  config: WorkerConfig,
  signal: AbortSignal,
  poller: typeof runMailPoller,
  logger: LoggerLike,
): Promise<void> {
  while (!signal.aborted) {
    try {
      await poller(config, signal)
      if (!signal.aborted) {
        logger.error({ err: 'mail poller returned without shutdown signal' }, 'mail poller exited unexpectedly')
      }
    } catch (error) {
      if (signal.aborted || isAbortError(error)) {
        return
      }
      logger.error({ err: sanitizeError(error) }, 'mail poller exited unexpectedly')
    }

    try {
      await sleep(mailPollerRestartDelayMs(config), undefined, { signal })
    } catch (error) {
      if (signal.aborted || isAbortError(error)) {
        return
      }
      throw error
    }
  }
}

function mailPollerRestartDelayMs(config: WorkerConfig): number {
  return Math.max(1000, Math.min(config.pollIntervalMs, 5000))
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
        'mockCardKey',
        'mock_card_key',
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


function trackPaidWatch(runtime: WorkerRuntime, promise: Promise<void>): void {
  runtime.activePaidWatches.add(promise)
  void promise.then(
    () => runtime.activePaidWatches.delete(promise),
    (error) => {
      runtime.logger.error({ err: sanitizeError(error) }, 'unhandled paid-watch error')
      runtime.activePaidWatches.delete(promise)
    },
  )
}

function trackSession(runtime: WorkerRuntime, promise: Promise<void>): void {
  runtime.activeSessions.add(promise)
  void promise.then(
    () => runtime.activeSessions.delete(promise),
    (error) => {
      runtime.logger.error({ err: sanitizeError(error) }, 'unhandled session error')
      runtime.activeSessions.delete(promise)
    },
  )
}

function sanitizeError(error: unknown): string {
  const raw = error instanceof Error ? `${error.name}: ${error.message}` : String(error)
  const normalized = raw.replace(/\\+(["'])/g, '$1')
  return normalized
    .replace(/data:image\/[A-Za-z0-9.+-]+;base64,[A-Za-z0-9+/=]+/g, '[redacted]')
    .replace(/\balipay:\/\/[^"',\s}]+/gi, '[redacted]')
    .replace(/(["']?\bAuthorization\b["']?\s*[:=]\s*\[\s*["']?Bearer\s+)[^"',\]\s}]+/gi, '$1[redacted]')
    .replace(/(["']?\b(?:X-LDXP-Worker-Token|x-api-key)\b["']?\s*[:=]\s*\[\s*["']?)[^"',\]\s}]+/gi, '$1[redacted]')
    .replace(
      /(["']?\b(?:workerToken|worker_token|worker-token|imapPassword|imap_password|imap-password|access_token|refresh_token|api_key|apikey|password|secret|token|authorization|card_key|card-key|worker_card_key|worker-card-key|qr_code|qr-code)\b["']?\s*[:=]\s*\[\s*["']?)[^"',\]\s}&]+/gi,
      '$1[redacted]',
    )
    .replace(/(["']?\bAuthorization\b["']?\s*[:=]\s*["']?Bearer\s+)[^"',\s}]+/gi, '$1[redacted]')
    .replace(/(["']?\b(?:X-LDXP-Worker-Token|x-api-key)\b["']?\s*[:=]\s*["']?)[^"',\s}]+/gi, '$1[redacted]')
    .replace(/([?&](?:access_token|refresh_token|api_key|apikey|token|password|secret|authorization)=)[^&"',\s}]+/gi, '$1[redacted]')
    .replace(
      /(["']?\b(?:workerToken|worker_token|worker-token|imapPassword|imap_password|imap-password|access_token|refresh_token|api_key|apikey|password|secret|token|authorization|card_key|card-key|worker_card_key|worker-card-key|qr_code|qr-code)\b["']?\s*[:=]\s*["']?)[^"',\s}&]+/gi,
      '$1[redacted]',
    )
    .replace(/((?:卡密|授权码)\s*[:：]\s*)[^\s"',，。}]+/g, '$1[redacted]')
    .replace(/\b(?:worker[-_ ]?token|token|password|secret|authorization|card[-_ ]?key|qr[-_ ]?code)\s+[^"',\s}]+/gi, '[redacted]')
}

function truncateMessage(message: string, maxLength: number): string {
  if (message.length <= maxLength) {
    return message
  }
  return `${message.slice(0, maxLength - 1)}…`
}

function extractSnapshotPath(message: string, snapshotDir: string): string | undefined {
  const match = message.match(/snapshot=([^:]+?)(?::\s*|$)/i)
  if (!match?.[1]) {
    return undefined
  }

  const normalizedDir = snapshotDir.replaceAll('\\', '/').replace(/\/+$/, '')
  const parts = match[1]
    .split(',')
    .map((part) => part.trim().replaceAll('\\', '/'))
    .filter(Boolean)
    .map((part) => {
      if (normalizedDir && part.startsWith(`${normalizedDir}/`)) {
        return part.slice(normalizedDir.length + 1)
      }
      return basename(part)
    })
    .filter(Boolean)

  return parts.length > 0 ? parts.join(',') : undefined
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
