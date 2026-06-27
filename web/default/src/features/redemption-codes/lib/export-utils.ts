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

type BatchLike = {
  batch_id?: string | null
}

type BusinessResponse = {
  success?: boolean
  message?: string
}

export function getSingleSelectedBatchId(items: BatchLike[]): string | null {
  if (items.length === 0) return null

  const batchIds = items.map((item) => item.batch_id?.trim() ?? '')
  if (batchIds.some((batchId) => batchId.length === 0)) return null

  const uniqueBatchIds = new Set(batchIds)
  if (uniqueBatchIds.size !== 1) return null

  return batchIds[0]
}

export async function resolveExportBlob(blob: Blob): Promise<Blob> {
  if (!blob.type.toLowerCase().includes('json')) return blob

  const text = await blob.text()
  const data = JSON.parse(text) as BusinessResponse
  if (data.success === false) {
    throw new Error(data.message || 'Failed to export redemption codes')
  }

  return blob
}
