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
import { buildSidebarData } from './sidebar-data-model'

const zh: Record<string, string> = {
  'Getting Started': '开始',
  'AI Usage': 'AI 使用',
  API: 'API',
  Wallet: '钱包',
  Account: '账户',
  General: 'General',
  Admin: 'Admin',
}

const t = (value: string) => zh[value] ?? value

test('ordinary users see quick-start and required user functions only', () => {
  const groups = buildSidebarData(t, 1).navGroups
  const items = groups.flatMap((group) => group.items)

  assert.deepEqual(groups.map((group) => group.title), [
    '开始',
    'AI 使用',
    'API',
    '钱包',
    '账户',
  ])
  assert.equal(groups.some((group) => group.id === 'admin'), false)
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/quick-start'),
    true
  )
  assert.deepEqual(
    items.map((item) => item.title),
    [
      'Quick Start',
      'Playground',
      'Chat',
      'API Keys',
      'Usage Logs',
      'Wallet / Top up',
      'Redeem codes',
      'Profile',
    ]
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/wallet?section=redeem'),
    true
  )
})

test('admin users keep the existing admin group', () => {
  const groups = buildSidebarData(t, 10).navGroups

  assert.equal(groups.some((group) => group.id === 'admin'), true)
  assert.equal(groups.some((group) => group.title === 'General'), true)
})
