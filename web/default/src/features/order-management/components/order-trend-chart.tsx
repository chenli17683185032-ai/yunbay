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
import { useMemo } from 'react'
import { VChart } from '@visactor/react-vchart'
import { useTranslation } from 'react-i18next'
import { VCHART_OPTION } from '@/lib/vchart'
import { useTheme } from '@/context/theme-provider'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { OrderDailyPoint } from '../types'

interface OrderTrendChartProps {
  daily: OrderDailyPoint[]
  isLoading?: boolean
}

export function OrderTrendChart({ daily, isLoading }: OrderTrendChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()

  const spec = useMemo(() => {
    const data = daily.flatMap((point) => [
      {
        date: point.date,
        type: t('Revenue amount'),
        value: point.site_amount,
      },
      {
        date: point.date,
        type: t('External paid amount'),
        value: point.external_paid_amount,
      },
    ])

    return {
      type: 'bar',
      data: [{ id: 'revenue', values: data }],
      xField: ['date', 'type'],
      yField: 'value',
      seriesField: 'type',
      legends: { visible: true, orient: 'bottom' },
      axes: [
        { orient: 'bottom', type: 'band' },
        { orient: 'left', type: 'linear' },
      ],
      tooltip: { visible: true },
      background: 'transparent',
      theme: resolvedTheme === 'dark' ? 'dark' : 'light',
    }
  }, [daily, resolvedTheme, t])

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Order analytics')}</CardTitle>
        <CardDescription>
          {t('Revenue amount')} / {t('External paid amount')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className='h-[260px] sm:h-[320px]'>
          {isLoading ? (
            <Skeleton className='size-full' />
          ) : (
            <VChart spec={spec} option={VCHART_OPTION} />
          )}
        </div>
      </CardContent>
    </Card>
  )
}
