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
  formatBpsRate,
  formatCny,
  formatPercentRate,
  formatUnixTime,
  getMailStatusLabelKey,
  isMailStatusError,
} from './format'

test('formatCny renders CNY with two decimals', () => {
  assert.equal(formatCny(10), '¥10.00')
  assert.equal(formatCny(10.3), '¥10.30')
  assert.equal(formatCny(425), '¥425.00')
})

test('formatPercentRate renders one decimal place', () => {
  assert.equal(formatPercentRate(0.968), '96.8%')
  assert.equal(formatPercentRate(1), '100.0%')
})

test('mail status labels are stable i18n keys', () => {
  assert.equal(getMailStatusLabelKey('verified'), 'Verified')
  assert.equal(getMailStatusLabelKey('amount_mismatch'), 'Amount mismatch')
  assert.equal(getMailStatusLabelKey('order_mismatch'), 'Order number mismatch')
  assert.equal(getMailStatusLabelKey('waiting_mail'), 'Pending mail')
})

test('unknown mail status labels fall back to raw status', () => {
  assert.equal(getMailStatusLabelKey('custom_unknown'), 'custom_unknown')
})

test('unknown mail status is not treated as an error', () => {
  assert.equal(isMailStatusError('custom_unknown'), false)
})

test('formatUnixTime handles empty and unix seconds', () => {
  assert.equal(formatUnixTime(), '-')
  assert.notEqual(formatUnixTime(1782589062), '-')
  assert.match(formatUnixTime(1782589062), /2026/)
})

test('formatBpsRate renders basis points as percent', () => {
  assert.equal(formatBpsRate(1250), '12.50%')
  assert.equal(formatBpsRate(0), '0.00%')
})
