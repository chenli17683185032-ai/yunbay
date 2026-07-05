import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(
  new URL('./value-package-card.tsx', import.meta.url),
  'utf8'
)
const pageSource = readFileSync(
  new URL('../index.tsx', import.meta.url),
  'utf8'
)

test('value package user cards expose redemption code input and action', () => {
  assert.match(source, /redemptionCode/)
  assert.match(source, /onRedeemCode/)
  assert.match(source, /Enter your redemption code or card key/)
  assert.match(source, /Redeem Code/)
})

test('value packages page refreshes package state after redemption succeeds', () => {
  assert.match(pageSource, /useRedemption/)
  assert.match(pageSource, /handleRedeemCode/)
  assert.match(pageSource, /valuePackages\.refresh\(\)/)
})
