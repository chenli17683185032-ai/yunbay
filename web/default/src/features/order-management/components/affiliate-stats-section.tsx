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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getAffiliateStats, markWithdrawalPaid, rejectWithdrawal } from '../api'
import { formatCny, formatUnixTime } from '../lib/format'
import { buildOrderManagementRangeParams } from '../lib/range'
import type {
  AffiliateStatsItem,
  AffiliateStatsSummary,
  AffiliateWithdrawal,
  DateRangeKey,
} from '../types'
import { SourceOrdersDrawer } from './source-orders-drawer'
import { WithdrawalActions } from './withdrawal-actions'

interface AffiliateStatsSectionProps {
  range: DateRangeKey
  startTime?: number
  endTime?: number
  onChanged: () => Promise<void> | void
}

function AffiliateMetricCard({
  title,
  value,
  description,
  isLoading,
}: {
  title: string
  value: string
  description: string
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
          <Skeleton className='h-7 w-24' />
        ) : (
          <div className='text-xl font-semibold tracking-tight tabular-nums'>
            {value}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function WithdrawalStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  if (status === 'paid') return <Badge variant='default'>{t('Paid')}</Badge>
  if (status === 'rejected') {
    return <Badge variant='destructive'>{t('Rejected')}</Badge>
  }
  return <Badge variant='outline'>{t('Pending withdrawals')}</Badge>
}

function WithdrawalInfo({
  withdrawal,
}: {
  withdrawal: AffiliateWithdrawal | null
}) {
  const { t } = useTranslation()

  if (!withdrawal) {
    return (
      <span className='text-muted-foreground'>
        {t('Available without withdrawal')}
      </span>
    )
  }

  return (
    <div className='flex min-w-64 flex-col gap-1'>
      <div className='flex flex-wrap items-center gap-2'>
        <WithdrawalStatusBadge status={withdrawal.status} />
        <span className='font-medium'>{formatCny(withdrawal.amount)}</span>
      </div>
      <span className='text-muted-foreground text-xs'>
        {withdrawal.withdrawal_id} · {formatUnixTime(withdrawal.created_time)}
      </span>
      <span className='text-muted-foreground text-xs'>
        {withdrawal.contact}
      </span>
      {withdrawal.remark ? (
        <span className='text-muted-foreground text-xs'>
          {t('Withdrawal request')}: {withdrawal.remark}
        </span>
      ) : null}
      {withdrawal.admin_remark ? (
        <span className='text-muted-foreground text-xs'>
          {t('Admin remark')}: {withdrawal.admin_remark}
        </span>
      ) : null}
    </div>
  )
}

function LoadingRows() {
  return Array.from({ length: 5 }, (_, index) => (
    <TableRow key={`affiliate-skeleton-${index}`}>
      {Array.from({ length: 7 }, (_, cellIndex) => (
        <TableCell key={cellIndex}>
          <Skeleton className='h-5 w-24' />
        </TableCell>
      ))}
    </TableRow>
  ))
}

function summaryFallback(): AffiliateStatsSummary {
  return {
    affiliate_user_count: 0,
    period_commission_amount: 0,
    pending_withdrawal_user_count: 0,
    pending_withdrawal_amount: 0,
    available_without_withdrawal_user_count: 0,
  }
}

export function AffiliateStatsSection({
  range,
  startTime,
  endTime,
  onChanged,
}: AffiliateStatsSectionProps) {
  const { t } = useTranslation()
  const [sourceOrdersUserId, setSourceOrdersUserId] = useState<number | null>(
    null
  )
  const params = useMemo(
    () => ({
      ...buildOrderManagementRangeParams(range, startTime, endTime),
      page: 1,
      page_size: 50,
    }),
    [endTime, range, startTime]
  )

  const statsQuery = useQuery({
    queryKey: ['order-management', 'affiliate', params],
    queryFn: async () => {
      const result = await getAffiliateStats(params)
      if (!result.success)
        throw new Error(result.message || t('Request failed'))
      return result.data
    },
  })

  const refresh = async () => {
    await statsQuery.refetch()
    await onChanged()
  }

  const handlePaid = async (withdrawalId: number, remark: string) => {
    const result = await markWithdrawalPaid(withdrawalId, remark)
    if (!result.success) throw new Error(result.message || t('Request failed'))
    toast.success(t('Mark as paid'))
    await refresh()
  }

  const handleReject = async (withdrawalId: number, remark: string) => {
    const result = await rejectWithdrawal(withdrawalId, remark)
    if (!result.success) throw new Error(result.message || t('Request failed'))
    toast.success(t('Reject withdrawal'))
    await refresh()
  }

  const summary = statsQuery.data?.summary ?? summaryFallback()
  const items: AffiliateStatsItem[] = statsQuery.data?.items ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Affiliate statistics')}</CardTitle>
        <CardDescription>
          {t('Users with rewards')} / {t('Withdrawal request')}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
          <AffiliateMetricCard
            title={t('Users with rewards')}
            description={t('Affiliate statistics')}
            value={String(summary.affiliate_user_count)}
            isLoading={statsQuery.isLoading}
          />
          <AffiliateMetricCard
            title={t('Period rewards')}
            description={t('Affiliate')}
            value={formatCny(summary.period_commission_amount)}
            isLoading={statsQuery.isLoading}
          />
          <AffiliateMetricCard
            title={t('Pending withdrawals')}
            description={String(summary.pending_withdrawal_user_count)}
            value={formatCny(summary.pending_withdrawal_amount)}
            isLoading={statsQuery.isLoading}
          />
          <AffiliateMetricCard
            title={t('Available without withdrawal')}
            description={t('Available rewards')}
            value={String(summary.available_without_withdrawal_user_count)}
            isLoading={statsQuery.isLoading}
          />
        </div>

        <ScrollArea className='h-[420px] rounded-lg border'>
          <Table>
            <TableHeader className='bg-background sticky top-0 z-10'>
              <TableRow>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Period rewards')}</TableHead>
                <TableHead>{t('Total rewards')}</TableHead>
                <TableHead>{t('Available rewards')}</TableHead>
                <TableHead>{t('Withdrawn rewards')}</TableHead>
                <TableHead>{t('Withdrawal request')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {statsQuery.isLoading ? (
                <LoadingRows />
              ) : items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7}>
                    <Empty className='border-0'>
                      <EmptyHeader>
                        <EmptyTitle>
                          {t('No affiliate records found')}
                        </EmptyTitle>
                        <EmptyDescription>
                          {t('No affiliate records found')}
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <TableRow key={item.user_id}>
                    <TableCell>
                      <div className='flex min-w-32 flex-col gap-1'>
                        <span className='font-medium'>
                          {item.username || '-'}
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          ID: {item.user_id}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      {formatCny(item.period_commission_amount)}
                    </TableCell>
                    <TableCell>
                      {formatCny(item.total_commission_amount)}
                    </TableCell>
                    <TableCell>{formatCny(item.available_amount)}</TableCell>
                    <TableCell>{formatCny(item.withdrawn_amount)}</TableCell>
                    <TableCell>
                      <WithdrawalInfo withdrawal={item.withdrawal} />
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex min-w-44 flex-col items-end gap-2'>
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={() => setSourceOrdersUserId(item.user_id)}
                        >
                          {t('Source orders')}
                        </Button>
                        {item.withdrawal ? (
                          <WithdrawalActions
                            withdrawalId={item.withdrawal.id}
                            status={item.withdrawal.status}
                            onPaid={(remark) =>
                              handlePaid(item.withdrawal!.id, remark)
                            }
                            onReject={(remark) =>
                              handleReject(item.withdrawal!.id, remark)
                            }
                          />
                        ) : null}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </ScrollArea>
      </CardContent>

      <SourceOrdersDrawer
        userId={sourceOrdersUserId}
        open={sourceOrdersUserId !== null}
        onOpenChange={(open) => !open && setSourceOrdersUserId(null)}
        range={range}
        startTime={startTime}
        endTime={endTime}
      />
    </Card>
  )
}
