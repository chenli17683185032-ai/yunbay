import assert from 'node:assert/strict'
import test from 'node:test'
import { getActiveValuePackageBillingLabel, getActiveValuePackageBillingRatio } from './billing-display'

test('active billing ratio comes from backend billing state', () => {
  const state = {
    preference: { id: 1, user_id: 1, enabled: true, active_user_subscription_id: 2, created_at: 1, updated_at: 1 },
    billing: { active: true, effective_ratio: 1, plan_title: '月卡', package_group: 'month-card' },
  }

  assert.equal(getActiveValuePackageBillingRatio(state), 1)
  assert.equal(getActiveValuePackageBillingLabel(state), '月卡 · month-card')
})

test('inactive package has no billing ratio', () => {
  const state = {
    preference: { id: 1, user_id: 1, enabled: false, active_user_subscription_id: 0, created_at: 1, updated_at: 2 },
    billing: { active: false },
  }

  assert.equal(getActiveValuePackageBillingRatio(state), undefined)
  assert.equal(getActiveValuePackageBillingLabel(state), null)
})
