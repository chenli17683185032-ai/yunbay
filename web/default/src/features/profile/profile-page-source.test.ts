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

const SOURCE = readFileSync(join(import.meta.dirname, 'index.tsx'), 'utf8')

test('profile page only shows sidebar module settings to admin users with permission', () => {
  assert.match(SOURCE, /userRole\s*>?=\s*ROLE\.ADMIN/)
  assert.match(SOURCE, /permissions\?\.sidebar_settings\s*!==\s*false/)
  assert.match(SOURCE, /canConfigureSidebar\s*&&\s*<SidebarModulesCard\s*\/>/)
})

test('profile page promotes check-in card before account settings grid', () => {
  const headerIndex = SOURCE.indexOf('<ProfileHeader')
  const checkinIndex = SOURCE.indexOf('<CheckinCalendarCard')
  const settingsIndex = SOURCE.indexOf('<ProfileSettingsCard')
  assert.ok(headerIndex >= 0, 'ProfileHeader should be rendered')
  assert.ok(checkinIndex > headerIndex, 'CheckinCalendarCard should follow ProfileHeader')
  assert.ok(settingsIndex > checkinIndex, 'ProfileSettingsCard should follow CheckinCalendarCard')
})
