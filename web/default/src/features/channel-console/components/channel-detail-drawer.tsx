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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import { checkChannelConsoleHealth, getChannelConsoleDetail } from '../api'
import type { ChannelConsoleDetail, ChannelConsoleListItem } from '../types'

export function ChannelDetailDrawer({
  item,
  onChecked,
  onOpenChange,
  open,
}: {
  item: ChannelConsoleListItem | null
  onChecked?: () => void
  onOpenChange: (open: boolean) => void
  open: boolean
}) {
  const { t } = useTranslation()
  const [detail, setDetail] = useState<ChannelConsoleDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const channel = item?.channel || null

  useEffect(() => {
    if (!channel || !open) return
    setLoading(true)
    getChannelConsoleDetail(channel.id)
      .then((res) => setDetail(res.data || null))
      .finally(() => setLoading(false))
  }, [channel, open])

  async function handleHealthCheck() {
    if (!channel) return
    const res = await checkChannelConsoleHealth(channel.id)
    if (!res.success) {
      toast.error(res.message || t('Health check failed'))
      return
    }
    toast.success(t('Health check completed'))
    const refreshed = await getChannelConsoleDetail(channel.id)
    setDetail(refreshed.data || null)
    onChecked?.()
  }

  return (
    <Drawer onOpenChange={onOpenChange} open={open}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{channel?.name || t('Channel details')}</DrawerTitle>
        </DrawerHeader>
        <div className='space-y-4 overflow-auto p-4'>
          <div className='grid gap-2 text-sm md:grid-cols-2'>
            <div>
              {t('Provider')}: {detail?.console.provider || item?.console.provider || '-'}
            </div>
            <div>
              {t('Health')}:{' '}
              {detail?.console.health_status || item?.console.health_status || 'unchecked'}
            </div>
            <div>
              {t('Price status')}:{' '}
              {detail?.console.price_sync_status || item?.console.price_sync_status || 'unchecked'}
            </div>
            <div>{t('Models')}: {channel?.models || '-'}</div>
            <div>
              {t('Last check')}:{' '}
              {detail?.console.last_health_check_at
                ? new Date(detail.console.last_health_check_at * 1000).toLocaleString()
                : '-'}
            </div>
            <div>
              {t('Last message')}:{' '}
              {detail?.console.last_error_message || item?.console.last_error_message || '-'}
            </div>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button disabled={!channel || loading} onClick={handleHealthCheck}>
              {t('Verify now')}
            </Button>
            <Link className={buttonVariants({ variant: 'outline' })} to='/channels'>
              {t('Advanced channel management')}
            </Link>
          </div>
          <div className='rounded-lg border p-3 text-sm'>
            <div className='font-medium'>{t('Recent health records')}</div>
            <pre className='mt-2 max-h-48 overflow-auto text-xs'>
              {JSON.stringify(detail?.health_checks || [], null, 2)}
            </pre>
          </div>
        </div>
      </DrawerContent>
    </Drawer>
  )
}
