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
  getApiKeyFormDefaultValues,
  resolveApiKeyCreateGroup,
  transformFormDataToPayload,
} from './api-key-form'

test('new API key defaults to the gpt-plus token group', () => {
  assert.equal(getApiKeyFormDefaultValues(false).group, 'gpt-plus')
})

test('new API key still defaults to gpt-plus when site auto group default is active', () => {
  const defaults = getApiKeyFormDefaultValues(true)

  assert.equal(defaults.group, 'gpt-plus')
  assert.equal(defaults.cross_group_retry, false)
})

test('API key payload preserves the gpt-plus token group and keeps cross-group retry disabled', () => {
  const payload = transformFormDataToPayload({
    ...getApiKeyFormDefaultValues(false),
    name: 'default-api-key',
  })

  assert.equal(payload.group, 'gpt-plus')
  assert.equal(payload.cross_group_retry, false)
})

test('API key group resolver prefers gpt-plus over default auto and website user group', () => {
  assert.equal(
    resolveApiKeyCreateGroup({
      availableGroups: ['default', 'auto', '体验用户', 'gpt-plus'],
      currentGroup: '体验用户',
    }),
    'gpt-plus'
  )
})

test('API key group resolver refuses silent fallback when gpt-plus is unavailable', () => {
  assert.equal(
    resolveApiKeyCreateGroup({
      availableGroups: ['default', 'auto', '体验用户'],
      currentGroup: 'missing',
    }),
    ''
  )
})

test('API key payload preserves explicit auto and cross-group retry', () => {
  const payload = transformFormDataToPayload({
    ...getApiKeyFormDefaultValues(false),
    name: 'auto-api-key',
    group: 'auto',
    cross_group_retry: true,
  })

  assert.equal(payload.group, 'auto')
  assert.equal(payload.cross_group_retry, true)
})
