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
import { useState, useEffect, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { getSelf } from '@/lib/api'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import {
  getAffiliateCode,
  getAffiliateSummary,
  requestAffiliateWithdrawal,
  transferAffiliateQuota,
} from '../api'
import {
  generateAffiliateLink,
  normalizeAffiliateWithdrawalAmount,
  validateAffiliateWithdrawalInput,
} from '../lib'
import type { AffiliateSummary } from '../types'

// ============================================================================
// Affiliate Hook
// ============================================================================

export function useAffiliate() {
  const [affiliateCode, setAffiliateCode] = useState<string>('')
  const [affiliateLink, setAffiliateLink] = useState<string>('')
  const [affiliateSummary, setAffiliateSummary] =
    useState<AffiliateSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [summaryLoading, setSummaryLoading] = useState(true)
  const [transferring, setTransferring] = useState(false)
  const [withdrawalSubmitting, setWithdrawalSubmitting] = useState(false)
  const { copyToClipboard } = useCopyToClipboard()

  // Fetch affiliate code
  const fetchAffiliateCode = useCallback(async () => {
    try {
      setLoading(true)
      const response = await getAffiliateCode()

      if (response.success && response.data) {
        setAffiliateCode(response.data)
        const link = generateAffiliateLink(response.data)
        setAffiliateLink(link)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate code:', error)
    } finally {
      setLoading(false)
    }
  }, [])

  // Fetch monetary affiliate reward summary
  const fetchAffiliateSummary = useCallback(async () => {
    try {
      setSummaryLoading(true)
      const response = await getAffiliateSummary()

      if (response.success && response.data) {
        setAffiliateSummary(response.data)
        if (response.data.aff_code) {
          setAffiliateCode(response.data.aff_code)
          setAffiliateLink(generateAffiliateLink(response.data.aff_code))
        }
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate summary:', error)
    } finally {
      setSummaryLoading(false)
    }
  }, [])

  // Copy affiliate link
  const copyAffiliateLink = useCallback(() => {
    copyToClipboard(affiliateLink)
  }, [affiliateLink, copyToClipboard])

  // Transfer affiliate quota to balance
  const transferQuota = useCallback(async (quota: number): Promise<boolean> => {
    try {
      setTransferring(true)
      const response = await transferAffiliateQuota({ quota })

      if (response.success) {
        toast.success(response.message || i18next.t('Transfer successful'))
        await getSelf()
        return true
      }

      toast.error(response.message || i18next.t('Transfer failed'))
      return false
    } catch (_error) {
      toast.error(i18next.t('Transfer failed'))
      return false
    } finally {
      setTransferring(false)
    }
  }, [])

  const requestWithdrawal = useCallback(
    async (
      amount: number,
      contact: string,
      remark?: string
    ): Promise<boolean> => {
      const availableMoney = affiliateSummary?.available_money ?? 0
      const validationError = validateAffiliateWithdrawalInput(
        amount,
        availableMoney,
        contact
      )
      if (validationError) {
        toast.error(i18next.t(validationError))
        return false
      }

      try {
        setWithdrawalSubmitting(true)
        const response = await requestAffiliateWithdrawal({
          amount: normalizeAffiliateWithdrawalAmount(amount),
          contact: contact.trim(),
          remark: remark?.trim(),
        })

        if (response.success) {
          toast.success(
            response.message || i18next.t('Withdrawal request submitted')
          )
          await fetchAffiliateSummary()
          return true
        }

        toast.error(
          response.message || i18next.t('Failed to submit withdrawal request')
        )
        return false
      } catch (_error) {
        toast.error(i18next.t('Failed to submit withdrawal request'))
        return false
      } finally {
        setWithdrawalSubmitting(false)
      }
    },
    [affiliateSummary?.available_money, fetchAffiliateSummary]
  )

  useEffect(() => {
    fetchAffiliateCode()
    fetchAffiliateSummary()
  }, [fetchAffiliateCode, fetchAffiliateSummary])

  return {
    affiliateCode,
    affiliateLink,
    affiliateSummary,
    loading,
    summaryLoading,
    transferring,
    withdrawalSubmitting,
    copyAffiliateLink,
    transferQuota,
    requestWithdrawal,
    refetch: fetchAffiliateCode,
    refetchSummary: fetchAffiliateSummary,
  }
}
