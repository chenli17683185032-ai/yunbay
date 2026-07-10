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
import { getValuePackagePeriodLimits } from './period-limits'

const legacyUsage = {
  total_used: 200,
  total_limit: 1_000,
  total_remaining: 800,
  total_percent: 20,
  used_5h: 100,
  limit_5h: 500,
  percent_5h: 20,
  reset_at_5h: 1_000,
  reset_seconds_5h: 300,
  limited_5h: false,
  used_7d: 150,
  limit_7d: 750,
  percent_7d: 20,
  reset_at_7d: 2_000,
  reset_seconds_7d: 600,
  limited_7d: false,
  exhausted: false,
  exhausted_reason: '',
  exhausted_message: '',
}

test('returns structured week periods unchanged and preserves their semantics', () => {
  const periodLimits = [
    {
      kind: 'five_hour' as const,
      label_unit: 'hour' as const,
      label_value: 5,
      limit: 900,
      used: 100,
      remaining: 800,
      percent: 100 / 9,
      refreshes: true,
      reset_at: 1_234,
      limited: false,
    },
    {
      kind: 'lifecycle' as const,
      label_unit: 'day' as const,
      label_value: 7,
      limit: 5_000,
      used: 1_000,
      remaining: 4_000,
      percent: 20,
      refreshes: false,
      reset_at: 0,
      limited: false,
    },
  ]
  const result = getValuePackagePeriodLimits(
    { ...legacyUsage, period_limits: periodLimits },
    'week'
  )

  assert.strictEqual(result, periodLimits)
  assert.deepEqual(
    result.map(({ kind, label_value, refreshes, remaining }) => ({
      kind,
      label_value,
      refreshes,
      remaining,
    })),
    [
      {
        kind: 'five_hour',
        label_value: 5,
        refreshes: true,
        remaining: 800,
      },
      {
        kind: 'lifecycle',
        label_value: 7,
        refreshes: false,
        remaining: 4_000,
      },
    ]
  )
})

test('does not invent a legacy lifecycle period when usage total_limit is zero', () => {
  const result = getValuePackagePeriodLimits(
    {
      ...legacyUsage,
      total_limit: 0,
      total_remaining: 0,
      total_percent: 0,
    },
    'week'
  )

  assert.deepEqual(
    result.map((period) => period.kind),
    ['five_hour']
  )
})

test('builds legacy day, week, and month periods in lifecycle order', () => {
  const dayPeriods = getValuePackagePeriodLimits(legacyUsage, 'day')
  const weekPeriods = getValuePackagePeriodLimits(legacyUsage, 'week')
  const monthPeriods = getValuePackagePeriodLimits(legacyUsage, 'month')

  assert.deepEqual(
    dayPeriods.map((period) => period.kind),
    ['five_hour', 'lifecycle']
  )
  assert.deepEqual(
    weekPeriods.map((period) => period.kind),
    ['five_hour', 'lifecycle']
  )
  assert.deepEqual(
    monthPeriods.map((period) => period.kind),
    ['five_hour', 'seven_day_stage', 'lifecycle']
  )
  assert.equal(dayPeriods.at(-1)?.label_value, 1)
  assert.equal(weekPeriods.at(-1)?.label_value, 7)
  assert.equal(monthPeriods.at(-1)?.label_value, 30)
})

test('keeps a structured zero-limit lifecycle period instead of dropping it', () => {
  const periodLimits = [
    {
      kind: 'lifecycle' as const,
      label_unit: 'day' as const,
      label_value: 7,
      limit: 0,
      used: 0,
      remaining: 0,
      percent: 0,
      refreshes: false,
      reset_at: 0,
      limited: false,
    },
  ]

  assert.strictEqual(
    getValuePackagePeriodLimits(
      { ...legacyUsage, period_limits: periodLimits },
      'week'
    ),
    periodLimits
  )
})
