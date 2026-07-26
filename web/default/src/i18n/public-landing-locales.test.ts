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
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'
import { publicLandingBrand } from '../components/layout/config/public-landing-brand.config'
import { MODEL_SQUARE_COPY } from '../features/home/landing-page-copy'

const LOCALE_DIR = join(import.meta.dirname, 'locales')
const LOCALE_NAMES = ['en', 'zh', 'fr', 'ru', 'ja', 'vi'] as const

const CHINESE_BRAND_COPY = [
  publicLandingBrand.homeHeadline,
  publicLandingBrand.homeSubheadline,
  publicLandingBrand.philosophy,
  publicLandingBrand.mission,
  publicLandingBrand.harborMeaning,
] as const

const CHINESE_MARKETING_TRANSLATIONS = new Map<string, string>([
  [MODEL_SQUARE_COPY.headline, '一把 Key，把世界模型搬到你的接口里'],
  [
    MODEL_SQUARE_COPY.promises[0],
    '云贝将主流模型 API 汇入统一 Key、路由、计费、额度、故障切换与 OpenAI 兼容接口。',
  ],
  [MODEL_SQUARE_COPY.promises[1], 'GBT5.5 全量满血版官方价格的 1/10。'],
])

function readTranslation(localeName: (typeof LOCALE_NAMES)[number]) {
  const localePath = join(LOCALE_DIR, `${localeName}.json`)
  return JSON.parse(readFileSync(localePath, 'utf8')).translation as Record<
    string,
    string
  >
}

test('yunbay manifesto keeps Chinese copy in zh and a non-empty translation everywhere else', () => {
  for (const localeName of LOCALE_NAMES) {
    const translation = readTranslation(localeName)

    for (const key of CHINESE_BRAND_COPY) {
      if (localeName === 'zh') {
        assert.equal(translation[key], key, `${localeName}: ${key}`)
      } else {
        assert.ok(
          typeof translation[key] === 'string' &&
            translation[key].trim().length > 0,
          `${localeName}: missing translation for ${key}`
        )
      }
    }

    for (const [key, value] of CHINESE_MARKETING_TRANSLATIONS) {
      if (localeName === 'zh') {
        assert.equal(translation[key], value, `${localeName}: ${key}`)
      } else {
        assert.ok(
          typeof translation[key] === 'string' &&
            translation[key].trim().length > 0,
          `${localeName}: missing translation for ${key}`
        )
      }
    }
  }
})
