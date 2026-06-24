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
import { redeemQuickStartCode } from './quick-start-redemption'

test('quick start redeems a pasted code in place and refreshes the user', async () => {
  const calls: string[] = []

  const result = await redeemQuickStartCode('  CODE-123  ', {
    redeemTopupCode: async (request) => {
      calls.push(`redeem:${request.key}`)
      return { success: true, data: 500 }
    },
    refreshSelf: async () => {
      calls.push('refresh')
    },
  })

  assert.equal(result.quotaAdded, 500)
  assert.deepEqual(calls, ['redeem:CODE-123', 'refresh'])
})

test('quick start refuses an empty redemption code before calling the API', async () => {
  await assert.rejects(
    redeemQuickStartCode('   ', {
      redeemTopupCode: async () => {
        throw new Error('API should not be called')
      },
      refreshSelf: async () => {
        throw new Error('refresh should not be called')
      },
    }),
    /redemption code/i
  )
})
