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
const DAY_SECONDS = 24 * HOUR_SECONDS

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

  if (seconds >= DAY_SECONDS) {
    const days = Math.floor(seconds / DAY_SECONDS)
    const remainingSeconds = seconds - days * DAY_SECONDS

    if (remainingSeconds === 0) {
      return t('{{count}} day', { count: days })
    }

    return t('{{days}} days {{hours}} hours', {
      days,
      hours: Math.floor(remainingSeconds / HOUR_SECONDS),
    })
  }

  if (seconds >= HOUR_SECONDS) {
    const hours = Math.floor(seconds / HOUR_SECONDS)
    const remainingSeconds = seconds - hours * HOUR_SECONDS

    if (remainingSeconds === 0) {
      return t('{{count}} hour', { count: hours })
    }

    return t('{{hours}} hours {{minutes}} minutes', {
      hours,
      minutes: Math.ceil(remainingSeconds / MINUTE_SECONDS),
    })
  }

  return t('{{count}} minutes', {
    count: Math.ceil(seconds / MINUTE_SECONDS),
  })
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
