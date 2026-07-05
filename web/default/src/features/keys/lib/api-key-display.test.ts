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
import { getApiKeyDisplayGroup } from './api-key-display'

test('API key display shows stored token group with active package billing ratio', () => {
  const display = getApiKeyDisplayGroup(
    {
      group: 'gpt-plus',
      effective_group: 'month-card',
      effective_group_ratio: 1,
      cross_group_retry: false,
    },
    { 'gpt-plus': 0.3, 'month-card': 0.8 },
    1
  )

  assert.equal(display.group, 'gpt-plus')
  assert.equal(display.ratio, 1)
  assert.equal(display.isEffective, true)
})

test('API key display falls back to stored group ratio without active package', () => {
  const display = getApiKeyDisplayGroup(
    {
      group: 'gpt-plus',
      effective_group: '',
      effective_group_ratio: undefined,
      cross_group_retry: false,
    },
    { 'gpt-plus': 0.3 }
  )

  assert.equal(display.group, 'gpt-plus')
  assert.equal(display.ratio, 0.3)
  assert.equal(display.isEffective, false)
})

test('API key display shows active package billing ratio for auto group', () => {
  const display = getApiKeyDisplayGroup(
    {
      group: 'auto',
      effective_group: '',
      effective_group_ratio: undefined,
      cross_group_retry: true,
    },
    { auto: 1 },
    1
  )

  assert.equal(display.group, 'auto')
  assert.equal(display.ratio, 1)
  assert.equal(display.isEffective, true)
})
