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

export type SubscriptionDurationUnit =
  | 'year'
  | 'month'
  | 'day'
  | 'hour'
  | 'custom'
  | (string & {})

export type SubscriptionQuotaResetPeriod =
  | 'never'
  | 'daily'
  | 'weekly'
  | 'monthly'
  | 'custom'
  | (string & {})

export type SubscriptionPlanKind =
  | 'subscription'
  | 'value_package'
  | ''
  | (string & {})

export type ValuePackageType = 'day' | 'week' | 'month'

export type UserSubscriptionStatus =
  | 'active'
  | 'expired'
  | 'cancelled'
  | 'covered'
  | (string & {})

export type ValuePackagePurchaseAction = 'create' | 'extend' | 'upgrade'
export type ValuePackageLevel = 1 | 2 | 3 | (number & {})
export type ValuePackageConcurrencyLimit = 1 | 2 | (number & {})

export interface SubscriptionPlanLike {
  id: number
  title: string
  subtitle?: string
  price_amount: number
  currency: string
  duration_unit: SubscriptionDurationUnit
  duration_value: number
  custom_seconds: number
  enabled: boolean
  sort_order: number
  plan_kind: SubscriptionPlanKind
  package_type?: ValuePackageType | '' | (string & {})
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
  allow_balance_pay?: boolean | null
  stripe_price_id?: string
  creem_product_id?: string
  waffo_pancake_product_id?: string
  max_purchase_per_user: number
  upgrade_group?: string
  total_amount: number
  quota_reset_period?: SubscriptionQuotaResetPeriod
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
  status: UserSubscriptionStatus
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
  reset_count: number
  created_at: number
  updated_at: number
}

export interface ValuePackageUsageSummary {
  total_used: number
  total_limit: number
  total_remaining: number
  total_percent: number
  used_5h: number
  limit_5h: number
  percent_5h: number
  reset_at_5h: number
  reset_seconds_5h: number
  limited_5h: boolean
  used_7d: number
  limit_7d: number
  percent_7d: number
  reset_at_7d: number
  reset_seconds_7d: number
  limited_7d: boolean
  exhausted: boolean
  exhausted_reason: string
  exhausted_message: string
}

export interface ValuePackagePlan extends SubscriptionPlanLike {
  plan_kind: 'value_package'
  package_type: ValuePackageType
  package_level: ValuePackageLevel
  model_group: string
  concurrency_limit: ValuePackageConcurrencyLimit
}

export interface ValuePackageBillingState {
  active: boolean
  routing_group?: string
  package_group?: string
  effective_ratio?: number
  original_group_ratio?: number
  plan_title?: string
  plan_id?: number
}

export interface ValuePackageState {
  preference: UserValuePackagePreference
  subscription?: UserSubscription | null
  plan?: ValuePackagePlan | null
  usage?: ValuePackageUsageSummary | null
  billing?: ValuePackageBillingState | null
}

export type ValuePackagePlanRecord = ValuePackagePlan

export interface ValuePackagePlansResponse {
  plans: ValuePackagePlan[]
  state: ValuePackageState | null
}

export interface ValuePackagePurchaseIntent {
  action: ValuePackagePurchaseAction
  requires_confirmation: boolean
  current_subscription?: UserSubscription | null
  current_plan?: ValuePackagePlan | null
  target_plan?: ValuePackagePlan | null
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

export interface ValuePackageLdxpSessionResponse {
  session: ValuePackageLdxpSession
  order_id: number
  trade_no: string
}
