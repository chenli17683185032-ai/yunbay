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
import { DEFAULT_API_KEY_GROUP } from '@/features/keys/constants'
import type { ApiKeyFormData } from '@/features/keys/types'

type ApiResult = {
  success: boolean
  message?: string
}

export const QUICK_START_API_KEY_NAME_PREFIX = 'yunbay-quick-start-'

export type QuickStartApiKeyResult = {
  name: string
  fullKey: string
  copied: boolean
}

type QuickStartApiKeyDependencies = {
  now?: () => number
  defaultGroup: string
  crossGroupRetry?: boolean
  createApiKey: (payload: ApiKeyFormData) => Promise<ApiResult>
  searchApiKeys: (params: {
    keyword: string
    p: number
    size: number
  }) => Promise<
    ApiResult & {
      data?: { items: Array<{ id: number; name: string }> }
    }
  >
  fetchTokenKey: (id: number) => Promise<ApiResult & { data?: { key: string } }>
  copyToClipboard: (text: string) => Promise<boolean>
}

type QuickStartApiKeyCandidate = {
  id: number
  name: string
  created_time?: number
  status?: number
}

type RecoverQuickStartApiKeyDependencies = {
  searchApiKeys: (params: {
    keyword: string
    p: number
    size: number
  }) => Promise<
    ApiResult & {
      data?: { items: QuickStartApiKeyCandidate[] }
    }
  >
  fetchTokenKey: (id: number) => Promise<ApiResult & { data?: { key: string } }>
}

export function getQuickStartApiKeyGroup(options: {
  defaultUseAutoGroup: boolean
  availableGroups: string[]
  preferredGroup?: string
}): { group: string; crossGroupRetry: boolean } {
  const availableGroups = options.availableGroups
    .map((group) => group.trim())
    .filter(Boolean)

  if (availableGroups.includes(DEFAULT_API_KEY_GROUP)) {
    return { group: DEFAULT_API_KEY_GROUP, crossGroupRetry: false }
  }

  return {
    group: '',
    crossGroupRetry: false,
  }
}

function normalizeFullApiKey(key: string): string {
  const normalized = key.trim()
  if (!normalized) return ''
  return normalized.startsWith('sk-') ? normalized : `sk-${normalized}`
}

function getQuickStartKeyTimestamp(
  candidate: QuickStartApiKeyCandidate
): number {
  const suffix = candidate.name.slice(QUICK_START_API_KEY_NAME_PREFIX.length)
  const timestampFromName = Number(suffix)
  if (Number.isFinite(timestampFromName)) return timestampFromName
  return Number(candidate.created_time || 0)
}

export async function recoverLatestQuickStartApiKey(
  dependencies: RecoverQuickStartApiKeyDependencies
): Promise<QuickStartApiKeyResult | null> {
  const searched = await dependencies.searchApiKeys({
    keyword: QUICK_START_API_KEY_NAME_PREFIX,
    p: 1,
    size: 50,
  })
  if (!searched.success) return null

  const candidate = (searched.data?.items ?? [])
    .filter(
      (item) =>
        item.name.startsWith(QUICK_START_API_KEY_NAME_PREFIX) &&
        item.status === 1
    )
    .sort(
      (left, right) =>
        getQuickStartKeyTimestamp(right) - getQuickStartKeyTimestamp(left)
    )[0]
  if (!candidate) return null

  const revealed = await dependencies.fetchTokenKey(candidate.id)
  const fullKey = normalizeFullApiKey(revealed.data?.key || '')
  if (!revealed.success || !fullKey) return null

  return {
    name: candidate.name,
    fullKey,
    copied: false,
  }
}

export async function generateAndCopyQuickStartApiKey(
  dependencies: QuickStartApiKeyDependencies
): Promise<QuickStartApiKeyResult> {
  const now = dependencies.now || Date.now
  const name = `${QUICK_START_API_KEY_NAME_PREFIX}${now()}`
  const group = dependencies.defaultGroup.trim()
  if (!group) {
    throw new Error('No available group for the new API key')
  }

  const created = await dependencies.createApiKey({
    name,
    remain_quota: 0,
    expired_time: -1,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    group,
    cross_group_retry:
      group === 'auto' ? dependencies.crossGroupRetry === true : false,
  })

  if (!created.success) {
    throw new Error(created.message || 'Failed to create API key')
  }

  const searched = await dependencies.searchApiKeys({
    keyword: name,
    p: 1,
    size: 10,
  })
  const token = searched.data?.items.find((item) => item.name === name)
  if (!searched.success || !token) {
    throw new Error(searched.message || 'Failed to find the new API key')
  }

  const revealed = await dependencies.fetchTokenKey(token.id)
  const key = revealed.data?.key?.trim()
  if (!revealed.success || !key) {
    throw new Error(revealed.message || 'Failed to reveal the new API key')
  }

  const fullKey = normalizeFullApiKey(key)
  let copied: boolean

  try {
    copied = await dependencies.copyToClipboard(fullKey)
  } catch {
    copied = false
  }

  return { name, fullKey, copied }
}
