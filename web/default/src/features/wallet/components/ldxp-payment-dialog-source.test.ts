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
const componentSource = readFileSync(
  resolve(currentDir, 'ldxp-payment-dialog.tsx'),
  'utf8'
)
const cssSource = readFileSync(
  resolve(currentDir, '../../../styles/index.css'),
  'utf8'
)

test('ldxp payment dialog uses a prominent popup spinner while QR code is being created', () => {
  assert.match(componentSource, /ldxp-qr-creation-pop/)
  assert.match(componentSource, /ldxp-qr-creation-spinner/)
  assert.match(componentSource, /size-24/)
  assert.match(componentSource, /Creating payment QR code/)
  assert.match(
    componentSource,
    /The payment QR code usually appears in about 20 seconds\. Please wait\./
  )
})

test('ldxp QR creation popup has explicit motion and reduced-motion styles', () => {
  assert.match(cssSource, /@keyframes ldxp-qr-creation-pop/)
  assert.match(cssSource, /@keyframes ldxp-qr-creation-pulse/)
  assert.match(cssSource, /@keyframes ldxp-qr-creation-spinner/)
  assert.match(cssSource, /\.ldxp-qr-creation-pop/)
  assert.match(cssSource, /prefers-reduced-motion: reduce/)
  assert.match(cssSource, /\.ldxp-qr-creation-spinner/)
})
