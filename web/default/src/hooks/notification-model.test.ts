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
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  getAnnouncementKey,
  getUnreadNotificationState,
} from './notification-model'

test('announcement key prefers production id when available', () => {
  assert.equal(
    getAnnouncementKey({
      id: 6,
      content: '开放签到，每日签到拿余额',
      extra: '',
      publishDate: '2026-06-28T12:39:59.326Z',
      type: 'default',
    }),
    'id:6'
  )
})

test('announcement key fallback changes when content changes', () => {
  const base = {
    content: 'first announcement',
    extra: '',
    publishDate: '2026-06-29T00:00:00.000Z',
    type: 'default',
  }

  assert.notEqual(
    getAnnouncementKey(base),
    getAnnouncementKey({ ...base, content: 'updated announcement' })
  )
})

test('notice is unread only when trimmed content differs from last read content', () => {
  assert.equal(
    getUnreadNotificationState({
      noticeContent: ' 本站开通签到，每期签到拿余额 ',
      announcements: [],
      lastReadNotice: '本站开通签到，每期签到拿余额',
      readAnnouncementKeys: [],
    }).unreadCounts.notice,
    0
  )

  assert.equal(
    getUnreadNotificationState({
      noticeContent: '本站开通签到，每期签到拿余额',
      announcements: [],
      lastReadNotice: '',
      readAnnouncementKeys: [],
    }).unreadCounts.notice,
    1
  )
})

test('production-shaped announcements return unread id keys', () => {
  const state = getUnreadNotificationState({
    noticeContent: '',
    announcements: [
      {
        id: 6,
        content: '开放签到，每日签到拿余额',
        extra: '',
        publishDate: '2026-06-28T12:39:59.326Z',
        type: 'default',
      },
      {
        id: 5,
        content: '所有新注册用户默认为体验用户小组',
        extra: '',
        publishDate: '2026-06-26T04:58:15.242Z',
        type: 'warning',
      },
    ],
    lastReadNotice: '',
    readAnnouncementKeys: [],
  })

  assert.deepEqual(state.unreadAnnouncementKeys, ['id:6', 'id:5'])
  assert.deepEqual(state.unreadCounts, {
    notice: 0,
    announcements: 2,
    total: 2,
  })
  assert.equal(state.hasDisplayableContent, true)
})

test('read announcement keys are excluded from unread state', () => {
  const state = getUnreadNotificationState({
    noticeContent: '',
    announcements: [
      { id: 6, content: '开放签到，每日签到拿余额' },
      { id: 5, content: '所有新注册用户默认为体验用户小组' },
    ],
    lastReadNotice: '',
    readAnnouncementKeys: ['id:6'],
  })

  assert.deepEqual(state.unreadAnnouncementKeys, ['id:5'])
  assert.deepEqual(state.unreadCounts, {
    notice: 0,
    announcements: 1,
    total: 1,
  })
})
