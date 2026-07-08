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
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { TitledCard } from '@/components/ui/titled-card'
import { VALUE_PACKAGE_TYPES } from '@/features/subscriptions/constants'
import { shouldExposeValuePackage7dPeriodLimit } from '@/features/subscriptions/lib/value-package-limit-labels'
import { formatValuePackageResetLine } from '@/features/value-packages/lib/reset-time'
import type {
  OrderManagementValuePackageUsageRow,
  OrderManagementValuePackageUsageSummary,
} from '../types'

interface ValuePackageUsageTableProps {
  rows?: OrderManagementValuePackageUsageRow[]
  isLoading?: boolean
}

function packageLabelKey(packageType?: string) {
  return (
    VALUE_PACKAGE_TYPES.find((type) => type.value === packageType)?.labelKey ||
    'Value Package'
  )
}

function remainingQuota(used: number, limit: number): number | null {
  if (!Number.isFinite(limit) || limit <= 0) return null
  return Math.max(0, limit - Math.max(0, used || 0))
}

function WindowQuotaCell({
  used,
  limit,
  percent,
  resetSeconds,
  limited,
}: {
  used: number
  limit: number
  percent: number
  resetSeconds?: number
  limited?: boolean
}) {
  const { t } = useTranslation()
  const remaining = remainingQuota(used, limit)

  if (remaining === null) {
    return <span className='font-semibold'>{t('Unlimited')}</span>
  }

  const resetLine = formatValuePackageResetLine({
    limit,
    resetSeconds,
    limited,
    t,
  })

  return (
    <div className='flex min-w-[160px] flex-col gap-1.5'>
      <div className='font-semibold tabular-nums'>
        {formatQuota(remaining)} / {formatQuota(limit)}
      </div>
      <Progress value={Math.min(Math.max(percent || 0, 0), 100)} />
      <div className='text-muted-foreground text-xs tabular-nums'>
        {t('Used: {{used}} / {{limit}}', {
          used: formatQuota(used || 0),
          limit: formatQuota(limit),
        })}
      </div>
      <div
        className={
          limited
            ? 'text-destructive text-xs font-medium'
            : 'text-muted-foreground text-xs'
        }
      >
        {resetLine}
      </div>
    </div>
  )
}

function Period7dQuotaCell({
  packageType,
  used,
  limit,
  percent,
  resetSeconds,
  limited,
}: {
  packageType?: string
  used: number
  limit: number
  percent: number
  resetSeconds?: number
  limited?: boolean
}) {
  const { t } = useTranslation()

  if (!shouldExposeValuePackage7dPeriodLimit(packageType) || limit <= 0) {
    return <span className='text-muted-foreground'>{t('Not applicable')}</span>
  }

  return (
    <WindowQuotaCell
      used={used}
      limit={limit}
      percent={percent}
      resetSeconds={resetSeconds}
      limited={limited}
    />
  )
}

function TotalRemainingCell({
  usage,
}: {
  usage: OrderManagementValuePackageUsageSummary | null
}) {
  const { t } = useTranslation()
  if (!usage || usage.total_limit <= 0) {
    return <span className='font-semibold'>{t('Unlimited')}</span>
  }
  return (
    <div className='flex flex-col gap-1'>
      <span className='font-semibold tabular-nums'>
        {formatQuota(usage.total_remaining || 0)}
      </span>
      <span className='text-muted-foreground text-xs tabular-nums'>
        {t('Used: {{used}} / {{limit}}', {
          used: formatQuota(usage.total_used || 0),
          limit: formatQuota(usage.total_limit),
        })}
      </span>
    </div>
  )
}

export function ValuePackageUsageTable({
  rows = [],
  isLoading,
}: ValuePackageUsageTableProps) {
  const { t } = useTranslation()

  return (
    <TitledCard
      title={t('Value Package Realtime Usage')}
      description={t(
        'Realtime 5-hour remaining, month-card 7-day period remaining, and package total quota for active value package users.'
      )}
      icon={<Sparkles className='size-4' />}
      action={
        <Badge variant='secondary'>{t('Auto-refresh every 15 seconds')}</Badge>
      }
      contentClassName='flex flex-col gap-3'
    >
      {isLoading ? (
        <div className='flex flex-col gap-2'>
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-16 w-full' />
          <Skeleton className='h-16 w-full' />
        </div>
      ) : rows.length === 0 ? (
        <Empty className='bg-card'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Sparkles />
            </EmptyMedia>
            <EmptyTitle>{t('No active value package users')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'Users who enable value package cards will appear here with synced 5-hour, month-card 7-day period, and package total usage.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Package')}</TableHead>
              <TableHead>{t('Model group')}</TableHead>
              <TableHead>{t('5-hour remaining')}</TableHead>
              <TableHead>{t('7-day period remaining')}</TableHead>
              <TableHead>{t('Package total remaining')}</TableHead>
              <TableHead>{t('Expires at')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => {
              const usage = row.usage
              return (
                <TableRow key={row.subscription.id}>
                  <TableCell>
                    <div className='flex flex-col gap-1'>
                      <span className='font-medium'>
                        {row.username || `#${row.user_id}`}
                      </span>
                      <span className='text-muted-foreground text-xs'>
                        #{row.user_id}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className='flex flex-col gap-1'>
                      <Badge variant='secondary' className='w-fit'>
                        {t(packageLabelKey(row.plan.package_type))}
                      </Badge>
                      <span className='max-w-[180px] truncate font-medium'>
                        {row.plan.title}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    {row.plan.model_group ? (
                      <Badge variant='outline'>{row.plan.model_group}</Badge>
                    ) : (
                      <span className='text-muted-foreground'>
                        {t('Not configured')}
                      </span>
                    )}
                  </TableCell>
                  <TableCell>
                    <WindowQuotaCell
                      used={usage?.used_5h || 0}
                      limit={usage?.limit_5h || 0}
                      percent={usage?.percent_5h || 0}
                      resetSeconds={usage?.reset_seconds_5h || 0}
                      limited={usage?.limited_5h || false}
                    />
                  </TableCell>
                  <TableCell>
                    <Period7dQuotaCell
                      packageType={row.plan.package_type}
                      used={usage?.used_7d || 0}
                      limit={usage?.limit_7d || 0}
                      percent={usage?.percent_7d || 0}
                      resetSeconds={usage?.reset_seconds_7d || 0}
                      limited={usage?.limited_7d || false}
                    />
                  </TableCell>
                  <TableCell>
                    <TotalRemainingCell usage={usage || null} />
                  </TableCell>
                  <TableCell>
                    <span className='text-muted-foreground tabular-nums'>
                      {formatTimestampToDate(row.subscription.end_time)}
                    </span>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </TitledCard>
  )
}
