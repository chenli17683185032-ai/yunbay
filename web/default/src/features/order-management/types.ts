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

export type MailCheckStatus =
  | 'not_required'
  | 'pending'
  | 'waiting_mail'
  | 'checking'
  | 'verified'
  | 'order_mismatch'
  | 'amount_mismatch'
  | 'mail_parse_failed'
  | 'mail_fetch_failed'
  | 'timeout'

export type DateRangeKey = '7d' | '30d' | 'custom'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface OrderManagementSummary {
  site_amount: number
  external_paid_amount: number
  order_count: number
  mail_verified_count: number
  mail_pending_count: number
  mail_error_count: number
  mail_verified_rate: number
  affiliate_user_count: number
  affiliate_amount: number
  withdrawal_pending_count: number
  withdrawal_pending_amount: number
}

export interface OrderDailyPoint {
  date: string
  site_amount: number
  external_paid_amount: number
  order_count: number
  mail_verified_count: number
  mail_error_count: number
}

export interface OrderAnalyticsResponse {
  summary: OrderManagementSummary
  daily: OrderDailyPoint[]
}

export interface OrderManagementAffiliateBrief {
  inviter_user_id: number
  commission_money: number
  status: string
}

export interface OrderManagementOrderItem {
  id: number
  order_type: string
  session_id: string
  user_id: number
  username: string
  site_amount: number
  external_paid_amount: number
  worker_order_no: string
  mail_order_no: string
  mail_paid_amount: number
  mail_status: MailCheckStatus
  mail_status_text: string
  error_code: string
  error_message: string
  affiliate: OrderManagementAffiliateBrief | null
  created_time: number
  verified_time: number
}

export interface PageData<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

export interface MailCheckResponse {
  job_id: string
  started: boolean
  affected_count: number
}

export type MailCheckJobStatusValue = 'running' | 'finished' | 'failed'

export interface MailCheckJobStatus {
  job_id: string
  status: MailCheckJobStatusValue
  affected_count: number
  error_message: string
  created_time: number
  finished_time: number
}

export interface MailCheckPayload {
  range?: DateRangeKey
  scope?: string
  limit?: number
  start_time?: string
  end_time?: string
}

export type AffiliateWithdrawalStatus = 'pending' | 'paid' | 'rejected'

export interface AffiliateWithdrawal {
  id: number
  withdrawal_id: string
  amount: number
  contact: string
  remark: string
  status: AffiliateWithdrawalStatus
  created_time: number
  admin_remark: string
  processed_time: number
}

export interface AffiliateStatsSummary {
  affiliate_user_count: number
  period_commission_amount: number
  pending_withdrawal_user_count: number
  pending_withdrawal_amount: number
  available_without_withdrawal_user_count: number
}

export interface AffiliateStatsItem {
  user_id: number
  username: string
  period_commission_amount: number
  total_commission_amount: number
  available_amount: number
  withdrawn_amount: number
  withdrawal: AffiliateWithdrawal | null
}


export interface AffiliateSourceOrder {
  order_time: number
  invitee_user_id: number
  invitee_username: string
  trade_no: string
  worker_order_no: string
  base_money: number
  rate_bps: number
  commission_money: number
  mail_status: MailCheckStatus
}

export interface AffiliateStatsResponse {
  summary: AffiliateStatsSummary
  items: AffiliateStatsItem[]
  total: number
}
