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
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const cardPath = resolve(currentDir, 'redemption-code-card.tsx')

test('wallet exposes a dedicated redemption code card instead of legacy account recharge UI', () => {
  assert.equal(existsSync(cardPath), true)
  const cardSource = readFileSync(cardPath, 'utf8')

  assert.match(cardSource, /title=\{t\('Redemption code'\)\}/)
  assert.match(cardSource, /Redeem balance with a code or card key\./)
  assert.match(cardSource, /id='wallet-redemption-code'/)
  assert.match(cardSource, /Enter your redemption code or card key/)
  assert.match(cardSource, /Redeem/)
  assert.match(cardSource, /topupLink/)
  assert.match(cardSource, /redemptionEnabled/)
  assert.doesNotMatch(cardSource, /Online topup is not enabled/)
  assert.doesNotMatch(cardSource, /Payment Method/)
  assert.doesNotMatch(cardSource, /Custom Amount/)
})
