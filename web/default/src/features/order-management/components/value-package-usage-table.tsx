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
import { formatTimestampToDate } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
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
import { ValuePackagePeriodList } from '@/features/value-packages/components/value-package-period-list'
import { getValuePackagePeriodLimits } from '@/features/value-packages/lib/period-limits'
import type { OrderManagementValuePackageUsageRow } from '../types'

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
              <TableHead>{t('Quota periods')}</TableHead>
              <TableHead>{t('Expires at')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => {
              const periods = getValuePackagePeriodLimits(
                row.usage,
                row.plan.package_type
              )
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
                    <ValuePackagePeriodList periods={periods} />
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
