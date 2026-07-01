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

export type AnnouncementRecord = Record<string, unknown>

export interface UnreadNotificationInput {
  noticeContent: string
  announcements: AnnouncementRecord[]
  lastReadNotice: string
  readAnnouncementKeys: string[]
}

export interface UnreadNotificationState {
  noticeUnread: boolean
  unreadAnnouncementKeys: string[]
  unreadCounts: {
    notice: number
    announcements: number
    total: number
  }
  hasDisplayableContent: boolean
}

export function normalizeNoticeContent(noticeContent: string): string {
  return noticeContent.trim()
}

export function hashString(input: string): string {
  let hash = 0
  if (!input) return '0'

  for (let i = 0; i < input.length; i += 1) {
    const chr = input.charCodeAt(i)
    hash = (hash << 5) - hash + chr
    hash |= 0
  }

  return hash.toString(36)
}

/**
 * Generate a unique key for an announcement.
 * Production currently provides stable numeric ids; the hash fallback keeps
 * imported or legacy items without ids readable and change-sensitive.
 */
export function getAnnouncementKey(item: AnnouncementRecord): string {
  if (!item) return ''

  if (item.id !== undefined && item.id !== null) {
    return `id:${item.id}`
  }

  const fingerprint = JSON.stringify({
    publishDate: (item?.publishDate as string) || '',
    content: ((item?.content as string) || '').trim(),
    extra: ((item?.extra as string) || '').trim(),
    type: (item?.type as string) || '',
    title: ((item?.title as string) || '').trim(),
    link: ((item?.link as string) || '').trim(),
  })
  return `hash:${hashString(fingerprint)}`
}

export function getUnreadNotificationState({
  noticeContent,
  announcements,
  lastReadNotice,
  readAnnouncementKeys,
}: UnreadNotificationInput): UnreadNotificationState {
  const normalizedNotice = normalizeNoticeContent(noticeContent)
  const normalizedLastReadNotice = normalizeNoticeContent(lastReadNotice)
  const noticeUnread = Boolean(
    normalizedNotice && normalizedNotice !== normalizedLastReadNotice
  )
  const readKeys = new Set(readAnnouncementKeys)
  const unreadAnnouncementKeys = announcements
    .map((item) => getAnnouncementKey(item))
    .filter((key) => key && !readKeys.has(key))

  const noticeCount = noticeUnread ? 1 : 0
  const announcementCount = unreadAnnouncementKeys.length

  return {
    noticeUnread,
    unreadAnnouncementKeys,
    unreadCounts: {
      notice: noticeCount,
      announcements: announcementCount,
      total: noticeCount + announcementCount,
    },
    hasDisplayableContent: Boolean(normalizedNotice || announcements.length > 0),
  }
}
