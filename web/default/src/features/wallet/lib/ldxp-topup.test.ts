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
// @ts-expect-error Bun resolves this module at test runtime; this project does not include Bun ambient types.
import { describe, expect, it } from 'bun:test'
import {
  LDXP_TOPUP_AMOUNTS,
  getLdxpStatusMessageKey,
  isLdxpTerminalStatus,
} from './ldxp-topup'

describe('ldxp topup helpers', () => {
  it('uses fixed allowed amounts', () => {
    expect(LDXP_TOPUP_AMOUNTS).toEqual([10, 20, 30, 50, 100, 500])
  })

  it('detects terminal status', () => {
    expect(isLdxpTerminalStatus('success')).toBe(true)
    expect(isLdxpTerminalStatus('qr_ready')).toBe(false)
    expect(isLdxpTerminalStatus('verify_failed')).toBe(true)
  })

  it('maps status to message keys', () => {
    expect(getLdxpStatusMessageKey('qr_ready')).toBe('Scan with Alipay to pay')
    expect(getLdxpStatusMessageKey('success')).toBe('Recharge successful')
  })
})
