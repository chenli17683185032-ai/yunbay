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
import { generateAndCopyQuickStartApiKey } from './quick-start-api-key'

test('one-click API key creation reveals and copies the new key', async () => {
  let createdPayload: ApiKeyFormData | undefined
  let copiedText = ''

  const result = await generateAndCopyQuickStartApiKey({
    now: () => 1_700_000_000_000,
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
  assert.equal(result.fullKey, 'sk-generated-key')
  assert.equal(copiedText, 'sk-generated-key')
})

test('one-click API key creation reports clipboard failure', async () => {
  await assert.rejects(
    generateAndCopyQuickStartApiKey({
      now: () => 1,
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
