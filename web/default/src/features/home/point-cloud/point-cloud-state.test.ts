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
  getFaceStateForQuota,
  getRuntimePointCloudSequence,
} from './point-cloud-state'

test('uses closed face when quota is missing or zero', () => {
  assert.equal(getFaceStateForQuota(undefined), 'closed')
  assert.equal(getFaceStateForQuota(null), 'closed')
  assert.equal(getFaceStateForQuota(0), 'closed')
  assert.equal(getFaceStateForQuota(-1), 'closed')
})

test('uses open face when quota is positive', () => {
  assert.equal(getFaceStateForQuota(1), 'open')
  assert.equal(getFaceStateForQuota(2500), 'open')
})

test('returns selected face, black hole, and Lorenz attractor sequence', () => {
  assert.deepEqual(getRuntimePointCloudSequence('closed'), [
    'face-closed',
    'black-hole',
    'lorenz-attractor',
  ])
  assert.deepEqual(getRuntimePointCloudSequence('open'), [
    'face-open',
    'black-hole',
    'lorenz-attractor',
  ])
})
