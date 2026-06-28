import { createHash } from 'node:crypto'

export interface ParsedMailEvent {
  message_id: string
  imap_uid: string
  raw_hash: string
  mail_from: string
  mail_to: string
  subject: string
  received_time: number
  order_no: string
  amount: number
  product_name: string
  card_key: string
  paid_time: number
  body_excerpt: string
}

const maxExcerptLength = 1000
const orderPattern = /(?:订单号|订单编号|订单|单号)\s*[:：]?\s*(LD[A-Z0-9-]+)\s*[,，]?/i
const amountPattern = /(?:支付金额|金额|实付)\s*[:：]?\s*(?:￥|¥|RMB)?\s*([0-9]+(?:\.[0-9]+)?)\s*元?/i
const productPatterns = [
  /(?:商品名称|商品名)[^\S\r\n]*[:：]?[^\S\r\n]*([^\n\r,，]+)/i,
  /感谢购买商品[^\S\r\n]*([^\n\r,，]+)/i,
  /(?:^|\n)购买内容[^\S\r\n]*[:：]?[^\S\r\n]*([^\n\r,，]+)/i,
]
const labeledCardPattern = /(?:卡密账号|卡密|兑换码)\s*[:：]?\s*([^\s\n\r,，]+)/i
const deliveryMarkerPattern = /以下是您的购买内容\s*[:：]?\s*\n+\s*([^\s\n\r,，]+)/i

export function hashRawMail(raw: Buffer): string {
  return createHash('sha256').update(raw).digest('hex')
}

export function parseLdxpMailBody(body: string): Pick<ParsedMailEvent, 'order_no' | 'amount' | 'product_name' | 'card_key'> {
  const normalized = normalizeMailBody(body)

  const order_no = matchRequired(normalized, orderPattern, 'order number')
  const amountRaw = matchRequired(normalized, amountPattern, 'amount')
  const amount = Number(amountRaw)
  if (!Number.isFinite(amount)) {
    throw new Error('Unable to parse LDXP mail amount')
  }

  const product_name = matchFirstRequired(normalized, productPatterns, 'product name')
  const card_key = extractCardKey(normalized, product_name)

  return {
    order_no,
    amount,
    product_name,
    card_key,
  }
}

export function makeBodyExcerpt(body: string): string {
  const normalized = normalizeMailBody(body)
  const redacted = redactSensitiveMailBody(normalized)
  return redacted.length <= maxExcerptLength ? redacted : redacted.slice(0, maxExcerptLength)
}

function extractCardKey(body: string, productName: string): string {
  const labeled = body.match(labeledCardPattern)?.[1]?.trim()
  if (labeled && labeled !== productName) {
    return labeled
  }

  const delivered = body.match(deliveryMarkerPattern)?.[1]?.trim()
  if (delivered && delivered !== productName) {
    return delivered
  }

  throw new Error('Unable to parse LDXP mail card key')
}

function matchRequired(body: string, pattern: RegExp, fieldName: string): string {
  const value = body.match(pattern)?.[1]?.trim()
  if (!value) {
    throw new Error(`Unable to parse LDXP mail ${fieldName}`)
  }
  return value
}

function matchFirstRequired(body: string, patterns: RegExp[], fieldName: string): string {
  for (const pattern of patterns) {
    const value = body.match(pattern)?.[1]?.trim()
    if (value) {
      return value
    }
  }
  throw new Error(`Unable to parse LDXP mail ${fieldName}`)
}

function normalizeMailBody(body: string): string {
  return decodeBasicHtmlEntities(stripHtmlPreservingBreaks(body))
    .replace(/\r\n?/g, '\n')
    .replace(/[\t\f\v\u00a0]+/g, ' ')
    .replace(/ *\n */g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

function stripHtmlPreservingBreaks(body: string): string {
  return body
    .replace(/<\s*(br|\/p|\/div|\/li|\/tr|\/h[1-6])\b[^>]*>/gi, '\n')
    .replace(/<\s*(p|div|li|tr|h[1-6])\b[^>]*>/gi, '\n')
    .replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, '')
    .replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, '')
    .replace(/<[^>]+>/g, '')
}

function decodeBasicHtmlEntities(value: string): string {
  return value
    .replace(/&nbsp;/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;|&apos;/gi, "'")
    .replace(/&#(\d+);/g, (_, codePoint: string) => String.fromCodePoint(Number(codePoint)))
    .replace(/&#x([0-9a-f]+);/gi, (_, codePoint: string) => String.fromCodePoint(Number.parseInt(codePoint, 16)))
}

function redactSensitiveMailBody(body: string): string {
  return body
    .replace(/((?:卡密账号|卡密|兑换码)\s*[:：]?\s*)[^\s\n\r,，]+/gi, '$1[redacted]')
    .replace(/((?:以下是您的购买内容)\s*[:：]?\s*\n+)\s*[^\s\n\r,，]+/gi, '$1[redacted]')
}
