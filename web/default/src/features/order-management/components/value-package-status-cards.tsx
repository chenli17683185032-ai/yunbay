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
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { VALUE_PACKAGE_TYPES } from '@/features/subscriptions/constants'
import type {
  OrderManagementValuePackagePlanRecord,
  SubscriptionPlanStats,
} from '../types'

interface ValuePackageStatusCardsProps {
  records?: OrderManagementValuePackagePlanRecord[]
  isLoading?: boolean
}

function formatRemainingQuota(
  stats: SubscriptionPlanStats | undefined,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  const quota = formatQuota(stats?.remaining_amount || 0)
  const unlimited = stats?.unlimited_count || 0
  if (unlimited > 0) {
    return t('{{quota}} + {{count}} unlimited', {
      quota,
      count: unlimited,
    })
  }
  return quota
}

export function ValuePackageStatusCards({
  records = [],
  isLoading,
}: ValuePackageStatusCardsProps) {
  const { t } = useTranslation()

  const valuePackageRecords = useMemo(
    () => records.filter((record) => record.plan.plan_kind === 'value_package'),
    [records]
  )

  const recordByType = useMemo(() => {
    const map = new Map<string, OrderManagementValuePackagePlanRecord>()
    for (const record of valuePackageRecords) {
      const packageType = record.plan.package_type
      if (packageType && !map.has(packageType)) {
        map.set(packageType, record)
      }
    }
    return map
  }, [valuePackageRecords])

  return (
    <TitledCard
      title={t('Value Package Realtime Status')}
      description={t(
        'Realtime active users and remaining quota for day, week, and month cards.'
      )}
      icon={<Sparkles className='size-4' />}
      contentClassName='grid gap-3 lg:grid-cols-3'
    >
      {VALUE_PACKAGE_TYPES.map((type) => {
        const record = recordByType.get(type.value)
        const stats = record?.stats

        return (
          <Card key={type.value} size='sm'>
            <CardHeader>
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <Badge variant='secondary'>{t(type.labelKey)}</Badge>
                  <CardTitle className='mt-2 truncate'>
                    {record?.plan.title || t(type.labelKey)}
                  </CardTitle>
                  <CardDescription>
                    {record ? t('Realtime package usage') : t('Not configured')}
                  </CardDescription>
                </div>
                {record ? (
                  <Badge variant={record.plan.enabled ? 'default' : 'outline'}>
                    {record.plan.enabled ? t('Enabled') : t('Disabled')}
                  </Badge>
                ) : null}
              </div>
            </CardHeader>
            <CardContent className='grid gap-3'>
              {isLoading ? (
                <>
                  <Skeleton className='h-14 w-full' />
                  <Skeleton className='h-14 w-full' />
                </>
              ) : (
                <>
                  <div className='rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs font-medium'>
                      {t('Active Users')}
                    </div>
                    <div className='mt-1 text-lg font-semibold tracking-tight tabular-nums'>
                      {t('{{users}} users / {{subscriptions}} subscriptions', {
                        users: stats?.active_user_count || 0,
                        subscriptions: stats?.active_subscription_count || 0,
                      })}
                    </div>
                  </div>

                  <div className='rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs font-medium'>
                      {t('Remaining Quota')}
                    </div>
                    <div className='mt-1 text-lg font-semibold tracking-tight tabular-nums'>
                      {formatRemainingQuota(stats, t)}
                    </div>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        )
      })}
    </TitledCard>
  )
}
