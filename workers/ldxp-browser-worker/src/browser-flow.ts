import { mkdir, writeFile } from 'node:fs/promises'
import { basename, join } from 'node:path'
import { chromium, type BrowserContext, type BrowserContextOptions, type ElementHandle, type Locator, type Page } from 'playwright'
import type { WorkerConfig } from './config.js'
import type { BrowserManager } from './browser-manager.js'
import { redactValue } from './redact.js'

export interface BrowserFlowInput {
  sessionId: string
  productUrl: string
  contactEmail: string
  expectedAmount: number
  expectedProductName?: string
}

export interface BrowserQrResult {
  worker_order_no: string
  worker_amount: number
  worker_product_name: string
  qr_code: string
  qr_page_url: string
}

export interface BrowserPaidResult {
  worker_order_no: string
  worker_amount: number
  worker_product_name: string
  worker_card_key: string
  worker_status_text: string
  worker_success_url: string
}

export interface BuildPaidResultInput {
  resultText: string
  fallbackOrderNo: string
  expectedAmount: number
  workerProductName: string
  successUrl: string
}

export interface BrowserQrPostedResult {
  kind: 'qr_posted'
  worker_order_no: string
}

export type BrowserFlowResult = BrowserPaidResult | BrowserQrPostedResult

export interface BrowserFlowStageTiming {
  stage: string
  ms: number
  detail?: string
}

export interface BrowserFlowDiagnostics {
  timings: BrowserFlowStageTiming[]
}

const orderNoPattern = /订单号\s*[:：]?\s*([A-Z0-9]{6,64})/i
const amountPatterns = [
  /(?:订单金额|金额|需支付|实付金额|付款金额)\s*[:：]?\s*￥?\s*([0-9]+(?:\.[0-9]+)?)\s*元?/i,
  /￥\s*([0-9]+(?:\.[0-9]+)?)\s*元?/i,
  /([0-9]+(?:\.[0-9]+)?)\s*元/i,
]
const cardTokenPattern = /[A-Za-z0-9_-]{6,128}/
const paidMarkers = ['已付款', '支付成功', '付款成功', '交易成功', '购买成功']
const unpaidMarkers = ['未付款', '等待支付', '支付超时', '超时', '已取消', '取消订单', '付款失败']

const defaultBrowserUserAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36'
const defaultBrowserLocale = 'zh-CN'
const defaultBrowserTimezone = 'Asia/Shanghai'
const defaultBrowserAcceptLanguage = 'zh-CN,zh;q=0.9,en;q=0.8'

type BrowserLaunchOptions = { headless: boolean }

export function buildBrowserLaunchOptions(env: Record<string, string | undefined> = process.env): BrowserLaunchOptions {
  return { headless: shouldRunHeadless(env) }
}

export function buildBrowserContextOptions(env: Record<string, string | undefined> = process.env): BrowserContextOptions {
  const userAgent = env.LDXP_BROWSER_USER_AGENT?.trim() || defaultBrowserUserAgent
  const locale = env.LDXP_BROWSER_LOCALE?.trim() || defaultBrowserLocale
  const timezoneId = env.LDXP_BROWSER_TIMEZONE?.trim() || defaultBrowserTimezone
  const acceptLanguage = env.LDXP_BROWSER_ACCEPT_LANGUAGE?.trim() || defaultBrowserAcceptLanguage

  return {
    userAgent,
    locale,
    timezoneId,
    viewport: { width: 1280, height: 720 },
    deviceScaleFactor: 1,
    isMobile: false,
    hasTouch: false,
    extraHTTPHeaders: {
      'Accept-Language': acceptLanguage,
    },
  }
}

export function extractOrderNo(text: string): string {
  const match = normalizeText(text).match(orderNoPattern)
  if (!match?.[1]) {
    throw new Error('Unable to extract order number from page text')
  }
  return match[1]
}

function extractOrderNoOrFallback(text: string, fallbackOrderNo: string): string {
  try {
    return extractOrderNo(text)
  } catch {
    const normalizedFallback = fallbackOrderNo.trim()
    if (!normalizedFallback) {
      throw new Error('Unable to extract order number from paid result text')
    }
    return normalizedFallback
  }
}

export function extractAmount(text: string): number {
  const normalized = normalizeText(text)
  for (const pattern of amountPatterns) {
    const match = normalized.match(pattern)
    if (!match?.[1]) {
      continue
    }
    const amount = Number(match[1])
    if (Number.isFinite(amount)) {
      return amount
    }
  }

  throw new Error('Unable to extract amount from page text')
}

function extractAmountOrFallback(text: string, fallbackAmount: number): number {
  try {
    return extractAmount(text)
  } catch {
    if (Number.isFinite(fallbackAmount) && fallbackAmount > 0) {
      return fallbackAmount
    }
    throw new Error('Unable to extract amount from paid result text and fallback amount is invalid')
  }
}

export function extractCardKey(text: string): string {
  const normalized = normalizeText(text)
  const directPatterns = [
    /第\s*1\s*张\s*[:：]?\s*([A-Za-z0-9_-]{6,128})/i,
    /卡密账号\s*[:：]?\s*([A-Za-z0-9_-]{6,128})/i,
    /卡密\s*[:：]?\s*([A-Za-z0-9_-]{6,128})/i,
  ]

  for (const pattern of directPatterns) {
    const match = normalized.match(pattern)
    if (match?.[1]) {
      return match[1]
    }
  }

  const fallbackMarkers = ['已发货 1 张', '已发货1张', '您购买的卡密']
  for (const marker of fallbackMarkers) {
    const index = normalized.indexOf(marker)
    if (index === -1) {
      continue
    }
    const tail = normalized.slice(index + marker.length)
    const token = tail.match(cardTokenPattern)?.[0]
    if (token) {
      return token
    }
  }

  throw new Error('Unable to extract card key from paid result text')
}

export function extractOptionalCardKey(text: string): string {
  try {
    return extractCardKey(text)
  } catch {
    return ''
  }
}

export function isPaidResultText(text: string): boolean {
  const normalized = normalizeText(text)
  if (unpaidMarkers.some((marker) => normalized.includes(marker))) {
    return false
  }
  return paidMarkers.some((marker) => normalized.includes(marker))
}

export function buildPaidResultFromText(input: BuildPaidResultInput): BrowserPaidResult {
  if (!isPaidResultText(input.resultText)) {
    throw new Error('Paid result text does not indicate payment success')
  }

  const resultOrderNo = extractOrderNoOrFallback(input.resultText, input.fallbackOrderNo)
  const resultAmount = extractAmountOrFallback(input.resultText, input.expectedAmount)
  assertExpectedAmount(resultAmount, input.expectedAmount)

  return {
    worker_order_no: resultOrderNo,
    worker_amount: resultAmount,
    worker_product_name: input.workerProductName,
    worker_card_key: extractOptionalCardKey(input.resultText),
    worker_status_text: summarizeStatusText(input.resultText),
    worker_success_url: input.successUrl,
  }
}

export function isCashierReadyText(text: string): boolean {
  const normalized = normalizeText(text)
  return orderNoPattern.test(normalized) && /(?:金额|元|￥|付款|收款方)/.test(normalized)
}

export function shouldClickManualJump(text: string): boolean {
  const normalized = normalizeText(text)
  return normalized.includes('立即跳转') && !isCashierReadyText(normalized)
}

export async function runBrowserFlow(
  input: BrowserFlowInput,
  callbacks: { onQr(result: BrowserQrResult): Promise<void> },
  config: WorkerConfig,
  signal?: AbortSignal,
  diagnostics?: BrowserFlowDiagnostics,
  browserManager?: BrowserManager,
): Promise<BrowserFlowResult> {
  const timing = createFlowTiming(diagnostics)
  const browserResources = await timing.record('browser_launch', openBrowserResources(browserManager))
  const context = await timing.record('browser_context', browserResources.contextPromise)
  const page = await timing.record('new_page', context.newPage())
  const abortBrowser = () => {
    void closeBrowserResources(browserResources).catch(() => undefined)
  }
  let qrCallbackCalled = false

  if (signal?.aborted) {
    await closeBrowserResources(browserResources)
    throw abortError()
  }
  signal?.addEventListener('abort', abortBrowser, { once: true })

  try {
    page.setDefaultTimeout(Math.max(config.productLoadTimeoutMs, config.qrTimeoutMs, config.resultTimeoutMs))

    await timing.record('product_goto', withAbort(page.goto(input.productUrl, {
      waitUntil: 'domcontentloaded',
      timeout: config.productLoadTimeoutMs,
    }), signal))

    await timing.record('fill_contact', withAbort(fillContactInput(page, input.contactEmail, config.productLoadTimeoutMs), signal))
    const workerProductName = input.expectedProductName ?? (await timing.record('extract_product_name', withAbort(extractProductName(page), signal)))
    await timing.record('select_alipay', withAbort(clickIfPresent(page.getByText('支付宝', { exact: false }), config.productLoadTimeoutMs), signal))
    const cashierPage = await timing.record(
      'click_purchase_to_cashier',
      clickPurchaseAndResolveCashierPage(page, config.productLoadTimeoutMs, config.qrTimeoutMs, signal),
    )

    await timing.record('wait_cashier_ready', withAbort(waitForCashierOrQr(cashierPage, config.qrTimeoutMs), signal))
    const cashierText = await timing.record('read_cashier_text', withAbort(cashierPage.locator('body').innerText({ timeout: config.qrTimeoutMs }), signal))
    const workerOrderNo = extractOrderNo(cashierText)
    const workerAmount = extractAmount(cashierText)
    assertExpectedAmount(workerAmount, input.expectedAmount)

    const qrCode = await timing.record('extract_qr', withAbort(extractQrCode(cashierPage, config.qrTimeoutMs), signal))
    const qrResult: BrowserQrResult = {
      worker_order_no: workerOrderNo,
      worker_amount: workerAmount,
      worker_product_name: workerProductName,
      qr_code: qrCode,
      qr_page_url: cashierPage.url(),
    }
    qrCallbackCalled = true
    await timing.record('post_qr_callback', withAbort(callbacks.onQr(qrResult), signal))
    if (config.releaseSessionSlotAfterQr) {
      return { kind: 'qr_posted', worker_order_no: workerOrderNo }
    }

    await timing.record('wait_paid_result', withAbort(waitForPaidResult(cashierPage, config.paymentTimeoutMs, config.resultTimeoutMs), signal))
    const resultText = await timing.record('read_paid_result_text', withAbort(cashierPage.locator('body').innerText({ timeout: config.resultTimeoutMs }), signal))

    return buildPaidResultFromText({
      resultText,
      fallbackOrderNo: workerOrderNo,
      expectedAmount: input.expectedAmount,
      workerProductName,
      successUrl: cashierPage.url(),
    })
  } catch (error) {
    if (signal?.aborted || isAbortError(error)) {
      throw abortError()
    }
    const snapshots = await saveDebugSnapshots(page, input.sessionId, config.debugSnapshotDir)
    const detail = safeErrorDetail(error)
    const safeProductUrl = redactValue(input.productUrl)
    const qrState = qrCallbackCalled ? 'called' : 'not_called'
    throw new Error(
      `Browser flow failed for session ${input.sessionId} product ${safeProductUrl} qr=${qrState} snapshot=${snapshots.summary}: ${detail}`,
    )
  } finally {
    signal?.removeEventListener('abort', abortBrowser)
    await closeBrowserResources(browserResources).catch(() => undefined)
  }
}

interface BrowserFlowResources {
  contextPromise: Promise<BrowserContext>
  close(): Promise<void>
}

async function openBrowserResources(browserManager?: BrowserManager): Promise<BrowserFlowResources> {
  if (browserManager) {
    const contextPromise = browserManager.getContext()
    return {
      contextPromise,
      async close() {
        const context = await contextPromise.catch(() => undefined)
        await context?.close().catch(() => undefined)
      },
    }
  }

  const browser = await chromium.launch(buildBrowserLaunchOptions())
  return {
    contextPromise: browser.newContext(buildBrowserContextOptions()),
    async close() {
      await browser.close().catch(() => undefined)
    },
  }
}

async function closeBrowserResources(resources: BrowserFlowResources): Promise<void> {
  await resources.close()
}

export async function clickPurchaseAndResolveCashierPage(
  page: Page,
  clickTimeoutMs: number,
  cashierTimeoutMs: number,
  signal?: AbortSignal,
): Promise<Page> {
  const firstPopupPromise = waitForNextPage(page, 5000, signal)
  await withAbort(clickFirstVisible(page.getByText('立即购买', { exact: false }), clickTimeoutMs), signal)

  const ready = await raceDefined([
    tryWaitForCashierPage(page, 5000, signal),
    firstPopupPromise.then((popup) => tryWaitForCashierPage(popup, 5000, signal)),
    clickManualJumpAndResolveCashierPage(page, clickTimeoutMs, 5000, signal),
  ])
  if (ready) {
    return ready
  }

  const fallbackTimeoutMs = Math.min(cashierTimeoutMs, 10000)
  return (await raceDefined([
    clickManualJumpAndResolveCashierPage(page, clickTimeoutMs, fallbackTimeoutMs, signal),
    tryWaitForCashierPage(page, fallbackTimeoutMs, signal),
  ])) ?? page
}

async function clickManualJumpAndResolveCashierPage(
  page: Page,
  clickTimeoutMs: number,
  cashierTimeoutMs: number,
  signal?: AbortSignal,
): Promise<Page | undefined> {
  const jumpAvailable = await waitForManualJumpAvailable(page, Math.min(clickTimeoutMs, 5000), signal)
  if (!jumpAvailable) {
    return undefined
  }

  const jumpPopupPromise = waitForNextPage(page, cashierTimeoutMs, signal)
  const clickedJump = await withAbort(clickManualJumpButton(page, Math.min(clickTimeoutMs, 1000)), signal)
  if (!clickedJump.clicked) {
    return undefined
  }

  const cashierPage = await raceDefined([
    jumpPopupPromise.then((popup) => tryWaitForCashierPage(popup, cashierTimeoutMs, signal)),
    tryWaitForCashierPage(page, cashierTimeoutMs, signal),
  ])
  if (cashierPage) {
    return withDetail(cashierPage, `manual_jump:${clickedJump.method}`)
  }
  return undefined
}

async function clickManualJumpButton(page: Page, timeout: number): Promise<{ clicked: boolean; method?: string }> {
  const candidateFactories: Array<[string, () => Locator]> = [
    ['role_button', () => page.getByRole('button', { name: /立即跳转/ })],
    ['button_or_link_text', () => page.locator('button, [role="button"], a').filter({ hasText: '立即跳转' })],
    ['exact_text', () => page.getByText('立即跳转', { exact: true })],
    ['text', () => page.getByText('立即跳转', { exact: false })],
  ]

  for (const [method, makeCandidate] of candidateFactories) {
    const candidate = tryMakeLocator(makeCandidate)
    if (!candidate) {
      continue
    }
    if (await clickIfPresent(candidate, timeout)) {
      return { clicked: true, method }
    }
  }
  return { clicked: false }
}

function tryMakeLocator(makeLocator: () => Locator): Locator | undefined {
  try {
    return makeLocator()
  } catch {
    return undefined
  }
}

async function waitForManualJumpAvailable(page: Page, timeout: number, signal?: AbortSignal): Promise<boolean> {
  try {
    await withAbort(
      page.waitForFunction(
        () => {
          const text = document.body?.innerText ?? ''
          return text.includes('立即跳转') && !(/订单号\s*[:：]?\s*[A-Z0-9]{6,64}/i.test(text) && /(?:金额|元|￥|付款|收款方)/.test(text))
        },
        undefined,
        { timeout },
      ),
      signal,
    )
  } catch {
    return false
  }
  return true
}

export async function raceDefined<T>(promises: Array<Promise<T | undefined>>): Promise<T | undefined> {
  const pending = new Set(promises)
  while (pending.size > 0) {
    const settled = await Promise.race(
      [...pending].map((promise) =>
        promise.then(
          (value) => ({ promise, value }),
          () => ({ promise, value: undefined }),
        ),
      ),
    )
    pending.delete(settled.promise)
    if (settled.value !== undefined) {
      return settled.value
    }
  }
  return undefined
}

async function waitForNextPage(page: Page, timeout: number, signal?: AbortSignal): Promise<Page | undefined> {
  try {
    const nextPage = await withAbort(page.context().waitForEvent('page', { timeout }), signal)
    await withAbort(nextPage.waitForLoadState('domcontentloaded', { timeout: Math.min(timeout, 10000) }).catch(() => undefined), signal)
    return nextPage
  } catch {
    return undefined
  }
}

async function tryWaitForCashierPage(page: Page | undefined, timeout: number, signal?: AbortSignal): Promise<Page | undefined> {
  if (!page) {
    return undefined
  }
  try {
    await withAbort(waitForCashierOrQr(page, timeout), signal)
    const text = await withAbort(page.locator('body').innerText({ timeout: Math.min(timeout, 5000) }), signal)
    if (isCashierReadyText(text)) {
      return page
    }
  } catch {
    return undefined
  }
  return undefined
}

function withAbort<T>(promise: Promise<T>, signal?: AbortSignal): Promise<T> {
  if (!signal) {
    return promise
  }
  if (signal.aborted) {
    return Promise.reject(abortError())
  }

  return new Promise<T>((resolve, reject) => {
    const onAbort = () => reject(abortError())
    signal.addEventListener('abort', onAbort, { once: true })
    promise.then(resolve, reject).finally(() => signal.removeEventListener('abort', onAbort))
  })
}

function createFlowTiming(diagnostics?: BrowserFlowDiagnostics): { record<T>(stage: string, promise: Promise<T>): Promise<T> } {
  return {
    async record<T>(stage: string, promise: Promise<T>): Promise<T> {
      const start = Date.now()
      let value: T | undefined
      let hasValue = false
      try {
        value = await promise
        hasValue = true
        return value
      } finally {
        const detail = hasValue && typeof value === 'object' && value !== null && flowTimingDetailSymbol in value
          ? String((value as Record<PropertyKey, unknown>)[flowTimingDetailSymbol] ?? '')
          : ''
        diagnostics?.timings.push({ stage, ms: Date.now() - start, ...(detail ? { detail } : {}) })
      }
    },
  }
}

const flowTimingDetailSymbol = Symbol('ldxpFlowTimingDetail')

function withDetail<T extends object>(value: T, detail: string): T {
  Object.defineProperty(value, flowTimingDetailSymbol, {
    value: detail,
    enumerable: false,
    configurable: true,
  })
  return value
}

function abortError(): Error {
  const error = new Error('worker shutdown aborted browser flow')
  error.name = 'AbortError'
  return error
}

function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

function safeErrorDetail(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error)
  return raw
    .replace(/data:image\/[A-Za-z0-9.+-]+;base64,[A-Za-z0-9+/=]+/g, 'data:image/png;base64,[redacted]')
    .replace(/((?:qr_code|worker_card_key|card_key|token|authorization|password)["']?\s*[:=]\s*["']?)([^"',\s}]+)/gi, (_match, prefix: string, value: string) => `${prefix}${redactValue(value)}`)
}

function normalizeText(text: string): string {
  return text.replace(/\s+/g, ' ').trim()
}

function shouldRunHeadless(env: Record<string, string | undefined> = process.env): boolean {
  const raw = env.LDXP_BROWSER_HEADLESS?.trim().toLowerCase()
  return raw !== 'false' && raw !== '0' && raw !== 'no'
}

export async function fillContactInput(page: Page, contactEmail: string, timeout: number): Promise<void> {
  const deadline = Date.now() + timeout
  let lastCandidateCount = 0

  while (Date.now() < deadline) {
    const remaining = Math.max(1, deadline - Date.now())
    const probeTimeout = Math.min(remaining, 1000)
    const byPlaceholder = page.getByPlaceholder('请输入联系方式方便查询订单')
    if (await isLocatorVisible(byPlaceholder, probeTimeout)) {
      await byPlaceholder.first().fill(contactEmail, { timeout: remaining })
      return
    }

    const inputs = page.locator('input[type="email"], input[type="text"], input:not([type])')
    const count = await inputs.count()
    lastCandidateCount = count
    for (let index = 0; index < count; index += 1) {
      const input = inputs.nth(index)
      if (await isLocatorVisible(input, probeTimeout)) {
        await input.fill(contactEmail, { timeout: Math.max(1, deadline - Date.now()) })
        return
      }
    }

    await page.waitForTimeout(Math.min(250, Math.max(1, deadline - Date.now()))).catch(() => undefined)
  }

  throw new Error(`Unable to find contact input after waiting for product page (candidate inputs: ${lastCandidateCount})`)
}

async function clickIfPresent(locator: Locator, timeout: number): Promise<boolean> {
  if (await isLocatorVisible(locator, timeout)) {
    await locator.first().click({ timeout })
    return true
  }
  return false
}

async function clickFirstVisible(locator: Locator, timeout: number): Promise<void> {
  if (!(await isLocatorVisible(locator, timeout))) {
    throw new Error('Unable to find purchase button')
  }
  await locator.first().click({ timeout })
}

async function isLocatorVisible(locator: Locator, timeout: number): Promise<boolean> {
  try {
    await locator.first().waitFor({ state: 'visible', timeout: Math.min(timeout, 3000) })
    return true
  } catch {
    return false
  }
}

export async function waitForCashierOrQr(page: Page, timeout: number): Promise<void> {
  await page.waitForFunction(
    () => {
      const text = document.body?.innerText ?? ''
      return /订单号\s*[:：]?\s*[A-Z0-9]{6,64}/i.test(text) && /(?:金额|元|￥|付款|收款方)/.test(text)
    },
    undefined,
    { timeout },
  )
}

function qrLocator(page: Page): Locator {
  return page.locator(
    [
      'img[alt*="二维码"]',
      'img[src^="data:image/"]',
      'img[src*="qr"]',
      'canvas',
      '.qr-code',
      '.qrcode',
      '[class*="qr"]',
      '[id*="qr"]',
    ].join(','),
  )
}

async function extractProductName(page: Page): Promise<string> {
  const title = normalizeText(await page.title().catch(() => ''))
  if (title) {
    return title
  }

  const heading = normalizeText(await page.locator('h1,h2,.title,.product-title').first().innerText({ timeout: 1000 }).catch(() => ''))
  return heading || 'LDXP product'
}

export function isExpectedAmountAcceptable(actual: number, expected: number): boolean {
  if (!Number.isFinite(actual) || !Number.isFinite(expected) || actual <= 0 || expected <= 0) {
    return false
  }
  const epsilon = 0.01
  if (Math.abs(actual - expected) <= epsilon) {
    return true
  }
  const cardNetworkFee = actual - expected
  const maxCardNetworkFee = expected * 0.05
  return cardNetworkFee >= -epsilon && cardNetworkFee <= maxCardNetworkFee + epsilon
}

function assertExpectedAmount(actual: number, expected: number): void {
  if (!isExpectedAmountAcceptable(actual, expected)) {
    throw new Error(`Amount mismatch: expected ${expected} plus card-network fee, got ${actual}`)
  }
}

async function extractQrCode(page: Page, timeout: number): Promise<string> {
  const imageQr = await findQrImageDataUrl(page, timeout)
  if (imageQr) {
    return imageQr
  }

  const canvasQr = await findCanvasQrDataUrl(page, timeout)
  if (canvasQr) {
    return canvasQr
  }

  const qrElement = await findQrElement(page, timeout)
  if (qrElement) {
    return elementScreenshotDataUrl(qrElement)
  }

  const pageImage = await page.screenshot({ type: 'png' })
  return `data:image/png;base64,${pageImage.toString('base64')}`
}

async function findQrImageDataUrl(page: Page, timeout: number): Promise<string | undefined> {
  const images = page.locator('img')
  await images.first().waitFor({ state: 'visible', timeout }).catch(() => undefined)
  const count = await images.count()
  for (let index = 0; index < count; index += 1) {
    const image = images.nth(index)
    if (!(await image.isVisible().catch(() => false))) {
      continue
    }
    const src = await image.getAttribute('src')
    if (!src) {
      continue
    }
    if (src.startsWith('data:image/')) {
      return src
    }
  }
  return undefined
}

async function findCanvasQrDataUrl(page: Page, timeout: number): Promise<string | undefined> {
  const canvases = page.locator('canvas')
  await canvases.first().waitFor({ state: 'visible', timeout }).catch(() => undefined)
  const count = await canvases.count()
  for (let index = 0; index < count; index += 1) {
    const canvas = canvases.nth(index)
    if (!(await canvas.isVisible().catch(() => false))) {
      continue
    }
    const dataUrl = await canvas.evaluate((node) => (node as HTMLCanvasElement).toDataURL('image/png')).catch(() => undefined)
    if (typeof dataUrl === 'string' && dataUrl.startsWith('data:image/png;base64,')) {
      return dataUrl
    }
  }
  return undefined
}

async function findQrElement(page: Page, timeout: number): Promise<ElementHandle<SVGElement | HTMLElement> | undefined> {
  const selectors = [
    'img[alt*="二维码"]',
    'img[src*="qr"]',
    'canvas',
    '.qr-code',
    '.qrcode',
    '[class*="qr"]',
    '[id*="qr"]',
  ]

  for (const selector of selectors) {
    const locator = page.locator(selector).first()
    if (!(await isLocatorVisible(locator, timeout))) {
      continue
    }
    const handle = await locator.elementHandle()
    if (handle) {
      return handle
    }
  }

  return undefined
}

async function elementScreenshotDataUrl(element: ElementHandle<SVGElement | HTMLElement>): Promise<string> {
  const image = await element.screenshot({ type: 'png' })
  return `data:image/png;base64,${image.toString('base64')}`
}

async function waitForPaidResult(page: Page, paymentTimeoutMs: number, resultTimeoutMs: number): Promise<void> {
  await Promise.race([
    page.waitForURL(/\/order\/result\//, { timeout: paymentTimeoutMs }),
    waitForPaidMarker(page, paymentTimeoutMs),
  ])

  await page.waitForLoadState('domcontentloaded', { timeout: resultTimeoutMs }).catch(() => undefined)
  await waitForPaidMarker(page, resultTimeoutMs)
}

async function waitForPaidMarker(page: Page, timeout: number): Promise<void> {
  await page.waitForFunction(
    ({ markers, rejected }) => {
      const text = document.body?.innerText ?? ''
      if (rejected.some((marker) => text.includes(marker))) {
        return false
      }
      return markers.some((marker) => text.includes(marker))
    },
    { markers: paidMarkers, rejected: unpaidMarkers },
    { timeout },
  )
}

function summarizeStatusText(text: string): string {
  const normalized = normalizeText(text)
  for (const marker of paidMarkers) {
    if (normalized.includes(marker)) {
      return marker
    }
  }
  return normalized.slice(0, 120)
}

async function saveDebugSnapshots(
  page: Page,
  sessionId: string,
  snapshotDir: string,
): Promise<{ summary: string; screenshotPath?: string; htmlPath?: string }> {
  const safeSessionId = sessionId.replace(/[^A-Za-z0-9_-]/g, '_') || 'unknown-session'
  const stamp = new Date().toISOString().replace(/[^0-9]/g, '').slice(0, 14)
  const screenshotPath = join(snapshotDir, `${safeSessionId}-${stamp}.png`)
  const htmlPath = join(snapshotDir, `${safeSessionId}-${stamp}.html`)

  try {
    await mkdir(snapshotDir, { recursive: true })
    await page.screenshot({ path: screenshotPath, fullPage: true })
    await writeFile(htmlPath, await page.content(), 'utf8')
    return { summary: `${basename(screenshotPath)},${basename(htmlPath)}`, screenshotPath, htmlPath }
  } catch {
    // Best-effort debug artifact only; never hide the original browser-flow failure.
    return { summary: 'unavailable' }
  }
}
