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
import { z } from 'zod'

// ============================================================================
// Redemption Schema & Types
// ============================================================================

export const redemptionKindSchema = z
  .enum(['legacy', 'paid_topup', 'promo_credit', 'coupon'])
  .catch('legacy')

export const redemptionSchema = z.object({
  id: z.number().catch(0),
  user_id: z.number().catch(0),
  name: z.string().catch(''),
  key: z.string().catch(''),
  status: z.number().catch(1), // 1: enabled, 2: disabled, 3: used
  quota: z.number().catch(0),
  type: z.preprocess(
    (value) =>
      value === undefined || value === null || value === '' ? 'quota' : value,
    z.enum(['quota', 'subscription', 'reset_card'])
  ),
  plan_id: z.preprocess(
    (value) =>
      value === undefined || value === null || value === '' ? 0 : value,
    z.number()
  ),
  plan_title: z.string().optional(),
  created_time: z.number().catch(0),
  redeemed_time: z.number().catch(0),
  expired_time: z.number().catch(0), // 0 for never expires
  used_user_id: z.number().catch(0),
  kind: redemptionKindSchema.default('legacy'),
  amount: z.number().catch(0),
  money: z.number().catch(0),
  count_as_topup: z.boolean().catch(false),
  batch_id: z.string().catch(''),
  source: z.string().catch(''),
  exported_time: z.number().catch(0),
  reset_card_count: z.number().catch(0),
})

export type Redemption = z.infer<typeof redemptionSchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetRedemptionsParams {
  p?: number
  page_size?: number
  /** Server-side status filter: '1' | '2' | '3' | 'expired', comma-separated for multiple */
  status?: string
}

export interface GetRedemptionsResponse {
  success: boolean
  message?: string
  data?: {
    items: Redemption[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchRedemptionsParams {
  keyword?: string
  p?: number
  page_size?: number
  /** Server-side status filter: '1' | '2' | '3' | 'expired', comma-separated for multiple */
  status?: string
}

export interface RedemptionFormData {
  id?: number
  name: string
  quota: number
  expired_time: number
  count?: number // Only for create
  status?: number // Only for status update
  kind?: z.infer<typeof redemptionKindSchema>
  amount?: number
  money?: number
  count_as_topup?: boolean
  batch_id?: string
  source?: string
  type?: 'quota' | 'subscription' | 'reset_card'
  plan_id?: number
  reset_card_count?: number
}

export type CreateRedemptionResponse = ApiResponse<string[]> & {
  batch_id?: string
}

// ============================================================================
// Dialog Types
// ============================================================================

export type RedemptionsDialogType = 'create' | 'update' | 'delete' | 'view'
