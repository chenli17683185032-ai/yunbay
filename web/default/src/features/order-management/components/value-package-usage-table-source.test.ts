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

const sourcePath = new URL('./value-package-usage-table.tsx', import.meta.url)
const pageSourcePath = new URL('../index.tsx', import.meta.url)

test('order management value package usage table shows per-user realtime 5h and 7d windows', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /ValuePackageUsageTable/)
  assert.match(source, /TableHeader/)
  assert.match(source, /5-hour remaining/)
  assert.match(source, /7-day remaining/)
  assert.match(source, /used_5h/)
  assert.match(source, /limit_5h/)
  assert.match(source, /used_7d/)
  assert.match(source, /limit_7d/)
  assert.match(source, /formatQuota/)
  assert.match(source, /Unlimited/)
})

test('value package usage table keeps per-user rolling quota columns', async () => {
  const source = await readFile(sourcePath, 'utf8')
  const pageSource = (await readFile(pageSourcePath, 'utf8')).replaceAll(
    '15_000',
    '15000'
  )

  assert.match(source, /used_5h/)
  assert.match(source, /limit_5h/)
  assert.match(source, /used_7d/)
  assert.match(source, /limit_7d/)
  assert.match(source, /total_remaining/)
  assert.match(pageSource, /refetchInterval:\s*15000/)
})
