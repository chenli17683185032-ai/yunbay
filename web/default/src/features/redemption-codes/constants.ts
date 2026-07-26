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
import { type TFunction } from 'i18next'
import { type StatusBadgeProps } from '@/components/status-badge'

// ============================================================================
// Redemption Status Configuration
// ============================================================================

export const REDEMPTION_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
  USED: 3,
} as const

export const REDEMPTION_STATUS_VALUES = Object.values(REDEMPTION_STATUS).map(
  (value) => String(value)
) as `${number}`[]

// labelKey values are i18n keys; use t(config.labelKey) in components
export const REDEMPTION_STATUSES: Record<
  number,
  Pick<StatusBadgeProps, 'variant'> & {
    labelKey: string
    value: number
  }
> = {
  [REDEMPTION_STATUS.ENABLED]: {
    labelKey: 'Unused',
    variant: 'success',
    value: REDEMPTION_STATUS.ENABLED,
  },
  [REDEMPTION_STATUS.DISABLED]: {
    labelKey: 'Disabled',
    variant: 'neutral',
    value: REDEMPTION_STATUS.DISABLED,
  },
  [REDEMPTION_STATUS.USED]: {
    labelKey: 'Used',
    variant: 'neutral',
    value: REDEMPTION_STATUS.USED,
  },
} as const

// Virtual status filter value for expired redemption codes
// Note: "Expired" is not a real DB status, it's computed from expired_time
export const REDEMPTION_FILTER_EXPIRED = 'expired'

export function getRedemptionStatusOptions(t: TFunction) {
  return [
    ...Object.values(REDEMPTION_STATUSES).map((config) => ({
      label: t(config.labelKey),
      value: String(config.value),
    })),
    {
      label: t('Expired'),
      value: REDEMPTION_FILTER_EXPIRED,
    },
  ]
}

// ============================================================================
// Redemption Type Configuration
// ============================================================================

export const REDEMPTION_KINDS = {
  LEGACY: 'legacy',
  PAID_TOPUP: 'paid_topup',
  PROMO_CREDIT: 'promo_credit',
  COUPON: 'coupon',
} as const

export const REDEMPTION_SOURCES = {
  LIANDONG: 'ldxp',
  PROMO: 'promo',
  MANUAL: 'manual',
} as const

export function getRedemptionKindOptions(t: TFunction) {
  return [
    { label: t('Promo / gift credit'), value: REDEMPTION_KINDS.PROMO_CREDIT },
    { label: t('Paid top-up card'), value: REDEMPTION_KINDS.PAID_TOPUP },
    { label: t('Legacy'), value: REDEMPTION_KINDS.LEGACY },
    { label: t('Coupon'), value: REDEMPTION_KINDS.COUPON },
  ]
}

export function getRedemptionSourceOptions(t: TFunction) {
  return [
    { label: t('Promotion'), value: REDEMPTION_SOURCES.PROMO },
    { label: t('LianDong card shop'), value: REDEMPTION_SOURCES.LIANDONG },
    { label: t('Manual'), value: REDEMPTION_SOURCES.MANUAL },
  ]
}

export function getRedemptionKindLabel(t: TFunction, kind: string) {
  const option = getRedemptionKindOptions(t).find((item) => item.value === kind)
  return option?.label ?? (kind || t('Legacy'))
}

export function getRedemptionSourceLabel(t: TFunction, source: string) {
  const option = getRedemptionSourceOptions(t).find(
    (item) => item.value === source
  )
  return option?.label ?? (source || '-')
}

// ============================================================================
// Validation Constants
// ============================================================================

export const REDEMPTION_VALIDATION = {
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 20,
  COUNT_MIN: 1,
  COUNT_MAX: 100,
  RESET_CARD_COUNT_MAX: 100,
} as const

// ============================================================================
// Error Messages
// ============================================================================

// i18n keys; use t(ERROR_MESSAGES.xxx) when displaying. For form schema with interpolation use getRedemptionFormErrorMessages(t).
export const ERROR_MESSAGES = {
  UNEXPECTED: 'An unexpected error occurred',
  LOAD_FAILED: 'Failed to load redemption codes',
  SEARCH_FAILED: 'Failed to search redemption codes',
  CREATE_FAILED: 'Failed to create redemption code',
  UPDATE_FAILED: 'Failed to update redemption code',
  DELETE_FAILED: 'Failed to delete redemption code',
  DELETE_INVALID_FAILED: 'Failed to delete invalid redemption codes',
  STATUS_UPDATE_FAILED: 'Failed to update redemption code status',
  NAME_LENGTH_INVALID: 'Name must be between {{min}} and {{max}} characters',
  COUNT_INVALID: 'Count must be between {{min}} and {{max}}',
  EXPIRED_TIME_INVALID: 'Expired time cannot be earlier than current time',
  FACE_AMOUNT_INTEGER_INVALID: 'Face amount must be a whole number.',
  PAID_TOPUP_INVALID:
    'Paid top-up cards require positive quota, amount, paid money, and top-up accounting.',
  PROMO_CREDIT_INVALID:
    'Promo / gift credit cannot be counted as paid top-up, and amount/money cannot be negative.',
  RESET_CARD_COUNT_INVALID:
    'Reset card count must be between {{min}} and {{max}}',
} as const

/** For form schema only: returns translated messages with interpolation. */
export function getRedemptionFormErrorMessages(t: TFunction) {
  return {
    NAME_LENGTH_INVALID: t(ERROR_MESSAGES.NAME_LENGTH_INVALID, {
      min: REDEMPTION_VALIDATION.NAME_MIN_LENGTH,
      max: REDEMPTION_VALIDATION.NAME_MAX_LENGTH,
    }),
    COUNT_INVALID: t(ERROR_MESSAGES.COUNT_INVALID, {
      min: REDEMPTION_VALIDATION.COUNT_MIN,
      max: REDEMPTION_VALIDATION.COUNT_MAX,
    }),
    EXPIRED_TIME_INVALID: t(ERROR_MESSAGES.EXPIRED_TIME_INVALID),
    FACE_AMOUNT_INTEGER_INVALID: t(ERROR_MESSAGES.FACE_AMOUNT_INTEGER_INVALID),
    PAID_TOPUP_INVALID: t(ERROR_MESSAGES.PAID_TOPUP_INVALID),
    PROMO_CREDIT_INVALID: t(ERROR_MESSAGES.PROMO_CREDIT_INVALID),
    RESET_CARD_COUNT_INVALID: t(ERROR_MESSAGES.RESET_CARD_COUNT_INVALID, {
      min: 1,
      max: REDEMPTION_VALIDATION.RESET_CARD_COUNT_MAX,
    }),
  } as const
}

// ============================================================================
// Success Messages (i18n keys; use t(SUCCESS_MESSAGES.xxx) when displaying)
// ============================================================================

export const SUCCESS_MESSAGES = {
  REDEMPTION_CREATED: 'Redemption code(s) created successfully',
  REDEMPTION_UPDATED: 'Redemption code updated successfully',
  REDEMPTION_DELETED: 'Redemption code deleted successfully',
  REDEMPTION_ENABLED: 'Redemption code enabled successfully',
  REDEMPTION_DISABLED: 'Redemption code disabled successfully',
  COPY_SUCCESS: 'Copied to clipboard',
  EXPORT_SUCCESS: 'Redemption codes exported successfully',
  EXPORT_FAILED: 'Failed to export redemption codes',
} as const
