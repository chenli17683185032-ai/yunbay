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
import test from 'node:test'

const sourcePath = new URL('./value-package-admin-cards.tsx', import.meta.url)

test('value package admin cards source contains all fixed package config fields', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /day/)
  assert.match(source, /week/)
  assert.match(source, /month/)
  assert.match(source, /ldxp_product_url/)
  assert.match(source, /plan\.currency/)
  assert.match(source, /concurrency_limit/)
  assert.match(source, /limit_5h_amount/)
  assert.match(source, /limit_7d_amount/)
  assert.match(source, /付款未配置|Payment not configured/)
})
