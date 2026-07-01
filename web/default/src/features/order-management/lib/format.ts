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
import type { MailCheckStatus } from '../types'

const mailStatusLabelKeys: Record<MailCheckStatus, string> = {
  not_required: 'Not required',
  pending: 'Pending verification',
  waiting_mail: 'Pending mail',
  checking: 'Checking...',
  verified: 'Verified',
  order_mismatch: 'Order number mismatch',
  amount_mismatch: 'Amount mismatch',
  mail_parse_failed: 'Mail parse failed',
  mail_fetch_failed: 'Mail fetch failed',
  timeout: 'Verification timeout',
}

const mailStatusErrorSet = new Set<MailCheckStatus>([
  'order_mismatch',
  'amount_mismatch',
  'mail_parse_failed',
  'mail_fetch_failed',
  'timeout',
])

export function formatCny(value: number): string {
  const amount = Number.isFinite(value) ? value : 0
  return `¥${amount.toFixed(2)}`
}

export function formatPercentRate(value: number): string {
  const rate = Number.isFinite(value) ? value * 100 : 0
  return `${rate.toFixed(1)}%`
}

export function formatUnixTime(value?: number): string {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '-'
  }

  return new Date(value * 1000).toLocaleString()
}

export function getMailStatusLabelKey(
  status: MailCheckStatus | string
): string {
  return mailStatusLabelKeys[status as MailCheckStatus] ?? status
}

export function isMailStatusError(status: MailCheckStatus | string): boolean {
  return mailStatusErrorSet.has(status as MailCheckStatus)
}

export function formatBpsRate(value: number): string {
  const rate = Number.isFinite(value) ? value / 100 : 0
  return `${rate.toFixed(2)}%`
}
