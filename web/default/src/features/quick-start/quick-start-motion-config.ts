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
export const QUICK_START_APPLE_EASE = [0.16, 1, 0.3, 1] as const

export const QUICK_START_SPRING_TRANSITION = {
  type: 'spring',
  stiffness: 430,
  damping: 36,
  mass: 0.78,
} as const

export const QUICK_START_REDUCED_TRANSITION = {
  duration: 0.08,
} as const
