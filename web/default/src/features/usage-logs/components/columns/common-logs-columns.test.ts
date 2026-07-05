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
import { getGroupRatioText } from './common-logs-columns'

test('common log token metadata displays Package 1x when subscription ratio is applied', () => {
  assert.equal(
    getGroupRatioText({
      group_ratio: 1,
      user_group_ratio: -1,
      subscription_ratio_applied: true,
    }),
    'Package 1x'
  )
})

test('common log token metadata uses provided package label', () => {
  assert.equal(
    getGroupRatioText(
      {
        group_ratio: 1,
        user_group_ratio: -1,
        subscription_ratio_applied: true,
      },
      '套餐'
    ),
    '套餐 1x'
  )
})

test('common log token metadata still hides plain group ratio 1x without package billing', () => {
  assert.equal(
    getGroupRatioText({
      group_ratio: 1,
      user_group_ratio: -1,
    }),
    null
  )
})
