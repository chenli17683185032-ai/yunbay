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
  QUICK_START_DEFAULT_PURPOSE,
  QUICK_START_ENTER_DASHBOARD_PATH,
  QUICK_START_MINIMUM_QUOTA,
  downloadCards,
  quickStartFullscreenPages,
  getBalanceState,
  getModelTags,
  purposeOptions,
} from './quick-start-data'

test('quick start fullscreen flow exposes exactly five pages in order', () => {
  assert.deepEqual(
    quickStartFullscreenPages.map((page) => page.id),
    ['purpose', 'model', 'balance', 'download', 'finish']
  )
})

test('quick start enter dashboard target points to dashboard overview', () => {
  assert.equal(QUICK_START_ENTER_DASHBOARD_PATH, '/dashboard/overview')
})

test('quick start exposes exactly the three requested purposes', () => {
  assert.deepEqual(
    purposeOptions.map((item) => item.id),
    ['web-coding', 'chat', 'other']
  )
  assert.equal(QUICK_START_DEFAULT_PURPOSE, 'web-coding')
  assert.equal(
    purposeOptions.find((item) => item.id === 'chat')?.nextActionPath,
    '/chat2link'
  )
})

test('download cards expose macOS package and Windows placeholder', () => {
  assert.deepEqual(
    downloadCards.map((item) => item.platform),
    ['macOS', 'Windows']
  )
  const macos = downloadCards.find((item) => item.platform === 'macOS')
  const windows = downloadCards.find((item) => item.platform === 'Windows')
  assert.equal(macos?.available, true)
  assert.equal(macos?.downloadHref, '/downloads/yunbei-macos.zip')
  assert.equal(windows?.available, false)
})

test('balance state compares quota against the minimum startup quota', () => {
  assert.equal(QUICK_START_MINIMUM_QUOTA, 50000)
  assert.equal(getBalanceState(50000).isEnough, true)
  assert.equal(getBalanceState(49999).isEnough, false)
})

test('model tags include coding, image, and reasoning hints', () => {
  assert.deepEqual(
    getModelTags({
      model_name: 'deepseek-r1-coder',
      tags: 'image',
      supported_endpoint_types: ['image-generation'],
    }).slice(0, 3),
    ['Coding', 'Image', 'Reasoning']
  )
})
