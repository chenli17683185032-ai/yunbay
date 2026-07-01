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
import { buildVisibleTopNavLinks } from './top-nav-link-policy'

test('top nav keeps only home, console, and model square links', () => {
  const links = buildVisibleTopNavLinks({
    modules: {
      home: true,
      console: true,
      pricing: { enabled: true, requireAuth: false },
      rankings: { enabled: true, requireAuth: false },
      docs: true,
      about: true,
    },
    isAuthed: false,
    t: (key) => key,
  })

  assert.deepEqual(
    links.map((link) => link.title),
    ['Home', 'Console', 'Model Square']
  )
})

test('top nav keeps pricing auth requirement when the user is unauthenticated', () => {
  const [home, consoleLink, pricingLink] = buildVisibleTopNavLinks({
    modules: {
      home: true,
      console: true,
      pricing: { enabled: true, requireAuth: true },
      rankings: { enabled: true, requireAuth: false },
      docs: true,
      about: true,
    },
    isAuthed: false,
    t: (key) => key,
  })

  assert.equal(home?.requiresAuth, undefined)
  assert.equal(consoleLink?.requiresAuth, undefined)
  assert.equal(pricingLink?.requiresAuth, true)
})
