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
import type { ValuePackagePlanRecord, ValuePackageState } from '../types'
import {
  getPackageCardState,
  getPackageLevelLabel,
  shouldShowPackageGlow,
} from './rules'

function createRecord(overrides: Partial<ValuePackagePlanRecord> = {}): ValuePackagePlanRecord {
  return {
    plan: {
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
    },
    ...overrides,
  }
}

function createState(overrides: Partial<ValuePackageState> = {}): ValuePackageState {
  const now = Math.floor(Date.now() / 1000)
  return {
    preference: {
      id: 1,
      user_id: 7,
      enabled: true,
      active_user_subscription_id: 501,
      created_at: 0,
      updated_at: 0,
    },
    subscription: {
      id: 501,
      user_id: 7,
      plan_id: 101,
      amount_total: 0,
      amount_used: 0,
      start_time: now - 60,
      end_time: now + 3600,
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
    },
    plan: createRecord().plan,
    ...overrides,
  }
}

test('unowned package shows purchase', () => {
  const record = createRecord()

  assert.deepEqual(getPackageCardState(record, null), {
    kind: 'purchase',
  })
})

test('active selected package shows running', () => {
  const record = createRecord()
  const state = createState()

  assert.deepEqual(getPackageCardState(record, state), {
    kind: 'running',
    userSubscriptionId: 501,
  })
})

test('enabled active package shows glow', () => {
  const state = createState()

  assert.equal(shouldShowPackageGlow(state), true)
})

test('owned but disabled preference shows start', () => {
  const record = createRecord()
  const state = createState({
    preference: {
      id: 1,
      user_id: 7,
      enabled: false,
      active_user_subscription_id: 501,
      created_at: 0,
      updated_at: 0,
    },
  })

  assert.deepEqual(getPackageCardState(record, state), {
    kind: 'start',
    userSubscriptionId: 501,
  })
})

test('package level labels map to Chinese names', () => {
  assert.equal(getPackageLevelLabel('day'), '日卡')
  assert.equal(getPackageLevelLabel('week'), '周卡')
  assert.equal(getPackageLevelLabel('month'), '月卡')
})
