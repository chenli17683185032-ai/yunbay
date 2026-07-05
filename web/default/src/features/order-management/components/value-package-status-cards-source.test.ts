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

const sourcePath = new URL('./value-package-status-cards.tsx', import.meta.url)

test('order management value package status cards show realtime users and quota', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /ValuePackageStatusCards/)
  assert.match(source, /plan_kind === 'value_package'/)
  assert.match(source, /Active Users/)
  assert.match(source, /Remaining Quota/)
  assert.match(source, /active_user_count/)
  assert.match(source, /active_subscription_count/)
  assert.match(source, /remaining_amount/)
  assert.match(source, /unlimited_count/)
})
