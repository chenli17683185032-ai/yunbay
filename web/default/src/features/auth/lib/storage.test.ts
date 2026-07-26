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
import { after, before, beforeEach, test } from 'node:test'
import {
  getAffiliateCode,
  removeAffiliateCode,
  saveAffiliateCode,
} from './storage'

const localStorageMock = (() => {
  let store: Record<string, string> = {}

  return {
    clear: () => {
      store = {}
    },
    getItem: (key: string) => store[key] ?? null,
    removeItem: (key: string) => {
      delete store[key]
    },
    setItem: (key: string, value: string) => {
      store[key] = value
    },
  }
})()

// Install the window stub only for this file's tests; a module-level
// stub leaks into later test files and breaks their module loading.
const originalWindowDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  'window'
)

before(() => {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { localStorage: localStorageMock },
  })
})

after(() => {
  if (originalWindowDescriptor) {
    Object.defineProperty(globalThis, 'window', originalWindowDescriptor)
  } else {
    delete (globalThis as { window?: unknown }).window
  }
})

beforeEach(() => {
  window.localStorage.clear()
})

test('affiliate code storage trims saved values and removes blank values', () => {
  saveAffiliateCode('  INVITE-CODE  ')

  assert.equal(getAffiliateCode(), 'INVITE-CODE')

  saveAffiliateCode('   ')

  assert.equal(getAffiliateCode(), '')
})

test('affiliate code storage can be removed explicitly', () => {
  saveAffiliateCode('INVITE-CODE')

  removeAffiliateCode()

  assert.equal(getAffiliateCode(), '')
})
