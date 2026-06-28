import test from 'node:test'
import assert from 'node:assert/strict'
import { parseLdxpMailBody, hashRawMail, makeBodyExcerpt } from '../src/mail-parser.js'

test('parseLdxpMailBody extracts order/card/amount', () => {
  const parsed = parseLdxpMailBody('订单号：LD260628UZJ97P\n支付金额：0.10 元\n商品名称：0.1 元测试\n卡密账号：abcd1234-card-key')
  assert.equal(parsed.order_no, 'LD260628UZJ97P')
  assert.equal(parsed.amount, 0.10)
  assert.equal(parsed.product_name, '0.1 元测试')
  assert.equal(parsed.card_key, 'abcd1234-card-key')
})

test('parseLdxpMailBody extracts realistic LDXP purchase body', () => {
  const parsed = parseLdxpMailBody(`感谢购买商品0.1 元测试
实付0.10元
数量:1,
付款时间2026-06-28 03:37:42
单号:LD260628UZJ97P,
以下是您的购买内容:
abcd1234-card-key`)

  assert.deepEqual(parsed, {
    order_no: 'LD260628UZJ97P',
    amount: 0.10,
    product_name: '0.1 元测试',
    card_key: 'abcd1234-card-key',
  })
})

test('parseLdxpMailBody normalizes simple html body', () => {
  const parsed = parseLdxpMailBody('<div>订单编号：LD260628UZJ97P</div><p>金额：￥0.10 元</p><p>商品名：0.1 元测试</p><br>兑换码：html-card-key')

  assert.equal(parsed.order_no, 'LD260628UZJ97P')
  assert.equal(parsed.amount, 0.10)
  assert.equal(parsed.product_name, '0.1 元测试')
  assert.equal(parsed.card_key, 'html-card-key')
})

test('makeBodyExcerpt redacts card labels and caps output length', () => {
  const excerpt = makeBodyExcerpt(`订单号：LD260628UZJ97P
商品名称：0.1 元测试
卡密账号：abcd1234-card-key
${'上下文'.repeat(400)}`)

  assert.match(excerpt, /订单号：LD260628UZJ97P/)
  assert.match(excerpt, /卡密账号：\[redacted\]/)
  assert.doesNotMatch(excerpt, /abcd1234-card-key/)
  assert.ok(excerpt.length <= 1000)
})

test('hashRawMail is stable sha256 hex', () => {
  assert.equal(hashRawMail(Buffer.from('abc')).length, 64)
  assert.equal(hashRawMail(Buffer.from('abc')), hashRawMail(Buffer.from('abc')))
})
