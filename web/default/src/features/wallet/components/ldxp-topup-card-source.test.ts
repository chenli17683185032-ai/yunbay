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
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const cardSource = readFileSync(
  resolve(currentDir, 'ldxp-topup-card.tsx'),
  'utf8'
)
const libSource = readFileSync(
  resolve(currentDir, '../lib/ldxp-topup.ts'),
  'utf8'
)

test('ldxp topup card renders large amount choices with discount evidence', () => {
  assert.match(cardSource, /getLdxpPricing/)
  assert.match(cardSource, /getLdxpDiscountLabel/)
  assert.match(cardSource, /min-h-32/)
  assert.match(cardSource, /sm:min-h-36/)
  assert.match(cardSource, /text-2xl/)
  assert.match(cardSource, /sm:text-3xl/)
  assert.match(cardSource, /line-through/)
  assert.match(cardSource, /Payment platform fees are borne by the user\./)
  assert.match(cardSource, /Saved {{amount}}/)
})

test('ldxp topup discount helpers define the required 50, 100, and 500 CNY discounts', () => {
  assert.match(libSource, /50:\s*0\.95/)
  assert.match(libSource, /100:\s*0\.9/)
  assert.match(libSource, /500:\s*0\.85/)
  assert.match(libSource, /export function getLdxpPricing/)
  assert.match(libSource, /export function getLdxpDiscountLabel/)
})
