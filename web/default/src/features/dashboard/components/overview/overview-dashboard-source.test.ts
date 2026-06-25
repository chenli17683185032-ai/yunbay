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
  new URL('./overview-dashboard.tsx', import.meta.url),
  'utf8'
)

test('overview dashboard keeps usage, announcements, and admin performance cards without restoring hidden panels', () => {
  assert.match(source, /import \{ SummaryCards \} from '\.\/summary-cards'/)
  assert.match(
    source,
    /import \{ PerformanceHealthPanel \} from '\.\/performance-health-panel'/
  )
  assert.match(
    source,
    /import \{ AnnouncementsPanel \} from '\.\/announcements-panel'/
  )

  assert.match(source, /<SummaryCards \/>/)
  assert.match(source, /<PerformanceHealthPanel \/>/)
  assert.match(source, /<AnnouncementsPanel \/>/)

  assert.doesNotMatch(source, /<ApiInfoPanel \/>/)
  assert.doesNotMatch(source, /<UptimePanel \/>/)
})
