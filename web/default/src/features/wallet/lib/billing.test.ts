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

  it('keeps existing payment method names', () => {
    expect(getPaymentMethodName('waffo_pancake')).toBe('Waffo Pancake')
  })
})
