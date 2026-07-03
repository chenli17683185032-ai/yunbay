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
import type { ValuePackagePlanRecord, ValuePackageState } from '../types'

export type ValuePackageCardStateKind =
  | 'purchase'
  | 'start'
  | 'running'
  | 'expired'
  | 'disabled'

export function getPackageLevelLabel(type?: string): string {
  switch ((type || '').trim()) {
    case 'day':
      return '日卡'
    case 'week':
      return '周卡'
    case 'month':
      return '月卡'
    default:
      return ''
  }
}

export function shouldShowPackageGlow(
  state: ValuePackageState | null
): boolean {
  const now = Math.floor(Date.now() / 1000)
  return Boolean(
    state?.preference.enabled &&
      state.subscription &&
      state.plan?.enabled &&
      state.subscription.status === 'active' &&
      state.subscription.end_time > now
  )
}

export function getPackageCardState(
  record: ValuePackagePlanRecord,
  state: ValuePackageState | null
): {
  kind: ValuePackageCardStateKind
  userSubscriptionId?: number
} {
  const now = Math.floor(Date.now() / 1000)
  const subscription = state?.subscription
  const activeSubscriptionId = state?.preference.active_user_subscription_id || 0
  const userSubscriptionId = subscription?.id || activeSubscriptionId || undefined
  const isOwned =
    !!subscription &&
    subscription.plan_id === record.plan.id &&
    activeSubscriptionId === subscription.id

  if (!isOwned) {
    return { kind: 'purchase' }
  }

  if (!record.plan.enabled) {
    return { kind: 'disabled', userSubscriptionId }
  }

  if (subscription.status !== 'active' || subscription.end_time <= now) {
    return { kind: 'expired', userSubscriptionId }
  }

  if (state?.preference.enabled) {
    return { kind: 'running', userSubscriptionId }
  }

  return { kind: 'start', userSubscriptionId }
}
