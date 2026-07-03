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

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface SubscriptionPlanLike {
  id: number
  title: string
  subtitle?: string
  price_amount: number
  currency: string
  duration_unit: 'year' | 'month' | 'day' | 'hour' | 'custom' | string
  duration_value: number
  custom_seconds: number
  enabled: boolean
  sort_order: number
  plan_kind: string
  package_type?: string
  package_level: number
  model_group?: string
  concurrency_limit: number
  limit_5h_amount: number
  limit_7d_amount: number
  benefits?: string
  ldxp_product_url?: string
  ldxp_product_name?: string
  ldxp_product_amount: number
  ldxp_product_ref?: string
  ldxp_session_ttl_seconds: number
  allow_balance_pay?: boolean
  stripe_price_id?: string
  creem_product_id?: string
  waffo_pancake_product_id?: string
  max_purchase_per_user: number
  upgrade_group?: string
  total_amount: number
  quota_reset_period?: 'never' | 'daily' | 'weekly' | 'monthly' | 'custom' | string
  quota_reset_custom_seconds?: number
  created_at: number
  updated_at: number
}

export interface UserSubscription {
  id: number
  user_id: number
  plan_id: number
  amount_total: number
  amount_used: number
  start_time: number
  end_time: number
  status: string
  source?: string
  last_reset_time?: number
  next_reset_time?: number
  covered_by_subscription_id?: number
  covered_time?: number
  upgrade_group?: string
  prev_user_group?: string
  created_at: number
  updated_at: number
}

export interface UserValuePackagePreference {
  id: number
  user_id: number
  enabled: boolean
  active_user_subscription_id: number
  created_at: number
  updated_at: number
}

export interface ValuePackageState {
  preference: UserValuePackagePreference
  subscription?: UserSubscription | null
  plan?: SubscriptionPlanLike | null
}

export interface ValuePackagePlanRecord {
  plan: SubscriptionPlanLike
}

export type ValuePackagePlansResponse = ApiResponse<{
  plans: ValuePackagePlanRecord[]
  state: ValuePackageState | null
}>

export interface ValuePackagePurchaseIntent {
  action: string
  requires_confirmation: boolean
  current_subscription?: UserSubscription | null
  current_plan?: SubscriptionPlanLike | null
  target_plan?: SubscriptionPlanLike | null
  message?: string
}

export interface ValuePackageLdxpSession {
  session_id: string
  amount: number
  money: number
  status: string
  qr_code?: string
  worker_order_no?: string
  expires_at: number
  poll_interval_ms?: number
  error_code?: string
  error_message?: string
}

export type ValuePackageLdxpSessionResponse = ApiResponse<{
  session: ValuePackageLdxpSession
  order_id: number
  trade_no: string
}>
