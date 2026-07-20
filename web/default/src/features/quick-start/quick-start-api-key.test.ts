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
  recoverLatestQuickStartApiKey,
} from './quick-start-api-key'

test('quick start API key group requires gpt-plus instead of falling back to another user-available group', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: false,
      availableGroups: ['plus'],
    }),
    {
      group: '',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group does not let auto override gpt-plus', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      availableGroups: ['plus'],
    }),
    {
      group: '',
      crossGroupRetry: false,
    }
  )

  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      availableGroups: ['plus', 'auto', 'gpt-plus'],
    }),
    {
      group: 'gpt-plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group does not use the website user group as token group', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      preferredGroup: '体验用户',
      availableGroups: ['auto', 'gpt-plus', '体验用户'],
    }),
    {
      group: 'gpt-plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group prefers gpt-plus over a non-default fallback', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: false,
      availableGroups: ['default', 'plus', 'gpt-plus'],
    }),
    {
      group: 'gpt-plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group prefers gpt-plus over the current user group when selectable', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: false,
      preferredGroup: '体验用户',
      availableGroups: ['default', 'gpt-plus', '体验用户'],
    }),
    {
      group: 'gpt-plus',
      crossGroupRetry: false,
    }
  )
})

test('quick start API key group prefers gpt-plus even when auto is enabled', () => {
  assert.deepEqual(
    getQuickStartApiKeyGroup({
      defaultUseAutoGroup: true,
      availableGroups: ['auto', 'gpt-plus'],
    }),
    {
      group: 'gpt-plus',
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

test('quick start recovers the newest enabled onboarding key without creating another one', async () => {
  let revealedId = 0
  const result = await recoverLatestQuickStartApiKey({
    searchApiKeys: async ({ keyword }) => ({
      success: true,
      data: {
        items: [
          { id: 1, name: `${keyword}1700000000000`, status: 1 },
          { id: 2, name: `${keyword}1800000000000`, status: 2 },
          { id: 3, name: `${keyword}1750000000000`, status: 1 },
          { id: 4, name: 'unrelated-key', status: 1 },
          { id: 5, name: `${keyword}1900000000000`, status: 3 },
          { id: 6, name: `${keyword}2000000000000`, status: 4 },
        ],
      },
    }),
    fetchTokenKey: async (id) => {
      revealedId = id
      return { success: true, data: { key: 'existing-key' } }
    },
  })

  assert.equal(revealedId, 3)
  assert.deepEqual(result, {
    name: 'yunbay-quick-start-1750000000000',
    fullKey: 'sk-existing-key',
    copied: false,
  })
})

test('quick start recovery returns null when no reusable key is available', async () => {
  const result = await recoverLatestQuickStartApiKey({
    searchApiKeys: async () => ({
      success: true,
      data: {
        items: [
          { id: 1, name: 'yunbay-quick-start-1', status: 2 },
          { id: 3, name: 'yunbay-quick-start-3', status: 3 },
          { id: 4, name: 'yunbay-quick-start-4', status: 4 },
          { id: 2, name: 'other-key', status: 1 },
        ],
      },
    }),
    fetchTokenKey: async () => {
      throw new Error('fetch should not be called')
    },
  })

  assert.equal(result, null)
})

test('one-click API key creation reveals and copies the new key', async () => {
  let createdPayload: ApiKeyFormData | undefined
  let copiedText = ''

  const result = await generateAndCopyQuickStartApiKey({
    now: () => 1_700_000_000_000,
    defaultGroup: 'gpt-plus',
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
  assert.equal(createdPayload?.group, 'gpt-plus')
  assert.equal(createdPayload?.cross_group_retry, false)
  assert.equal(result.fullKey, 'sk-generated-key')
  assert.equal(result.copied, true)
  assert.equal(copiedText, 'sk-generated-key')
})

test('one-click API key creation allows explicit auto group when already resolved upstream', async () => {
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

test('one-click API key creation returns generated key when clipboard copy fails', async () => {
  const result = await generateAndCopyQuickStartApiKey({
    now: () => 1,
    defaultGroup: 'gpt-plus',
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
  })

  assert.equal(result.name, 'yunbay-quick-start-1')
  assert.equal(result.fullKey, 'sk-key')
  assert.equal(result.copied, false)
})

test('one-click API key creation returns generated key when clipboard copy rejects', async () => {
  const result = await generateAndCopyQuickStartApiKey({
    now: () => 2,
    defaultGroup: 'gpt-plus',
    createApiKey: async () => ({ success: true }),
    searchApiKeys: async ({ keyword }) => ({
      success: true,
      data: { items: [{ id: 8, name: keyword || '' }] },
    }),
    fetchTokenKey: async () => ({
      success: true,
      data: { key: 'rejected-key' },
    }),
    copyToClipboard: async () => {
      throw new Error('clipboard denied')
    },
  })

  assert.equal(result.name, 'yunbay-quick-start-2')
  assert.equal(result.fullKey, 'sk-rejected-key')
  assert.equal(result.copied, false)
})
