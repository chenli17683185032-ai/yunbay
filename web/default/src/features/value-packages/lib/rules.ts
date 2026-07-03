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
import type { ValuePackagePlan, ValuePackageState } from '../types'

export type ValuePackageCardStateKind =
  | 'purchase'
  | 'start'
  | 'running'
  | 'expired'
  | 'disabled'

function currentUnixSeconds(): number {
  return Math.floor(Date.now() / 1000)
}

export function getPackageLevelLabel(type?: string): string {
  switch ((type || '').trim()) {
    case 'day':
      return '日卡'
    case 'week':
      return '周卡'
    case 'month':
      return '月卡'
    default:
      return '套餐'
  }
}

function isCurrentValuePackageState(
  state: ValuePackageState | null,
  now: number
): boolean {
  const subscription = state?.subscription
  const statePlan = state?.plan
  const activeSubscriptionId =
    state?.preference.active_user_subscription_id || 0

  return Boolean(
    state?.preference.enabled &&
    subscription &&
    statePlan &&
    activeSubscriptionId === subscription.id &&
    subscription.plan_id === statePlan.id &&
    subscription.status === 'active' &&
    subscription.end_time > now
  )
}

export function shouldShowPackageGlow(
  state: ValuePackageState | null,
  now = currentUnixSeconds()
): boolean {
  return isCurrentValuePackageState(state, now)
}

export function getPackageCardState(
  plan: ValuePackagePlan,
  state: ValuePackageState | null,
  now = currentUnixSeconds()
): {
  kind: ValuePackageCardStateKind
  userSubscriptionId?: number
} {
  const subscription = state?.subscription
  const statePlan = state?.plan
  const activeSubscriptionId =
    state?.preference.active_user_subscription_id || 0
  const userSubscriptionId =
    subscription?.id || activeSubscriptionId || undefined
  const isOwned = Boolean(
    subscription &&
    statePlan &&
    subscription.plan_id === plan.id &&
    statePlan.id === plan.id &&
    activeSubscriptionId === subscription.id
  )

  if (!plan.enabled) {
    return {
      kind: 'disabled',
      userSubscriptionId: isOwned ? userSubscriptionId : undefined,
    }
  }

  if (!isOwned || !subscription) {
    return { kind: 'purchase' }
  }

  if (subscription.status !== 'active' || subscription.end_time <= now) {
    return { kind: 'expired', userSubscriptionId }
  }

  if (state?.preference.enabled) {
    return { kind: 'running', userSubscriptionId }
  }

  return { kind: 'start', userSubscriptionId }
}
