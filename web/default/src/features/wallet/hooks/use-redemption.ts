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
import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'
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

function normalizeRedemptionResult(
  data: unknown
):
  | { type: 'quota'; quota: number }
  | { type: 'subscription'; planTitle: string }
  | null {
  const numericQuota = typeof data === 'number' ? parseFiniteQuota(data) : null
  if (numericQuota !== null) return { type: 'quota', quota: numericQuota }
  if (data && typeof data === 'object') {
    const record = data as {
      type?: string
      quota?: number | string | null
      plan_title?: string
    }
    if (record.type === 'subscription') {
      return { type: 'subscription', planTitle: record.plan_title || '' }
    }
    if (record.type === 'quota') {
      const quota = parseFiniteQuota(record.quota)
      if (quota !== null) return { type: 'quota', quota }
    }
  }
  return null
}

async function refreshSelfBestEffort() {
  try {
    await getSelf()
  } catch {
    // Refresh is best-effort; the redemption itself already succeeded.
  }
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

  const redeemCode = useCallback(async (code: string): Promise<boolean> => {
    if (!code || code.trim() === '') {
      toast.error(i18next.t('Please enter a redemption code'))
      return false
    }

    try {
      setRedeeming(true)
      const response = await redeemTopupCode(
        { key: code },
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
          await refreshSelfBestEffort()
          return true
        }
        if (result?.type === 'quota') {
          toast.success(
            i18next.t(getRedemptionSuccessMessageKey(response.redemption), {
              quota: formatQuota(result.quota),
            })
          )
          await refreshSelfBestEffort()
          return true
        }
        toast.success(i18next.t('Redemption successful'))
        await refreshSelfBestEffort()
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
  }
}
