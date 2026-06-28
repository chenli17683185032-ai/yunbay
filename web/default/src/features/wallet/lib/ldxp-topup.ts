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
  created: 'Creating recharge session...',
  worker_claimed: 'Preparing payment worker...',
  qr_ready: 'Scan with Alipay to pay',
  worker_paid: 'Payment submitted, waiting for verification...',
  verified: 'Payment verified, redeeming...',
  redeemed: 'Recharge credited, finalizing...',
  success: 'Recharge successful',
  canceled: 'Recharge canceled',
  expired: 'Recharge expired',
  worker_failed: 'Payment worker failed',
  mail_timeout: 'Payment verification timed out',
  verify_failed: 'Payment verification failed',
  redeem_failed: 'Recharge redemption failed',
}

export function isLdxpTerminalStatus(status: LdxpTopupStatus): boolean {
  return LDXP_TERMINAL_STATUSES.has(status)
}

export function getLdxpStatusMessageKey(status: LdxpTopupStatus): string {
  return LDXP_STATUS_MESSAGE_KEYS[status]
}
