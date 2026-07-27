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
  getBenefitGlowMode,
  hasVipUpgradeModalSeen,
  isSvipByValidTopupCents,
  isSvipUser,
  shouldShowSvipCelebration,
  shouldShowVipCelebration,
  withSvipCelebrationSeen,
  withVipUpgradeModalSeen,
} from './benefit-effects'

test('package glow takes priority over vip glow', () => {
  assert.equal(
    getBenefitGlowMode({ packageGlow: true, isVipUser: true }),
    'package'
  )
})

test('svip glow takes priority over package and vip glow', () => {
  assert.equal(
    getBenefitGlowMode({
      packageGlow: true,
      isVipUser: true,
      isSvipUser: true,
    }),
    'svip'
  )
  assert.equal(
    getBenefitGlowMode({
      packageGlow: false,
      isVipUser: false,
      isSvipUser: true,
    }),
    'svip'
  )
})

test('svip is derived from is_svip flag or valid topup cents threshold', () => {
  assert.equal(isSvipByValidTopupCents(20000), true)
  assert.equal(isSvipByValidTopupCents(19999), false)
  assert.equal(isSvipByValidTopupCents(undefined), false)
  assert.equal(isSvipUser({ is_svip: true }), true)
  assert.equal(isSvipUser({ valid_topup_cents: 25000 }), true)
  assert.equal(isSvipUser({ is_svip: false, valid_topup_cents: 100 }), false)
  assert.equal(isSvipUser(null), false)
})

test('svip celebration shows for svip users until seen setting is true', () => {
  assert.equal(shouldShowSvipCelebration({ isSvip: true, setting: '{}' }), true)
  assert.equal(
    shouldShowSvipCelebration({
      isSvip: true,
      setting: '{"svip_celebration_seen":true}',
    }),
    false
  )
  assert.equal(
    shouldShowSvipCelebration({ isSvip: false, setting: '{}' }),
    false
  )
})

test('withSvipCelebrationSeen preserves existing user settings', () => {
  assert.deepEqual(withSvipCelebrationSeen('{"language":"zh"}'), {
    language: 'zh',
    svip_celebration_seen: true,
  })
})

test('vip glow shows only when no package glow is active', () => {
  assert.equal(
    getBenefitGlowMode({ packageGlow: false, isVipUser: true }),
    'vip'
  )
  assert.equal(
    getBenefitGlowMode({ packageGlow: false, isVipUser: false }),
    'none'
  )
})

test('vip celebration shows for vip users until seen setting is true', () => {
  assert.equal(shouldShowVipCelebration({ group: 'vip', setting: '{}' }), true)
  assert.equal(
    shouldShowVipCelebration({
      group: 'vip',
      setting: '{"vip_upgrade_modal_seen":true}',
    }),
    false
  )
  assert.equal(
    shouldShowVipCelebration({
      group: 'default',
      setting: '{"vip_upgrade_modal_seen":false}',
    }),
    false
  )
})

test('vip seen setting parser supports object and json string settings', () => {
  assert.equal(hasVipUpgradeModalSeen({ vip_upgrade_modal_seen: true }), true)
  assert.equal(hasVipUpgradeModalSeen('{"vip_upgrade_modal_seen":true}'), true)
  assert.equal(hasVipUpgradeModalSeen('not-json'), false)
})

test('withVipUpgradeModalSeen preserves existing user settings', () => {
  assert.deepEqual(withVipUpgradeModalSeen('{"language":"zh"}'), {
    language: 'zh',
    vip_upgrade_modal_seen: true,
  })
})
