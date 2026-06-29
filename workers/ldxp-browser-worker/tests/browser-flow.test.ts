import test from 'node:test'
import assert from 'node:assert/strict'
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
  shouldClickManualJump,
} from '../src/browser-flow.js'

async function readFixture(name: string): Promise<string> {
  return readFile(new URL(`../../tests/fixtures/${name}`, import.meta.url), 'utf8')
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
    isCashierReadyText('订单号：LD260629PQ1G7D 收款方：链动小铺 0.10 元 扫一扫付款'),
    true,
  )
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
