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
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Progress } from '@/components/ui/progress'
import { formatValuePackageResetLine } from '../lib/reset-time'
import type { ValuePackagePeriodLimit } from '../types'

interface ValuePackagePeriodListProps {
  periods: ValuePackagePeriodLimit[]
}

const LIFECYCLE_LABEL_KEYS: Record<number, string> = {
  1: '1-day total remaining',
  7: '7-day total remaining',
  30: '30-day total remaining',
}

function getPeriodLabelKey(period: ValuePackagePeriodLimit): string {
  if (period.kind === 'five_hour') {
    return '5-hour remaining'
  }

  if (period.kind === 'seven_day_stage') {
    return 'Current 7-day stage remaining'
  }

  return (
    LIFECYCLE_LABEL_KEYS[period.label_value] ||
    `${period.label_value}-day total remaining`
  )
}

function clampPercent(percent: number): number {
  if (!Number.isFinite(percent)) {
    return 0
  }

  return Math.min(Math.max(percent, 0), 100)
}

function getResetSeconds(resetAt: number): number {
  if (!Number.isFinite(resetAt)) {
    return 0
  }

  return Math.max(0, resetAt - Math.floor(Date.now() / 1_000))
}

export function ValuePackagePeriodList({
  periods,
}: ValuePackagePeriodListProps) {
  const { t } = useTranslation()

  if (periods.length === 0) {
    return <span className='text-muted-foreground text-sm'>{t('No data')}</span>
  }

  return (
    <div className='flex min-w-[180px] flex-col gap-3'>
      {periods.map((period) => {
        const resetLine = period.refreshes
          ? formatValuePackageResetLine({
              limit: period.limit,
              resetSeconds: getResetSeconds(period.reset_at),
              limited: period.limited,
              t,
            })
          : t('Does not refresh')

        return (
          <div
            key={`${period.kind}-${period.label_value}`}
            className='flex flex-col gap-1.5'
          >
            <div className='flex items-center justify-between gap-3 text-xs'>
              <span className='text-muted-foreground font-medium'>
                {t(getPeriodLabelKey(period))}
              </span>
              <span className='font-semibold tabular-nums'>
                {formatQuota(period.remaining)} / {formatQuota(period.limit)}
              </span>
            </div>
            <Progress value={clampPercent(period.percent)} className='h-1.5' />
            <div className='text-muted-foreground text-xs tabular-nums'>
              {t('Used: {{used}} / {{limit}}', {
                used: formatQuota(period.used),
                limit: formatQuota(period.limit),
              })}
            </div>
            <div
              className={cn(
                'text-muted-foreground text-xs',
                period.limited ? 'text-destructive font-medium' : null
              )}
            >
              {resetLine}
            </div>
          </div>
        )
      })}
    </div>
  )
}
