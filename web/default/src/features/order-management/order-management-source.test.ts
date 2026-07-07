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

const sourcePath = new URL('./index.tsx', import.meta.url)
const apiSourcePath = new URL('./api.ts', import.meta.url)

test('order management page mounts value package realtime usage table and refreshes it', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /ValuePackageUsageTable/)
  assert.match(source, /getOrderManagementValuePackageUsage/)
  assert.match(source, /orderManagementKeys\.valuePackageUsage/)
  assert.match(source, /queryKey: orderManagementKeys\.valuePackageUsage\(\)/)
  assert.match(source, /refetchInterval:\s*15_000/)
  assert.match(
    source,
    /invalidateQueries\(\{\s*queryKey: orderManagementKeys\.valuePackageUsage\(\)/s
  )
})

test('order management API uses dedicated value package usage endpoint', async () => {
  const source = await readFile(apiSourcePath, 'utf8')

  assert.match(source, /getOrderManagementValuePackageUsage/)
  assert.match(source, /\/api\/order-management\/admin\/value-package-usage/)
})

test('order management index keeps standalone value package management page separate', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.doesNotMatch(source, /ValuePackageManagementPage/)
  assert.doesNotMatch(source, /adjustOrderManagementValuePackageResetCount/)
})
