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
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  generateAffiliateLink,
  normalizeAffiliateWithdrawalAmount,
  roundAffiliateMoney,
  validateAffiliateWithdrawalAmount,
  validateAffiliateWithdrawalContact,
  validateAffiliateWithdrawalInput,
} from './affiliate'

test('affiliate link uses the current browser origin', () => {
  const originalWindow = globalThis.window

  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      location: {
        origin: 'https://example.test',
      },
    },
  })

  try {
    assert.equal(
      generateAffiliateLink('abc123'),
      'https://example.test/sign-up?aff=abc123'
    )
  } finally {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    })
  }
})

test('affiliate money is rounded to two decimals', () => {
  assert.equal(roundAffiliateMoney(1.005), 1.01)
  assert.equal(roundAffiliateMoney(3.456), 3.46)
  assert.equal(roundAffiliateMoney(Number.NaN), 0)
})

test('withdrawal amount input is normalized to cents', () => {
  assert.equal(normalizeAffiliateWithdrawalAmount(12.345), 12.35)
  assert.equal(normalizeAffiliateWithdrawalAmount(Number.POSITIVE_INFINITY), 0)
})

test('withdrawal amount validation rejects invalid and overdrawn amounts', () => {
  assert.equal(
    validateAffiliateWithdrawalAmount(0, 10),
    'Withdrawal amount must be greater than 0'
  )
  assert.equal(
    validateAffiliateWithdrawalAmount(Number.NaN, 10),
    'Withdrawal amount must be greater than 0'
  )
  assert.equal(
    validateAffiliateWithdrawalAmount(4, 3),
    'Withdrawal amount exceeds available rewards'
  )
  assert.equal(validateAffiliateWithdrawalAmount(3, 3), null)
})

test('withdrawal contact validation requires non-empty contact details', () => {
  assert.equal(
    validateAffiliateWithdrawalContact('   '),
    'Withdrawal contact is required'
  )
  assert.equal(validateAffiliateWithdrawalContact('alipay@example.test'), null)
})

test('withdrawal input validation returns the first field error', () => {
  assert.equal(
    validateAffiliateWithdrawalInput(0, 10, ''),
    'Withdrawal amount must be greater than 0'
  )
  assert.equal(
    validateAffiliateWithdrawalInput(5, 10, ''),
    'Withdrawal contact is required'
  )
  assert.equal(validateAffiliateWithdrawalInput(5, 10, 'wechat-id'), null)
})
