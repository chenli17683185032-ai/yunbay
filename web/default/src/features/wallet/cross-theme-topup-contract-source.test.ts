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
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const classicTopupPath = fileURLToPath(
  new URL('../../../../classic/src/components/topup/index.jsx', import.meta.url)
)
const valuePackageHookPath = fileURLToPath(
  new URL('../value-packages/hooks/use-value-packages.ts', import.meta.url)
)

test('classic topup only mutates quota for numeric redemption responses', async () => {
  const source = await readFile(classicTopupPath, 'utf8')

  assert.match(source, /typeof data === 'number'/)
  assert.match(source, /data\?\.type === 'subscription'/)
  assert.match(source, /data\?\.type === 'reset_card'/)
  assert.match(source, /quotaRedeemed !== null && userState\.user/)
})

test('value package gift animation uses the server order snapshot', async () => {
  const source = await readFile(valuePackageHookPath, 'utf8')

  assert.match(source, /session\.gift_reset_count/)
  assert.doesNotMatch(source, /purchasedPlan\?\.gift_reset_count/)
})
