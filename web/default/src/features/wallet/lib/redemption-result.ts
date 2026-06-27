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
export type RedemptionResultMeta = {
  id?: number
  kind?: 'legacy' | 'paid_topup' | 'promo_credit' | 'coupon' | string
  quota?: number
  amount?: number
  money?: number
  count_as_topup?: boolean
  batch_id?: string
  source?: string
}

export function getRedemptionSuccessMessageKey(
  meta?: RedemptionResultMeta
): string {
  if (meta?.kind === 'paid_topup' && meta.count_as_topup) {
    return 'Top-up card redeemed successfully! Added: {{quota}}'
  }
  if (meta?.kind === 'promo_credit') {
    return 'Promo code redeemed successfully! Added bonus quota: {{quota}}'
  }
  return 'Redemption successful! Added: {{quota}}'
}
