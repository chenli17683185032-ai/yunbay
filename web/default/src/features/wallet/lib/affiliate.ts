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
// ============================================================================
// Affiliate Functions
// ============================================================================

/**
 * Generate affiliate registration link
 */
export function generateAffiliateLink(affCode: string): string {
  if (typeof window === 'undefined') return ''
  return `${window.location.origin}/sign-up?aff=${affCode}`
}

/**
 * Round affiliate money values to cents.
 */
export function roundAffiliateMoney(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.round((value + Number.EPSILON) * 100) / 100
}

/**
 * Normalize withdrawal amount input before validation/submission.
 */
export function normalizeAffiliateWithdrawalAmount(value: number): number {
  return roundAffiliateMoney(value)
}

/**
 * Validate withdrawal amount against currently available rewards.
 */
export function validateAffiliateWithdrawalAmount(
  amount: number,
  available: number
): string | null {
  const normalizedAmount = normalizeAffiliateWithdrawalAmount(amount)
  const normalizedAvailable = roundAffiliateMoney(available)

  if (normalizedAmount <= 0) {
    return 'Withdrawal amount must be greater than 0'
  }

  if (normalizedAmount > normalizedAvailable) {
    return 'Withdrawal amount exceeds available rewards'
  }

  return null
}

/**
 * Validate payout contact details.
 */
export function validateAffiliateWithdrawalContact(
  contact: string
): string | null {
  if (!contact.trim()) {
    return 'Withdrawal contact is required'
  }

  return null
}

/**
 * Validate a complete withdrawal request and return the first error key.
 */
export function validateAffiliateWithdrawalInput(
  amount: number,
  available: number,
  contact: string
): string | null {
  return (
    validateAffiliateWithdrawalAmount(amount, available) ||
    validateAffiliateWithdrawalContact(contact)
  )
}
