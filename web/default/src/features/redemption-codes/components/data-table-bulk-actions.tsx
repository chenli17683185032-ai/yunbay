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
import { useState, useMemo } from 'react'
import { type Table } from '@tanstack/react-table'
import { Download, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { deleteInvalidRedemptions, exportRedemptions } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import { type Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useRedemptions()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] =
    useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [exportingFormat, setExportingFormat] = useState<'txt' | 'csv' | null>(
    null
  )
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const selectedRedemptions = useMemo(
    () => selectedRows.map((row) => row.original as Redemption),
    [selectedRows]
  )

  const contentToCopy = useMemo(() => {
    return selectedRedemptions.map((redemption) => redemption.key).join('\n')
  }, [selectedRedemptions])

  const selectedBatchId = useMemo(() => {
    const batchIds = new Set(
      selectedRedemptions
        .map((redemption) => redemption.batch_id)
        .filter((batchId) => batchId.length > 0)
    )

    if (batchIds.size !== 1 || batchIds.size !== selectedRedemptions.length) {
      return null
    }

    return [...batchIds][0]
  }, [selectedRedemptions])

  const handleExport = async (format: 'txt' | 'csv') => {
    if (!selectedBatchId) return

    setExportingFormat(format)
    try {
      const blob = await exportRedemptions(selectedBatchId, format)
      downloadBlob(blob, `redemptions-${selectedBatchId}.${format}`)
      toast.success(t(SUCCESS_MESSAGES.EXPORT_SUCCESS))
    } catch {
      toast.error(t(SUCCESS_MESSAGES.EXPORT_FAILED))
    } finally {
      setExportingFormat(null)
    }
  }

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidRedemptions()

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid redemption codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      }
    } finally {
      setIsDeleting(false)
    }
  }

  const exportDisabled = !selectedBatchId || exportingFormat !== null
  const exportTooltip = selectedBatchId
    ? t('Export selected batch')
    : t('Select codes from exactly one batch to export')

  return (
    <>
      <BulkActionsToolbar table={table} entityName={t('redemption code')}>
        <CopyButton
          value={contentToCopy}
          variant='outline'
          size='icon'
          className='size-8'
          tooltip={t('Copy selected codes')}
          successTooltip={t('Codes copied!')}
          aria-label={t('Copy selected codes')}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='sm'
                onClick={() => handleExport('txt')}
                disabled={exportDisabled}
                className='h-8'
                aria-label={t('Export TXT')}
              />
            }
          >
            <Download data-icon='inline-start' />
            TXT
          </TooltipTrigger>
          <TooltipContent>
            <p>{exportTooltip}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='sm'
                onClick={() => handleExport('csv')}
                disabled={exportDisabled}
                className='h-8'
                aria-label={t('Export CSV')}
              />
            }
          >
            <Download data-icon='inline-start' />
            CSV
          </TooltipTrigger>
          <TooltipContent>
            <p>{exportTooltip}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => setShowDeleteInvalidConfirm(true)}
                className='size-8'
                aria-label={t('Delete invalid redemption codes')}
                title={t('Delete invalid redemption codes')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete invalid codes')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete invalid codes (used/disabled/expired)')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        handleConfirm={handleDeleteInvalid}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Invalid Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('used')}</strong>,{' '}
            <strong>{t('disabled')}</strong>
            {t(', and')} <strong>{t('expired')}</strong>{' '}
            {t('redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}
