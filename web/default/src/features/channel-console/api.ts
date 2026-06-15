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

import { api } from '@/lib/api'
import type {
  ApiResponse,
  ChannelConsoleDetail,
  ChannelConsoleHealthCheck,
  ChannelConsoleListResult,
  ImportCommitRequest,
  ImportCommitResult,
  ImportPreview,
} from './types'

export interface ChannelConsoleListParams {
  p?: number
  page_size?: number
}

export async function previewChannelConsoleImport(
  rawInput: string
): Promise<ApiResponse<ImportPreview>> {
  const res = await api.post('/api/channel-console/import/preview', {
    raw_input: rawInput,
  })
  return res.data
}

export async function commitChannelConsoleImport(
  payload: ImportCommitRequest
): Promise<ApiResponse<ImportCommitResult>> {
  const res = await api.post('/api/channel-console/import/commit', payload)
  return res.data
}

export async function listChannelConsoleChannels(
  params: ChannelConsoleListParams = {}
): Promise<ApiResponse<ChannelConsoleListResult>> {
  const res = await api.get('/api/channel-console/channels', { params })
  return res.data
}

export async function getChannelConsoleDetail(
  id: number
): Promise<ApiResponse<ChannelConsoleDetail>> {
  const res = await api.get(`/api/channel-console/channels/${id}`)
  return res.data
}

export async function checkChannelConsoleHealth(
  id: number
): Promise<ApiResponse<ChannelConsoleHealthCheck>> {
  const res = await api.post(`/api/channel-console/channels/${id}/health-check`)
  return res.data
}
