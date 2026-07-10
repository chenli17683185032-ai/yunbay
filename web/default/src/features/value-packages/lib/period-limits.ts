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
import type {
  ValuePackagePeriodLimit,
  ValuePackageType,
  ValuePackageUsageSummary,
} from '../types'

const LIFECYCLE_DAYS: Record<ValuePackageType, number> = {
  day: 1,
  week: 7,
  month: 30,
}

function legacyRemaining(used: number, limit: number): number {
  return Math.max(0, limit - Math.max(0, used || 0))
}

export function getValuePackagePeriodLimits(
  usage: ValuePackageUsageSummary | null | undefined,
  packageType: ValuePackageType | string | null | undefined
): ValuePackagePeriodLimit[] {
  if (!usage) {
    return []
  }

  if (usage.period_limits !== undefined) {
    return usage.period_limits
  }

  const periods: ValuePackagePeriodLimit[] = [
    {
      kind: 'five_hour',
      label_unit: 'hour',
      label_value: 5,
      limit: usage.limit_5h,
      used: usage.used_5h,
      remaining: legacyRemaining(usage.used_5h, usage.limit_5h),
      percent: usage.percent_5h,
      refreshes: true,
      reset_at: usage.reset_at_5h,
      limited: usage.limited_5h,
    },
  ]

  if (packageType === 'month' && usage.limit_7d > 0) {
    periods.push({
      kind: 'seven_day_stage',
      label_unit: 'day',
      label_value: 7,
      limit: usage.limit_7d,
      used: usage.used_7d,
      remaining: legacyRemaining(usage.used_7d, usage.limit_7d),
      percent: usage.percent_7d,
      refreshes: true,
      reset_at: usage.reset_at_7d,
      limited: usage.limited_7d,
    })
  }

  const lifecycleDays = LIFECYCLE_DAYS[packageType as ValuePackageType]
  if (lifecycleDays && usage.total_limit > 0) {
    periods.push({
      kind: 'lifecycle',
      label_unit: 'day',
      label_value: lifecycleDays,
      limit: usage.total_limit,
      used: usage.total_used,
      remaining: usage.total_remaining,
      percent: usage.total_percent,
      refreshes: false,
      reset_at: 0,
      limited: usage.total_used >= usage.total_limit,
    })
  }

  return periods
}
