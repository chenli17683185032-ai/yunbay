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

const sourcePath = new URL('./order-details-table.tsx', import.meta.url)

test('order management table exposes delete action for billing orders', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /billing_order_type/)
  assert.match(source, /trade_no/)
  assert.match(source, /Delete Order/)
  assert.match(source, /This order will be hidden from order management/)
  assert.match(source, /onDelete/)
})
