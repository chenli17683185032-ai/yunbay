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
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import {
  formatCurrencyUSD,
  formatNumber,
  formatQuota,
  formatTimestampToDate,
} from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  REDEMPTION_FILTER_EXPIRED,
  REDEMPTION_STATUSES,
  getRedemptionKindLabel,
  getRedemptionSourceLabel,
} from '../constants'
import { isRedemptionExpired, isTimestampExpired } from '../lib'
import { type Redemption } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

export function useRedemptionsColumns(): ColumnDef<Redemption>[] {
  const { t } = useTranslation()
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        return (
          <TableId value={row.getValue('id') as number} className='w-[60px]' />
        )
      },
      size: 80,
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      meta: { mobileTitle: true },
      cell: ({ row }) => {
        return (
          <div className='max-w-[150px] truncate font-medium'>
            {row.getValue('name')}
          </div>
        )
      },
      size: 180,
    },
    {
      accessorFn: (row) => row.type || 'quota',
      id: 'type',
      header: t('Type'),
      cell: ({ row }) => {
        const type = row.original.type || 'quota'
        let label = t('Quota Code')
        let variant: StatusVariant = 'neutral'
        if (type === 'subscription') {
          label = t('Subscription Code')
          variant = 'info'
        } else if (type === 'reset_card') {
          label = t('Reset Card Code')
          variant = 'warning'
        }
        return <StatusBadge label={label} variant={variant} copyable={false} />
      },
      size: 130,
    },
    {
      id: 'plan',
      header: t('Plan'),
      cell: ({ row }) => {
        const redemption = row.original
        if (redemption.type !== 'subscription') {
          return <span className='text-muted-foreground'>-</span>
        }
        const planLabel =
          redemption.plan_title ||
          (redemption.plan_id > 0 ? `#${redemption.plan_id}` : '-')
        return <span className='text-muted-foreground'>{planLabel}</span>
      },
      size: 140,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const redemption = row.original
        const statusValue = row.getValue('status') as number

        // Check if expired
        if (isRedemptionExpired(redemption.expired_time, statusValue)) {
          return (
            <StatusBadge
              label={t('Expired')}
              variant='warning'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }

        const statusConfig = REDEMPTION_STATUSES[statusValue]

        if (!statusConfig) {
          return null
        }

        return (
          <StatusBadge
            label={t(statusConfig.labelKey)}
            variant={statusConfig.variant}
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      filterFn: (row, id, value) => {
        const redemption = row.original
        const statusValue = row.getValue(id) as number

        // Check if expired status is being filtered
        if (value.includes(REDEMPTION_FILTER_EXPIRED)) {
          if (isRedemptionExpired(redemption.expired_time, statusValue)) {
            return true
          }
        }

        // Check regular status
        return value.includes(String(statusValue))
      },
      size: 120,
    },
    {
      accessorKey: 'kind',
      header: t('Type'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const kind = row.original.kind
        return (
          <Badge variant='secondary'>{getRedemptionKindLabel(t, kind)}</Badge>
        )
      },
      size: 160,
    },
    {
      id: 'code',
      accessorKey: 'key',
      header: t('Code'),
      cell: function CodeCell({ row }) {
        const redemption = row.original
        const key = redemption.key
        const maskedKey = `${key.slice(0, 8)}${'*'.repeat(16)}${key.slice(-8)}`

        return (
          <MaskedValueDisplay
            label={t('Full Code')}
            fullValue={key}
            maskedValue={maskedKey}
            copyTooltip={t('Copy code')}
            copyAriaLabel={t('Copy redemption code')}
          />
        )
      },
      enableSorting: false,
      size: 320,
    },
    {
      accessorKey: 'amount',
      header: t('Face amount'),
      meta: { mobileHidden: true },
      cell: ({ row }) => formatNumber(row.original.amount),
      size: 120,
    },
    {
      accessorKey: 'money',
      header: t('Paid money'),
      meta: { mobileHidden: true },
      cell: ({ row }) => formatCurrencyUSD(row.original.money),
      size: 120,
    },
    {
      accessorKey: 'batch_id',
      header: t('Batch ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const batchId = row.original.batch_id
        if (!batchId)
          return <span className='text-muted-foreground text-sm'>-</span>
        return (
          <div className='max-w-[140px] truncate font-mono text-sm'>
            {batchId}
          </div>
        )
      },
      size: 160,
    },
    {
      accessorKey: 'source',
      header: t('Source'),
      meta: { mobileHidden: true },
      cell: ({ row }) => getRedemptionSourceLabel(t, row.original.source),
      size: 140,
    },
    {
      accessorKey: 'quota',
      header: t('Quota'),
      cell: ({ row }) => {
        const redemption = row.original
        if (redemption.type === 'subscription') {
          return <span className='text-muted-foreground'>-</span>
        }
        if (redemption.type === 'reset_card') {
          return (
            <StatusBadge
              label={t('{{count}} card(s)', {
                count: redemption.reset_card_count || 0,
              })}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        const quota = row.getValue('quota') as number
        return (
          <StatusBadge
            label={formatQuota(quota)}
            variant='neutral'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 120,
    },
    {
      accessorKey: 'created_time',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        return (
          <div className='min-w-[160px] font-mono text-sm'>
            {formatTimestampToDate(row.getValue('created_time'))}
          </div>
        )
      },
      size: 180,
    },
    {
      accessorKey: 'expired_time',
      header: t('Expires'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const expiredTime = row.getValue('expired_time') as number
        if (expiredTime === 0) {
          return (
            <StatusBadge
              label={t('Never')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        const isExpired = isTimestampExpired(expiredTime)
        return (
          <div
            className={`min-w-[160px] font-mono text-sm ${isExpired ? 'text-destructive' : ''}`}
          >
            {formatTimestampToDate(expiredTime)}
          </div>
        )
      },
      size: 180,
    },
    {
      accessorKey: 'exported_time',
      header: t('Exported'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const exportedTime = row.original.exported_time
        if (!exportedTime) {
          return <span className='text-muted-foreground text-sm'>-</span>
        }
        return (
          <div className='min-w-[160px] font-mono text-sm'>
            {formatTimestampToDate(exportedTime)}
          </div>
        )
      },
      size: 180,
    },
    {
      accessorKey: 'used_user_id',
      header: t('Redeemed By'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const userId = row.getValue('used_user_id') as number
        const redemption = row.original

        if (userId === 0) {
          return <span className='text-muted-foreground text-sm'>-</span>
        }

        return (
          <Tooltip>
            <TooltipTrigger
              render={
                <StatusBadge
                  label={t('User {{id}}', { id: userId })}
                  variant='neutral'
                  copyable={false}
                  className='cursor-help'
                />
              }
            ></TooltipTrigger>
            <TooltipContent>
              <div className='space-y-1 text-xs'>
                <div>
                  {t('User ID:')} {userId}
                </div>
                {redemption.redeemed_time > 0 && (
                  <div>
                    {t('Redeemed:')}{' '}
                    {formatTimestampToDate(redemption.redeemed_time)}
                  </div>
                )}
              </div>
            </TooltipContent>
          </Tooltip>
        )
      },
      size: 140,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => <DataTableRowActions row={row} />,
      meta: { pinned: 'right' as const },
      size: 88,
    },
  ]
}
