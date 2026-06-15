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
  getPostLoginPath,
  getPublicHeaderAuthedTarget,
  isAdminRole,
  isOrdinaryUserAllowedPath,
} from './post-login-routing'

test('admin roles default to dashboard overview', () => {
  assert.equal(isAdminRole(10), true)
  assert.equal(isAdminRole(100), true)
  assert.equal(getPostLoginPath({ role: 10 }), '/dashboard/overview')
})

test('ordinary users default to quick start', () => {
  assert.equal(isAdminRole(1), false)
  assert.equal(getPostLoginPath({ role: 1 }), '/quick-start')
})

test('ordinary users may keep ordinary redirects only', () => {
  assert.equal(isOrdinaryUserAllowedPath('/wallet?section=redeem'), true)
  assert.equal(
    getPostLoginPath({ role: 1 }, '/wallet?section=redeem'),
    '/wallet?section=redeem'
  )
  assert.equal(getPostLoginPath({ role: 1 }, '/keys'), '/keys')
  assert.equal(getPostLoginPath({ role: 1 }, '/chat2link'), '/chat2link')
})

test('ordinary users are redirected away from management paths', () => {
  assert.equal(isOrdinaryUserAllowedPath('/redemption-codes'), false)
  assert.equal(
    getPostLoginPath({ role: 1 }, '/redemption-codes'),
    '/wallet?section=redeem'
  )
  assert.equal(getPostLoginPath({ role: 1 }, '/channels'), '/quick-start')
  assert.equal(
    getPostLoginPath({ role: 1 }, '/dashboard/overview'),
    '/quick-start'
  )
})

test('unsafe redirects are ignored', () => {
  assert.equal(getPostLoginPath({ role: 1 }, 'https://example.com'), '/quick-start')
  assert.equal(getPostLoginPath({ role: 10 }, '//example.com'), '/dashboard/overview')
})

test('public header authenticated target is role-aware', () => {
  assert.equal(getPublicHeaderAuthedTarget({ role: 1 }), '/quick-start')
  assert.equal(getPublicHeaderAuthedTarget({ role: 10 }), '/dashboard/overview')
})
