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
import { getActiveValuePackageBillingLabel, getActiveValuePackageBillingRatio } from './billing-display'

test('active billing ratio comes from backend billing state', () => {
  const state = {
    preference: { id: 1, user_id: 1, enabled: true, active_user_subscription_id: 2, created_at: 1, updated_at: 1 },
    billing: { active: true, routing_group: '', package_group: 'month-card', effective_ratio: 1, original_group_ratio: 0, plan_title: '月卡', plan_id: 2 },
  }

  assert.equal(getActiveValuePackageBillingRatio(state), 1)
  assert.equal(getActiveValuePackageBillingLabel(state), '月卡 · month-card')
})

test('inactive package has no billing ratio', () => {
  const state = {
    preference: { id: 1, user_id: 1, enabled: false, active_user_subscription_id: 0, created_at: 1, updated_at: 2 },
    billing: { active: false, routing_group: '', package_group: '', effective_ratio: 0, original_group_ratio: 0, plan_title: '', plan_id: 0 },
  }

  assert.equal(getActiveValuePackageBillingRatio(state), undefined)
  assert.equal(getActiveValuePackageBillingLabel(state), null)
})
