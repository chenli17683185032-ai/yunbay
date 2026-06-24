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
import { legacyConsoleLogSearchToUsageLogsSearch } from './legacy-console-log-route.ts'

test('maps legacy console log query filters to usage logs search filters', () => {
  assert.deepEqual(
    legacyConsoleLogSearchToUsageLogsSearch({
      p: '3',
      page_size: '50',
      type: '7',
      username: 'alice',
      token_name: 'prod-key',
      model_name: 'gpt-4o',
      channel: '12',
      group: 'vip',
      request_id: 'req-123',
      upstream_request_id: 'up-456',
      start_timestamp: '1717200000',
      end_timestamp: '1717286400',
    }),
    {
      page: 3,
      pageSize: 50,
      type: ['7'],
      username: 'alice',
      token: 'prod-key',
      model: 'gpt-4o',
      channel: '12',
      group: 'vip',
      requestId: 'req-123',
      upstreamRequestId: 'up-456',
      startTime: 1717200000000,
      endTime: 1717286400000,
    }
  )
})

test('defaults legacy console log query to the first usage logs page', () => {
  assert.deepEqual(legacyConsoleLogSearchToUsageLogsSearch({}), { page: 1 })
})

test('keeps normalized usage logs filters when redirecting from the index route', () => {
  assert.deepEqual(
    legacyConsoleLogSearchToUsageLogsSearch({
      page: '2',
      pageSize: '25',
      type: '7',
      model: 'gpt-test',
      token: 'token-a',
      requestId: 'req-789',
      upstreamRequestId: 'up-789',
      startTime: '1717200000000',
      endTime: '1717286400000',
    }),
    {
      page: 2,
      pageSize: 25,
      type: ['7'],
      model: 'gpt-test',
      token: 'token-a',
      requestId: 'req-789',
      upstreamRequestId: 'up-789',
      startTime: 1717200000000,
      endTime: 1717286400000,
    }
  )
})
