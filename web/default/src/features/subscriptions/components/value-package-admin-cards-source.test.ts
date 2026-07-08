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

test('value package admin cards show stats and redemption entry on each card', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /record\?\.stats/)
  assert.match(source, /Active Users/)
  assert.match(source, /Remaining Quota/)
  assert.match(source, /Generate Codes/)
  assert.match(source, /setOpen\('generate-redemptions'\)/)
})

test('value package admin cards use dynamic total and month-only 7d period labels', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /getValuePackageTotalLimitLabelKey/)
  assert.match(source, /shouldExposeValuePackage7dPeriodLimit/)
  assert.match(source, /VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY/)
  assert.match(source, /Number\(plan\.total_amount \|\| 0\)/)
  assert.match(source, /\{t\('5-hour limit'\)\}/)
  assert.match(
    source,
    /shouldExposeValuePackage7dPeriodLimit\([\s\S]*?plan\.package_type[\s\S]*?\) && Number\(plan\.limit_7d_amount \|\| 0\) > 0/
  )
})
