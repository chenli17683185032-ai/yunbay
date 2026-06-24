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
import type { ApiKeyFormData } from '@/features/keys/types'

type ApiResult = {
  success: boolean
  message?: string
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

export function getQuickStartApiKeyGroup(options: {
  defaultUseAutoGroup: boolean
  availableGroups: string[]
  preferredGroup?: string
}): { group: string; crossGroupRetry: boolean } {
  const availableGroups = options.availableGroups
    .map((group) => group.trim())
    .filter(Boolean)
  const preferredGroup = options.preferredGroup?.trim()

  if (
    preferredGroup &&
    availableGroups.some((group) => group === preferredGroup)
  ) {
    return { group: preferredGroup, crossGroupRetry: false }
  }

  if (
    options.defaultUseAutoGroup &&
    availableGroups.some((group) => group === 'auto')
  ) {
    return { group: 'auto', crossGroupRetry: true }
  }

  const defaultGroup = availableGroups.find((group) => group === 'default')
  if (defaultGroup) {
    return { group: defaultGroup, crossGroupRetry: false }
  }

  return {
    group: availableGroups[0] || '',
    crossGroupRetry: false,
  }
}

export async function generateAndCopyQuickStartApiKey(
  dependencies: QuickStartApiKeyDependencies
): Promise<{ name: string; fullKey: string }> {
  const now = dependencies.now || Date.now
  const name = `yunbay-quick-start-${now()}`
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

  const fullKey = key.startsWith('sk-') ? key : `sk-${key}`
  if (!(await dependencies.copyToClipboard(fullKey))) {
    throw new Error('Failed to copy the new API key')
  }

  return { name, fullKey }
}
