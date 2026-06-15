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
export type PointCloudFaceState = 'closed' | 'open'
export type RuntimePointCloudId =
  | 'face-closed'
  | 'face-open'
  | 'black-hole'
  | 'lorenz-attractor'

export function getFaceStateForQuota(
  quota: number | null | undefined
): PointCloudFaceState {
  return typeof quota === 'number' && quota > 0 ? 'open' : 'closed'
}

export function getRuntimePointCloudSequence(
  faceState: PointCloudFaceState
): RuntimePointCloudId[] {
  return [
    faceState === 'open' ? 'face-open' : 'face-closed',
    'black-hole',
    'lorenz-attractor',
  ]
}
