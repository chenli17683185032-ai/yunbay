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
import type { Redemption } from '../types'
import {
  getRedemptionFormSchema,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
  type RedemptionFormValues,
} from './redemption-form'

function assert(condition: unknown, message: string): asserts condition {
  if (!condition) {
    throw new Error(message)
  }
}

const t = ((key: string) => key) as TFunction

const validBase: RedemptionFormValues = {
  name: 'batch',
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

function expectValidPaidTopupPayload() {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    kind: 'paid_topup',
    quota_dollars: 20,
    amount: 100,
    money: 88,
    count_as_topup: true,
    source: 'liandong',
  })

  assert(
    result.success,
    'paid_topup with positive quota, amount, money, and top-up accounting should be valid'
  )

  const payload = transformFormDataToPayload(result.data)
  assert(payload.kind === 'paid_topup', 'payload should keep kind')
  assert(payload.amount === 100, 'payload should keep amount')
  assert(payload.money === 88, 'payload should keep money')
  assert(payload.count_as_topup === true, 'payload should keep count_as_topup')
  assert(payload.source === 'liandong', 'payload should keep source')
}

function expectPromoCreditCannotCountAsTopup() {
  const schema = getRedemptionFormSchema(t)
  const result = schema.safeParse({
    ...validBase,
    kind: 'promo_credit',
    amount: 0,
    money: 0,
    count_as_topup: true,
  })

  assert(!result.success, 'promo_credit should reject count_as_topup=true')
}

function expectEditDefaultsPreserveTypeMetadata() {
  const defaults = transformRedemptionToFormDefaults({
    id: 1,
    user_id: 2,
    name: 'existing',
    key: 'abc',
    status: 1,
    quota: 500000,
    created_time: 0,
    redeemed_time: 0,
    expired_time: 0,
    used_user_id: 0,
    kind: 'paid_topup',
    amount: 100,
    money: 88,
    count_as_topup: true,
    batch_id: 'batch-1',
    source: 'liandong',
    exported_time: 0,
  } satisfies Redemption)

  assert(defaults.kind === 'paid_topup', 'edit defaults should keep kind')
  assert(defaults.amount === 100, 'edit defaults should keep amount')
  assert(defaults.money === 88, 'edit defaults should keep money')
  assert(
    defaults.count_as_topup === true,
    'edit defaults should keep count_as_topup'
  )
  assert(defaults.batch_id === 'batch-1', 'edit defaults should keep batch_id')
  assert(defaults.source === 'liandong', 'edit defaults should keep source')
}

expectValidPaidTopupPayload()
expectPromoCreditCannotCountAsTopup()
expectEditDefaultsPreserveTypeMetadata()
