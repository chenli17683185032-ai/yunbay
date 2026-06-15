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
export const LANDING_SECTION_IDS = ['home', 'models', 'about'] as const

export type LandingSectionId = (typeof LANDING_SECTION_IDS)[number]
export type PageDirection = 'next' | 'previous'

const WHEEL_PAGE_STEP_THRESHOLD = 16

function clampPageIndex(index: number, total: number): number {
  return Math.max(0, Math.min(Math.max(total - 1, 0), index))
}

function clampSectionIndex(
  index: number,
  sectionCount: number = LANDING_SECTION_IDS.length
): number {
  return clampPageIndex(index, sectionCount)
}

export function getNextPageIndex(
  currentIndex: number,
  direction: PageDirection,
  total: number
): number {
  const safeIndex = clampPageIndex(currentIndex, total)
  return clampPageIndex(safeIndex + (direction === 'next' ? 1 : -1), total)
}

export function getNextMorphSignal(
  currentSignal: number,
  previousIndex: number,
  nextIndex: number
): number {
  return previousIndex === nextIndex ? currentSignal : currentSignal + 1
}

export function getNextLandingSectionIndex(
  currentIndex: number,
  deltaY: number
): number {
  const safeIndex = clampSectionIndex(currentIndex)
  if (Math.abs(deltaY) < WHEEL_PAGE_STEP_THRESHOLD) return safeIndex
  return getNextPageIndex(
    safeIndex,
    deltaY > 0 ? 'next' : 'previous',
    LANDING_SECTION_IDS.length
  )
}

export function normalizeSectionHash(
  sectionIds: readonly string[],
  value: string
): number | null {
  const hashIndex = value.indexOf('#')
  if (hashIndex === -1) return null

  const hash = value.slice(hashIndex + 1)
  const index = sectionIds.findIndex((id) => id === hash)
  return index === -1 ? null : index
}

export function normalizeLandingSectionHash(value: string): number | null {
  return normalizeSectionHash(LANDING_SECTION_IDS, value)
}

export function getSectionHash<TSectionId extends string>(
  sectionIds: readonly TSectionId[],
  index: number
): `#${TSectionId}` {
  return `#${sectionIds[clampSectionIndex(index, sectionIds.length)]}`
}

export function getLandingSectionHash(index: number): `#${LandingSectionId}` {
  return getSectionHash(LANDING_SECTION_IDS, index)
}
