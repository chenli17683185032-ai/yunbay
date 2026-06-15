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
import test from 'node:test'
import {
  ABOUT_COPY,
  MODEL_SQUARE_COPY,
} from './landing-page-copy'

const OLD_MODEL_SQUARE_PROMISE =
  'A model relay should feel like infrastructure: one key, one bill, one control surface, many upstream skies.'

test('model square owns the former hero promise and the GBT5.5 price promise', () => {
  assert.equal(
    MODEL_SQUARE_COPY.headline,
    'One key carries the world models to your interface'
  )
  assert.deepEqual(MODEL_SQUARE_COPY.promises, [
    'yunbay connects mainstream model APIs through unified keys, routing, billing, quotas, failover, and OpenAI-compatible interfaces.',
    'GBT5.5 full-power official edition at one tenth of the official price.',
  ])
  assert.equal(
    (MODEL_SQUARE_COPY.supportingPoints as readonly string[]).includes(
      OLD_MODEL_SQUARE_PROMISE
    ),
    false
  )
})

test('about page owns the former model-square narrative', () => {
  assert.equal(ABOUT_COPY.headline, 'Mainstream models, one outbound channel')
  assert.deepEqual(ABOUT_COPY.points.slice(0, 2), [
    'Connect OpenAI, Claude, Gemini, DeepSeek, Qwen, Llama, and other upstream routes behind one customer-facing API.',
    OLD_MODEL_SQUARE_PROMISE,
  ])
})
