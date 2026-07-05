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
import { describe, expect, it } from 'bun:test'
import {
  getOrderTypeLabel,
  getPlanSummary,
  getPaymentMethodName,
} from './billing'

describe('billing helpers', () => {
  it('formats unified order types', () => {
    expect(getOrderTypeLabel('topup')).toBe('Top-up')
    expect(getOrderTypeLabel('subscription')).toBe('Subscription')
    expect(getOrderTypeLabel('other')).toBe('other')
  })

  it('formats subscription plan summary', () => {
    expect(
      getPlanSummary({
        order_type: 'subscription',
        plan_title: '月卡',
        duration_unit: 'month',
        duration_value: 1,
      })
    ).toBe('月卡 · 1 month')
  })

  it('falls back for missing order type', () => {
    expect(getOrderTypeLabel(undefined)).toBe('Top-up')
  })

  it('returns only deleted plan label when subscription duration is missing', () => {
    expect(getPlanSummary({ order_type: 'subscription' })).toBe('Deleted plan')
  })

  it('keeps existing payment method names', () => {
    expect(getPaymentMethodName('waffo_pancake')).toBe('Waffo Pancake')
  })
})
