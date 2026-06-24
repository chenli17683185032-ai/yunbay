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

export type LegacyConsoleLogSearch = Record<string, unknown>

export interface UsageLogsCommonSearch {
  page: number
  pageSize?: number
  type?: string[]
  model?: string
  token?: string
  channel?: string
  group?: string
  username?: string
  requestId?: string
  upstreamRequestId?: string
  startTime?: number
  endTime?: number
}

function firstValue(value: unknown): unknown {
  return Array.isArray(value) ? value[0] : value
}

function getString(value: unknown): string | undefined {
  const raw = firstValue(value)
  if (raw == null) return undefined
  const text = String(raw).trim()
  return text === '' ? undefined : text
}

function getNumber(value: unknown): number | undefined {
  const raw = getString(value)
  if (raw == null) return undefined
  const num = Number(raw)
  return Number.isFinite(num) ? num : undefined
}

function getSecondsAsMilliseconds(value: unknown): number | undefined {
  const seconds = getNumber(value)
  return seconds == null ? undefined : seconds * 1000
}

export function legacyConsoleLogSearchToUsageLogsSearch(
  search: LegacyConsoleLogSearch
): UsageLogsCommonSearch {
  const page = getNumber(search.p) ?? getNumber(search.page) ?? 1
  const pageSize = getNumber(search.page_size) ?? getNumber(search.pageSize)
  const type = getString(search.type)
  const model = getString(search.model_name) ?? getString(search.model)
  const token = getString(search.token_name) ?? getString(search.token)
  const channel = getString(search.channel)
  const group = getString(search.group)
  const username = getString(search.username)
  const requestId = getString(search.request_id) ?? getString(search.requestId)
  const upstreamRequestId =
    getString(search.upstream_request_id) ?? getString(search.upstreamRequestId)
  const startTime =
    getSecondsAsMilliseconds(search.start_timestamp) ??
    getNumber(search.startTime)
  const endTime =
    getSecondsAsMilliseconds(search.end_timestamp) ?? getNumber(search.endTime)

  return {
    page,
    ...(pageSize != null ? { pageSize } : {}),
    ...(type ? { type: [type] } : {}),
    ...(model ? { model } : {}),
    ...(token ? { token } : {}),
    ...(channel ? { channel } : {}),
    ...(group ? { group } : {}),
    ...(username ? { username } : {}),
    ...(requestId ? { requestId } : {}),
    ...(upstreamRequestId ? { upstreamRequestId } : {}),
    ...(startTime != null ? { startTime } : {}),
    ...(endTime != null ? { endTime } : {}),
  }
}
