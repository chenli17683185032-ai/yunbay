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
  AffiliateSourceOrder,
  AffiliateStatsResponse,
  AffiliateWithdrawal,
  AffiliateWithdrawalStatus,
  ApiResponse,
  DateRangeKey,
  MailCheckJobStatus,
  MailCheckPayload,
  MailCheckResponse,
  MailCheckStatus,
  OrderAnalyticsResponse,
  OrderManagementOrderItem,
  OrderManagementValuePackagePlanRecord,
  PageData,
} from './types'

export interface OrderManagementRangeParams {
  range?: DateRangeKey
  start_time?: string | number
  end_time?: string | number
}

export interface GetOrderManagementOrdersParams extends OrderManagementRangeParams {
  page?: number
  page_size?: number
  mail_status?: MailCheckStatus
  keyword?: string
}

export interface GetAffiliateStatsParams extends OrderManagementRangeParams {
  page?: number
  page_size?: number
  withdrawal_status?: AffiliateWithdrawalStatus
}

export function withDefinedParams(params: object): string {
  const query = new URLSearchParams()

  Object.entries(params as Record<string, unknown>).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return
    query.set(key, String(value))
  })

  const value = query.toString()
  return value ? `?${value}` : ''
}

export async function getOrderAnalytics(
  params: OrderManagementRangeParams = {}
): Promise<ApiResponse<OrderAnalyticsResponse>> {
  const res = await api.get(
    `/api/order-management/admin/analytics${withDefinedParams(params)}`
  )
  return res.data
}

export async function getOrderManagementOrders(
  params: GetOrderManagementOrdersParams = {}
): Promise<ApiResponse<PageData<OrderManagementOrderItem>>> {
  const res = await api.get(
    `/api/order-management/admin/orders${withDefinedParams(params)}`
  )
  return res.data
}

export async function getOrderManagementValuePackagePlans(): Promise<
  ApiResponse<OrderManagementValuePackagePlanRecord[]>
> {
  const res = await api.get('/api/subscription/admin/plans')
  return res.data
}

export async function startSingleMailCheck(
  orderId: number
): Promise<ApiResponse<MailCheckResponse>> {
  const res = await api.post(
    `/api/order-management/admin/orders/${orderId}/mail-check`
  )
  return res.data
}

export async function startBatchMailCheck(
  payload: MailCheckPayload
): Promise<ApiResponse<MailCheckResponse>> {
  const normalizedPayload = {
    ...payload,
    start_time:
      payload.start_time === undefined ? undefined : String(payload.start_time),
    end_time:
      payload.end_time === undefined ? undefined : String(payload.end_time),
  }
  const res = await api.post(
    '/api/order-management/admin/mail-check',
    normalizedPayload
  )
  return res.data
}

export async function getMailCheckJob(
  jobId: string
): Promise<ApiResponse<MailCheckJobStatus>> {
  const res = await api.get(`/api/order-management/admin/mail-check/${jobId}`, {
    disableDuplicate: true,
  })
  return res.data
}

export async function getAffiliateStats(
  params: GetAffiliateStatsParams = {}
): Promise<ApiResponse<AffiliateStatsResponse>> {
  const res = await api.get(
    `/api/order-management/admin/affiliate-stats${withDefinedParams(params)}`
  )
  return res.data
}

export async function getAffiliateSourceOrders(
  userId: number,
  params: OrderManagementRangeParams & { limit?: number } = {}
): Promise<ApiResponse<AffiliateSourceOrder[]>> {
  const res = await api.get(
    `/api/order-management/admin/affiliate-stats/${userId}/source-orders${withDefinedParams(params)}`
  )
  return res.data
}

export async function markWithdrawalPaid(
  id: number,
  adminRemark: string
): Promise<ApiResponse<AffiliateWithdrawal>> {
  const res = await api.post(
    `/api/order-management/admin/affiliate-withdrawals/${id}/paid`,
    { admin_remark: adminRemark }
  )
  return res.data
}

export async function rejectWithdrawal(
  id: number,
  adminRemark: string
): Promise<ApiResponse<AffiliateWithdrawal>> {
  const res = await api.post(
    `/api/order-management/admin/affiliate-withdrawals/${id}/reject`,
    { admin_remark: adminRemark }
  )
  return res.data
}

export async function deleteBillingOrder(request: {
  order_type: 'topup' | 'subscription'
  trade_no: string
  reason?: string
}): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/order-management/admin/billing-orders/${encodeURIComponent(request.order_type)}/${encodeURIComponent(request.trade_no)}`,
    { data: { reason: request.reason || '' } }
  )
  return res.data
}
