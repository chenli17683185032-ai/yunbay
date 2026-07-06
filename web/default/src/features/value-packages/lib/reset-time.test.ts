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
  formatValuePackageResetLine,
  formatValuePackageResetTime,
} from './reset-time'

function t(key: string, values?: Record<string, string | number>): string {
  if (!values) return key
  return Object.entries(values).reduce(
    (text, [name, value]) => text.replaceAll(`{{${name}}}`, String(value)),
    key
  )
}

test('reset time shows fully restored for zero, negative, and NaN values', () => {
  assert.equal(formatValuePackageResetTime(0, t), 'Fully restored')
  assert.equal(formatValuePackageResetTime(-1, t), 'Fully restored')
  assert.equal(formatValuePackageResetTime(Number.NaN, t), 'Fully restored')
})

test('reset time shows less than 1 minute for sub-minute positive values', () => {
  assert.equal(formatValuePackageResetTime(59, t), 'less than 1 minute')
})

test('reset time formats minute values with correct singular and plural forms', () => {
  assert.equal(formatValuePackageResetTime(60, t), '1 minute')
  assert.equal(formatValuePackageResetTime(61, t), '2 minutes')
  assert.equal(formatValuePackageResetTime(120, t), '2 minutes')
})

test('reset time formats hour values with normalized rounded minutes', () => {
  assert.equal(formatValuePackageResetTime(3600, t), '1 hour')
  assert.equal(formatValuePackageResetTime(7200, t), '2 hours')
  assert.equal(formatValuePackageResetTime(3601, t), '1 hour 1 minute')
  assert.equal(formatValuePackageResetTime(7199, t), '2 hours')
})

test('reset time shows hours and minutes for mixed hour values', () => {
  assert.equal(formatValuePackageResetTime(3 * 3600 + 15 * 60, t), '3 hours 15 minutes')
})

test('reset time formats day values with correct singular and plural forms', () => {
  assert.equal(formatValuePackageResetTime(24 * 3600, t), '1 day')
  assert.equal(formatValuePackageResetTime(2 * 24 * 3600, t), '2 days')
  assert.equal(formatValuePackageResetTime(7 * 24 * 3600, t), '7 days')
})

test('reset time formats day values with normalized rounded hours', () => {
  assert.equal(formatValuePackageResetTime(24 * 3600 + 1, t), '1 day 1 hour')
  assert.equal(formatValuePackageResetTime(4 * 24 * 3600 + 6 * 3600 + 30, t), '4 days 7 hours')
  assert.equal(formatValuePackageResetTime(7 * 24 * 3600 - 1, t), '7 days')
})

test('reset line shows unlimited when limit is zero or invalid', () => {
  assert.equal(
    formatValuePackageResetLine({
      limit: 0,
      resetSeconds: 3600,
      limited: true,
      t,
    }),
    'Unlimited'
  )
  assert.equal(
    formatValuePackageResetLine({
      limit: Number.NaN,
      resetSeconds: 3600,
      limited: false,
      t,
    }),
    'Unlimited'
  )
})

test('reset line shows fully restored for positive limit with elapsed reset time', () => {
  assert.equal(
    formatValuePackageResetLine({
      limit: 10,
      resetSeconds: 0,
      limited: false,
      t,
    }),
    'Fully restored'
  )
})

test('reset line shows reset time when package is not limited', () => {
  assert.equal(
    formatValuePackageResetLine({
      limit: 10,
      resetSeconds: 61,
      limited: false,
      t,
    }),
    'Resets in 2 minutes'
  )
})

test('reset line shows limit reached message when package is limited', () => {
  assert.equal(
    formatValuePackageResetLine({
      limit: 10,
      resetSeconds: 4 * 24 * 3600 + 6 * 3600 + 30,
      limited: true,
      t,
    }),
    'Limit reached · Resets in 4 days 7 hours'
  )
})
