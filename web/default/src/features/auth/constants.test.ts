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
import { registerFormSchema } from './constants'

const validRegistration = {
  username: 'qquser',
  email: 'qquser@qq.com',
  password: 'password123',
  confirmPassword: 'password123',
}

test('register form requires a QQ email address', () => {
  assert.equal(registerFormSchema.safeParse(validRegistration).success, true)

  for (const email of [
    '',
    'qquser@gmail.com',
    'qquser@foxmail.com',
    'qquser@sub.qq.com',
    'not-an-email',
  ]) {
    const result = registerFormSchema.safeParse({
      ...validRegistration,
      email,
    })
    assert.equal(result.success, false, `${email} should be rejected`)
  }
})

test('register form keeps the invitation code field', () => {
  const result = registerFormSchema.safeParse({
    ...validRegistration,
    affCode: 'INVITE001',
  })

  assert.equal(result.success, true)
  assert.equal(result.data.affCode, 'INVITE001')
})
