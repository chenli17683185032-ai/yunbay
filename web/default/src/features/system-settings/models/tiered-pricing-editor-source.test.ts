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
const editorSource = readFileSync(
  resolve(currentDir, 'tiered-pricing-editor.tsx'),
  'utf8'
)

test('Claude pricing presets use family-level labels without changing expression semantics', () => {
  assert.match(editorSource, /label: 'Claude Opus 4\.6 \/ 4\.7 \/ 4\.8'/)
  assert.match(editorSource, /label: 'Claude Sonnet 4\.5 \/ 4\.6'/)
  assert.match(editorSource, /label: 'Claude Opus Fast'/)
  assert.doesNotMatch(editorSource, /label: 'Claude Opus 4\.6'/)
  assert.doesNotMatch(editorSource, /label: 'Claude Sonnet 4\.5'/)
  assert.doesNotMatch(editorSource, /label: 'Claude Opus 4\.6 Fast'/)
})

test('Claude Sonnet preset still uses len-based long context tiering and Claude 1h cache create', () => {
  assert.match(editorSource, /len <= 200000 \? tier\("standard"/)
  assert.match(editorSource, /tier\("long_context", p \* 6 \+ c \* 22\.5 \+ cr \* 0\.6 \+ cc \* 7\.5 \+ cc1h \* 12\)/)
  assert.match(editorSource, /path: 'anthropic-beta'/)
  assert.match(editorSource, /value: 'fast-mode-2026-02-01'/)
  assert.match(editorSource, /multiplier: '6'/)
})
