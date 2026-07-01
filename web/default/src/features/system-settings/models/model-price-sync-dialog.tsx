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
import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { RefreshCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { applyModelPriceSync, previewModelPriceSync } from '../api'
import type {
  CanonicalModelPrice,
  ModelPriceSyncItem,
  ModelPriceSyncResult,
} from '../types'

type ModelPriceSyncDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedModels: string[]
  onApplied: () => void
}

const PRICE_DIMENSIONS: Array<keyof CanonicalModelPrice> = [
  'input',
  'output',
  'cache_read',
  'cache_write',
  'cache_write_1h',
  'image_input',
  'audio_input',
  'audio_output',
]

function formatPrice(value: number | undefined): string {
  if (value === undefined || value === null) return '-'
  return `$${value.toLocaleString(undefined, {
    maximumFractionDigits: 9,
  })}`
}

function getPriceDimensionLabel(
  key: keyof CanonicalModelPrice,
  t: (key: string) => string
): string {
  switch (key) {
    case 'input':
      return t('Input')
    case 'output':
      return t('Output')
    case 'cache_read':
      return t('Cache read')
    case 'cache_write':
      return t('Cache write')
    case 'cache_write_1h':
      return t('Cache write 1h')
    case 'image_input':
      return t('Image input')
    case 'audio_input':
      return t('Audio input')
    case 'audio_output':
      return t('Audio output')
    default:
      return String(key)
  }
}

function summarizePrice(
  price: CanonicalModelPrice,
  t: (key: string) => string
): string {
  const parts = PRICE_DIMENSIONS.filter((key) => price[key] !== undefined)
    .slice(0, 5)
    .map(
      (key) => `${getPriceDimensionLabel(key, t)}: ${formatPrice(price[key])}`
    )
  return parts.length > 0 ? parts.join(' · ') : '-'
}

function getStatusVariant(
  item: ModelPriceSyncItem
): 'success' | 'neutral' | 'warning' {
  if (item.status === 'ready') return item.changed ? 'success' : 'neutral'
  return 'warning'
}

function getStatusLabel(item: ModelPriceSyncItem): string {
  if (item.status === 'ready') {
    return item.changed ? 'Ready to update' : 'No change'
  }
  if (item.reason === 'not_in_model_square') return 'Not in model square'
  if (item.reason === 'no_openrouter_match') return 'No OpenRouter match'
  if (item.reason === 'multiple_openrouter_matches')
    return 'Multiple matches skipped'
  if (item.reason?.startsWith('invalid_billing_expr')) {
    return 'Invalid billing expression'
  }
  return 'Skipped'
}

export function ModelPriceSyncDialog({
  open,
  onOpenChange,
  selectedModels,
  onApplied,
}: ModelPriceSyncDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [preview, setPreview] = useState<ModelPriceSyncResult | null>(null)

  useEffect(() => {
    if (!open) {
      setPreview(null)
    }
  }, [open])

  const requestPayload = () => ({
    models: selectedModels,
  })

  const previewMutation = useMutation({
    mutationFn: previewModelPriceSync,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to preview model price sync'))
        return
      }
      setPreview(data.data)
      if (data.data.syncable === 0) {
        toast.warning(t('No selected models can be synced'))
      } else {
        toast.success(t('Model price sync preview is ready'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to preview model price sync'))
    },
  })

  const applyMutation = useMutation({
    mutationFn: applyModelPriceSync,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to apply model price sync'))
        return
      }
      setPreview(data.data)
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(
        t('Applied price sync to {{count}} models', {
          count: data.data.applied_count ?? 0,
        })
      )
      onApplied()
      onOpenChange(false)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to apply model price sync'))
    },
  })

  const canPreview = selectedModels.length > 0
  const canApply = Boolean(preview && preview.syncable > 0)

  const handlePreview = () => {
    if (!canPreview) {
      toast.error(t('Select at least one target model'))
      return
    }
    previewMutation.mutate(requestPayload())
  }

  const handleApply = () => {
    if (!canApply) return
    applyMutation.mutate(requestPayload())
  }

  const footer = (
    <>
      <Button
        variant='outline'
        onClick={() => onOpenChange(false)}
        disabled={previewMutation.isPending || applyMutation.isPending}
      >
        {t('Cancel')}
      </Button>
      <Button
        variant='outline'
        onClick={handlePreview}
        disabled={
          !canPreview || previewMutation.isPending || applyMutation.isPending
        }
      >
        {previewMutation.isPending ? (
          <Spinner data-icon='inline-start' />
        ) : (
          <RefreshCcw data-icon='inline-start' />
        )}
        {t('Preview sync')}
      </Button>
      <Button
        onClick={handleApply}
        disabled={
          !canApply || previewMutation.isPending || applyMutation.isPending
        }
      >
        {applyMutation.isPending && <Spinner data-icon='inline-start' />}
        {t('Apply sync')}
      </Button>
    </>
  )

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Sync selected model prices')}
      description={t(
        'Compare OpenRouter and official prices for selected existing models, then apply the higher price per dimension.'
      )}
      contentClassName='flex max-h-[90vh] max-w-[calc(100%-2rem)] flex-col sm:max-w-[90vw] xl:max-w-[1280px]'
      contentHeight='min(72vh, 720px)'
      bodyClassName='flex flex-col gap-4'
      footer={footer}
    >
      <div className='flex flex-col gap-4'>
        <Alert>
          <AlertTitle>
            {t('Selected models: {{count}}', { count: selectedModels.length })}
          </AlertTitle>
          <AlertDescription>
            {t(
              'Only checked models that already exist in the model square will be updated. Unmatched OpenRouter models are ignored.'
            )}
          </AlertDescription>
        </Alert>

        <div className='min-h-0 overflow-auto rounded-lg border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Current')}</TableHead>
                <TableHead>{t('Official')}</TableHead>
                <TableHead>{t('OpenRouter')}</TableHead>
                <TableHead>{t('Final')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {preview?.items?.length ? (
                preview.items.map((item) => (
                  <TableRow key={item.model_name}>
                    <TableCell className='font-medium'>
                      <div className='flex max-w-56 flex-col gap-1'>
                        <span className='truncate'>{item.model_name}</span>
                        {item.openrouter_id &&
                          item.openrouter_id !== item.model_name && (
                            <span className='text-muted-foreground truncate text-xs'>
                              {item.openrouter_id}
                            </span>
                          )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={t(getStatusLabel(item))}
                        variant={getStatusVariant(item)}
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell className='text-muted-foreground max-w-64 truncate text-xs'>
                      {summarizePrice(item.current, t)}
                    </TableCell>
                    <TableCell className='text-muted-foreground max-w-64 truncate text-xs'>
                      {summarizePrice(item.official, t)}
                    </TableCell>
                    <TableCell className='text-muted-foreground max-w-64 truncate text-xs'>
                      {summarizePrice(item.openrouter, t)}
                    </TableCell>
                    <TableCell className='max-w-72 truncate text-xs'>
                      {summarizePrice(item.final, t)}
                    </TableCell>
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-28 text-center'
                  >
                    {previewMutation.isPending
                      ? t('Loading preview...')
                      : t('Run preview to see price changes')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>
    </Dialog>
  )
}
