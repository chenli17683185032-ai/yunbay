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

const componentPath = new URL(
  './api-keys-package-billing-alert.tsx',
  import.meta.url
)
const pagePath = new URL('../index.tsx', import.meta.url)

test('API keys page has active value package billing explanation alert', async () => {
  const source = await readFile(componentPath, 'utf8')
  const pageSource = await readFile(pagePath, 'utf8')

  assert.match(source, /getValuePackageSelf/)
  assert.match(source, /valuePackageSelfQueryKey/)
  assert.match(source, /preference\.enabled/)
  assert.match(source, /currentPlan\.model_group/)
  assert.match(source, /user\?\.group/)
  assert.match(source, /Personal profile shows your user group/)
  assert.match(source, /currently billed through/)
  assert.match(source, /When you close package usage, API keys return to/)
  assert.match(source, /GroupBadge/)
  assert.match(pageSource, /ApiKeysPackageBillingAlert/)
})
