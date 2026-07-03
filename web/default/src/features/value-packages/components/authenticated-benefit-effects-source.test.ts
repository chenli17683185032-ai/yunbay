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
  './authenticated-benefit-effects.tsx',
  import.meta.url
)
const layoutPath = new URL(
  '../../../components/layout/components/authenticated-layout.tsx',
  import.meta.url
)

test('authenticated benefit effects source wires package and vip global effects', async () => {
  const source = await readFile(componentPath, 'utf8')

  assert.match(source, /getValuePackageSelf/)
  assert.match(source, /shouldShowPackageGlow/)
  assert.match(source, /getBenefitGlowMode/)
  assert.match(source, /mode === 'package'/)
  assert.match(source, /yunbay-viewport-benefit-glow--package/)
  assert.match(source, /mode === 'vip'/)
  assert.match(source, /markVipUpgradeModalSeen/)
  assert.match(source, /withVipUpgradeModalSeen/)
  assert.match(source, /恭喜你获得会员权益|VIP membership benefits/)
})

test('authenticated layout mounts benefit effects globally', async () => {
  const source = await readFile(layoutPath, 'utf8')

  assert.match(source, /AuthenticatedBenefitEffects/)
})
