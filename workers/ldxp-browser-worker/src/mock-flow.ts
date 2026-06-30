import { createHash } from 'node:crypto'
import type { ClaimedSession, WorkerMailEventPayload, WorkerQrPayload, WorkerResultPayload } from './backend.js'
import type { WorkerConfig } from './config.js'

export interface MockFlowArtifacts {
  qr: WorkerQrPayload
  result: WorkerResultPayload
  mailEvent: WorkerMailEventPayload
}

const testQrDataUrl =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII='

export function buildMockFlowArtifacts(session: ClaimedSession, config: WorkerConfig): MockFlowArtifacts {
  const cardKey = config.mockCardKey?.trim()
  if (!cardKey) {
    throw new Error('LDXP mock flow requires LDXP_WORKER_MOCK_CARD_KEY')
  }

  const orderNo = buildMockOrderNo(session.session_id)
  const amount = session.money
  const productName = session.product_name
  const safeSessionId = safeIdentifier(session.session_id).toLowerCase()
  const stableHashInput = [session.session_id, orderNo, amount.toString(), productName, cardKey].join('|')
  const rawHash = createHash('sha256').update(stableHashInput).digest('hex')
  const stableTimestamp = stableTimestampFromHash(rawHash)

  return {
    qr: {
      worker_order_no: orderNo,
      worker_amount: amount,
      worker_product_name: productName,
      qr_code: testQrDataUrl,
      qr_page_url: `https://example.test/ldxp/mock/${encodeURIComponent(safeSessionId)}`,
    },
    result: {
      worker_order_no: orderNo,
      worker_amount: amount,
      worker_product_name: productName,
      worker_card_key: cardKey,
      worker_status_text: 'mock paid',
      worker_success_url: `https://example.test/ldxp/mock/${encodeURIComponent(safeSessionId)}/success`,
    },
    mailEvent: {
      message_id: `<ldxp-mock-${safeSessionId}@example.test>`,
      imap_uid: String(Number.parseInt(rawHash.slice(0, 12), 16)),
      raw_hash: rawHash,
      from: 'ldxp-mock@example.test',
      to: session.contact_email,
      subject: `LDXP mock paid ${orderNo}`,
      received_time: stableTimestamp,
      order_no: orderNo,
      amount,
      product_name: productName,
      card_key: cardKey,
      paid_time: stableTimestamp,
      body_excerpt: `订单号：${orderNo} 商品名称：${productName} 支付金额：${amount}`,
    },
  }
}

function buildMockOrderNo(sessionId: string): string {
  const safeSessionId = safeIdentifier(sessionId).toUpperCase()
  const digest = createHash('sha256').update(sessionId).digest('hex').slice(0, 12).toUpperCase()
  const suffix = `${safeSessionId}${digest}`.slice(0, 58)
  return `LDMOCK${suffix}`
}

function safeIdentifier(value: string): string {
  const normalized = value.replace(/[^A-Za-z0-9]/g, '')
  return normalized || 'SESSION'
}

function stableTimestampFromHash(hash: string): number {
  const offset = Number.parseInt(hash.slice(0, 8), 16) % (24 * 60 * 60)
  return 1782604800 + offset
}
