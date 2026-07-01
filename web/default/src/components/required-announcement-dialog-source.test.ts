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

const source = readFileSync(
  new URL('./required-announcement-dialog.tsx', import.meta.url),
  'utf8'
)

test('required announcement dialog cannot be dismissed without read confirmation', () => {
  assert.match(source, /showCloseButton=\{false\}/)
  assert.match(source, /const handleOpenChange = \(nextOpen: boolean\)/)
  assert.match(source, /if \(nextOpen\) return/)
  assert.match(source, /onOpenChange=\{handleOpenChange\}/)
  assert.match(source, /onClick=\{onConfirmRead\}/)
  assert.match(source, /t\('I have read'\)/)
  assert.match(source, /t\('Unread system announcements'\)/)
  assert.match(source, /t\('Please read these announcements before continuing\.'\)/)
})
