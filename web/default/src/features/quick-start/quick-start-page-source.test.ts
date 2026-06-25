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
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./index.tsx', import.meta.url), 'utf8')

test('quick start final page keeps the CC Switch import card wired to generated setup data', () => {
  assert.match(source, /buildQuickStartCCSwitchImportURL/)
  assert.match(source, /getQuickStartCCSwitchImportState/)
  assert.match(source, /maskQuickStartApiKey/)
  assert.match(source, /normalizeQuickStartCodexEndpoint/)
  assert.match(source, /handleImportToCCSwitch/)
  assert.match(source, /Import current setup to CC Switch/)
  assert.match(source, /Launch CC Switch from your browser/)
  assert.match(source, /Configured API/)
  assert.match(source, /Configured model/)
  assert.match(source, /Generated API key/)
  assert.match(source, /One-click import/)
})
