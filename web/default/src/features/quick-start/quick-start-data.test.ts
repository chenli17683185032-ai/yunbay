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
  codexDownloadCards,
  fallbackModels,
  getDefaultQuickStartModelName,
  quickStartFullscreenPages,
  getModelTags,
  nextStepGuideKeys,
  purposeOptions,
} from './quick-start-data'

test('quick start fullscreen flow exposes the requested five pages in order', () => {
  assert.deepEqual(
    quickStartFullscreenPages.map((page) => page.id),
    ['purpose', 'model', 'wallet', 'api-key', 'codex']
  )
})

test('every non-final page explains what the next page does', () => {
  assert.deepEqual(Object.keys(nextStepGuideKeys), [
    'purpose',
    'model',
    'wallet',
    'api-key',
  ])
  assert.equal(nextStepGuideKeys.purpose.length > 0, true)
  assert.equal(nextStepGuideKeys['api-key'].length > 0, true)
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

test('Codex download cards point to Yunbay-hosted macOS and Windows downloads', () => {
  assert.deepEqual(
    codexDownloadCards.map((item) => item.platform),
    ['macOS', 'Windows']
  )
  const macos = codexDownloadCards.find((item) => item.platform === 'macOS')
  const windows = codexDownloadCards.find((item) => item.platform === 'Windows')
  assert.equal(macos?.descriptionKey, 'Download starts now.')
  assert.equal(windows?.descriptionKey, 'Download starts now.')
  assert.match(
    macos?.downloadHref ?? '',
    /^\/downloads\/yunbay-codex-macos-\d{8}-\d{6}-[a-f0-9]{12}\.zip$/
  )
  assert.notEqual(macos?.downloadHref, '/downloads/yunbay-codex-macos.zip')
  assert.equal(macos?.buttonLabelKey, 'Download one-click launcher')

  assert.equal(
    macos?.quarantineFixCommand,
    'xattr -dr com.apple.quarantine "$HOME/Downloads/Yunbay Codex.app" && open "$HOME/Downloads/Yunbay Codex.app"'
  )
  assert.match(
    macos?.terminalInstallCommand ?? '',
    /^curl -L "https:\/\/yunbay\.xyz\/downloads\/yunbay-codex-macos-\d{8}-\d{6}-[a-f0-9]{12}\.zip" -o \/tmp\/yunbay-codex\.zip && rm -rf "\$HOME\/Downloads\/Yunbay Codex\.app" && ditto -x -k \/tmp\/yunbay-codex\.zip "\$HOME\/Downloads" && xattr -dr com\.apple\.quarantine "\$HOME\/Downloads\/Yunbay Codex\.app" && open "\$HOME\/Downloads\/Yunbay Codex\.app"$/
  )
  assert.equal(
    macos?.terminalInstallCommand?.includes(
      `https://yunbay.xyz${macos?.downloadHref}`
    ),
    true
  )
  assert.match(
    windows?.downloadHref ?? '',
    /^\/downloads\/yunbay-codex-windows-\d{8}-\d{6}-[a-f0-9]{12}\.exe$/
  )
  assert.notEqual(
    windows?.downloadHref,
    'https://get.microsoft.com/installer/download/9PLM9XGG6VKS?cid=website_cta_psi'
  )
  assert.equal(windows?.buttonLabelKey, 'Download one-click launcher')
  assert.equal(
    windows?.guideTitleKey,
    'What the Windows one-click launcher can do'
  )
  assert.ok(windows?.guideDescriptionKey?.includes('Yunbay Codex'))
  assert.ok(windows?.guideDescriptionKey?.includes('https://yunbay.xyz/v1'))
  assert.deepEqual(windows?.guideStepKeys, [
    'Download and run the Windows installer.',
    'Open Yunbay Codex and paste your Yunbay API key into Quick Start.',
    'Save and enable it, then start Codex, test model connectivity, and manage historical sessions.',
  ])
})

test('quick start does not synthesize fallback models when backend model square is empty', () => {
  assert.deepEqual(fallbackModels, [])
})

test('quick start defaults to GPT-5.5 when it is available', () => {
  assert.equal(
    getDefaultQuickStartModelName([
      { model_name: 'deepseek-chat' },
      { model_name: 'GPT-5.5' },
      { model_name: 'gpt-4.1' },
    ]),
    'GPT-5.5'
  )
})

test('quick start defaults to the first backend model when GPT-5.5 is unavailable', () => {
  assert.equal(
    getDefaultQuickStartModelName([
      { model_name: 'deepseek-chat' },
      { model_name: 'gpt-4.1' },
    ]),
    'deepseek-chat'
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
