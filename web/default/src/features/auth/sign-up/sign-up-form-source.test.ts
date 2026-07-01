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
const formSource = readFileSync(
  resolve(currentDir, 'components/sign-up-form.tsx'),
  'utf8'
)

test('sign-up form keeps the QQ email field visible outside the verification-only block', () => {
  assert.match(formSource, /name='email'/)
  assert.match(formSource, /Email \(QQ mailbox only\)/)
  assert.match(formSource, /name@qq\.com/)
  assert.match(formSource, /Only QQ email addresses are supported/)
  assert.doesNotMatch(formSource, /Email \(required for verification\)/)
  assert.doesNotMatch(formSource, /name@example\.com/)

  const emailFieldIndex = formSource.indexOf("name='email'")
  const verificationBlockIndex = formSource.indexOf(
    '{emailVerificationRequired &&'
  )
  assert.ok(emailFieldIndex > -1)
  assert.ok(verificationBlockIndex > -1)
  assert.ok(
    emailFieldIndex < verificationBlockIndex,
    'email field should stay visible even when email verification is disabled'
  )
})

test('sign-up form keeps manual invitation code input and submits it', () => {
  assert.match(formSource, /removeAffiliateCode/)
  assert.match(formSource, /affCode: ''/)
  assert.match(formSource, /form\.setValue\('affCode', initialAff\)/)
  assert.match(formSource, /name='affCode'/)
  assert.match(formSource, /Invitation code \(optional\)/)
  assert.match(formSource, /Leave blank if you do not have one/)
  assert.match(formSource, /const affCode = data\.affCode\?\.trim\(\) \|\| ''/)
  assert.match(formSource, /aff_code: affCode \|\| undefined/)
  assert.doesNotMatch(formSource, /aff_code: getAffiliateCode\(\)/)
})
