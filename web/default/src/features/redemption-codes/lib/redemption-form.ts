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
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import {
  REDEMPTION_KINDS,
  REDEMPTION_SOURCES,
  REDEMPTION_VALIDATION,
  getRedemptionFormErrorMessages,
} from '../constants'
import {
  type RedemptionFormData,
  type Redemption,
  redemptionKindSchema,
} from '../types'

// ============================================================================
// Form Schema (use getRedemptionFormSchema(t) in components for i18n messages)
// ============================================================================

export function getRedemptionFormSchema(t: TFunction) {
  const msg = getRedemptionFormErrorMessages(t)
  return z
    .object({
      name: z
        .string()
        .min(REDEMPTION_VALIDATION.NAME_MIN_LENGTH, msg.NAME_LENGTH_INVALID)
        .max(REDEMPTION_VALIDATION.NAME_MAX_LENGTH, msg.NAME_LENGTH_INVALID),
      quota_dollars: z.number().min(0, t('Quota must be a positive number')),
      expired_time: z.date().optional(),
      count: z
        .number()
        .min(REDEMPTION_VALIDATION.COUNT_MIN, msg.COUNT_INVALID)
        .max(REDEMPTION_VALIDATION.COUNT_MAX, msg.COUNT_INVALID)
        .optional(),
      kind: redemptionKindSchema,
      amount: z.number().min(0, msg.PROMO_CREDIT_INVALID),
      money: z.number().min(0, msg.PROMO_CREDIT_INVALID),
      count_as_topup: z.boolean(),
      batch_id: z.string(),
      source: z.string(),
    })
    .superRefine((data, ctx) => {
      if (data.kind === REDEMPTION_KINDS.PAID_TOPUP) {
        if (
          data.quota_dollars <= 0 ||
          data.amount <= 0 ||
          data.money <= 0 ||
          !data.count_as_topup
        ) {
          ctx.addIssue({
            code: 'custom',
            message: msg.PAID_TOPUP_INVALID,
            path: ['kind'],
          })
        }
      }

      if (data.kind === REDEMPTION_KINDS.PROMO_CREDIT) {
        if (data.count_as_topup || data.amount < 0 || data.money < 0) {
          ctx.addIssue({
            code: 'custom',
            message: msg.PROMO_CREDIT_INVALID,
            path: ['kind'],
          })
        }
      }
    })
}

export type RedemptionFormValues = {
  name: string
  quota_dollars: number
  expired_time?: Date
  count?: number
  kind: z.infer<typeof redemptionKindSchema>
  amount: number
  money: number
  count_as_topup: boolean
  batch_id: string
  source: string
}

// ============================================================================
// Form Defaults
// ============================================================================

export const REDEMPTION_FORM_DEFAULT_VALUES: RedemptionFormValues = {
  name: '',
  quota_dollars: 10,
  expired_time: undefined,
  count: 1,
  kind: REDEMPTION_KINDS.PROMO_CREDIT,
  amount: 0,
  money: 0,
  count_as_topup: false,
  batch_id: '',
  source: REDEMPTION_SOURCES.PROMO,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: RedemptionFormValues
): RedemptionFormData {
  return {
    name: data.name,
    quota: parseQuotaFromDollars(data.quota_dollars),
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : 0,
    count: data.count || 1,
    kind: data.kind,
    amount: data.amount,
    money: data.money,
    count_as_topup: data.count_as_topup,
    batch_id: data.batch_id,
    source: data.source,
  }
}

/**
 * Transform redemption data to form defaults
 */
export function transformRedemptionToFormDefaults(
  redemption: Redemption
): RedemptionFormValues {
  return {
    name: redemption.name,
    quota_dollars: quotaUnitsToDollars(redemption.quota),
    expired_time:
      redemption.expired_time > 0
        ? new Date(redemption.expired_time * 1000)
        : undefined,
    count: 1,
    kind: redemption.kind,
    amount: redemption.amount,
    money: redemption.money,
    count_as_topup: redemption.count_as_topup,
    batch_id: redemption.batch_id,
    source: redemption.source,
  }
}
