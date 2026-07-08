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

const staleUsageTableCopy = [
  [
    'Realtime 5-hour and 7-day remaining ',
    'quota for active day, week, and month card users.',
  ].join(''),
  [
    'Users who enable day, week, or month cards will appear here with synced 5-hour and 7-day ',
    'usage.',
  ].join(''),
  ['7-day ', 'remaining'].join(''),
]

test('order management value package usage table shows month-card 7-day period semantics', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /ValuePackageUsageTable/)
  assert.match(source, /TableHeader/)
  assert.match(source, /5-hour remaining/)
  assert.match(source, /7-day period remaining/)
  assert.match(source, /shouldExposeValuePackage7dPeriodLimit/)
  assert.match(source, /Period7dQuotaCell/)
  assert.match(source, /Not applicable/)
  assert.match(source, /used_5h/)
  assert.match(source, /limit_5h/)
  assert.match(source, /used_7d/)
  assert.match(source, /limit_7d/)
  assert.match(source, /formatQuota/)
  assert.match(source, /formatValuePackageResetLine/)
  assert.match(source, /resetSeconds\?: number/)
  assert.match(source, /limited\?: boolean/)
  assert.match(source, /reset_seconds_5h/)
  assert.match(source, /reset_seconds_7d/)
  assert.match(source, /limited_5h/)
  assert.match(source, /limited_7d/)

  for (const staleCopy of staleUsageTableCopy) {
    assert.equal(source.includes(staleCopy), false, staleCopy)
  }
})

test('value package usage table keeps per-user period quota columns', async () => {
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
  assert.match(source, /Period7dQuotaCell/)
  assert.match(source, /shouldExposeValuePackage7dPeriodLimit/)
  assert.match(source, /resetSeconds=\{usage\?\.reset_seconds_5h \|\| 0\}/)
  assert.match(source, /resetSeconds=\{usage\?\.reset_seconds_7d \|\| 0\}/)
  assert.match(source, /limited=\{usage\?\.limited_5h \|\| false\}/)
  assert.match(source, /limited=\{usage\?\.limited_7d \|\| false\}/)
  assert.match(pageSource, /refetchInterval:\s*15000/)
})
