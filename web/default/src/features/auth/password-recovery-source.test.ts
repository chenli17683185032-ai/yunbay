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
const signInSource = readFileSync(
  resolve(currentDir, 'sign-in/components/user-auth-form.tsx'),
  'utf8'
)
const forgotPasswordSource = readFileSync(
  resolve(currentDir, 'forgot-password/components/forgot-password-form.tsx'),
  'utf8'
)
const resetConfirmSource = readFileSync(
  resolve(currentDir, 'reset-password-confirm/index.tsx'),
  'utf8'
)

test('sign-in form exposes a dedicated password recovery button', () => {
  assert.match(signInSource, /render=\{<Link to='\/forgot-password' \/>\}/)
  assert.match(signInSource, /t\('Recover password'\)/)
  assert.doesNotMatch(signInSource, /t\('Forgot password\?'\)/)
})

test('forgot password form verifies a code and submits a custom password', () => {
  assert.match(forgotPasswordSource, /sendPasswordResetEmail/)
  assert.match(forgotPasswordSource, /resetPassword/)
  assert.match(forgotPasswordSource, /name='code'/)
  assert.match(forgotPasswordSource, /name='password'/)
  assert.match(forgotPasswordSource, /name='confirmPassword'/)
  assert.match(forgotPasswordSource, /InputOTP/)
  assert.match(forgotPasswordSource, /password: data\.password/)
})

test('reset link confirmation lets the user choose a new password', () => {
  assert.match(resetConfirmSource, /resetPassword/)
  assert.match(resetConfirmSource, /PasswordInput/)
  assert.match(resetConfirmSource, /password: data\.password/)
  assert.doesNotMatch(resetConfirmSource, /setNewPassword\(password\)/)
})
