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
import { formatTimestampToDate } from '@/lib/format'
import type { StatusBadgeProps } from '@/components/status-badge'
import type { BillingRecord, OrderType, TopupStatus } from '../types'

// ============================================================================
// Billing Utility Functions
// ============================================================================

interface StatusConfig {
  variant: StatusBadgeProps['variant']
  label: string
}

/**
 * All topup statuses the backend can persist. Extends the shared
 * TopupStatus union with 'failed' (persisted for failed topups).
 */
export type BillingTopupStatus = TopupStatus | 'failed'

/**
 * Status badge configuration
 */
export const STATUS_CONFIG: Record<BillingTopupStatus, StatusConfig> = {
  success: {
    variant: 'success',
    label: 'Success',
  },
  pending: {
    variant: 'warning',
    label: 'Pending',
  },
  expired: {
    variant: 'danger',
    label: 'Expired',
  },
  failed: {
    variant: 'danger',
    label: 'Failed',
  },
}

/**
 * Get status badge configuration.
 * Unknown statuses fall back to a neutral badge showing the raw status text.
 */
export function getStatusConfig(status: string): StatusConfig {
  return (
    STATUS_CONFIG[status as BillingTopupStatus] || {
      variant: 'neutral',
      label: status || 'Unknown',
    }
  )
}

export function getOrderTypeLabel(
  orderType: string | undefined,
  t?: (key: string) => string
): string {
  const labels: Record<OrderType, string> = {
    topup: 'Top-up',
    subscription: 'Subscription',
  }
  const label =
    orderType === 'topup' || orderType === 'subscription'
      ? labels[orderType]
      : orderType || 'Top-up'
  return t ? t(label) : label
}

export function getPlanSummary(
  record: Partial<BillingRecord>,
  t?: (key: string) => string
): string {
  if (!('order_type' in record) || record.order_type !== 'subscription') {
    return ''
  }
  const title = record.plan_title || (t ? t('Deleted plan') : 'Deleted plan')
  const value = record.duration_value || 0
  const unit = record.duration_unit || ''
  if (!value || !unit) return title
  const unitLabel = t ? t(unit) : unit
  return `${title} · ${value} ${unitLabel}`
}

/**
 * Payment method display names
 */
export const PAYMENT_METHOD_NAMES: Record<string, string> = {
  stripe: 'Stripe',
  alipay: 'Alipay',
  wxpay: 'WeChat Pay',
  waffo: 'Waffo',
  waffo_pancake: 'Waffo Pancake',
  balance: 'Balance',
  creem: 'Creem',
}

/**
 * Get payment method display name
 */
export function getPaymentMethodName(
  method: string,
  t?: (key: string) => string
): string {
  const name = PAYMENT_METHOD_NAMES[method] || method
  return t ? t(name) : name
}

/**
 * Format timestamp to readable date string
 */
export function formatTimestamp(timestamp: number): string {
  return formatTimestampToDate(timestamp)
}
