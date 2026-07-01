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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getAffiliateSourceOrders } from '../api'
import { formatBpsRate, formatCny, formatUnixTime } from '../lib/format'
import { buildOrderManagementRangeParams } from '../lib/range'
import type { DateRangeKey } from '../types'
import { MailCheckStatusBadge } from './mail-check-status-badge'

interface SourceOrdersDrawerProps {
  userId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
  range: DateRangeKey
  startTime?: number
  endTime?: number
}

export function SourceOrdersDrawer({
  userId,
  open,
  onOpenChange,
  range,
  startTime,
  endTime,
}: SourceOrdersDrawerProps) {
  const { t } = useTranslation()
  const params = useMemo(
    () => ({
      ...buildOrderManagementRangeParams(range, startTime, endTime),
      limit: 50,
    }),
    [endTime, range, startTime]
  )

  const sourceOrdersQuery = useQuery({
    queryKey: ['order-management', 'affiliate-source-orders', userId, params],
    enabled: open && Boolean(userId),
    queryFn: async () => {
      const result = await getAffiliateSourceOrders(userId ?? 0, params)
      if (!result.success)
        throw new Error(result.message || t('Request failed'))
      return result.data ?? []
    },
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-[min(920px,100vw)] sm:max-w-3xl'>
        <SheetHeader>
          <SheetTitle>{t('Source orders')}</SheetTitle>
          <SheetDescription>
            {t('User')} #{userId ?? '-'} · {t('Affiliate')}
          </SheetDescription>
        </SheetHeader>
        <ScrollArea className='min-h-0 flex-1 px-4 pb-4'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Order details')}</TableHead>
                <TableHead>{t('Site amount')}</TableHead>
                <TableHead>{t('Affiliate')}</TableHead>
                <TableHead>{t('Mail verification')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sourceOrdersQuery.isLoading ? (
                Array.from({ length: 5 }, (_, index) => (
                  <TableRow key={`source-order-skeleton-${index}`}>
                    {Array.from({ length: 6 }, (_, cellIndex) => (
                      <TableCell key={cellIndex}>
                        <Skeleton className='h-5 w-24' />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : (sourceOrdersQuery.data ?? []).length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No order details found')}
                  </TableCell>
                </TableRow>
              ) : (
                (sourceOrdersQuery.data ?? []).map((order) => (
                  <TableRow key={`${order.trade_no}-${order.worker_order_no}`}>
                    <TableCell>{formatUnixTime(order.order_time)}</TableCell>
                    <TableCell>
                      {order.invitee_username || '-'} #{order.invitee_user_id}
                    </TableCell>
                    <TableCell>
                      <div className='flex min-w-44 flex-col gap-1'>
                        <span>{order.trade_no || '-'}</span>
                        <span className='text-muted-foreground text-xs'>
                          {order.worker_order_no || '-'}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>{formatCny(order.base_money)}</TableCell>
                    <TableCell>
                      {formatCny(order.commission_money)} ·{' '}
                      {formatBpsRate(order.rate_bps)}
                    </TableCell>
                    <TableCell>
                      <MailCheckStatusBadge status={order.mail_status} />
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}
