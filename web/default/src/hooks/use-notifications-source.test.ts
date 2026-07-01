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
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./use-notifications.ts', import.meta.url), 'utf8')

function functionBody(name: string) {
  const marker = `const ${name} = `
  const start = source.indexOf(marker)
  assert.notEqual(start, -1, `${name} should exist`)
  const nextConst = source.indexOf('\n  const ', start + marker.length)
  const returnIndex = source.indexOf('\n  return {', start + marker.length)
  const endCandidates = [nextConst, returnIndex].filter((index) => index !== -1)
  const end = Math.min(...endCandidates)
  return source.slice(start, end)
}

test('opening notification popover does not mark content as read', () => {
  const body = functionBody('handleOpenPopover')

  assert.doesNotMatch(body, /markNoticeRead\(/)
  assert.doesNotMatch(body, /markAnnouncementsRead\(/)
})

test('switching notification tabs does not mark announcements as read', () => {
  const body = functionBody('handleTabChange')

  assert.doesNotMatch(body, /markNoticeRead\(/)
  assert.doesNotMatch(body, /markAnnouncementsRead\(/)
})

test('explicit confirmation is the only hook action that marks notifications read', () => {
  const body = functionBody('confirmRead')

  assert.match(body, /markNoticeRead\(noticeContent\)/)
  assert.match(body, /markAnnouncementsRead\(unreadState\.unreadAnnouncementKeys\)/)
  assert.match(source, /requiredDialogOpen/)
})
