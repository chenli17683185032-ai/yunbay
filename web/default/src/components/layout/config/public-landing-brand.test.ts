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
import { publicLandingBrand } from './public-landing-brand.config'

test('public landing brand uses the yunbay customer-facing identity', () => {
  assert.equal(publicLandingBrand.name, '云贝')
  assert.equal(publicLandingBrand.slug, 'yunbay')
  assert.equal(publicLandingBrand.homeHeadline, '永续的 Vibe Coding 助手')
  assert.equal(publicLandingBrand.homeSubheadline, '无需担心掉线。')
  assert.equal(
    publicLandingBrand.philosophy,
    '我们不生产水，我们只是大自然的搬运工。'
  )
  assert.equal(
    publicLandingBrand.mission,
    '为中国的 token 出海贡献一份自己的力量。'
  )
  assert.match(publicLandingBrand.harborMeaning, /AI 港口/)
})
