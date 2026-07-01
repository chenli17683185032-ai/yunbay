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
import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import {
  getOrderAnalytics,
  getOrderManagementOrders,
  startBatchMailCheck,
  startSingleMailCheck,
} from './api'
import { AffiliateStatsSection } from './components/affiliate-stats-section'
import { OrderAnalyticsCards } from './components/order-analytics-cards'
import { OrderDetailsTable } from './components/order-details-table'
import { OrderTrendChart } from './components/order-trend-chart'
import { RangeToolbar } from './components/range-toolbar'
import { buildOrderManagementRangeParams } from './lib/range'
import type { DateRangeKey, MailCheckStatus } from './types'

const route = getRouteApi('/_authenticated/order-management/')

const MAIL_CHECK_STATUSES = new Set<MailCheckStatus>([
  'not_required',
  'pending',
  'waiting_mail',
  'checking',
  'verified',
  'order_mismatch',
  'amount_mismatch',
  'mail_parse_failed',
  'mail_fetch_failed',
  'timeout',
])

function isMailCheckStatus(value: unknown): value is MailCheckStatus {
  return (
    typeof value === 'string' &&
    MAIL_CHECK_STATUSES.has(value as MailCheckStatus)
  )
}

const orderManagementKeys = {
  analytics: (params: Record<string, unknown>) =>
    ['order-management', 'analytics', params] as const,
  orders: (params: Record<string, unknown>) =>
    ['order-management', 'orders', params] as const,
  affiliate: (params: Record<string, unknown>) =>
    ['order-management', 'affiliate', params] as const,
}

function isDateRangeKey(value: unknown): value is DateRangeKey {
  return value === '7d' || value === '30d' || value === 'custom'
}

function toDefinedSearchNumber(value: number | undefined) {
  return value && Number.isFinite(value) ? value : undefined
}

function downloadCsv(filename: string, rows: string[][]) {
  const csv = rows
    .map((row) =>
      row
        .map((cell) => `"${String(cell ?? '').replaceAll('"', '""')}"`)
        .join(',')
    )
    .join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export function OrderManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const [verifyingId, setVerifyingId] = useState<number | null>(null)

  const range: DateRangeKey = isDateRangeKey(search.range) ? search.range : '7d'
  const page = search.page && search.page > 0 ? search.page : 1
  const pageSize =
    search.page_size && search.page_size > 0 ? search.page_size : 20
  const startTime = toDefinedSearchNumber(search.start_time)
  const endTime = toDefinedSearchNumber(search.end_time)
  const mailStatus = isMailCheckStatus(search.mail_status)
    ? search.mail_status
    : undefined

  const rangeParams = useMemo(
    () => buildOrderManagementRangeParams(range, startTime, endTime),
    [endTime, range, startTime]
  )

  const orderParams = useMemo(
    () => ({
      ...rangeParams,
      page,
      page_size: pageSize,
      mail_status: mailStatus,
      keyword: search.keyword || undefined,
    }),
    [mailStatus, page, pageSize, rangeParams, search.keyword]
  )

  const analyticsQuery = useQuery({
    queryKey: orderManagementKeys.analytics(rangeParams),
    queryFn: async () => {
      const result = await getOrderAnalytics(rangeParams)
      if (!result.success)
        throw new Error(result.message || t('Request failed'))
      return result.data
    },
  })

  const ordersQuery = useQuery({
    queryKey: orderManagementKeys.orders(orderParams),
    queryFn: async () => {
      const result = await getOrderManagementOrders(orderParams)
      if (!result.success)
        throw new Error(result.message || t('Request failed'))
      return result.data
    },
    placeholderData: (previousData) => previousData,
  })

  const invalidateOrderManagement = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ['order-management', 'analytics'],
      }),
      queryClient.invalidateQueries({
        queryKey: ['order-management', 'orders'],
      }),
      queryClient.invalidateQueries({
        queryKey: ['order-management', 'affiliate'],
      }),
    ])
  }, [queryClient])

  const singleMailCheckMutation = useMutation({
    mutationFn: startSingleMailCheck,
    onMutate: (orderId) => setVerifyingId(orderId),
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(result.message || t('Request failed'))
        return
      }
      toast.success(t('Verify now'))
      await invalidateOrderManagement()
    },
    onSettled: () => setVerifyingId(null),
  })

  const batchMailCheckMutation = useMutation({
    mutationFn: () =>
      startBatchMailCheck({
        ...rangeParams,
        scope: 'unfinished',
      }),
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(result.message || t('Request failed'))
        return
      }
      toast.success(t('Verify unfinished orders now'))
      await invalidateOrderManagement()
    },
  })

  const handleRangeChange = useCallback(
    (nextRange: DateRangeKey) => {
      void navigate({
        search: (previous) => ({
          ...previous,
          range: nextRange,
          start_time: nextRange === 'custom' ? previous.start_time : undefined,
          end_time: nextRange === 'custom' ? previous.end_time : undefined,
          page: 1,
        }),
      })
    },
    [navigate]
  )

  const handleCustomRangeChange = useCallback(
    (nextStartTime?: number, nextEndTime?: number) => {
      void navigate({
        search: (previous) => ({
          ...previous,
          range: 'custom',
          start_time: nextStartTime,
          end_time: nextEndTime,
          page: 1,
        }),
      })
    },
    [navigate]
  )

  const handlePageChange = useCallback(
    (nextPage: number) => {
      void navigate({
        search: (previous) => ({
          ...previous,
          page: nextPage,
        }),
      })
    },
    [navigate]
  )

  const handleExportCsv = useCallback(() => {
    const rows = ordersQuery.data?.items ?? []
    downloadCsv('order-management.csv', [
      [
        t('Time'),
        t('User'),
        t('Order details'),
        t('Site amount'),
        t('External paid amount'),
        t('Mail paid amount'),
        t('Worker order number'),
        t('Mail verification'),
        t('Affiliate'),
      ],
      ...rows.map((row) => [
        String(row.created_time),
        `${row.username || ''}#${row.user_id}`,
        row.session_id,
        String(row.site_amount),
        String(row.external_paid_amount),
        String(row.mail_paid_amount),
        row.worker_order_no,
        row.mail_status,
        row.affiliate
          ? `${row.affiliate.inviter_user_id}:${row.affiliate.commission_money}`
          : '',
      ]),
    ])
  }, [ordersQuery.data?.items, t])

  const isChecking =
    batchMailCheckMutation.isPending || singleMailCheckMutation.isPending

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Order Management')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={ordersQuery.isFetching || analyticsQuery.isFetching}
          onClick={() => void invalidateOrderManagement()}
        >
          <HugeiconsIcon
            icon={RefreshIcon}
            data-icon='inline-start'
            className={ordersQuery.isFetching ? 'animate-spin' : undefined}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <RangeToolbar
            range={range}
            startTime={startTime}
            endTime={endTime}
            isChecking={isChecking}
            onRangeChange={handleRangeChange}
            onCustomRangeChange={handleCustomRangeChange}
            onBatchCheck={() => batchMailCheckMutation.mutate()}
            onExportCsv={handleExportCsv}
          />

          <OrderAnalyticsCards
            summary={analyticsQuery.data?.summary}
            isLoading={analyticsQuery.isLoading}
          />

          <OrderTrendChart
            daily={analyticsQuery.data?.daily ?? []}
            isLoading={analyticsQuery.isLoading}
          />

          <OrderDetailsTable
            data={ordersQuery.data}
            isLoading={ordersQuery.isLoading}
            verifyingId={verifyingId}
            page={page}
            pageSize={pageSize}
            onPageChange={handlePageChange}
            onVerify={(id) => singleMailCheckMutation.mutate(id)}
          />

          <AffiliateStatsSection
            range={range}
            startTime={startTime}
            endTime={endTime}
            onChanged={invalidateOrderManagement}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
