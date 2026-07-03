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
  Data: 'Data',
  API: 'API',
  Wallet: '钱包',
  'Value Packages': '超值套餐',
  Account: '账户',
  General: 'General',
  Admin: 'Admin',
}

const t = (value: string) => zh[value] ?? value

function findValuePackageItem(role: number) {
  const items = buildSidebarData(t, role).navGroups.flatMap((group) => group.items)
  const item = items.find((entry) => 'url' in entry && entry.url === '/value-packages')
  assert.ok(item)
  return item
}

test('ordinary value package sidebar entry is attention marked', () => {
  const item = findValuePackageItem(1)
  assert.equal('attention' in item ? item.attention : undefined, 'value-packages')
})

test('admin value package sidebar entry is attention marked', () => {
  const item = findValuePackageItem(10)
  assert.equal('attention' in item ? item.attention : undefined, 'value-packages')
})

test('ordinary users see quick-start and required user functions only', () => {
  const groups = buildSidebarData(t, 1).navGroups
  const items = groups.flatMap((group) => group.items)

  assert.deepEqual(
    groups.map((group) => group.title),
    ['开始', 'AI 使用', 'Data', 'API', '钱包', '账户']
  )
  assert.equal(
    groups.some((group) => group.id === 'admin'),
    false
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/quick-start'),
    true
  )
  assert.deepEqual(
    items.map((item) => item.title),
    [
      'Quick Start',
      'Playground',
      'Dashboard',
      'Usage Logs',
      'API Keys',
      '超值套餐',
      'Wallet / Top up',
      'Profile',
    ]
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/dashboard/models'),
    true
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/value-packages'),
    true
  )
  assert.equal(
    items.some((item) => 'type' in item && item.type === 'chat-presets'),
    false
  )
  assert.equal(
    items.some(
      (item) => 'url' in item && item.url === '/wallet?section=redeem'
    ),
    false
  )
  assert.equal(
    items.some((item) => item.title === 'Redeem codes'),
    false
  )
  assert.equal(
    items.some((item) => item.title === 'Rankings'),
    false
  )
  assert.equal(
    items.some((item) => item.title === 'Docs'),
    false
  )
  assert.equal(
    items.some((item) => item.title === 'About'),
    false
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/order-management'),
    false
  )
})

test('admin users keep admin functions but lose the chat preset widget', () => {
  const groups = buildSidebarData(t, 10).navGroups
  const items = groups.flatMap((group) => group.items)

  assert.equal(
    groups.some((group) => group.id === 'admin'),
    true
  )
  assert.equal(
    groups.some((group) => group.title === 'General'),
    true
  )
  assert.equal(
    groups.some((group) => group.title === 'AI 使用'),
    true
  )
  assert.equal(
    items.some((item) => 'type' in item && item.type === 'chat-presets'),
    false
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/channels'),
    true
  )
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/order-management'),
    true
  )
})
