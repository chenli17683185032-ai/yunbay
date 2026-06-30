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
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const cardSource = readFileSync(
  resolve(currentDir, 'affiliate-rewards-card.tsx'),
  'utf8'
)
const walletSource = readFileSync(resolve(currentDir, '../index.tsx'), 'utf8')
const dialogPath = resolve(
  currentDir,
  'dialogs/affiliate-withdrawal-dialog.tsx'
)

test('affiliate rewards card displays monetary commission stats and withdrawal action', () => {
  assert.match(cardSource, /affiliateSummary/)
  assert.match(cardSource, /Available Rewards/)
  assert.match(cardSource, /Frozen Rewards/)
  assert.match(cardSource, /Withdrawn Rewards/)
  assert.match(cardSource, /Commission Rate/)
  assert.match(cardSource, /Apply for Withdrawal/)
})

test('wallet page wires affiliate summary and withdrawal dialog callbacks', () => {
  assert.match(walletSource, /affiliateWithdrawalDialogOpen/)
  assert.match(walletSource, /AffiliateWithdrawalDialog/)
  assert.match(walletSource, /requestWithdrawal/)
  assert.match(walletSource, /refetchSummary/)
})

test('affiliate withdrawal dialog uses accessible dialog and field composition', () => {
  assert.equal(existsSync(dialogPath), true)
  const dialogSource = readFileSync(dialogPath, 'utf8')

  assert.match(dialogSource, /title=\{t\('Apply for Withdrawal'\)\}/)
  assert.match(dialogSource, /FieldGroup/)
  assert.match(dialogSource, /FieldLabel/)
  assert.match(dialogSource, /FieldError/)
  assert.match(dialogSource, /validateAffiliateWithdrawalInput/)
  assert.match(dialogSource, /Contact information/)
  assert.match(dialogSource, /After submitting, this amount will be frozen/)
})
