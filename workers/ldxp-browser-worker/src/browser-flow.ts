import { mkdir, writeFile } from 'node:fs/promises'
import { basename, join } from 'node:path'
import { chromium, type ElementHandle, type Locator, type Page } from 'playwright'
import type { WorkerConfig } from './config.js'
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

const orderNoPattern = /订单号\s*[:：]?\s*([A-Z0-9]{6,64})/i
const amountPatterns = [
  /(?:订单金额|金额|需支付|实付金额|付款金额)\s*[:：]?\s*￥?\s*([0-9]+(?:\.[0-9]+)?)\s*元?/i,
  /￥\s*([0-9]+(?:\.[0-9]+)?)\s*元?/i,
  /([0-9]+(?:\.[0-9]+)?)\s*元/i,
]
const cardTokenPattern = /[A-Za-z0-9_-]{6,128}/
const paidMarkers = ['已付款', '支付成功', '付款成功', '交易成功', '购买成功']
const unpaidMarkers = ['未付款', '等待支付', '支付超时', '超时', '已取消', '取消订单', '付款失败']

export function extractOrderNo(text: string): string {
  const match = normalizeText(text).match(orderNoPattern)
  if (!match?.[1]) {
    throw new Error('Unable to extract order number from page text')
  }
  return match[1]
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

export function isPaidResultText(text: string): boolean {
  const normalized = normalizeText(text)
  if (unpaidMarkers.some((marker) => normalized.includes(marker))) {
    return false
  }
  return paidMarkers.some((marker) => normalized.includes(marker))
}

export async function runBrowserFlow(
  input: BrowserFlowInput,
  callbacks: { onQr(result: BrowserQrResult): Promise<void> },
  config: WorkerConfig,
  signal?: AbortSignal,
): Promise<BrowserPaidResult> {
  const browser = await chromium.launch({ headless: shouldRunHeadless() })
  const page = await browser.newPage()
  const abortBrowser = () => {
    void browser.close().catch(() => undefined)
  }
  let qrCallbackCalled = false

  if (signal?.aborted) {
    await browser.close()
    throw abortError()
  }
  signal?.addEventListener('abort', abortBrowser, { once: true })

  try {
    page.setDefaultTimeout(Math.max(config.productLoadTimeoutMs, config.qrTimeoutMs, config.resultTimeoutMs))

    await withAbort(page.goto(input.productUrl, {
      waitUntil: 'domcontentloaded',
      timeout: config.productLoadTimeoutMs,
    }), signal)

    await withAbort(fillContactInput(page, input.contactEmail, config.productLoadTimeoutMs), signal)
    const workerProductName = input.expectedProductName ?? (await withAbort(extractProductName(page), signal))
    await withAbort(clickIfPresent(page.getByText('支付宝', { exact: false }), config.productLoadTimeoutMs), signal)
    await withAbort(clickFirstVisible(page.getByText('立即购买', { exact: false }), config.productLoadTimeoutMs), signal)

    await withAbort(waitForCashierOrQr(page, config.qrTimeoutMs), signal)
    const cashierText = await withAbort(page.locator('body').innerText({ timeout: config.qrTimeoutMs }), signal)
    const workerOrderNo = extractOrderNo(cashierText)
    const workerAmount = extractAmount(cashierText)
    assertExpectedAmount(workerAmount, input.expectedAmount)

    const qrCode = await withAbort(extractQrCode(page, config.qrTimeoutMs), signal)
    const qrResult: BrowserQrResult = {
      worker_order_no: workerOrderNo,
      worker_amount: workerAmount,
      worker_product_name: workerProductName,
      qr_code: qrCode,
      qr_page_url: page.url(),
    }
    qrCallbackCalled = true
    await withAbort(callbacks.onQr(qrResult), signal)

    await withAbort(waitForPaidResult(page, config.paymentTimeoutMs, config.resultTimeoutMs), signal)
    const resultText = await withAbort(page.locator('body').innerText({ timeout: config.resultTimeoutMs }), signal)
    const resultOrderNo = extractOrderNo(resultText)
    const resultAmount = extractAmount(resultText)
    const cardKey = extractCardKey(resultText)
    assertExpectedAmount(resultAmount, input.expectedAmount)

    return {
      worker_order_no: resultOrderNo,
      worker_amount: resultAmount,
      worker_product_name: workerProductName,
      worker_card_key: cardKey,
      worker_status_text: summarizeStatusText(resultText),
      worker_success_url: page.url(),
    }
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
    await browser.close().catch(() => undefined)
  }
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

function shouldRunHeadless(): boolean {
  const raw = process.env.LDXP_BROWSER_HEADLESS?.trim().toLowerCase()
  return raw !== 'false' && raw !== '0' && raw !== 'no'
}

async function fillContactInput(page: Page, contactEmail: string, timeout: number): Promise<void> {
  const byPlaceholder = page.getByPlaceholder('请输入联系方式方便查询订单')
  if (await isLocatorVisible(byPlaceholder, timeout)) {
    await byPlaceholder.first().fill(contactEmail, { timeout })
    return
  }

  const inputs = page.locator('input[type="email"], input[type="text"], input:not([type])')
  const count = await inputs.count()
  for (let index = 0; index < count; index += 1) {
    const input = inputs.nth(index)
    if (await isLocatorVisible(input, timeout)) {
      await input.fill(contactEmail, { timeout })
      return
    }
  }

  throw new Error('Unable to find contact input')
}

async function clickIfPresent(locator: Locator, timeout: number): Promise<void> {
  if (await isLocatorVisible(locator, timeout)) {
    await locator.first().click({ timeout })
  }
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

async function waitForCashierOrQr(page: Page, timeout: number): Promise<void> {
  await Promise.race([
    page.waitForURL(/cashier|checkout|order|trade|alipay|payment/i, { timeout }),
    qrLocator(page).first().waitFor({ state: 'visible', timeout }),
    page.waitForFunction(
      () => {
        const text = document.body?.innerText ?? ''
        return text.includes('订单号') && /(?:金额|元|￥)/.test(text)
      },
      undefined,
      { timeout },
    ),
  ])
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

function assertExpectedAmount(actual: number, expected: number): void {
  if (Math.abs(actual - expected) > 0.001) {
    throw new Error(`Amount mismatch: expected ${expected}, got ${actual}`)
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
    page.waitForFunction(
      ({ markers, rejected }) => {
        const text = document.body?.innerText ?? ''
        if (rejected.some((marker) => text.includes(marker))) {
          return false
        }
        return markers.some((marker) => text.includes(marker))
      },
      { markers: paidMarkers, rejected: unpaidMarkers },
      { timeout: paymentTimeoutMs },
    ),
  ])

  await page.waitForLoadState('domcontentloaded', { timeout: resultTimeoutMs }).catch(() => undefined)
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
