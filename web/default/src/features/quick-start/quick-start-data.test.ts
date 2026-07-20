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
  QUICK_START_PREFERRED_MODEL,
  QUICK_START_REASONING_EFFORT,
  codexDownloadCards,
  fallbackModels,
  getDefaultQuickStartModelName,
  getQuickStartModelDisplayName,
  quickStartFullscreenPages,
  getModelTags,
  nextStepGuideKeys,
  orderQuickStartModels,
  purposeOptions,
} from './quick-start-data'

test('quick start fullscreen flow exposes the requested five pages in order', () => {
  assert.deepEqual(
    quickStartFullscreenPages.map((page) => page.id),
    ['purpose', 'model', 'software', 'account', 'readiness']
  )
})

test('every non-final page explains what the next page does', () => {
  assert.deepEqual(Object.keys(nextStepGuideKeys), [
    'purpose',
    'model',
    'software',
    'account',
  ])
  assert.equal(nextStepGuideKeys.purpose.length > 0, true)
  assert.equal(nextStepGuideKeys.account.length > 0, true)
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
})

test('CC Switch download cards expose one official Mac and Windows installer', () => {
  assert.deepEqual(
    codexDownloadCards.map((item) => item.platform),
    ['macOS', 'Windows']
  )
  const macos = codexDownloadCards.find((item) => item.platform === 'macOS')
  const windows = codexDownloadCards.find((item) => item.platform === 'Windows')
  assert.equal(macos?.titleKey, 'Mac')
  assert.match(macos?.descriptionKey ?? '', /Intel and Apple Silicon/)
  assert.equal(macos?.buttonLabelKey, 'Download for Mac')
  assert.equal(
    macos?.downloadHref,
    'https://github.com/farion1231/cc-switch/releases/download/v3.17.0/CC-Switch-v3.17.0-macOS.dmg'
  )
  assert.equal(windows?.titleKey, 'Windows')
  assert.equal(windows?.buttonLabelKey, 'Download for Windows')
  assert.equal(
    windows?.downloadHref,
    'https://github.com/farion1231/cc-switch/releases/download/v3.17.0/CC-Switch-v3.17.0-Windows.msi'
  )
})

test('quick start does not synthesize fallback models when backend model square is empty', () => {
  assert.deepEqual(fallbackModels, [])
})

test('quick start defaults to GPT-5.6 Sol with a separate extreme reasoning setting', () => {
  assert.equal(
    getDefaultQuickStartModelName([
      { model_name: 'deepseek-chat' },
      { model_name: 'GPT-5.6-Sol' },
      { model_name: 'gpt-4.1' },
    ]),
    'GPT-5.6-Sol'
  )
  assert.equal(QUICK_START_PREFERRED_MODEL, 'gpt-5.6-sol')
  assert.equal(QUICK_START_REASONING_EFFORT, 'xhigh')
  assert.equal(getQuickStartModelDisplayName('GPT-5.6-Sol'), 'GPT 5.6 Sol')
})

test('quick start defaults to the first backend model when GPT-5.6 Sol is unavailable', () => {
  assert.equal(
    getDefaultQuickStartModelName([
      { model_name: 'deepseek-chat' },
      { model_name: 'gpt-4.1' },
    ]),
    'deepseek-chat'
  )
})

test('quick start moves the active model to the visible first position', () => {
  const models = [
    { model_name: 'grok-4.5' },
    { model_name: 'gpt-5.6-sol' },
    { model_name: 'gpt-4.1' },
  ]

  assert.deepEqual(
    orderQuickStartModels(models, 'gpt-5.6-sol').map(
      (model) => model.model_name
    ),
    ['gpt-5.6-sol', 'grok-4.5', 'gpt-4.1']
  )
  assert.deepEqual(
    models.map((model) => model.model_name),
    ['grok-4.5', 'gpt-5.6-sol', 'gpt-4.1']
  )
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
