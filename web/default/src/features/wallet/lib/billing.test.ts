import {
  getOrderTypeLabel,
  getPlanSummary,
  getPaymentMethodName,
} from './billing'

declare const describe: (name: string, fn: () => void) => void
declare const expect: <T>(actual: T) => { toBe: (expected: T) => void }
declare const it: (name: string, fn: () => void) => void

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
