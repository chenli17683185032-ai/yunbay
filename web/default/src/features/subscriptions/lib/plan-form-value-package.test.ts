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
import {
  formValuesToPlanPayload,
  planToFormValues,
  PLAN_FORM_DEFAULTS,
} from './plan-form'
import type { SubscriptionPlan } from '../types'

test('value package limit fields convert dollars to quota payload', () => {
  const values = {
    ...PLAN_FORM_DEFAULTS,
    title: '日卡',
    plan_kind: 'value_package' as const,
    package_type: 'day' as const,
    package_level: 1,
    model_group: 'day-card',
    concurrency_limit: 1,
    limit_5h_amount: 100,
    limit_7d_amount: 500,
    ldxp_product_url: 'https://ldxp.example.test/day',
    ldxp_product_name: '日卡商品',
    ldxp_product_amount: 9.9,
  }
  const payload = formValuesToPlanPayload(values)
  assert.equal(payload.plan.plan_kind, 'value_package')
  assert.equal(payload.plan.package_type, 'day')
  assert.equal(payload.plan.model_group, 'day-card')
  assert.equal(payload.plan.concurrency_limit, 1)
  assert.equal(typeof payload.plan.limit_5h_amount, 'number')
  assert.equal(typeof payload.plan.limit_7d_amount, 'number')
  assert.equal(payload.plan.ldxp_product_url, 'https://ldxp.example.test/day')
  assert.equal(payload.plan.allow_balance_pay, false)
  assert.equal(payload.plan.upgrade_group, '')
})

test('planToFormValues preserves per-card ldxp payment config', () => {
  const plan: SubscriptionPlan = {
    id: 1,
    title: '日卡',
    subtitle: '',
    price_amount: 9.9,
    currency: 'USD',
    duration_unit: 'day',
    duration_value: 1,
    quota_reset_period: 'never',
    enabled: true,
    sort_order: 0,
    allow_balance_pay: false,
    max_purchase_per_user: 0,
    total_amount: 0,
    plan_kind: 'value_package',
    package_type: 'day',
    package_level: 1,
    model_group: 'day-card',
    concurrency_limit: 1,
    limit_5h_amount: 100,
    limit_7d_amount: 700,
    ldxp_product_url: 'https://ldxp.example.test/day',
    ldxp_product_name: '日卡商品',
    ldxp_product_amount: 9.9,
  }
  const values = planToFormValues(plan)
  assert.equal(values.ldxp_product_url, 'https://ldxp.example.test/day')
  assert.equal(values.ldxp_product_name, '日卡商品')
  assert.equal(values.ldxp_product_amount, 9.9)
  assert.equal(values.plan_kind, 'value_package')
  assert.equal(values.package_type, 'day')
})
