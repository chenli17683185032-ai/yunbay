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
import assert from 'node:assert/strict'
import test from 'node:test'
import type {
  ValuePackagePlan,
  UserSubscription,
  UserValuePackagePreference,
  ValuePackageState,
} from '../types'
import {
  getPackageCardState,
  getPackageLevelLabel,
  shouldShowPackageGlow,
} from './rules'

const NOW = 1_700_000_000

function createPlan(
  overrides: Partial<ValuePackagePlan> = {}
): ValuePackagePlan {
  return {
    id: 101,
    title: 'Month value package',
    subtitle: '',
    price_amount: 99,
    currency: 'CNY',
    duration_unit: 'month',
    duration_value: 1,
    custom_seconds: 0,
    enabled: true,
    sort_order: 10,
    plan_kind: 'value_package',
    package_type: 'month',
    package_level: 3,
    model_group: 'default',
    concurrency_limit: 1,
    limit_5h_amount: 0,
    limit_7d_amount: 0,
    benefits: '',
    ldxp_product_url: '',
    ldxp_product_name: '',
    ldxp_product_amount: 0,
    ldxp_product_ref: '',
    ldxp_session_ttl_seconds: 1800,
    allow_balance_pay: false,
    stripe_price_id: '',
    creem_product_id: '',
    waffo_pancake_product_id: '',
    max_purchase_per_user: 0,
    upgrade_group: '',
    total_amount: 0,
    quota_reset_period: 'never',
    quota_reset_custom_seconds: 0,
    created_at: 0,
    updated_at: 0,
    ...overrides,
  }
}

function createPreference(
  overrides: Partial<UserValuePackagePreference> = {}
): UserValuePackagePreference {
  return {
    id: 1,
    user_id: 7,
    enabled: true,
    active_user_subscription_id: 501,
    reset_count: 0,
    created_at: 0,
    updated_at: 0,
    ...overrides,
  }
}

function createSubscription(
  overrides: Partial<UserSubscription> = {}
): UserSubscription {
  return {
    id: 501,
    user_id: 7,
    plan_id: 101,
    amount_total: 0,
    amount_used: 0,
    start_time: NOW - 60,
    end_time: NOW + 3600,
    status: 'active',
    source: 'order',
    last_reset_time: 0,
    next_reset_time: 0,
    covered_by_subscription_id: 0,
    covered_time: 0,
    upgrade_group: '',
    prev_user_group: '',
    created_at: 0,
    updated_at: 0,
    ...overrides,
  }
}

function createState(
  overrides: Partial<ValuePackageState> = {}
): ValuePackageState {
  return {
    preference: createPreference(),
    subscription: createSubscription(),
    plan: createPlan(),
    ...overrides,
  }
}

test('unowned package shows purchase for naked plan response records', () => {
  const plan = createPlan()

  assert.deepEqual(getPackageCardState(plan, null, NOW), {
    kind: 'purchase',
  })
})

test('active selected package shows running', () => {
  const plan = createPlan()
  const state = createState()

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'running',
    userSubscriptionId: 501,
  })
})

test('owned but disabled preference shows start', () => {
  const plan = createPlan()
  const state = createState({
    preference: createPreference({ enabled: false }),
  })

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'start',
    userSubscriptionId: 501,
  })
})

test('disabled package card shows disabled when it is unowned', () => {
  const plan = createPlan({ enabled: false })

  assert.deepEqual(getPackageCardState(plan, null, NOW), {
    kind: 'disabled',
    userSubscriptionId: undefined,
  })
})

test('disabled package card shows disabled when it is owned', () => {
  const plan = createPlan({ enabled: false })
  const state = createState({ plan })

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'disabled',
    userSubscriptionId: 501,
  })
})

test('inactive owned subscription shows expired', () => {
  const plan = createPlan()
  const state = createState({
    subscription: createSubscription({ status: 'cancelled' }),
  })

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'expired',
    userSubscriptionId: 501,
  })
})

test('subscription ending exactly at now shows expired', () => {
  const plan = createPlan()
  const state = createState({
    subscription: createSubscription({ end_time: NOW }),
  })

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'expired',
    userSubscriptionId: 501,
  })
})

test('state subscription for another card shows purchase', () => {
  const plan = createPlan({ id: 202 })
  const state = createState()

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'purchase',
  })
})

test('mismatched active subscription id shows purchase', () => {
  const plan = createPlan()
  const state = createState({
    preference: createPreference({ active_user_subscription_id: 999 }),
  })

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'purchase',
  })
})

test('missing active subscription still shows purchase even when preference has an id', () => {
  const plan = createPlan()
  const state = createState({
    subscription: null,
    preference: createPreference({ active_user_subscription_id: 501 }),
  })

  assert.deepEqual(getPackageCardState(plan, state, NOW), {
    kind: 'purchase',
  })
})

test('enabled active package shows glow', () => {
  const state = createState()

  assert.equal(shouldShowPackageGlow(state, NOW), true)
})

test('glow remains visible for an active entitlement even after a plan is disabled', () => {
  const disabledPlan = createPlan({ enabled: false })
  const state = createState({ plan: disabledPlan })

  assert.equal(shouldShowPackageGlow(state, NOW), true)
})

test('glow is hidden when preference is disabled', () => {
  const state = createState({
    preference: createPreference({ enabled: false }),
  })

  assert.equal(shouldShowPackageGlow(state, NOW), false)
})

test('glow is hidden when subscription is expired', () => {
  const state = createState({
    subscription: createSubscription({ end_time: NOW }),
  })

  assert.equal(shouldShowPackageGlow(state, NOW), false)
})

test('glow is hidden when active subscription id does not match state subscription', () => {
  const state = createState({
    preference: createPreference({ active_user_subscription_id: 999 }),
  })

  assert.equal(shouldShowPackageGlow(state, NOW), false)
})

test('glow is hidden when state plan does not match state subscription', () => {
  const state = createState({
    plan: createPlan({ id: 202 }),
  })

  assert.equal(shouldShowPackageGlow(state, NOW), false)
})

test('package level labels map to Chinese names with generic fallback', () => {
  assert.equal(getPackageLevelLabel('day'), 'Day package')
  assert.equal(getPackageLevelLabel('week'), 'Week package')
  assert.equal(getPackageLevelLabel('month'), 'Month package')
  assert.equal(getPackageLevelLabel(''), 'Value Package')
  assert.equal(getPackageLevelLabel(undefined), 'Value Package')
})
