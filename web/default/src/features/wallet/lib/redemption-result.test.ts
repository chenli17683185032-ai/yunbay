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
import { getRedemptionSuccessMessageKey } from './redemption-result'

test('paid topup cards use top-up success copy when they count as topups', () => {
  assert.equal(
    getRedemptionSuccessMessageKey({
      kind: 'paid_topup',
      count_as_topup: true,
    }),
    'Top-up card redeemed successfully! Added: {{quota}}'
  )
})

test('promo credit codes use bonus quota success copy', () => {
  assert.equal(
    getRedemptionSuccessMessageKey({ kind: 'promo_credit' }),
    'Promo code redeemed successfully! Added bonus quota: {{quota}}'
  )
})

test('legacy or missing redemption metadata uses generic success copy', () => {
  assert.equal(
    getRedemptionSuccessMessageKey({ kind: 'legacy' }),
    'Redemption successful! Added: {{quota}}'
  )
  assert.equal(
    getRedemptionSuccessMessageKey(),
    'Redemption successful! Added: {{quota}}'
  )
})
