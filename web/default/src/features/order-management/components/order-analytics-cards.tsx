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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCny, formatPercentRate } from '../lib/format'
import type { OrderManagementSummary } from '../types'

interface OrderAnalyticsCardsProps {
  summary?: OrderManagementSummary
  isLoading?: boolean
}

function MetricCard({
  title,
  description,
  value,
  isLoading,
}: {
  title: string
  description: string
  value: string
  isLoading?: boolean
}) {
  return (
    <Card size='sm'>
      <CardHeader>
        <CardDescription>{title}</CardDescription>
        <CardTitle>{description}</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-8 w-28' />
        ) : (
          <div className='text-2xl font-semibold tracking-tight tabular-nums'>
            {value}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function OrderAnalyticsCards({
  summary,
  isLoading,
}: OrderAnalyticsCardsProps) {
  const { t } = useTranslation()
  const safeSummary = summary ?? {
    site_amount: 0,
    external_paid_amount: 0,
    order_count: 0,
    mail_verified_count: 0,
    mail_pending_count: 0,
    mail_error_count: 0,
    mail_verified_rate: 0,
    affiliate_user_count: 0,
    affiliate_amount: 0,
    withdrawal_pending_count: 0,
    withdrawal_pending_amount: 0,
  }

  return (
    <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-5'>
      <MetricCard
        title={t('Revenue amount')}
        description={t('Order analytics')}
        value={formatCny(safeSummary.site_amount)}
        isLoading={isLoading}
      />
      <MetricCard
        title={t('30-day revenue')}
        description={t('Revenue amount')}
        value={formatCny(safeSummary.site_amount)}
        isLoading={isLoading}
      />
      <MetricCard
        title={t('External paid amount')}
        description={`${safeSummary.order_count} ${t('Orders')}`}
        value={formatCny(safeSummary.external_paid_amount)}
        isLoading={isLoading}
      />
      <MetricCard
        title={t('Mail verification')}
        description={`${safeSummary.mail_verified_count}/${safeSummary.order_count}`}
        value={formatPercentRate(safeSummary.mail_verified_rate)}
        isLoading={isLoading}
      />
      <MetricCard
        title={t('Pending verification')}
        description={t('Pending mail')}
        value={String(safeSummary.mail_pending_count + safeSummary.mail_error_count)}
        isLoading={isLoading}
      />
    </div>
  )
}
