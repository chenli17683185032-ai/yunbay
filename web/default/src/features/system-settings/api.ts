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
import { api, type ApiRequestConfig } from '@/lib/api'
import {
  GROUP_RATIO_REQUEST_CONFIG,
  requireGroupRatioOptionsData,
} from './models/group-ratio-save'
import type {
  ConfirmPaymentComplianceResponse,
  DeleteLogsResponse,
  FetchUpstreamRatiosRequest,
  GroupRatioOptionsResponse,
  ModelPriceSyncRequest,
  ModelPriceSyncResponse,
  SystemOptionsResponse,
  UpdateGroupRatioOptionsRequest,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  return res.data
}

export async function updateSystemOption(
  request: UpdateOptionRequest,
  config?: ApiRequestConfig
) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/',
    request,
    config
  )
  return res.data
}

export async function getGroupRatioOptions() {
  const res = await api.get<GroupRatioOptionsResponse>(
    '/api/option/group-ratios',
    GROUP_RATIO_REQUEST_CONFIG
  )
  requireGroupRatioOptionsData(res.data)
  return res.data
}

export async function updateGroupRatioOptions(
  request: UpdateGroupRatioOptionsRequest
) {
  const res = await api.put<GroupRatioOptionsResponse>(
    '/api/option/group-ratios',
    request,
    GROUP_RATIO_REQUEST_CONFIG
  )
  requireGroupRatioOptionsData(res.data)
  return res.data
}

export async function confirmPaymentCompliance() {
  const res = await api.post<ConfirmPaymentComplianceResponse>(
    '/api/option/payment_compliance',
    { confirmed: true }
  )
  return res.data
}

export async function deleteLogsBefore(targetTimestamp: number) {
  const res = await api.delete<DeleteLogsResponse>('/api/log/', {
    params: { target_timestamp: targetTimestamp },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

export async function previewModelPriceSync(request: ModelPriceSyncRequest) {
  const res = await api.post<ModelPriceSyncResponse>(
    '/api/ratio_sync/model_price/preview',
    request
  )
  return res.data
}

export async function applyModelPriceSync(request: ModelPriceSyncRequest) {
  const res = await api.post<ModelPriceSyncResponse>(
    '/api/ratio_sync/model_price/apply',
    request
  )
  return res.data
}
