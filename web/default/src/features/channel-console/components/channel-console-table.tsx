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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import type { ChannelConsoleListItem, ChannelConsoleStatus } from '../types'

function statusVariant(status: ChannelConsoleStatus) {
  if (status === 'healthy') return 'default'
  if (status === 'failed' || status === 'disabled') return 'destructive'
  if (status === 'warning') return 'secondary'
  return 'outline'
}

function modelCount(models: string) {
  return models
    .split(',')
    .map((model) => model.trim())
    .filter(Boolean).length
}

export function ChannelConsoleTable({
  items,
  onOpen,
  onBatchDelete,
}: {
  items: ChannelConsoleListItem[]
  onOpen: (item: ChannelConsoleListItem) => void
  onBatchDelete: (ids: number[]) => Promise<void>
}) {
  const { t } = useTranslation()
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [deleting, setDeleting] = useState(false)
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds])
  const allSelected =
    items.length > 0 && items.every((item) => selectedSet.has(item.channel.id))

  useEffect(() => {
    const visibleIds = new Set(items.map((item) => item.channel.id))
    setSelectedIds((current) => current.filter((id) => visibleIds.has(id)))
  }, [items])

  function toggleOne(id: number, checked: boolean) {
    setSelectedIds((current) => {
      if (checked) {
        return current.includes(id) ? current : [...current, id]
      }
      return current.filter((item) => item !== id)
    })
  }

  function toggleAll(checked: boolean) {
    setSelectedIds(checked ? items.map((item) => item.channel.id) : [])
  }

  async function handleBatchDelete() {
    if (selectedIds.length === 0) return
    setDeleting(true)
    try {
      await onBatchDelete(selectedIds)
      setSelectedIds([])
    } finally {
      setDeleting(false)
    }
  }

  if (items.length === 0) {
    return (
      <div className='text-muted-foreground flex min-h-64 items-center justify-center rounded-lg border text-sm'>
        {t('No channel console channels yet')}
      </div>
    )
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2'>
        <div className='text-muted-foreground text-sm'>
          {selectedIds.length > 0
            ? t('{{count}} channels selected', { count: selectedIds.length })
            : t('Select failed red channels and delete them in batch')}
        </div>
        <Button
          disabled={selectedIds.length === 0 || deleting}
          onClick={handleBatchDelete}
          size='sm'
          variant='destructive'
        >
          {t('Batch delete')}
        </Button>
      </div>
      <div className='overflow-hidden rounded-lg border'>
        <table className='w-full text-sm'>
          <thead className='bg-muted/50 text-left'>
            <tr>
              <th className='w-10 p-3'>
                <Checkbox
                  aria-label={t('Select all')}
                  checked={allSelected}
                  onCheckedChange={(value) => toggleAll(Boolean(value))}
                />
              </th>
              <th className='p-3'>{t('Channel')}</th>
              <th className='p-3'>{t('Base URL')}</th>
              <th className='p-3'>{t('Models')}</th>
              <th className='p-3'>{t('Health')}</th>
              <th className='p-3'>{t('Actions')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => {
              const channel = item.channel
              const failed =
                item.console.health_status === 'failed' ||
                item.console.health_status === 'disabled'
              return (
                <tr
                  className={
                    failed ? 'bg-destructive/5 border-t' : 'border-t'
                  }
                  key={channel.id}
                >
                  <td className='p-3'>
                    <Checkbox
                      aria-label={t('Select channel')}
                      checked={selectedSet.has(channel.id)}
                      onCheckedChange={(value) =>
                        toggleOne(channel.id, Boolean(value))
                      }
                    />
                  </td>
                  <td className='p-3'>
                    <div className='font-medium'>{channel.name}</div>
                    <div className='text-muted-foreground'>#{channel.id}</div>
                  </td>
                  <td className='max-w-72 truncate p-3'>
                    {channel.base_url || '-'}
                  </td>
                  <td className='p-3'>{modelCount(channel.models)}</td>
                  <td className='p-3'>
                    <Badge variant={statusVariant(item.console.health_status)}>
                      {item.console.health_status || 'unchecked'}
                    </Badge>
                  </td>
                  <td className='p-3'>
                    <Button
                      onClick={() => onOpen(item)}
                      size='sm'
                      variant='outline'
                    >
                      {t('Details')}
                    </Button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
