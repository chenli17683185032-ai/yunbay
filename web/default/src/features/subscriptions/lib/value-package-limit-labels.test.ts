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
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'
import {
  getValuePackageTotalLimitDescriptionKey,
  getValuePackageTotalLimitLabelKey,
  VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY,
  shouldExposeValuePackage7dPeriodLimit,
} from './value-package-limit-labels'

const localeDir = join(import.meta.dirname, '../../../i18n/locales')
const localeNames = ['en', 'zh', 'fr', 'ru', 'ja', 'vi'] as const
const expectedResetConfirmMessageKey =
  "This will consume 1 reset count and clear the current package's used quota. The total quota and expiration time will remain unchanged."
const staleLocaleKeys = [
  'Day cards can use this total quota from activation time until the 1-day expiration. 0 means unlimited total quota.',
  'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.',
  'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.',
  [
    'This will consume 1 reset count and clear your current package',
    "'s ",
    '5-hour and 7-day usage windows. It will not restore total quota or extend expiration.',
  ].join(''),
  'This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.',
  [
    '7-day limit',
    ' in displayed dollars; converted to quota units when saved.',
  ].join(''),
  ['7-day ', 'remaining'].join(''),
  [
    'Realtime 5-hour and 7-day remaining ',
    'quota for active day, week, and month card users.',
  ].join(''),
  [
    'Users who enable day, week, or month cards will appear here with synced 5-hour and 7-day ',
    'usage.',
  ].join(''),
  ['5-hour limit and 7-day limit ', 'protection'].join(''),
]
const requiredTotalDescriptionKeys = [
  'Day cards can use this total quota from activation time until the 1-day expiration. The total quota must be greater than 0.',
  'Week cards can use this total quota from activation time until the 7-day expiration. The total quota must be greater than 0.',
  'Month cards can use this total quota from activation time until the 30-day expiration. The total quota must be greater than 0.',
]

test('value package total limit labels describe the full valid period', () => {
  assert.equal(getValuePackageTotalLimitLabelKey('day'), '1-day total limit')
  assert.equal(getValuePackageTotalLimitLabelKey('week'), '7-day total limit')
  assert.equal(getValuePackageTotalLimitLabelKey('month'), '30-day total limit')
})

test('week card total limit description uses the 7-day activation window wording', () => {
  assert.equal(
    getValuePackageTotalLimitDescriptionKey('week'),
    'Week cards can use this total quota from activation time until the 7-day expiration. The total quota must be greater than 0.'
  )
})

test('only month cards expose the optional fixed 7-day period limit', () => {
  assert.equal(shouldExposeValuePackage7dPeriodLimit('day'), false)
  assert.equal(shouldExposeValuePackage7dPeriodLimit('week'), false)
  assert.equal(shouldExposeValuePackage7dPeriodLimit('month'), true)
})

test('value package reset confirmation describes clearing current used quota only', () => {
  assert.equal(
    VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY,
    expectedResetConfirmMessageKey
  )

  for (const localeName of localeNames) {
    const localePath = join(localeDir, `${localeName}.json`)
    const translation = JSON.parse(readFileSync(localePath, 'utf8'))
      .translation as Record<string, string>

    assert.ok(
      translation[expectedResetConfirmMessageKey],
      `${localeName}: ${expectedResetConfirmMessageKey}`
    )
  }
})

test('value package limit locales do not keep stale rolling-window-era copy', () => {
  assert.ok(!staleLocaleKeys.includes(VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY))

  for (const localeName of localeNames) {
    const localePath = join(localeDir, `${localeName}.json`)
    const translation = JSON.parse(readFileSync(localePath, 'utf8'))
      .translation as Record<string, string>

    for (const staleKey of staleLocaleKeys) {
      assert.equal(
        translation[staleKey],
        undefined,
        `${localeName}: ${staleKey}`
      )
    }
    for (const requiredKey of requiredTotalDescriptionKeys) {
      assert.equal(
        typeof translation[requiredKey],
        'string',
        `${localeName}: ${requiredKey}`
      )
      assert.ok(translation[requiredKey], `${localeName}: ${requiredKey}`)
    }
  }
})
