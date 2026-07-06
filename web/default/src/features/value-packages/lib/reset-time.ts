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
export type Translate = (
  key: string,
  values?: Record<string, string | number>
) => string

const MINUTE_SECONDS = 60
const HOUR_SECONDS = 60 * MINUTE_SECONDS

function isPositiveFiniteNumber(value: number | null | undefined): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

export function formatValuePackageResetTime(
  seconds: number | null | undefined,
  t: Translate
): string {
  if (!isPositiveFiniteNumber(seconds)) {
    return t('Fully restored')
  }

  if (seconds < MINUTE_SECONDS) {
    return t('less than 1 minute')
  }

  const totalMinutes = Math.ceil(seconds / MINUTE_SECONDS)
  if (totalMinutes < 60) {
    return formatMinutes(totalMinutes, t)
  }

  if (totalMinutes < 24 * 60) {
    return formatHoursAndMinutes(
      Math.floor(totalMinutes / 60),
      totalMinutes % 60,
      t
    )
  }

  const totalHours = Math.ceil(seconds / HOUR_SECONDS)
  return formatDaysAndHours(
    Math.floor(totalHours / 24),
    totalHours % 24,
    t
  )
}

function formatMinutes(minutes: number, t: Translate): string {
  if (minutes === 1) {
    return t('1 minute')
  }
  return t('{{count}} minutes', { count: minutes })
}

function formatHours(hours: number, t: Translate): string {
  if (hours === 1) {
    return t('1 hour')
  }
  return t('{{count}} hours', { count: hours })
}

function formatHoursAndMinutes(
  hours: number,
  minutes: number,
  t: Translate
): string {
  if (minutes === 0) {
    return formatHours(hours, t)
  }
  if (hours === 1 && minutes === 1) {
    return t('1 hour 1 minute')
  }
  if (hours === 1) {
    return t('1 hour {{minutes}} minutes', { minutes })
  }
  if (minutes === 1) {
    return t('{{hours}} hours 1 minute', { hours })
  }
  return t('{{hours}} hours {{minutes}} minutes', { hours, minutes })
}

function formatDays(days: number, t: Translate): string {
  if (days === 1) {
    return t('1 day')
  }
  return t('{{count}} days', { count: days })
}

function formatDaysAndHours(
  days: number,
  hours: number,
  t: Translate
): string {
  if (hours === 0) {
    return formatDays(days, t)
  }
  if (days === 1 && hours === 1) {
    return t('1 day 1 hour')
  }
  if (days === 1) {
    return t('1 day {{hours}} hours', { hours })
  }
  if (hours === 1) {
    return t('{{days}} days 1 hour', { days })
  }
  return t('{{days}} days {{hours}} hours', { days, hours })
}

export function formatValuePackageResetLine(args: {
  limit: number | null | undefined
  resetSeconds: number | null | undefined
  limited: boolean | null | undefined
  t: Translate
}): string {
  const { limit, resetSeconds, limited, t } = args

  if (!isPositiveFiniteNumber(limit)) {
    return t('Unlimited')
  }

  if (!isPositiveFiniteNumber(resetSeconds)) {
    return t('Fully restored')
  }

  const resetLine = t('Resets in {{time}}', {
    time: formatValuePackageResetTime(resetSeconds, t),
  })

  if (limited) {
    return t('Limit reached · {{reset}}', { reset: resetLine })
  }

  return resetLine
}
