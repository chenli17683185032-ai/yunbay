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
  checkinDisplayAmountToQuotaUnits,
  checkinQuotaUnitsToDisplayAmount,
  normalizeCheckinQuotaUnits,
} from './checkin-quota'

test('check-in display amount saves as quota units', () => {
  assert.equal(checkinDisplayAmountToQuotaUnits(0), 0)
  assert.equal(checkinDisplayAmountToQuotaUnits(1), 500000)
})

test('check-in quota displays as configured amount', () => {
  assert.equal(checkinQuotaUnitsToDisplayAmount(0), 0)
  assert.equal(checkinQuotaUnitsToDisplayAmount(500000), 1)
})

test('legacy tiny check-in quota values are treated as display amounts', () => {
  assert.equal(normalizeCheckinQuotaUnits(1), 500000)
  assert.equal(normalizeCheckinQuotaUnits(5), 2500000)
})
