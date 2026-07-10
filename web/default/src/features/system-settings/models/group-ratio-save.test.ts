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
  buildGroupRatioOverrideRows,
  GROUP_RATIO_REQUEST_CONFIG,
  getPackageGroupDisplayState,
  requireGroupRatioOptionsData,
  requireSuccessfulOptionResponse,
  saveGroupRatioChanges,
  serializeGroupRatioOverrideRows,
  type GroupRatioFormValues,
  type GroupRatioOptionsResponse,
} from './group-ratio-save'

const baseline: GroupRatioFormValues = {
  GroupRatio: '{"default":1}',
  TopupGroupRatio: '{"default":1}',
  UserUsableGroups: '{"default":"Default"}',
  GroupGroupRatio: '{"regular":{"default":1}}',
  AutoGroups: '["default"]',
  DefaultUseAutoGroup: false,
  GroupSpecialUsableGroup: '{}',
}

function successfulPairResponse(
  groupRatio: string,
  groupGroupRatio: string
): GroupRatioOptionsResponse {
  return {
    success: true,
    message: '',
    data: {
      group_ratio: groupRatio,
      group_group_ratio: groupGroupRatio,
      package_groups: ['day-card'],
    },
  }
}

test('business failure throws even when the transport returned a response', () => {
  assert.throws(
    () =>
      requireSuccessfulOptionResponse({
        success: false,
        message: 'validation failed',
      }),
    /validation failed/
  )
})

test('group ratio requests leave error presentation to the owning UI flow', () => {
  assert.deepEqual(GROUP_RATIO_REQUEST_CONFIG, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
})

test('a successful group ratio response without data throws', () => {
  assert.throws(
    () => requireGroupRatioOptionsData({ success: true, message: '' }),
    /missing group ratio snapshot/i
  )
})

test('a changed ratio pair is sent exactly once as two normalized JSON strings', async () => {
  const requests: Array<{
    group_ratio: string
    group_group_ratio: string
  }> = []
  let genericCalls = 0

  const result = await saveGroupRatioChanges(
    {
      ...baseline,
      GroupRatio: '{\n  "vip": 2,\n  "default": 1\n}',
      GroupGroupRatio: '{\n  "day-card": { "vip": 0.5 }\n}',
    },
    baseline,
    {
      updateGroupRatioOptions: async (request) => {
        requests.push(request)
        return successfulPairResponse(
          '{"vip":2,"default":1}',
          '{"day-card":{"vip":0.5}}'
        )
      },
      updateSystemOption: async () => {
        genericCalls += 1
        return { success: true, message: '' }
      },
    }
  )

  assert.deepEqual(requests, [
    {
      group_ratio: '{"vip":2,"default":1}',
      group_group_ratio: '{"day-card":{"vip":0.5}}',
    },
  ])
  assert.equal(genericCalls, 0)
  assert.equal(result.changed, true)
  assert.deepEqual(result.baseline, {
    ...baseline,
    GroupRatio: '{"vip":2,"default":1}',
    GroupGroupRatio: '{"day-card":{"vip":0.5}}',
  })
})

test('changing only GroupGroupRatio still sends both pair values once', async () => {
  const requests: Array<{
    group_ratio: string
    group_group_ratio: string
  }> = []

  await saveGroupRatioChanges(
    {
      ...baseline,
      GroupGroupRatio: '{ "regular": { "vip": 0.75 } }',
    },
    baseline,
    {
      updateGroupRatioOptions: async (request) => {
        requests.push(request)
        return successfulPairResponse(
          request.group_ratio,
          request.group_group_ratio
        )
      },
      updateSystemOption: async () => ({ success: true, message: '' }),
    }
  )

  assert.deepEqual(requests, [
    {
      group_ratio: baseline.GroupRatio,
      group_group_ratio: '{"regular":{"vip":0.75}}',
    },
  ])
})

test('changing only GroupRatio still sends both pair values once', async () => {
  const requests: Array<{
    group_ratio: string
    group_group_ratio: string
  }> = []

  await saveGroupRatioChanges(
    { ...baseline, GroupRatio: '{ "default": 1, "vip": 2 }' },
    baseline,
    {
      updateGroupRatioOptions: async (request) => {
        requests.push(request)
        return successfulPairResponse(
          request.group_ratio,
          request.group_group_ratio
        )
      },
      updateSystemOption: async () => ({ success: true, message: '' }),
    }
  )

  assert.deepEqual(requests, [
    {
      group_ratio: '{"default":1,"vip":2}',
      group_group_ratio: baseline.GroupGroupRatio,
    },
  ])
})

test('a successful pair response must contain a server snapshot', async () => {
  await assert.rejects(
    saveGroupRatioChanges(
      { ...baseline, GroupRatio: '{"default":2}' },
      baseline,
      {
        updateGroupRatioOptions: async () => ({ success: true, message: '' }),
        updateSystemOption: async () => ({ success: true, message: '' }),
      }
    ),
    /missing group ratio snapshot/i
  )
})

test('server pair readback must match the normalized submission', async () => {
  await assert.rejects(
    saveGroupRatioChanges(
      { ...baseline, GroupRatio: '{"default":2}' },
      baseline,
      {
        updateGroupRatioOptions: async () =>
          successfulPairResponse('{"default":3}', baseline.GroupGroupRatio),
        updateSystemOption: async () => ({ success: true, message: '' }),
      }
    ),
    /does not match/i
  )
})

test('pair submission mirrors server key trimming and empty-parent removal', async () => {
  const requests: Array<{
    group_ratio: string
    group_group_ratio: string
  }> = []

  const result = await saveGroupRatioChanges(
    {
      ...baseline,
      GroupRatio: '{ " vip ": 2, " default ": 1 }',
      GroupGroupRatio: '{ " empty ": {}, " day-card ": { " vip ": 0.8 } }',
    },
    baseline,
    {
      updateGroupRatioOptions: async (request) => {
        requests.push(request)
        return successfulPairResponse(
          '{"default":1,"vip":2}',
          '{"day-card":{"vip":0.8}}'
        )
      },
      updateSystemOption: async () => ({ success: true, message: '' }),
    }
  )

  assert.deepEqual(requests, [
    {
      group_ratio: '{"vip":2,"default":1}',
      group_group_ratio: '{"day-card":{"vip":0.8}}',
    },
  ])
  assert.deepEqual(result.baseline, {
    ...baseline,
    GroupRatio: '{"default":1,"vip":2}',
    GroupGroupRatio: '{"day-card":{"vip":0.8}}',
  })
})

test('an empty parent still participates in trimmed parent conflict checks', async () => {
  let pairCalls = 0

  await assert.rejects(
    saveGroupRatioChanges(
      {
        ...baseline,
        GroupGroupRatio: '{ " duplicate ": {}, "duplicate": {"vip": 0.8} }',
      },
      baseline,
      {
        updateGroupRatioOptions: async () => {
          pairCalls += 1
          return successfulPairResponse(
            baseline.GroupRatio,
            '{"duplicate":{"vip":0.8}}'
          )
        },
        updateSystemOption: async () => ({ success: true, message: '' }),
      }
    ),
    /parent conflicts after trimming/i
  )

  assert.equal(pairCalls, 0)
})

test('generic options use the direct API contract and reject success false', async () => {
  const requests: Array<{ key: string; value: string | boolean | number }> = []

  await assert.rejects(
    saveGroupRatioChanges({ ...baseline, AutoGroups: '["vip"]' }, baseline, {
      updateGroupRatioOptions: async () =>
        successfulPairResponse(baseline.GroupRatio, baseline.GroupGroupRatio),
      updateSystemOption: async (request) => {
        requests.push(request)
        return { success: false, message: 'generic failed' }
      },
    }),
    /generic failed/
  )

  assert.deepEqual(requests, [{ key: 'AutoGroups', value: '["vip"]' }])
})

test('a later generic failure leaves the caller baseline untouched', async () => {
  const originalBaseline = structuredClone(baseline)
  let baselineCommits = 0
  let pairCalls = 0

  await assert.rejects(
    saveGroupRatioChanges(
      {
        ...baseline,
        GroupRatio: '{"default":2}',
        TopupGroupRatio: '{"default":1.2}',
      },
      baseline,
      {
        updateGroupRatioOptions: async (request) => {
          pairCalls += 1
          return successfulPairResponse(
            request.group_ratio,
            request.group_group_ratio
          )
        },
        updateSystemOption: async () => ({
          success: false,
          message: 'later request failed',
        }),
        commitBaseline: () => {
          baselineCommits += 1
        },
      }
    ),
    /later request failed/
  )

  assert.equal(pairCalls, 1)
  assert.equal(baselineCommits, 0)
  assert.deepEqual(baseline, originalBaseline)
})

test('baseline commits once at the end using server pair and submitted generic snapshots', async () => {
  const events: string[] = []
  const commits: GroupRatioFormValues[] = []

  const result = await saveGroupRatioChanges(
    {
      ...baseline,
      GroupRatio: '{ "vip": 2, "default": 1 }',
      GroupGroupRatio: '{ "regular": { "vip": 0.75 } }',
      TopupGroupRatio: '{ "default": 1.2 }',
      AutoGroups: '[ "vip" ]',
    },
    baseline,
    {
      updateGroupRatioOptions: async () => {
        events.push('pair')
        return successfulPairResponse(
          '{"default":1,"vip":2}',
          '{"regular":{"vip":0.75}}'
        )
      },
      updateSystemOption: async ({ key }) => {
        events.push(`generic:${key}`)
        return { success: true, message: '' }
      },
      commitBaseline: (nextBaseline) => {
        events.push('commit')
        commits.push(nextBaseline)
      },
    }
  )

  const expectedBaseline: GroupRatioFormValues = {
    ...baseline,
    GroupRatio: '{"default":1,"vip":2}',
    GroupGroupRatio: '{"regular":{"vip":0.75}}',
    TopupGroupRatio: '{"default":1.2}',
    AutoGroups: '["vip"]',
  }
  assert.deepEqual(events, [
    'pair',
    'generic:TopupGroupRatio',
    'generic:AutoGroups',
    'commit',
  ])
  assert.equal(commits.length, 1)
  assert.deepEqual(commits[0], expectedBaseline)
  assert.deepEqual(result.baseline, expectedBaseline)
})

test('no-op saves perform no requests', async () => {
  let calls = 0
  const result = await saveGroupRatioChanges(baseline, baseline, {
    updateGroupRatioOptions: async () => {
      calls += 1
      return successfulPairResponse(
        baseline.GroupRatio,
        baseline.GroupGroupRatio
      )
    },
    updateSystemOption: async () => {
      calls += 1
      return { success: true, message: '' }
    },
  })

  assert.equal(calls, 0)
  assert.equal(result.changed, false)
  assert.deepEqual(result.baseline, baseline)
})

test('empty loading defaults remain a no-op without issuing requests', async () => {
  const emptyBaseline: GroupRatioFormValues = {
    GroupRatio: '',
    TopupGroupRatio: '',
    UserUsableGroups: '',
    GroupGroupRatio: '',
    AutoGroups: '',
    DefaultUseAutoGroup: false,
    GroupSpecialUsableGroup: '',
  }
  let calls = 0

  const result = await saveGroupRatioChanges(emptyBaseline, emptyBaseline, {
    updateGroupRatioOptions: async () => {
      calls += 1
      return successfulPairResponse('{}', '{}')
    },
    updateSystemOption: async () => {
      calls += 1
      return { success: true, message: '' }
    },
  })

  assert.equal(calls, 0)
  assert.equal(result.changed, false)
  assert.deepEqual(result.baseline, emptyBaseline)
})

test('package group query errors are not treated as a ready empty list', () => {
  assert.deepEqual(
    getPackageGroupDisplayState({ isPending: false, isError: true }),
    { status: 'error' }
  )
  assert.deepEqual(
    getPackageGroupDisplayState({ isPending: true, isError: false }),
    { status: 'loading' }
  )
  assert.deepEqual(
    getPackageGroupDisplayState({
      isPending: false,
      isError: false,
      packageGroups: [],
    }),
    { status: 'ready', packageGroups: [] }
  )
})

test('package groups merge into override rows without persisting empty parents', () => {
  const rows = buildGroupRatioOverrideRows(
    '{"day-card":{"vip":0.8},"regular":{"default":1},"empty":{}}',
    ['day-card', 'week-card', 'week-card', '']
  )

  assert.deepEqual(rows, [
    {
      userGroup: 'day-card',
      overrides: [{ targetGroup: 'vip', ratio: 0.8 }],
      isPackageGroup: true,
      isVirtual: false,
    },
    {
      userGroup: 'regular',
      overrides: [{ targetGroup: 'default', ratio: 1 }],
      isPackageGroup: false,
      isVirtual: false,
    },
    {
      userGroup: 'empty',
      overrides: [],
      isPackageGroup: false,
      isVirtual: false,
    },
    {
      userGroup: 'week-card',
      overrides: [],
      isPackageGroup: true,
      isVirtual: true,
    },
  ])
  assert.equal(
    serializeGroupRatioOverrideRows(rows),
    '{\n  "day-card": {\n    "vip": 0.8\n  },\n  "regular": {\n    "default": 1\n  },\n  "empty": {}\n}'
  )

  const materializedRows = rows.map((row) =>
    row.userGroup === 'week-card'
      ? {
          ...row,
          overrides: [{ targetGroup: 'default', ratio: 1 }],
          isVirtual: false,
        }
      : row
  )
  assert.equal(
    serializeGroupRatioOverrideRows(materializedRows),
    '{\n  "day-card": {\n    "vip": 0.8\n  },\n  "regular": {\n    "default": 1\n  },\n  "empty": {},\n  "week-card": {\n    "default": 1\n  }\n}'
  )
})
