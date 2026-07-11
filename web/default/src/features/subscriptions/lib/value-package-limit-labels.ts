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
export type ValuePackageType = 'day' | 'week' | 'month'

export const VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY = '7-day period limit'
export const VALUE_PACKAGE_7D_PERIOD_LIMIT_DESCRIPTION_KEY =
  'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.'
export const VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY =
  "This will consume 1 reset count and clear the current package's used quota. The total quota and expiration time will remain unchanged."

const VALUE_PACKAGE_TOTAL_LIMIT_LABEL_KEYS: Record<ValuePackageType, string> = {
  day: '1-day total limit',
  week: '7-day total limit',
  month: '30-day total limit',
}

const VALUE_PACKAGE_TOTAL_LIMIT_DESCRIPTION_KEYS: Record<
  ValuePackageType,
  string
> = {
  day: 'Day cards can use this total quota from activation time until the 1-day expiration. The total quota must be greater than 0.',
  week: 'Week cards can use this total quota from activation time until the 7-day expiration. The total quota must be greater than 0.',
  month:
    'Month cards can use this total quota from activation time until the 30-day expiration. The total quota must be greater than 0.',
}

export function getValuePackageTotalLimitLabelKey(
  packageType?: string
): string | undefined {
  return VALUE_PACKAGE_TOTAL_LIMIT_LABEL_KEYS[packageType as ValuePackageType]
}

export function getValuePackageTotalLimitDescriptionKey(
  packageType?: string
): string | undefined {
  return VALUE_PACKAGE_TOTAL_LIMIT_DESCRIPTION_KEYS[
    packageType as ValuePackageType
  ]
}

export function shouldExposeValuePackage7dPeriodLimit(
  packageType?: string
): boolean {
  return packageType === 'month'
}
