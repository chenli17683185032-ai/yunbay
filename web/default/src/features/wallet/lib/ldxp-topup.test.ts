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
  LDXP_TOPUP_AMOUNTS,
  getLdxpStatusMessageKey,
  isLdxpTerminalStatus,
} from './ldxp-topup'

test('ldxp topup uses fixed allowed amounts', () => {
  assert.deepEqual(LDXP_TOPUP_AMOUNTS, [10, 20, 30, 50, 100, 500])
})

test('ldxp topup detects terminal status', () => {
  assert.equal(isLdxpTerminalStatus('success'), true)
  assert.equal(isLdxpTerminalStatus('qr_ready'), false)
  assert.equal(isLdxpTerminalStatus('verify_failed'), true)
})

test('ldxp topup maps status to message keys', () => {
  assert.equal(getLdxpStatusMessageKey('qr_ready'), 'Scan with Alipay to pay')
  assert.equal(getLdxpStatusMessageKey('success'), 'Recharge successful')
})

test('ldxp topup groups creation status message keys', () => {
  assert.equal(
    getLdxpStatusMessageKey('created'),
    'Creating payment QR code'
  )
  assert.equal(
    getLdxpStatusMessageKey('worker_claimed'),
    'Creating payment QR code'
  )
})

test('ldxp topup groups verified status message keys', () => {
  assert.equal(getLdxpStatusMessageKey('verified'), 'Verifying order')
  assert.equal(getLdxpStatusMessageKey('redeemed'), 'Verifying order')
})

test('ldxp topup groups failure status message keys', () => {
  assert.equal(getLdxpStatusMessageKey('worker_failed'), 'Recharge failed')
  assert.equal(getLdxpStatusMessageKey('mail_timeout'), 'Recharge failed')
  assert.equal(getLdxpStatusMessageKey('verify_failed'), 'Recharge failed')
  assert.equal(getLdxpStatusMessageKey('redeem_failed'), 'Recharge failed')
})
