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
import { useState, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import type { ResetCardGiftCelebration } from '@/components/reset-card-gift-dialog'
import { redeemTopupCode } from '../api'
import { getRedemptionSuccessMessageKey } from '../lib/redemption-result'

// ============================================================================
// Redemption Hook
// ============================================================================

function parseFiniteQuota(
  value: number | string | null | undefined
): number | null {
  if (value === null || value === undefined || value === '') return 0
  const quota = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(quota) ? quota : null
}

function parseFiniteCount(value: unknown): number {
  const count = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(count) && count > 0 ? count : 0
}

function normalizeRedemptionResult(
  data: unknown
):
  | { type: 'quota'; quota: number }
  | { type: 'subscription'; planTitle: string; giftResetCount: number }
  | { type: 'reset_card'; resetCardCount: number }
  | null {
  const numericQuota = typeof data === 'number' ? parseFiniteQuota(data) : null
  if (numericQuota !== null) return { type: 'quota', quota: numericQuota }
  if (data && typeof data === 'object') {
    const record = data as {
      type?: string
      quota?: number | string | null
      plan_title?: string
      reset_card_count?: number
      gift_reset_count?: number
    }
    if (record.type === 'subscription') {
      return {
        type: 'subscription',
        planTitle: record.plan_title || '',
        giftResetCount: parseFiniteCount(record.gift_reset_count),
      }
    }
    if (record.type === 'reset_card') {
      return {
        type: 'reset_card',
        resetCardCount: parseFiniteCount(record.reset_card_count),
      }
    }
    if (record.type === 'quota') {
      const quota = parseFiniteQuota(record.quota)
      if (quota !== null) return { type: 'quota', quota }
    }
  }
  return null
}

function getRedemptionErrorMessage(error: unknown): string {
  const maybeError = error as {
    response?: { data?: { message?: unknown } }
    message?: unknown
  }
  const responseMessage = maybeError.response?.data?.message
  if (typeof responseMessage === 'string' && responseMessage) {
    return responseMessage
  }
  if (typeof maybeError.message === 'string' && maybeError.message) {
    return maybeError.message
  }
  return i18next.t('Redemption failed')
}

export function useRedemption() {
  const [redeeming, setRedeeming] = useState(false)
  const [giftCelebration, setGiftCelebration] =
    useState<ResetCardGiftCelebration | null>(null)

  const clearGiftCelebration = useCallback(() => {
    setGiftCelebration(null)
  }, [])

  const redeemCode = useCallback(async (code: string): Promise<boolean> => {
    const trimmedCode = code?.trim() ?? ''
    if (trimmedCode === '') {
      toast.error(i18next.t('Please enter a redemption code'))
      return false
    }

    try {
      setRedeeming(true)
      const response = await redeemTopupCode(
        { key: trimmedCode },
        { skipBusinessError: true, skipErrorHandler: true }
      )

      if (response.success) {
        const result = normalizeRedemptionResult(response.data)
        if (result?.type === 'subscription') {
          toast.success(
            i18next.t('Redemption successful! Activated plan: {{plan}}', {
              plan: result.planTitle || i18next.t('Subscription'),
            })
          )
          if (result.giftResetCount > 0) {
            setGiftCelebration({
              count: result.giftResetCount,
              planTitle: result.planTitle,
              fromRedemption: false,
            })
          }
          return true
        }
        if (result?.type === 'reset_card') {
          if (result.resetCardCount > 0) {
            setGiftCelebration({
              count: result.resetCardCount,
              fromRedemption: true,
            })
          } else {
            toast.success(i18next.t('Redemption successful'))
          }
          return true
        }
        if (result?.type === 'quota') {
          toast.success(
            i18next.t(getRedemptionSuccessMessageKey(response.redemption), {
              quota: formatQuota(result.quota),
            })
          )
          return true
        }
        toast.success(i18next.t('Redemption successful'))
        return true
      }

      toast.error(response.message || i18next.t('Redemption failed'))
      return false
    } catch (error) {
      toast.error(getRedemptionErrorMessage(error))
      return false
    } finally {
      setRedeeming(false)
    }
  }, [])

  return {
    redeeming,
    redeemCode,
    giftCelebration,
    clearGiftCelebration,
  }
}
