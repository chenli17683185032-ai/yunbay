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
  './api-keys-wallet-fallback-toggle.tsx',
  import.meta.url
)
const primaryButtonsPath = new URL(
  './api-keys-primary-buttons.tsx',
  import.meta.url
)

test('API keys page exposes default-on value package wallet fallback toggle', async () => {
  const source = await readFile(componentPath, 'utf8')
  const primaryButtonsSource = await readFile(primaryButtonsPath, 'utf8')

  assert.match(source, /wallet_fallback_enabled !== false/)
  assert.match(source, /updateValuePackageWalletFallback/)
  assert.match(source, /Uninterrupted Boost/)
  assert.match(source, /ZapIcon/)
  assert.match(source, /CircleQuestionMarkIcon/)
  assert.match(source, /TooltipProvider/)
  assert.match(source, /PopoverDescription/)
  assert.match(source, /hidden sm:block/)
  assert.match(source, /sm:hidden/)
  assert.match(source, /<Switch/)
  assert.match(primaryButtonsSource, /<ApiKeysWalletFallbackToggle \/>/)
})
