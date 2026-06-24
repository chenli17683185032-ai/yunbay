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
import type { ApiKeyFormData } from '@/features/keys/types'
import {
  generateAndCopyQuickStartApiKey,
  getQuickStartApiKeyGroup,
} from './quick-start-api-key'

test('quick start API key group falls back to the first user-available group', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: false,
      availableGroups: ['plus'],
    }),
    {
      group: 'plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group uses auto only when the user can select auto', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      availableGroups: ['plus'],
    }),
    {
      group: 'plus',
      crossGroupRetry: false,
    }
  )

  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      availableGroups: ['plus', 'auto'],
    }),
    {
      group: 'auto',
      crossGroupRetry: true,
    }
  )
})

test('quick start API key group keeps the current user group ahead of auto', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      preferredGroup: 'plus',
      availableGroups: ['auto', 'plus'],
    }),
    {
      group: 'plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group prefers default only when the user can select it', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: false,
      availableGroups: ['plus', 'default'],
    }),
    {
      group: 'default',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group prefers the current user group when it is selectable', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: false,
      preferredGroup: 'plus',
      availableGroups: ['default', 'plus'],
    }),
    {
      group: 'plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key creation refuses to create an unusable key without available groups', async () => {
  await assert.rejects(
    generateAndCopyQuickStartApiKey({
      now: () => 1,
      defaultGroup: '',
      createApiKey: async () => {
        throw new Error('create should not be called')
      },
      searchApiKeys: async () => ({ success: true, data: { items: [] } }),
      fetchTokenKey: async () => ({
        success: true,
        data: { key: 'key' },
      }),
      copyToClipboard: async () => true,
    }),
    /available group/i
  )
})

test('one-click API key creation reveals and copies the new key', async () => {
  let createdPayload: ApiKeyFormData | undefined
  let copiedText = ''

  const result = await generateAndCopyQuickStartApiKey({
    now: () => 1_700_000_000_000,
    defaultGroup: 'plus',
    createApiKey: async (payload) => {
      createdPayload = payload
      return { success: true }
    },
    searchApiKeys: async ({ keyword }) => ({
      success: true,
      data: {
        items: [
          {
            id: 42,
            name: keyword || '',
          },
        ],
      },
    }),
    fetchTokenKey: async (id) => ({
      success: id === 42,
      data: { key: 'generated-key' },
    }),
    copyToClipboard: async (text) => {
      copiedText = text
      return true
    },
  })

  assert.equal(createdPayload?.name, 'yunbay-quick-start-1700000000000')
  assert.equal(createdPayload?.unlimited_quota, true)
  assert.equal(createdPayload?.group, 'plus')
  assert.equal(createdPayload?.cross_group_retry, false)
  assert.equal(result.fullKey, 'sk-generated-key')
  assert.equal(copiedText, 'sk-generated-key')
})

test('one-click API key creation follows the site auto-group default', async () => {
  let createdPayload: ApiKeyFormData | undefined

  await generateAndCopyQuickStartApiKey({
    now: () => 1_700_000_000_000,
    defaultGroup: 'auto',
    crossGroupRetry: true,
    createApiKey: async (payload) => {
      createdPayload = payload
      return { success: true }
    },
    searchApiKeys: async ({ keyword }) => ({
      success: true,
      data: { items: [{ id: 42, name: keyword || '' }] },
    }),
    fetchTokenKey: async () => ({
      success: true,
      data: { key: 'generated-key' },
    }),
    copyToClipboard: async () => true,
  })

  assert.equal(createdPayload?.group, 'auto')
  assert.equal(createdPayload?.cross_group_retry, true)
})

test('one-click API key creation reports clipboard failure', async () => {
  await assert.rejects(
    generateAndCopyQuickStartApiKey({
      now: () => 1,
      defaultGroup: 'plus',
      createApiKey: async () => ({ success: true }),
      searchApiKeys: async ({ keyword }) => ({
        success: true,
        data: { items: [{ id: 7, name: keyword || '' }] },
      }),
      fetchTokenKey: async () => ({
        success: true,
        data: { key: 'key' },
      }),
      copyToClipboard: async () => false,
    }),
    /copy/i
  )
})
