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
  EMPTY_QUICK_START_SESSION,
  normalizeQuickStartSession,
} from './quick-start-session'

test('quick start session keeps only non-sensitive completion state', () => {
  const state = normalizeQuickStartSession({
    modelName: 'gpt-5.6-sol',
    platform: 'macOS',
    softwareConfirmed: true,
    importAttempted: true,
    importConfirmed: true,
    completionPromptPending: true,
    apiKey: 'sk-should-never-be-persisted',
  })

  assert.deepEqual(state, {
    modelName: 'gpt-5.6-sol',
    platform: 'macOS',
    softwareConfirmed: true,
    importAttempted: true,
    importConfirmed: true,
    completionPromptPending: true,
  })
  assert.equal(
    JSON.stringify(state).includes('sk-should-never-be-persisted'),
    false
  )
})

test('quick start session rejects malformed persisted values', () => {
  assert.deepEqual(normalizeQuickStartSession(null), EMPTY_QUICK_START_SESSION)
  assert.deepEqual(normalizeQuickStartSession({ platform: 'Linux' }), {
    ...EMPTY_QUICK_START_SESSION,
  })
})
