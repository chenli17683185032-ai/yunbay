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
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
}: {
  items: ChannelConsoleListItem[]
  onOpen: (item: ChannelConsoleListItem) => void
}) {
  const { t } = useTranslation()

  if (items.length === 0) {
    return (
      <div className='text-muted-foreground flex min-h-64 items-center justify-center rounded-lg border text-sm'>
        {t('No channel console channels yet')}
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <table className='w-full text-sm'>
        <thead className='bg-muted/50 text-left'>
          <tr>
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
            return (
              <tr className='border-t' key={channel.id}>
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
  )
}
