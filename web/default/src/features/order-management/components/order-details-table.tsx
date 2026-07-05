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
import { useState } from 'react'
import { RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
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
import { formatCny, formatUnixTime, isMailStatusError } from '../lib/format'
import type { OrderManagementOrderItem, PageData } from '../types'
import { MailCheckStatusBadge } from './mail-check-status-badge'

interface OrderDetailsTableProps {
  data?: PageData<OrderManagementOrderItem>
  isLoading?: boolean
  verifyingId?: number | null
  page: number
  pageSize: number
  deletingOrderKey?: string | null
  onPageChange: (page: number) => void
  onVerify: (id: number) => void
  onDelete: (orderType: 'topup' | 'subscription', tradeNo: string) => void
}

function UserCell({ row }: { row: OrderManagementOrderItem }) {
  return (
    <div className='flex min-w-32 flex-col gap-1'>
      <span className='font-medium'>{row.username || '-'}</span>
      <span className='text-muted-foreground text-xs'>ID: {row.user_id}</span>
    </div>
  )
}

function LocalOrderCell({ row }: { row: OrderManagementOrderItem }) {
  const { t } = useTranslation()
  return (
    <div className='flex min-w-48 flex-col gap-1'>
      <span className='font-medium'>
        {row.plan_title || row.trade_no || row.session_id || '-'}
      </span>
      <span className='text-muted-foreground text-xs'>
        {row.trade_no || row.session_id || '-'}
      </span>
      <span className='text-muted-foreground text-xs'>
        {row.billing_order_type === 'subscription'
          ? t('Subscription')
          : row.billing_order_type === 'topup'
            ? t('Top-up')
            : row.order_type}
        {row.payment_method ? ` · ${row.payment_method}` : ''}
      </span>
    </div>
  )
}

function AffiliateCell({ row }: { row: OrderManagementOrderItem }) {
  if (!row.affiliate) return <span className='text-muted-foreground'>-</span>

  return (
    <div className='flex min-w-28 flex-col gap-1'>
      <span className='font-medium'>#{row.affiliate.inviter_user_id}</span>
      <span className='text-muted-foreground text-xs'>
        {formatCny(row.affiliate.commission_money)} · {row.affiliate.status}
      </span>
    </div>
  )
}

function LoadingRows() {
  return Array.from({ length: 8 }, (_, index) => (
    <TableRow key={`order-skeleton-${index}`}>
      {Array.from({ length: 10 }, (_, cellIndex) => (
        <TableCell key={cellIndex}>
          <Skeleton className='h-5 w-24' />
        </TableCell>
      ))}
    </TableRow>
  ))
}

export function OrderDetailsTable({
  data,
  isLoading,
  verifyingId,
  page,
  pageSize,
  deletingOrderKey,
  onPageChange,
  onVerify,
  onDelete,
}: OrderDetailsTableProps) {
  const { t } = useTranslation()
  const [deleteTarget, setDeleteTarget] = useState<{
    orderType: 'topup' | 'subscription'
    tradeNo: string
  } | null>(null)
  const rows = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <>
      <Card className='min-h-[520px]'>
        <CardHeader>
          <CardTitle>{t('Order details')}</CardTitle>
          <CardDescription>
            {t('Site amount')} / {t('External paid amount')} /{' '}
            {t('Mail paid amount')}
          </CardDescription>
        </CardHeader>
        <CardContent className='min-h-0 flex-1'>
          <ScrollArea className='h-[460px] rounded-lg border'>
            <Table>
              <TableHeader className='bg-background sticky top-0 z-10'>
                <TableRow>
                  <TableHead>{t('Time')}</TableHead>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead>{t('Order details')}</TableHead>
                  <TableHead>{t('Site amount')}</TableHead>
                  <TableHead>{t('External paid amount')}</TableHead>
                  <TableHead>{t('Mail paid amount')}</TableHead>
                  <TableHead>{t('Worker order number')}</TableHead>
                  <TableHead>{t('Mail verification')}</TableHead>
                  <TableHead>{t('Affiliate')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <LoadingRows />
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={10}>
                      <Empty className='border-0'>
                        <EmptyHeader>
                          <EmptyTitle>{t('No order details found')}</EmptyTitle>
                          <EmptyDescription>
                            {t('No order details found')}
                          </EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    </TableCell>
                  </TableRow>
                ) : (
                  rows.map((row) => (
                    <TableRow
                      key={row.id}
                      className={cn(
                        isMailStatusError(row.mail_status) && 'bg-destructive/5'
                      )}
                    >
                      <TableCell>{formatUnixTime(row.created_time)}</TableCell>
                      <TableCell>
                        <UserCell row={row} />
                      </TableCell>
                      <TableCell>
                        <LocalOrderCell row={row} />
                      </TableCell>
                      <TableCell>{formatCny(row.site_amount)}</TableCell>
                      <TableCell>
                        {formatCny(row.external_paid_amount)}
                      </TableCell>
                      <TableCell>{formatCny(row.mail_paid_amount)}</TableCell>
                      <TableCell>
                        <div className='flex min-w-36 flex-col gap-1'>
                          <span>{row.worker_order_no || '-'}</span>
                          {row.mail_order_no ? (
                            <span className='text-muted-foreground text-xs'>
                              {row.mail_order_no}
                            </span>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className='flex min-w-36 flex-col gap-1'>
                          <MailCheckStatusBadge status={row.mail_status} />
                          {row.error_message ? (
                            <span className='text-destructive text-xs'>
                              {row.error_message}
                            </span>
                          ) : row.verified_time ? (
                            <span className='text-muted-foreground text-xs'>
                              {formatUnixTime(row.verified_time)}
                            </span>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell>
                        <AffiliateCell row={row} />
                      </TableCell>
                      <TableCell className='text-right'>
                        <div className='flex justify-end gap-2'>
                          <Button
                            variant='outline'
                            size='sm'
                            disabled={
                              row.mail_status === 'checking' ||
                              verifyingId === row.id
                            }
                            onClick={() => onVerify(row.id)}
                          >
                            {verifyingId === row.id ? (
                              <HugeiconsIcon
                                icon={RefreshIcon}
                                data-icon='inline-start'
                                className='animate-spin'
                              />
                            ) : null}
                            {row.mail_status === 'verified'
                              ? t('Recheck')
                              : t('Verify now')}
                          </Button>
                          {row.billing_order_type && row.trade_no ? (
                            <Button
                              variant='destructive'
                              size='sm'
                              disabled={
                                deletingOrderKey ===
                                `${row.billing_order_type}:${row.trade_no}`
                              }
                              onClick={() =>
                                setDeleteTarget({
                                  orderType: row.billing_order_type as
                                    | 'topup'
                                    | 'subscription',
                                  tradeNo: row.trade_no || '',
                                })
                              }
                            >
                              <Trash2 data-icon='inline-start' />
                              {t('Delete Order')}
                            </Button>
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
        <CardFooter className='justify-between gap-3'>
          <div className='text-muted-foreground text-sm'>
            {total} {t('Order details')}
          </div>
          <Pagination className='mx-0 w-auto'>
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  href='#'
                  text={t('Previous')}
                  aria-disabled={page <= 1}
                  onClick={(event) => {
                    event.preventDefault()
                    if (page > 1) onPageChange(page - 1)
                  }}
                />
              </PaginationItem>
              <PaginationItem>
                <span className='text-muted-foreground px-2 text-sm tabular-nums'>
                  {page} / {totalPages}
                </span>
              </PaginationItem>
              <PaginationItem>
                <PaginationNext
                  href='#'
                  text={t('Next')}
                  aria-disabled={page >= totalPages}
                  onClick={(event) => {
                    event.preventDefault()
                    if (page < totalPages) onPageChange(page + 1)
                  }}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </CardFooter>
      </Card>

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Order')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This order will be hidden from order management. Deleted orders will not appear again after future scans.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() => {
                if (!deleteTarget) return
                onDelete(deleteTarget.orderType, deleteTarget.tradeNo)
                setDeleteTarget(null)
              }}
            >
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
