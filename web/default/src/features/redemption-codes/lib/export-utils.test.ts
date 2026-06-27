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
import { getSingleSelectedBatchId, resolveExportBlob } from './export-utils'

test('allows selecting multiple rows from exactly one batch', () => {
  const batchId = getSingleSelectedBatchId([
    { batch_id: 'batch-1' },
    { batch_id: 'batch-1' },
  ])

  assert.equal(batchId, 'batch-1')
})

test('rejects mixed, empty, or missing batches', () => {
  assert.equal(
    getSingleSelectedBatchId([
      { batch_id: 'batch-1' },
      { batch_id: 'batch-2' },
    ]),
    null
  )
  assert.equal(getSingleSelectedBatchId([{ batch_id: '' }]), null)
  assert.equal(getSingleSelectedBatchId([{ batch_id: null }]), null)
  assert.equal(getSingleSelectedBatchId([]), null)
})

test('rejects JSON business errors returned as blobs', async () => {
  const blob = new Blob(
    [JSON.stringify({ success: false, message: 'batch not found' })],
    {
      type: 'application/json',
    }
  )

  await assert.rejects(() => resolveExportBlob(blob), /batch not found/)
})

test('returns non-JSON export blobs unchanged', async () => {
  const blob = new Blob(['code-one\ncode-two'], { type: 'text/plain' })
  const resolved = await resolveExportBlob(blob)

  assert.equal(resolved, blob)
})
