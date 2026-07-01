/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { LdxpTopupStatus } from '../types'

export const LDXP_TOPUP_AMOUNTS = [10, 20, 30, 50, 100, 500] as const
export const LDXP_QR_CREATION_ANIMATION_SECONDS = 30

export const LDXP_TOPUP_DISCOUNTS: Partial<Record<number, number>> = {
  50: 0.95,
  100: 0.9,
  500: 0.85,
}

export type LdxpPricing = {
  amount: number
  discount: number
  hasDiscount: boolean
  payable: number
  saved: number
}

const LDXP_TERMINAL_STATUSES = new Set<LdxpTopupStatus>([
  'success',
  'canceled',
  'expired',
  'worker_failed',
  'mail_timeout',
  'verify_failed',
  'redeem_failed',
])

const LDXP_STATUS_MESSAGE_KEYS: Record<LdxpTopupStatus, string> = {
  created: 'Creating payment QR code',
  worker_claimed: 'Creating payment QR code',
  qr_ready: 'Scan with Alipay to pay',
  worker_paid: 'Payment detected, verifying email',
  verified: 'Verifying order',
  redeemed: 'Verifying order',
  success: 'Recharge successful',
  canceled: 'The recharge session was canceled',
  expired: 'Payment expired',
  worker_failed: 'Recharge failed',
  mail_timeout: 'Recharge failed',
  verify_failed: 'Recharge failed',
  redeem_failed: 'Recharge failed',
}

function roundMoney(value: number): number {
  return Math.round((value + Number.EPSILON) * 100) / 100
}

export function getLdxpDiscountForAmount(amount: number): number {
  return LDXP_TOPUP_DISCOUNTS[amount] ?? 1
}

export function getLdxpPricing(amount: number): LdxpPricing {
  const discount = getLdxpDiscountForAmount(amount)
  const payable = roundMoney(amount * discount)
  const saved = roundMoney(amount - payable)

  return {
    amount,
    discount,
    hasDiscount: discount > 0 && discount < 1,
    payable,
    saved,
  }
}

export function getLdxpDiscountLabel(discount: number, locale = 'en'): string {
  if (discount >= 1) {
    return locale.toLowerCase().startsWith('zh') ? '标准价' : 'Standard'
  }

  const percent = Math.round(discount * 100)
  if (locale.toLowerCase().startsWith('zh')) {
    return percent % 10 === 0 ? `${percent / 10}折` : `${percent}折`
  }

  return `${percent}%`
}

const LDXP_SAFE_QR_DATA_PREFIXES = [
  'data:image/png;base64,',
  'data:image/jpeg;base64,',
  'data:image/jpg;base64,',
  'data:image/webp;base64,',
  'data:image/gif;base64,',
]

export function isLdxpTerminalStatus(status: LdxpTopupStatus): boolean {
  return LDXP_TERMINAL_STATUSES.has(status)
}

export function getLdxpStatusMessageKey(status: LdxpTopupStatus): string {
  return LDXP_STATUS_MESSAGE_KEYS[status]
}

export function shouldShowLdxpQrCreationHint(status: LdxpTopupStatus): boolean {
  return status === 'created' || status === 'worker_claimed'
}

export function getSafeLdxpQrCodeSrc(qrCode?: string): string | undefined {
  const src = qrCode?.trim()
  if (!src) {
    return undefined
  }

  if (LDXP_SAFE_QR_DATA_PREFIXES.some((prefix) => src.startsWith(prefix))) {
    return src
  }

  if (src.startsWith('https://')) {
    return src
  }

  return undefined
}
