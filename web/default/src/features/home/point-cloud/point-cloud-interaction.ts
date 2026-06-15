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
export type PointCloudDragRotation = {
  rotationX: number
  rotationY: number
  targetRotationX: number
  targetRotationY: number
}

export type PointCloudDragDelta = {
  deltaX: number
  deltaY: number
}

const ROTATION_Y_PER_PIXEL = 0.008
const ROTATION_X_PER_PIXEL = 0.004
const MIN_ROTATION_X = -0.65
const MAX_ROTATION_X = 0.65

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

export function createPointCloudDragRotation(): PointCloudDragRotation {
  return {
    rotationX: 0,
    rotationY: 0,
    targetRotationX: 0,
    targetRotationY: 0,
  }
}

export function applyPointCloudDragRotation(
  rotation: PointCloudDragRotation,
  delta: PointCloudDragDelta
): void {
  rotation.targetRotationY += delta.deltaX * ROTATION_Y_PER_PIXEL
  rotation.targetRotationX = clamp(
    rotation.targetRotationX + delta.deltaY * ROTATION_X_PER_PIXEL,
    MIN_ROTATION_X,
    MAX_ROTATION_X
  )
}

export function settlePointCloudDragRotation(
  rotation: PointCloudDragRotation,
  easing = 0.12
): void {
  rotation.rotationY += (rotation.targetRotationY - rotation.rotationY) * easing
  rotation.rotationX += (rotation.targetRotationX - rotation.rotationX) * easing
}
