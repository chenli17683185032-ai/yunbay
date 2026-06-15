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
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  computeBounds,
  generateBlackHolePoints,
  generateLorenzAttractorPoints,
  normalizePointCount,
} from './pointcloud-utils.mjs'

test('generated black hole has exactly requested float count and is deterministic', () => {
  const a = generateBlackHolePoints(30000)
  const b = generateBlackHolePoints(30000)
  assert.equal(a.length, 90000)
  assert.equal(b.length, 90000)
  assert.deepEqual(Array.from(a.slice(0, 24)), Array.from(b.slice(0, 24)))
})

test('generated black hole spans the full background field without changing point count', () => {
  const points = generateBlackHolePoints(30000)
  const bounds = computeBounds(points)
  const largestSpan = Math.max(bounds.width, bounds.height, bounds.depth)

  assert.equal(points.length, 90000)
  assert.ok(largestSpan >= 4.55 && largestSpan <= 4.65)
})

test('generated Lorenz attractor has exactly requested float count and is deterministic', () => {
  const a = generateLorenzAttractorPoints(30000)
  const b = generateLorenzAttractorPoints(30000)
  assert.equal(a.length, 90000)
  assert.equal(b.length, 90000)
  assert.deepEqual(Array.from(a.slice(0, 24)), Array.from(b.slice(0, 24)))
})

test('normalizePointCount resamples smaller clouds to the requested point count', () => {
  const source = new Float32Array([0, 0, 0, 1, 0, 0, 0, 1, 0])
  const out = normalizePointCount(source, 5, 7)
  assert.equal(out.length, 15)
  assert.deepEqual(Array.from(out.slice(0, 9)), Array.from(source))
})
