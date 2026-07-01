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
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const pageSource = readFileSync(resolve(currentDir, 'index.tsx'), 'utf8')
const ccSwitchSource = readFileSync(
  resolve(currentDir, 'quick-start-cc-switch.ts'),
  'utf8'
)

test('quick start bottom controls make the primary CTA more prominent without changing flow logic', () => {
  assert.match(pageSource, /shadow-\[0_18px_60px_rgba\(255,255,255,0\.22\)\]/)
  assert.match(pageSource, /ring-1 ring-white\/30/)
  assert.match(pageSource, /font-black/)
  assert.match(pageSource, /hover:-translate-y-0\.5/)
  assert.match(pageSource, /const nextLabel = props\.api\.canGoNext \? t\('Next'\) : t\('Enter dashboard'\)/)
  assert.match(
    pageSource,
    /const handleNext = props\.api\.canGoNext\s*\?\s*props\.api\.goNext\s*:\s*props\.onEnterDashboard/
  )
})

test('quick start secondary dashboard CTA remains visible but lower priority', () => {
  assert.match(pageSource, /border-white\/18/)
  assert.match(pageSource, /bg-white\/\[0\.07\]/)
  assert.match(pageSource, /text-white\/82/)
  assert.match(pageSource, /hover:bg-white\/\[0\.11\]/)
  assert.match(pageSource, /onClick=\{props\.onEnterDashboard\}/)
  assert.match(pageSource, /onClick=\{handleNext\}/)
})


test('quick start fifth page keeps Codex downloads, software guide, and CC Switch one-click import', () => {
  assert.match(pageSource, /Codex one-click launcher/)
  assert.match(pageSource, /Codex one-click setup/)
  assert.match(pageSource, /Download the Codex one-click launcher and connect it to your Yunbay API key\./)
  assert.match(pageSource, /card\.guideTitleKey/)
  assert.match(pageSource, /card\.guideDescriptionKey/)
  assert.match(pageSource, /card\.guideStepKeys/)
  assert.match(pageSource, /Import current setup to CC Switch/)
  assert.match(pageSource, /Launch CC Switch from your browser with this API and model prefilled\./)
  assert.match(pageSource, /buildQuickStartCCSwitchImportURL/)
  assert.match(pageSource, /handleImportToCCSwitch/)
  assert.match(pageSource, /QuickStartConfigPill/)
  assert.match(pageSource, /Configured API/)
  assert.match(pageSource, /Configured model/)
  assert.match(pageSource, /Generated API key/)
  assert.match(ccSwitchSource, /ccswitch:\/\/v1\/import/)
  assert.match(pageSource, /CC Switch/)
  assert.doesNotMatch(pageSource, /Official Codex/)
  assert.doesNotMatch(pageSource, /Download Codex/)
})
