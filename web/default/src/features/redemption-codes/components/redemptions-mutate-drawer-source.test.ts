import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(
  new URL('./redemptions-mutate-drawer.tsx', import.meta.url),
  'utf8'
)

test('redemption drawer can create subscription value package redemption codes', () => {
  assert.match(source, /getAdminPlans/)
  assert.match(source, /valuePackagePlanOptions/)
  assert.match(source, /plan_kind === 'value_package'/)
  assert.match(source, /value === 'subscription'/)
  assert.match(source, /plan_id/)
  assert.match(source, /Value package plan/)
})
