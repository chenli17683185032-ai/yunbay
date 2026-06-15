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
import { getRegisterPromptState } from './register-link-visibility'

test('self-use mode does not hide the public register link', () => {
  assert.deepEqual(
    getRegisterPromptState({
      self_use_mode_enabled: true,
      register_enabled: true,
    }),
    {
      kind: 'link',
      textKey: "Don't have an account?",
      linkTextKey: 'Sign up now',
    }
  )
})

test('registration disabled shows a closed message', () => {
  assert.deepEqual(getRegisterPromptState({ register_enabled: false }), {
    kind: 'closed',
    textKey: 'Registration is currently closed',
  })
})
