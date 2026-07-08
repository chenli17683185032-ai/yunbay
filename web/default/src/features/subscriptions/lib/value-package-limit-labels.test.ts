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
  getValuePackageTotalLimitDescriptionKey,
  getValuePackageTotalLimitLabelKey,
  shouldExposeValuePackage7dPeriodLimit,
} from './value-package-limit-labels'

test('value package total limit labels describe the full valid period', () => {
  assert.equal(getValuePackageTotalLimitLabelKey('day'), '1-day total limit')
  assert.equal(getValuePackageTotalLimitLabelKey('week'), '7-day total limit')
  assert.equal(getValuePackageTotalLimitLabelKey('month'), '30-day total limit')
})

test('week card total limit description uses the 7-day activation window wording', () => {
  assert.equal(
    getValuePackageTotalLimitDescriptionKey('week'),
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.'
  )
})

test('only month cards expose the optional fixed 7-day period limit', () => {
  assert.equal(shouldExposeValuePackage7dPeriodLimit('day'), false)
  assert.equal(shouldExposeValuePackage7dPeriodLimit('week'), false)
  assert.equal(shouldExposeValuePackage7dPeriodLimit('month'), true)
})
