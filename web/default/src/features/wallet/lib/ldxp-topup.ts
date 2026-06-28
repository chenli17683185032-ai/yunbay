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

export function isLdxpTerminalStatus(status: LdxpTopupStatus): boolean {
  return LDXP_TERMINAL_STATUSES.has(status)
}

export function getLdxpStatusMessageKey(status: LdxpTopupStatus): string {
  return LDXP_STATUS_MESSAGE_KEYS[status]
}
