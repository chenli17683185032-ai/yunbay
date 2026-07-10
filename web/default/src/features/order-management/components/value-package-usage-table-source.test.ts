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

test('order management value package usage table uses the shared period renderer', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /ValuePackageUsageTable/)
  assert.match(source, /TableHeader/)
  assert.match(source, /Quota periods/)
  assert.match(source, /getValuePackagePeriodLimits/)
  assert.match(source, /ValuePackagePeriodList/)
  assert.match(
    source,
    /getValuePackagePeriodLimits\(\s*row\.usage,\s*row\.plan\.package_type\s*\)/
  )
  assert.doesNotMatch(source, /WindowQuotaCell/)
  assert.doesNotMatch(source, /Period7dQuotaCell/)
  assert.doesNotMatch(source, /TotalRemainingCell/)
  assert.doesNotMatch(source, /Not applicable/)
  assert.doesNotMatch(source, /Package total remaining/)

  for (const staleCopy of staleUsageTableCopy) {
    assert.equal(source.includes(staleCopy), false, staleCopy)
  }
})

test('value package usage table keeps realtime refresh with one period column', async () => {
  const source = await readFile(sourcePath, 'utf8')
  const pageSource = (await readFile(pageSourcePath, 'utf8')).replaceAll(
    '15_000',
    '15000'
  )

  assert.match(source, /<ValuePackagePeriodList periods=\{periods\} \/>/)
  assert.doesNotMatch(source, /plan\.total_amount/)
  assert.match(pageSource, /refetchInterval:\s*15000/)
})
