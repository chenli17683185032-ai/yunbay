import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { extractOrderNo, extractAmount, extractCardKey, isPaidResultText } from '../src/browser-flow.js'

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
