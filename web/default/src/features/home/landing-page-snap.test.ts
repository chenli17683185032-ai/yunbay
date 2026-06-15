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
  LANDING_SECTION_IDS,
  getNextMorphSignal,
  getNextLandingSectionIndex,
  getNextPageIndex,
  getSectionHash,
  normalizeLandingSectionHash,
  normalizeSectionHash,
} from './landing-page-snap'

test('landing surface contains exactly the three public sections in order', () => {
  assert.deepEqual(LANDING_SECTION_IDS, ['home', 'models', 'about'])
})

test('wheel intent advances by one full section and clamps at edges', () => {
  assert.equal(getNextLandingSectionIndex(0, 88), 1)
  assert.equal(getNextLandingSectionIndex(1, 42), 2)
  assert.equal(getNextLandingSectionIndex(2, 120), 2)
  assert.equal(getNextLandingSectionIndex(2, -64), 1)
  assert.equal(getNextLandingSectionIndex(0, -64), 0)
})

test('explicit controls move by one full page and clamp at edges', () => {
  assert.equal(getNextPageIndex(0, 'next', 3), 1)
  assert.equal(getNextPageIndex(1, 'next', 3), 2)
  assert.equal(getNextPageIndex(2, 'next', 3), 2)
  assert.equal(getNextPageIndex(2, 'previous', 3), 1)
  assert.equal(getNextPageIndex(0, 'previous', 3), 0)
})

test('tiny wheel deltas do not trigger page changes', () => {
  assert.equal(getNextLandingSectionIndex(1, 4), 1)
  assert.equal(getNextLandingSectionIndex(1, -4), 1)
})

test('landing hashes normalize to section indexes', () => {
  assert.equal(normalizeLandingSectionHash('/#home'), 0)
  assert.equal(normalizeLandingSectionHash('#models'), 1)
  assert.equal(normalizeLandingSectionHash('/#about'), 2)
  assert.equal(normalizeLandingSectionHash('/pricing'), null)
})

test('generic section hashes use the provided section ids', () => {
  const sectionIds = ['purpose', 'model', 'finish'] as const

  assert.equal(normalizeSectionHash(sectionIds, '/quick-start#purpose'), 0)
  assert.equal(normalizeSectionHash(sectionIds, '#model'), 1)
  assert.equal(normalizeSectionHash(sectionIds, '/quick-start#finish'), 2)
  assert.equal(normalizeSectionHash(sectionIds, '/quick-start#unknown'), null)
  assert.equal(getSectionHash(sectionIds, 2), '#finish')
  assert.equal(getSectionHash(sectionIds, 99), '#finish')
})

test('page changes increment the morph signal only when the index changes', () => {
  assert.equal(getNextMorphSignal(0, 0, 1), 1)
  assert.equal(getNextMorphSignal(7, 1, 2), 8)
  assert.equal(getNextMorphSignal(7, 1, 1), 7)
})
