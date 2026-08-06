import test from 'node:test'
import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { readFile } from 'node:fs/promises'
import {
  extractOrderNo,
  extractAmount,
  extractCardKey,
  extractOptionalCardKey,
  buildPaidResultFromText,
  isPaidResultText,
  buildBrowserLaunchOptions,
  buildBrowserContextOptions,
  isCashierReadyText,
  isExpectedAmountAcceptable,
  shouldClickManualJump,
  raceDefined,
  fillContactInput,
  clickPurchaseAndResolveCashierPage,
  waitForCashierOrQr,
  withAbort,
} from '../src/browser-flow.js'

const execFileAsync = promisify(execFile)

async function readFixture(name: string): Promise<string> {
  const candidates = [
    new URL(`./fixtures/${name}`, import.meta.url),
    new URL(`../../tests/fixtures/${name}`, import.meta.url),
  ]
  for (const candidate of candidates) {
    try {
      return await readFile(candidate, 'utf8')
    } catch {
      // Try the next path so both direct Bun tests and compiled Node tests work.
    }
  }
  return readFile(candidates[0], 'utf8')
}

test('extracts order no from cashier text', () => {
  assert.equal(extractOrderNo('订单号 LD260628UZJ97P 金额 0.10 元'), 'LD260628UZJ97P')
})

test('extracts paid result and card key', () => {
  const text = '支付成功 已付款 订单号 LD260628UZJ97P 卡密账号 abcd1234-card-key'
  assert.equal(isPaidResultText(text), true)
  assert.equal(extractCardKey(text), 'abcd1234-card-key')
})

test('extracts amount formats from cashier text', () => {
  assert.equal(extractAmount('金额 0.10 元'), 0.1)
  assert.equal(extractAmount('订单金额：￥20.50'), 20.5)
  assert.equal(extractAmount('需支付 ￥100 元'), 100)
})

test('extracts values from html fixtures', async () => {
  const itemHtml = await readFixture('item-page.html')
  const cashierHtml = await readFixture('cashier-page.html')
  const resultHtml = await readFixture('result-page.html')

  assert.match(itemHtml, /请输入联系方式方便查询订单/)
  assert.match(itemHtml, /支付宝/)
  assert.match(itemHtml, /立即购买/)
  assert.equal(extractOrderNo(cashierHtml), 'LD260628UZJ97P')
  assert.equal(extractAmount(cashierHtml), 0.1)
  assert.equal(isPaidResultText(resultHtml), true)
  assert.equal(extractCardKey(resultHtml), 'abcd1234-card-key')
})

test('extracts card key using delivery fallback markers', () => {
  assert.equal(extractCardKey('已发货 1 张\nabcd1234_card_key'), 'abcd1234_card_key')
  assert.equal(extractCardKey('您购买的卡密：\nzzzz-9999-yyyy'), 'zzzz-9999-yyyy')
})

test('throws for missing required extracted values and rejects unpaid text', () => {
  assert.throws(() => extractOrderNo('暂无订单'), /order/i)
  assert.throws(() => extractAmount('金额待确认'), /amount/i)
  assert.throws(() => extractCardKey('已付款 但暂无卡密'), /card/i)
  assert.equal(isPaidResultText('未付款 等待支付 超时'), false)
})

test('allows paid result without card key for direct website topup mode', () => {
  const text = '支付成功 已付款 订单号 LD26062976D90E 订单金额 0.10 元'

  assert.equal(isPaidResultText(text), true)
  assert.equal(extractOptionalCardKey(text), '')
})

test('buildPaidResultFromText uses expected amount when paid result page omits amount', () => {
  const result = buildPaidResultFromText({
    resultText: '订单详情 - 链动小铺\n已付款\n卡密信息 ABCD1234EFGH',
    fallbackOrderNo: 'LD260629PROD001',
    expectedAmount: 0.1,
    workerProductName: '0.1 元测试',
    successUrl: 'https://pay.ldxp.cn/order/result/redacted',
  })

  assert.equal(result.worker_order_no, 'LD260629PROD001')
  assert.equal(result.worker_amount, 0.1)
  assert.equal(result.worker_status_text, '已付款')
})

test('buildPaidResultFromText allows missing card key for instant topup flow', () => {
  const result = buildPaidResultFromText({
    resultText: '订单详情 - 链动小铺\n已付款\n订单号：LD260629PROD002',
    fallbackOrderNo: 'LD260629PROD002',
    expectedAmount: 0.1,
    workerProductName: '0.1 元测试',
    successUrl: 'https://pay.ldxp.cn/order/result/redacted',
  })

  assert.equal(result.worker_card_key, '')
  assert.equal(result.worker_amount, 0.1)
  assert.equal(result.worker_status_text, '已付款')
})

test('builds paid result by reusing cashier order number when result page omits it', () => {
  const result = buildPaidResultFromText({
    resultText: '支付成功 已付款 订单金额 0.10 元',
    fallbackOrderNo: 'LD260629IK87P7',
    expectedAmount: 0.1,
    workerProductName: '0.1元测试',
    successUrl: 'https://pay.ldxp.cn/order/result/LD260629IK87P7',
  })

  assert.deepEqual(result, {
    worker_order_no: 'LD260629IK87P7',
    worker_amount: 0.1,
    worker_product_name: '0.1元测试',
    worker_card_key: '',
    worker_status_text: '已付款',
    worker_success_url: 'https://pay.ldxp.cn/order/result/LD260629IK87P7',
  })
})

test('does not build paid result from blank result page text', () => {
  assert.throws(() => buildPaidResultFromText({
    resultText: '   ',
    fallbackOrderNo: 'LD260629IK87P7',
    expectedAmount: 0.1,
    workerProductName: '0.1元测试',
    successUrl: 'https://pay.ldxp.cn/order/result/LD260629IK87P7',
  }), /paid result/i)
})

test('browser flow uses a real Chrome fingerprint for ldxp item pages', () => {
  assert.deepEqual(buildBrowserLaunchOptions(), { headless: true })
  assert.deepEqual(buildBrowserLaunchOptions({ LDXP_BROWSER_HEADLESS: 'false' }), { headless: false })
  assert.deepEqual(
    buildBrowserLaunchOptions({ LDXP_BROWSER_PROXY_SERVER: ' socks5://127.0.0.1:7891 ' }),
    { headless: true, proxy: { server: 'socks5://127.0.0.1:7891' } },
  )

  for (const proxyServer of [
    'ftp://127.0.0.1:7891',
    'socks5://127.0.0.1',
    'socks5://user:password@127.0.0.1:7891',
    'socks5://127.0.0.1:7891/path',
    'not-a-url',
  ]) {
    assert.throws(
      () => buildBrowserLaunchOptions({ LDXP_BROWSER_PROXY_SERVER: proxyServer }),
      /LDXP_BROWSER_PROXY_SERVER/,
    )
  }

  const context = buildBrowserContextOptions()
  assert.match(context.userAgent ?? '', /Chrome\/126\.0\.0\.0 Safari\/537\.36/)
  assert.equal(context.locale, 'zh-CN')
  assert.equal(context.timezoneId, 'Asia/Shanghai')
  assert.deepEqual(context.viewport, { width: 1280, height: 720 })
  assert.equal(context.extraHTTPHeaders?.['Accept-Language'], 'zh-CN,zh;q=0.9,en;q=0.8')
})


test('cashier readiness requires an order number and does not match the item page QR', () => {
  assert.equal(
    isCashierReadyText('0.1 元测试 ￥0.1 手机扫一扫 使用移动端访问 立即购买'),
    false,
  )
  assert.equal(
    isCashierReadyText('正在为你跳转支付宝支付页面，请稍候'),
    false,
  )
  assert.equal(
    isCashierReadyText('订单号：LD260629PQ1G7D 收款方：链动小铺 0.10 元 扫一扫付款'),
    true,
  )
})

test('accepts card-network service fee on top of the configured ldxp amount', () => {
  assert.equal(isExpectedAmountAcceptable(10.3, 10), true)
  assert.equal(isExpectedAmountAcceptable(20.6, 20), true)
  assert.equal(isExpectedAmountAcceptable(48.93, 47.5), true)
  assert.equal(isExpectedAmountAcceptable(92.7, 90), true)
  assert.equal(isExpectedAmountAcceptable(437.75, 425), true)
  assert.equal(isExpectedAmountAcceptable(10, 10), true)
  assert.equal(isExpectedAmountAcceptable(9.98, 10), false)
  assert.equal(isExpectedAmountAcceptable(12, 10), false)
  assert.equal(isExpectedAmountAcceptable(437.75, 500), false)
})

test('cashier wait does not resolve just because the payApi transition URL is reached', async () => {
  let waitForFunctionCalled = false
  const page = {
    waitForFunction: async (
      predicate: () => boolean,
      _arg: undefined,
      _options: { timeout: number },
    ) => {
      waitForFunctionCalled = true
      assert.equal(predicate(), true)
    },
    waitForURL: async () => {
      throw new Error('waitForCashierOrQr should not use transition URLs as readiness')
    },
  }

  const originalDocument = globalThis.document
  try {
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: { body: { innerText: '订单号：LD260629PQ1G7D 收款方：链动小铺 10.30 元 扫一扫付款' } },
    })
    await waitForCashierOrQr(page as never, 100)
  } finally {
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: originalDocument,
    })
  }

  assert.equal(waitForFunctionCalled, true)
})


test('withAbort consumes late Playwright rejections under strict unhandled rejection handling', async () => {
  const browserFlowModuleUrl = new URL('../src/browser-flow.js', import.meta.url).href
  const childScript = `
    import assert from 'node:assert/strict'
    import { withAbort } from ${JSON.stringify(browserFlowModuleUrl)}

    const controller = new AbortController()
    let rejectSource
    const source = new Promise((_resolve, reject) => {
      rejectSource = reject
    })

    if (process.argv[1] === 'already-aborted') {
      controller.abort()
    }
    const guarded = withAbort(source, controller.signal)
    if (process.argv[1] === 'abort-first') {
      controller.abort()
    }

    await assert.rejects(guarded, { name: 'AbortError' })
    setImmediate(() => rejectSource(new Error('Target page, context or browser has been closed')))
    await new Promise((resolve) => setTimeout(resolve, 25))
  `

  for (const scenario of ['already-aborted', 'abort-first']) {
    const { stderr } = await execFileAsync(
      process.execPath,
      ['--unhandled-rejections=strict', '--input-type=module', '--eval', childScript, scenario],
      { timeout: 5000 },
    )
    assert.equal(stderr, '')
  }
})

test('withAbort preserves a real source failure before cancellation', async () => {
  const controller = new AbortController()
  await assert.rejects(
    withAbort(Promise.reject(new Error('browser disconnected')), controller.signal),
    /browser disconnected/,
  )
})

test('raceDefined returns as soon as a promise resolves with a defined value', async () => {
  const slowTimer = setTimeout(() => undefined, 30000)
  slowTimer.unref?.()
  const slowUndefined = new Promise<undefined>((resolve) => {
    slowTimer.refresh?.()
    setTimeout(() => resolve(undefined), 30000).unref?.()
  })
  const start = Date.now()

  const value = await raceDefined([
    slowUndefined,
    new Promise<string>((resolve) => setTimeout(() => resolve('ready'), 50)),
  ])

  assert.equal(value, 'ready')
  assert.ok(Date.now() - start < 500)
})

test('detects ldxp manual jump prompt before cashier readiness', () => {
  assert.equal(
    shouldClickManualJump('提示 如页面未自动跳转支付页，请点击下方按钮跳转！ 立即跳转'),
    true,
  )
  assert.equal(
    shouldClickManualJump('订单号：LD260629PQ1G7D 金额 0.10 元 立即跳转'),
    false,
  )
})

test('clickPurchaseAndResolveCashierPage clicks manual jump as soon as it appears', async () => {
  let purchaseClickedAt = 0
  let jumpClickedAt = 0
  let jumped = false
  const startedAt = Date.now()
  const slowNoCashierDelayMs = 120

  const purchaseButton = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      purchaseClickedAt = Date.now()
    },
  }
  const jumpButton = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      jumpClickedAt = Date.now()
      jumped = true
    },
  }
  const fakePage = {
    getByText(text: string) {
      if (text.includes('立即购买')) {
        return purchaseButton
      }
      if (text.includes('立即跳转')) {
        return jumpButton
      }
      throw new Error(`unexpected text locator ${text}`)
    },
    context() {
      return {
        waitForEvent() {
          return new Promise((_resolve, reject) => {
            setTimeout(() => reject(new Error('no popup yet')), slowNoCashierDelayMs)
          })
        },
      }
    },
    async waitForURL() {
      if (jumped) {
        return undefined
      }
      return new Promise((_resolve, reject) => {
        setTimeout(() => reject(new Error('cashier url not ready yet')), slowNoCashierDelayMs)
      })
    },
    async waitForFunction(fn: () => unknown) {
      const source = fn.toString()
      if (source.includes('立即跳转')) {
        return undefined
      }
      if (jumped) {
        return undefined
      }
      return new Promise((_resolve, reject) => {
        setTimeout(() => reject(new Error('cashier text not ready yet')), slowNoCashierDelayMs)
      })
    },
    locator(selector: string) {
      assert.equal(selector, 'body')
      return {
        async innerText() {
          if (jumped) {
            return '订单号：LD260629FAST01 金额 0.10 元 使用支付宝扫码付款'
          }
          return '提示 如页面未自动跳转支付页，请点击下方按钮跳转！ 立即跳转'
        },
      }
    },
  }

  const result = await clickPurchaseAndResolveCashierPage(fakePage as never, 5000, 90000)

  assert.equal(result, fakePage)
  assert.ok(purchaseClickedAt >= startedAt)
  assert.ok(jumpClickedAt > 0)
  assert.ok(
    jumpClickedAt - purchaseClickedAt < 80,
    `manual jump was clicked after ${jumpClickedAt - purchaseClickedAt}ms instead of immediately`,
  )
})

test('clickPurchaseAndResolveCashierPage prefers the real manual jump button over dialog text', async () => {
  let clickedDialogText = false
  let clickedButton = false
  const noCashierDelayMs = 10

  const purchaseButton = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      return undefined
    },
  }
  const dialogText = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      clickedDialogText = true
    },
  }
  const realJumpButton = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      clickedButton = true
    },
  }
  const fakePage = {
    getByRole(role: string) {
      assert.equal(role, 'button')
      return realJumpButton
    },
    getByText(text: string) {
      if (text.includes('立即购买')) {
        return purchaseButton
      }
      if (text.includes('立即跳转')) {
        return dialogText
      }
      throw new Error(`unexpected text locator ${text}`)
    },
    context() {
      return {
        waitForEvent() {
          return new Promise((_resolve, reject) => {
            setTimeout(() => reject(new Error('no popup')), noCashierDelayMs)
          })
        },
      }
    },
    async waitForURL() {
      if (clickedButton) {
        return undefined
      }
      return new Promise((_resolve, reject) => {
        setTimeout(() => reject(new Error('cashier url not ready')), noCashierDelayMs)
      })
    },
    async waitForFunction(fn: () => unknown) {
      const source = fn.toString()
      if (source.includes('立即跳转')) {
        return undefined
      }
      if (clickedButton) {
        return undefined
      }
      return new Promise((_resolve, reject) => {
        setTimeout(() => reject(new Error('cashier text not ready')), noCashierDelayMs)
      })
    },
    locator(selector: string) {
      assert.equal(selector, 'body')
      return {
        async innerText() {
          return clickedButton
            ? '订单号：LD260629FAST03 金额 0.10 元 使用支付宝扫码付款'
            : '提示 如页面未自动跳转支付页，请点击下方按钮跳转！ 立即跳转'
        },
      }
    },
  }

  const result = await clickPurchaseAndResolveCashierPage(fakePage as never, 5000, 20)

  assert.equal(result, fakePage)
  assert.equal(clickedButton, true)
  assert.equal(clickedDialogText, false)
})

test('clickPurchaseAndResolveCashierPage keeps watching manual jump after the initial cashier probe', async () => {
  let manualJumpProbeCount = 0
  let jumped = false
  const noCashierDelayMs = 10

  const purchaseButton = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      return undefined
    },
  }
  const jumpButton = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async click() {
      jumped = true
    },
  }
  const fakePage = {
    getByText(text: string) {
      if (text.includes('立即购买')) {
        return purchaseButton
      }
      if (text.includes('立即跳转')) {
        return jumpButton
      }
      throw new Error(`unexpected text locator ${text}`)
    },
    context() {
      return {
        waitForEvent() {
          return new Promise((_resolve, reject) => {
            setTimeout(() => reject(new Error('no popup')), noCashierDelayMs)
          })
        },
      }
    },
    async waitForURL() {
      if (jumped) {
        return undefined
      }
      return new Promise((_resolve, reject) => {
        setTimeout(() => reject(new Error('cashier url not ready')), noCashierDelayMs)
      })
    },
    async waitForFunction(fn: () => unknown) {
      const source = fn.toString()
      if (source.includes('立即跳转')) {
        manualJumpProbeCount += 1
        if (manualJumpProbeCount >= 2) {
          return undefined
        }
        return new Promise((_resolve, reject) => {
          setTimeout(() => reject(new Error('manual jump appears after first probe')), noCashierDelayMs)
        })
      }
      if (jumped) {
        return undefined
      }
      return new Promise((_resolve, reject) => {
        setTimeout(() => reject(new Error('cashier text not ready')), noCashierDelayMs)
      })
    },
    locator(selector: string) {
      assert.equal(selector, 'body')
      return {
        async innerText() {
          return jumped
            ? '订单号：LD260629FAST02 金额 0.10 元 使用支付宝扫码付款'
            : '提示 如页面未自动跳转支付页，请点击下方按钮跳转！ 立即跳转'
        },
      }
    },
  }

  const result = await clickPurchaseAndResolveCashierPage(fakePage as never, 5000, 20)

  assert.equal(result, fakePage)
  assert.equal(jumped, true)
  assert.ok(manualJumpProbeCount >= 2)
})

test('fillContactInput waits for delayed LDXP contact input before failing loading page', async () => {
  let inputProbeCount = 0
  const filledValues: string[] = []
  const hiddenPlaceholder = {
    first() {
      return this
    },
    async waitFor() {
      throw new Error('placeholder is still hidden')
    },
  }
  const visibleInput = {
    first() {
      return this
    },
    async waitFor() {
      return undefined
    },
    async fill(value: string) {
      filledValues.push(value)
    },
  }
  const delayedInputs = {
    async count() {
      inputProbeCount += 1
      return inputProbeCount >= 3 ? 1 : 0
    },
    nth(index: number) {
      assert.equal(index, 0)
      return visibleInput
    },
  }
  const fakePage = {
    getByPlaceholder() {
      return hiddenPlaceholder
    },
    locator(selector: string) {
      if (selector.includes('aliyunCaptcha')) {
        return { count: async () => 0 }
      }
      assert.match(selector, /input/)
      return delayedInputs
    },
    async waitForTimeout() {
      return undefined
    },
  }

  await fillContactInput(fakePage as never, 'support@yunbay.xyz', 120)

  assert.deepEqual(filledValues, ['support@yunbay.xyz'])
  assert.ok(inputProbeCount >= 3)
})

test('fillContactInput fails fast when Aliyun ESA blocks the product page', async () => {
  const fakePage = {
    locator(selector: string) {
      assert.match(selector, /aliyunCaptcha/)
      return { count: async () => 1 }
    },
  }

  await assert.rejects(
    fillContactInput(fakePage as never, 'support@yunbay.xyz', 30_000),
    /LDXP WAF challenge/,
  )
})
