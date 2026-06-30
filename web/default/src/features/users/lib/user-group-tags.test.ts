import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildUserGroupTagOptions } from './user-group-tags'

test('user group tag options keep configured user tags and current legacy value', () => {
  const options = buildUserGroupTagOptions(
    [
      { value: '体验用户', label: '体验用户' },
      { value: 'vip', label: 'VIP 用户' },
    ],
    'default'
  )

  assert.deepEqual(options, [
    { value: 'default', label: 'default（当前值）' },
    { value: '体验用户', label: '体验用户' },
    { value: 'vip', label: 'VIP 用户' },
  ])
  assert.equal(options.some((option) => option.value === 'gpt-plus'), false)
  assert.equal(options.some((option) => option.value === 'gpt-pro'), false)
})

test('user group tag options do not duplicate current configured tag', () => {
  const options = buildUserGroupTagOptions(
    [
      { value: '体验用户', label: '体验用户' },
      { value: 'vip', label: 'VIP 用户' },
    ],
    'vip'
  )

  assert.deepEqual(options, [
    { value: '体验用户', label: '体验用户' },
    { value: 'vip', label: 'VIP 用户' },
  ])
})
