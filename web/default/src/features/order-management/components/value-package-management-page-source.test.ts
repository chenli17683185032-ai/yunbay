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

const pageSourcePath = new URL(
  './value-package-management-page.tsx',
  import.meta.url
)
const apiSourcePath = new URL('../api.ts', import.meta.url)
const typesSourcePath = new URL('../types.ts', import.meta.url)
const routeSourcePath = new URL(
  '../../../routes/_authenticated/order-management/value-packages.tsx',
  import.meta.url
)
const sidebarSourcePath = new URL(
  '../../../hooks/sidebar-data-model.ts',
  import.meta.url
)

const staleManagementCopy = [
  [
    'Users who enable day, week, or month cards will appear here with synced 5-hour and 7-day ',
    'usage.',
  ].join(''),
  ['7-day ', 'remaining'].join(''),
]

test('value package management page uses dedicated admin APIs and realtime refresh', async () => {
  const source = await readFile(pageSourcePath, 'utf8')

  assert.match(source, /ValuePackageManagementPage/)
  assert.match(source, /getOrderManagementValuePackageUsers/)
  assert.match(source, /adjustOrderManagementValuePackageResetCount/)
  assert.match(source, /orderManagementValuePackageManagementKeys\.users/)
  assert.match(source, /refetchInterval:\s*15_000/)
  assert.match(source, /Value Package Management/)
  assert.match(
    source,
    /Manage day, week, and month package reset counts and realtime quota\./
  )
})

test('value package management page shows reset-count and shared quota periods', async () => {
  const source = await readFile(pageSourcePath, 'utf8')

  assert.match(source, /Reset count/)
  assert.match(source, /Last reset/)
  assert.match(source, /Quota periods/)
  assert.match(source, /getValuePackagePeriodLimits/)
  assert.match(source, /ValuePackagePeriodList/)
  assert.match(
    source,
    /getValuePackagePeriodLimits\(\s*row\.usage,\s*row\.package_type\s*\)/
  )
  assert.match(source, /<ValuePackagePeriodList periods=\{periods\} \/>/)
  assert.doesNotMatch(source, /WindowQuotaCell/)
  assert.doesNotMatch(source, /Period7dQuotaCell/)
  assert.doesNotMatch(source, /TotalRemainingCell/)
  assert.doesNotMatch(source, /Not applicable/)
  assert.doesNotMatch(source, /plan\.total_amount/)
  assert.match(source, /reset_count/)
  assert.match(source, /last_reset_at/)

  for (const staleCopy of staleManagementCopy) {
    assert.equal(source.includes(staleCopy), false, staleCopy)
  }
})

test('value package management page provides filters and reset-count adjustment controls', async () => {
  const source = await readFile(pageSourcePath, 'utf8')

  assert.match(source, /useEffect/)
  assert.match(source, /Search user/)
  assert.match(source, /All packages/)
  assert.match(source, /Day package/)
  assert.match(source, /Week package/)
  assert.match(source, /Month package/)
  assert.match(source, /Adjust reset count/)
  assert.match(source, /Mode/)
  assert.match(source, /Set/)
  assert.match(source, /Add/)
  assert.match(source, /Subtract/)
  assert.match(source, /Reason/)
  assert.match(source, /Reset count must be a non-negative number/)
  assert.match(
    source,
    /useEffect\(\(\) => \{[\s\S]*setMode\('add'\)[\s\S]*setValue\('1'\)[\s\S]*setReason\(''\)[\s\S]*\}, \[open, row\?\.user_id\]\)/
  )
})

test('value package management route and sidebar point to the standalone page', async () => {
  const routeSource = await readFile(routeSourcePath, 'utf8')
  const sidebarSource = await readFile(sidebarSourcePath, 'utf8')

  assert.match(
    routeSource,
    /createFileRoute\([\s\S]*'\/_authenticated\/order-management\/value-packages'[\s\S]*\)/
  )
  assert.match(routeSource, /ValuePackageManagementPage/)
  assert.match(routeSource, /ROLE\.ADMIN/)
  assert.match(sidebarSource, /Value Package Management/)
  assert.match(sidebarSource, /\/order-management\/value-packages/)
})

test('value package management API and types match backend contracts', async () => {
  const apiSource = await readFile(apiSourcePath, 'utf8')
  const typesSource = await readFile(typesSourcePath, 'utf8')

  assert.match(apiSource, /getOrderManagementValuePackageUsers/)
  assert.match(
    apiSource,
    /\/api\/order-management\/admin\/value-packages\/users/
  )
  assert.match(apiSource, /adjustOrderManagementValuePackageResetCount/)
  assert.match(
    apiSource,
    /\/api\/order-management\/admin\/value-packages\/users\/\$\{userId\}\/reset-count/
  )
  assert.match(typesSource, /OrderManagementValuePackageManagementRow/)
  assert.match(typesSource, /OrderManagementValuePackageResetCountAdjustment/)
  assert.match(typesSource, /reset_count: number/)
  assert.match(typesSource, /last_reset_at: number/)
})

test('order management usage summary aliases the shared value package contract', async () => {
  const typesSource = await readFile(typesSourcePath, 'utf8')

  assert.match(
    typesSource,
    /import type \{[\s\S]*?\bValuePackageUsageSummary\b[\s\S]*?\} from '@\/features\/value-packages\/types'/
  )
  assert.match(
    typesSource,
    /export type OrderManagementValuePackageUsageSummary\s*=\s*ValuePackageUsageSummary/
  )
  assert.doesNotMatch(
    typesSource,
    /export interface OrderManagementValuePackageUsageSummary/
  )
})
