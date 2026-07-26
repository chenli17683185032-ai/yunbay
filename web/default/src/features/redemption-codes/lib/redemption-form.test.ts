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
import type { TFunction } from 'i18next'
import assert from 'node:assert/strict'
import test from 'node:test'
import type { Redemption } from '../types'
import {
  getRedemptionFormSchema,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
  type RedemptionFormValues,
} from './redemption-form'

const t = ((key: string) => key) as TFunction

const validBase: RedemptionFormValues = {
  name: 'batch',
  type: 'quota',
  plan_id: 0,
  reset_card_count: 1,
  quota_dollars: 10,
  expired_time: undefined,
  count: 1,
  kind: 'promo_credit',
  amount: 0,
  money: 0,
  count_as_topup: false,
  batch_id: '',
  source: 'promo',
}

test('paid_topup keeps billing metadata in create payload', () => {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    kind: 'paid_topup',
    quota_dollars: 20,
    amount: 100,
    money: 88,
    count_as_topup: true,
    source: 'ldxp',
  })

  assert.equal(result.success, true)
  if (!result.success) throw new Error('expected paid_topup form to be valid')

  const payload = transformFormDataToPayload(result.data)
  assert.equal(payload.kind, 'paid_topup')
  assert.equal(payload.amount, 100)
  assert.equal(payload.money, 88)
  assert.equal(payload.count_as_topup, true)
  assert.equal(payload.source, 'ldxp')
})

test('promo_credit cannot count as paid top-up', () => {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    kind: 'promo_credit',
    amount: 0,
    money: 0,
    count_as_topup: true,
  })

  assert.equal(result.success, false)
})

test('rejects zero quota', () => {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    quota_dollars: 0,
  })

  assert.equal(result.success, false)
})

test('rejects fractional face amount', () => {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    kind: 'paid_topup',
    quota_dollars: 20,
    amount: 10.5,
    money: 10.5,
    count_as_topup: true,
    source: 'ldxp',
  })

  assert.equal(result.success, false)
})

test('edit defaults preserve type metadata from API data', () => {
  const defaults = transformRedemptionToFormDefaults({
    id: 1,
    user_id: 2,
    name: 'existing',
    key: 'abc',
    status: 1,
    quota: 500000,
    type: 'quota',
    plan_id: 0,
    created_time: 0,
    redeemed_time: 0,
    expired_time: 0,
    used_user_id: 0,
    kind: 'paid_topup',
    amount: 100,
    money: 88,
    count_as_topup: true,
    batch_id: 'batch-1',
    source: 'ldxp',
    exported_time: 0,
    reset_card_count: 0,
  } satisfies Redemption)

  assert.equal(defaults.kind, 'paid_topup')
  assert.equal(defaults.amount, 100)
  assert.equal(defaults.money, 88)
  assert.equal(defaults.count_as_topup, true)
  assert.equal(defaults.batch_id, 'batch-1')
  assert.equal(defaults.source, 'ldxp')
})

test('subscription value package payload keeps selected plan and zero quota', () => {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    type: 'subscription',
    plan_id: 42,
    quota_dollars: 0,
    kind: 'coupon',
    amount: 0,
    money: 0,
    count_as_topup: false,
  })

  assert.equal(result.success, true)
  if (!result.success) throw new Error('expected subscription form to be valid')

  const payload = transformFormDataToPayload(result.data)
  assert.equal(payload.type, 'subscription')
  assert.equal(payload.plan_id, 42)
  assert.equal(payload.quota, 0)
})

test('reset_card schema requires a count between 1 and 100', () => {
  const schema = getRedemptionFormSchema(t)
  const ok = schema.safeParse({
    ...validBase,
    type: 'reset_card',
    quota_dollars: 0,
    reset_card_count: 3,
  })
  assert.equal(ok.success, true)

  for (const count of [0, 101]) {
    const bad = schema.safeParse({
      ...validBase,
      type: 'reset_card',
      quota_dollars: 0,
      reset_card_count: count,
    })
    assert.equal(bad.success, false, `count=${count} should fail`)
  }
})

test('reset_card payload zeroes quota and plan and keeps card count', () => {
  const payload = transformFormDataToPayload({
    ...validBase,
    type: 'reset_card',
    plan_id: 7,
    quota_dollars: 10,
    reset_card_count: 5,
  })
  assert.equal(payload.type, 'reset_card')
  assert.equal(payload.quota, 0)
  assert.equal(payload.plan_id, 0)
  assert.equal(payload.reset_card_count, 5)

  const quotaPayload = transformFormDataToPayload({ ...validBase })
  assert.equal(quotaPayload.reset_card_count, 0)
})

test('reset_card edit defaults keep type and count', () => {
  const defaults = transformRedemptionToFormDefaults({
    id: 2,
    user_id: 1,
    name: 'reset',
    key: 'def',
    status: 1,
    quota: 0,
    type: 'reset_card',
    plan_id: 0,
    created_time: 0,
    redeemed_time: 0,
    expired_time: 0,
    used_user_id: 0,
    kind: 'promo_credit',
    amount: 0,
    money: 0,
    count_as_topup: false,
    batch_id: 'batch-2',
    source: 'promo',
    exported_time: 0,
    reset_card_count: 4,
  } satisfies Redemption)

  assert.equal(defaults.type, 'reset_card')
  assert.equal(defaults.reset_card_count, 4)
  assert.equal(defaults.quota_dollars, 0)
})
