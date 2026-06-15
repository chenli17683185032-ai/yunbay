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
  ChannelConsoleBatchDeleteResult,
  ChannelConsoleDetail,
  ChannelConsoleHealthCheck,
  ChannelConsoleListResult,
  CredentialBatchDeleteResult,
  CredentialPool,
  CredentialPoolCredential,
  CredentialPoolDetail,
  CredentialPoolKind,
  CredentialPoolListResult,
  CliProxyAuthFilesResult,
  CliProxyAuthURLResult,
  CliProxyStatus,
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

export async function listCredentialPools(): Promise<
  ApiResponse<CredentialPoolListResult>
> {
  const res = await api.get('/api/channel-console/pools')
  return res.data
}

export async function createCredentialPool(payload: {
  name: string
  provider_kind: CredentialPoolKind
  base_url?: string
  provider?: string
}): Promise<ApiResponse<CredentialPool>> {
  const res = await api.post('/api/channel-console/pools', payload)
  return res.data
}

export async function getCredentialPoolDetail(
  id: number
): Promise<ApiResponse<CredentialPoolDetail>> {
  const res = await api.get(`/api/channel-console/pools/${id}`)
  return res.data
}

export async function addThirdPartyCredential(
  poolId: number,
  payload: { api_key: string; display_name?: string }
): Promise<ApiResponse<CredentialPoolCredential>> {
  const res = await api.post(
    `/api/channel-console/pools/${poolId}/credentials/api-key`,
    payload
  )
  return res.data
}

export async function addCliProxyCredential(
  poolId: number,
  payload: { name: string; raw_credential: string }
): Promise<ApiResponse<CredentialPoolCredential>> {
  const res = await api.post(
    `/api/channel-console/pools/${poolId}/credentials/cliproxy-auth`,
    payload
  )
  return res.data
}

export async function batchDeleteCredentials(
  ids: number[]
): Promise<ApiResponse<CredentialBatchDeleteResult>> {
  const res = await api.post('/api/channel-console/credentials/batch-delete', {
    ids,
  })
  return res.data
}

export async function batchDeleteChannelConsoleChannels(
  ids: number[]
): Promise<ApiResponse<ChannelConsoleBatchDeleteResult>> {
  const res = await api.post('/api/channel-console/channels/batch-delete', {
    ids,
  })
  return res.data
}

export async function checkChannelConsoleHealth(
  id: number
): Promise<ApiResponse<ChannelConsoleHealthCheck>> {
  const res = await api.post(`/api/channel-console/channels/${id}/health-check`)
  return res.data
}

export async function getCliProxyStatus(): Promise<ApiResponse<CliProxyStatus>> {
  const res = await api.get('/api/channel-console/cliproxy/status')
  return res.data
}

export async function listCliProxyAuthFiles(): Promise<
  ApiResponse<CliProxyAuthFilesResult>
> {
  const res = await api.get('/api/channel-console/cliproxy/auth-files')
  return res.data
}

export async function uploadCliProxyAuthFile(payload: {
  name: string
  raw_credential: string
}): Promise<ApiResponse<{ status: string }>> {
  const res = await api.post('/api/channel-console/cliproxy/auth-files', payload)
  return res.data
}

export async function deleteCliProxyAuthFiles(
  names: string[]
): Promise<ApiResponse<{ deleted: number; failed: string[] }>> {
  const res = await api.post(
    '/api/channel-console/cliproxy/auth-files/batch-delete',
    { names }
  )
  return res.data
}

export async function getCliProxyAuthURL(
  provider: string
): Promise<ApiResponse<CliProxyAuthURLResult>> {
  const res = await api.get('/api/channel-console/cliproxy/auth-url', {
    params: { provider },
  })
  return res.data
}
