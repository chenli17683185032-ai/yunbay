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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNotificationStore } from '@/stores/notification-store'
import { getNotice } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'
import {
  getUnreadNotificationState,
  type AnnouncementRecord,
} from './notification-model'

/**
 * Hook to manage notifications (Notice + Announcements)
 * Provides unread counts and explicit read status management
 */
export function useNotifications() {
  const [popoverOpen, setPopoverOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'notice' | 'announcements'>(
    'notice'
  )

  // Fetch Notice from API
  const {
    data: noticeResponse,
    isLoading: noticeLoading,
    refetch: refetchNotice,
  } = useQuery({
    queryKey: ['notice'],
    queryFn: getNotice,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })

  // Fetch Announcements from status
  const { status, loading: statusLoading } = useStatus()
  const announcementsEnabled = status?.announcements_enabled ?? false
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const announcements: AnnouncementRecord[] = announcementsEnabled
    ? ((status?.announcements || []) as AnnouncementRecord[]).slice(0, 20)
    : []

  // Notification store
  const {
    lastReadNotice,
    readAnnouncementKeys,
    markNoticeRead,
    markAnnouncementsRead,
  } = useNotificationStore()

  // Extract notice content
  const noticeContent = noticeResponse?.success
    ? (noticeResponse.data || '').trim()
    : ''

  // Calculate unread state without mutating read status
  const unreadState = useMemo(
    () =>
      getUnreadNotificationState({
        noticeContent,
        announcements,
        lastReadNotice,
        readAnnouncementKeys,
      }),
    [noticeContent, announcements, lastReadNotice, readAnnouncementKeys]
  )

  const unreadCounts = unreadState.unreadCounts

  const confirmRead = () => {
    if (unreadState.noticeUnread && noticeContent) {
      markNoticeRead(noticeContent)
    }

    if (unreadState.unreadAnnouncementKeys.length > 0) {
      markAnnouncementsRead(unreadState.unreadAnnouncementKeys)
    }

    setPopoverOpen(false)
  }

  // Handle popover open without marking content as read
  const handleOpenPopover = (tab?: 'notice' | 'announcements') => {
    const nextTab = tab || activeTab

    setActiveTab(nextTab)
    setPopoverOpen(true)
  }

  const handlePopoverOpenChange = (open: boolean) => {
    if (open) {
      handleOpenPopover(activeTab)
      return
    }

    setPopoverOpen(false)
  }

  // Handle tab change without marking announcements as read
  const handleTabChange = (tab: 'notice' | 'announcements') => {
    setActiveTab(tab)
  }

  const requiredDialogOpen =
    !noticeLoading &&
    !statusLoading &&
    unreadCounts.total > 0 &&
    unreadState.hasDisplayableContent

  return {
    // Data
    notice: noticeContent,
    announcements,
    loading: noticeLoading || statusLoading,

    // Unread counts
    unreadCount: unreadCounts.total,
    unreadNoticeCount: unreadCounts.notice,
    unreadAnnouncementsCount: unreadCounts.announcements,
    unreadAnnouncementKeys: unreadState.unreadAnnouncementKeys,

    // Required modal state
    requiredDialogOpen,

    // Popover state
    popoverOpen,
    setPopoverOpen: handlePopoverOpenChange,
    activeTab,
    setActiveTab: handleTabChange,

    // Actions
    openPopover: handleOpenPopover,
    closePopover: () => setPopoverOpen(false),
    confirmRead,
    refetchNotice,
  }
}
