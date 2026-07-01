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
import { Bell, Megaphone } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Dialog } from '@/components/dialog'
import {
  AnnouncementsContent,
  NoticeContent,
  type AnnouncementItem,
} from './notification-popover'

interface RequiredAnnouncementDialogProps {
  open: boolean
  activeTab: 'notice' | 'announcements'
  onTabChange: (tab: 'notice' | 'announcements') => void
  notice: string
  announcements: AnnouncementItem[]
  loading: boolean
  onConfirmRead: () => void
}

export function RequiredAnnouncementDialog({
  open,
  activeTab,
  onTabChange,
  notice,
  announcements,
  loading,
  onConfirmRead,
}: RequiredAnnouncementDialogProps) {
  const { t } = useTranslation()

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) return
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      showCloseButton={false}
      title={t('Unread system announcements')}
      description={t('Please read these announcements before continuing.')}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
      footer={
        <Button onClick={onConfirmRead} className='w-full sm:w-auto'>
          {t('I have read')}
        </Button>
      }
    >
      <Tabs
        value={activeTab}
        onValueChange={onTabChange as (value: string) => void}
      >
        <TabsList className='grid w-full grid-cols-2'>
          <TabsTrigger value='notice' className='gap-1.5'>
            <Bell className='size-3.5' />
            {t('Notice')}
          </TabsTrigger>
          <TabsTrigger value='announcements' className='gap-1.5'>
            <Megaphone className='size-3.5' />
            {t('Timeline')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='notice' className='mt-2'>
          <NoticeContent notice={notice} loading={loading} t={t} />
        </TabsContent>

        <TabsContent value='announcements' className='mt-2'>
          <AnnouncementsContent
            announcements={announcements}
            loading={loading}
            t={t}
          />
        </TabsContent>
      </Tabs>
    </Dialog>
  )
}
