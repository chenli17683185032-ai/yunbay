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

test('order management page mounts value package realtime stats and refreshes them', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /ValuePackageStatusCards/)
  assert.match(source, /getOrderManagementValuePackagePlans/)
  assert.match(source, /orderManagementKeys\.valuePackages/)
  assert.match(source, /queryKey: orderManagementKeys\.valuePackages\(\)/)
  assert.match(
    source,
    /invalidateQueries\(\{\s*queryKey: orderManagementKeys\.valuePackages\(\)/s
  )
})

test('order management API reuses admin subscription plans endpoint for value package stats', async () => {
  const source = await readFile(apiSourcePath, 'utf8')

  assert.match(source, /getOrderManagementValuePackagePlans/)
  assert.match(source, /\/api\/subscription\/admin\/plans/)
})
