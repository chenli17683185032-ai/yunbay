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
import { join } from 'node:path'
import test from 'node:test'

const SOURCE = readFileSync(
  join(import.meta.dirname, 'model-price-sync-dialog.tsx'),
  'utf8'
)

test('model price sync dialog does not require an OpenRouter channel', () => {
  assert.doesNotMatch(SOURCE, /getUpstreamChannels/)
  assert.doesNotMatch(SOURCE, /OPENROUTER_CHANNEL_TYPE/)
  assert.doesNotMatch(SOURCE, /OpenRouter channel/)
  assert.doesNotMatch(SOURCE, /No OpenRouter channel found/)
  assert.doesNotMatch(SOURCE, /openrouter_channel_id/)
  assert.match(SOURCE, /const canPreview = selectedModels\.length > 0/)
  assert.match(SOURCE, /models: selectedModels/)
})
