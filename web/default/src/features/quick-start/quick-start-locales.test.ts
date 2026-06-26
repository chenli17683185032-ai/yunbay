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
import {
  codexDownloadCards,
  nextStepGuideKeys,
  purposeOptions,
} from './quick-start-data'

const LOCALE_NAMES = ['en', 'zh', 'fr', 'ru', 'ja', 'vi'] as const
const LOCALIZED_LOCALE_NAMES = ['zh', 'fr', 'ru', 'ja', 'vi'] as const
const FULLY_LOCALIZED_LOCALE_NAMES = ['fr', 'ru', 'ja', 'vi'] as const
const LOCALE_DIR = join(import.meta.dirname, '../../i18n/locales')

const MODEL_TAG_KEYS = [
  'Coding',
  'Image',
  'Reasoning',
  'Vision',
  'Audio',
  'Video',
  'Chat',
] as const

const QUICK_START_COMPONENT_KEYS = [
  'Quick Start Yunbay',
  'Quick Start',
  'Choose how you will use AI',
  'This helps Yunbay recommend a practical first path.',
  'Model routes',
  'Choose a model',
  'All supported models are listed with OpenRouter-style rates.',
  'No models are currently enabled in the model square. Configure backend channels and model access first.',
  'Model provider',
  'Input',
  'Output',
  'Wallet',
  'Wallet and redemption code',
  'Add balance in the wallet or redeem a code before you begin.',
  'Current Balance',
  'Selected model',
  'Open wallet',
  'View your balance and choose a top-up method.',
  'Top up',
  'Redeem a code',
  'Use a redemption code to add balance to your account.',
  'Use redemption code',
  'Enter your redemption code',
  'Please enter a redemption code',
  'Redemption successful! Added: {{quota}}',
  'Redemption failed',
  'Generate your first API key',
  'Create a ready-to-use key with one click. Yunbay copies it automatically.',
  'Selected purpose',
  'API key is ready',
  'One-click API key',
  'Already copied to clipboard',
  'Failed to copy API key',
  'Failed to create API key',
  'No available group for the new API key',
  'Click generate. The new API key will be copied to your clipboard.',
  'Generating...',
  'Copy API key again',
  'Generate API key',
  'Codex one-click launcher',
  'Codex one-click setup',
  'Download the Codex one-click launcher and connect it to your Yunbay API key.',
  'Download one-click launcher',
  'What the Windows one-click launcher can do',
  'After downloading and running the installer, open Yunbay Codex and paste your Yunbay API key into Quick Start. It will automatically write a custom API configuration and connect to https://yunbay.xyz/v1. The app also supports model provider management, connectivity testing, balance and usage queries, and Codex session management.',
  'Download and run the Windows installer.',
  'Open Yunbay Codex and paste your Yunbay API key into Quick Start.',
  'Save and enable it, then start Codex, test model connectivity, and manage historical sessions.',
  'Import current setup to CC Switch',
  'Launch CC Switch from your browser with this API and model prefilled.',
  'Configured API',
  'Configured model',
  'Generated API key',
  'One-click import',
  'Generate an API key first',
  'No model selected',
  'CC Switch will import this Codex provider and enable it automatically.',
  'Trying to open CC Switch',
  'If macOS says the app is damaged',
  'This build is not notarized by Apple yet. If Gatekeeper blocks it, run the terminal command below after downloading.',
  'Copy repair command',
  'Copy one-line terminal install',
  'Terminal command copied',
  'Failed to copy terminal command',
  'Previous',
  'Next',
  'Enter dashboard',
] as const

function readLocale(localeName: (typeof LOCALE_NAMES)[number]): {
  raw: string
  translation: Record<string, string>
} {
  const raw = readFileSync(join(LOCALE_DIR, `${localeName}.json`), 'utf8')
  return {
    raw,
    translation: JSON.parse(raw).translation as Record<string, string>,
  }
}

test('quick start copy has translations in every supported locale', () => {
  const keys = [
    ...QUICK_START_COMPONENT_KEYS,
    ...Object.values(nextStepGuideKeys),
    ...purposeOptions.flatMap((option) => [
      option.titleKey,
      option.descriptionKey,
    ]),
    ...MODEL_TAG_KEYS,
    ...codexDownloadCards.flatMap((card) => [
      card.descriptionKey,
      card.buttonLabelKey,
    ]),
  ]

  for (const localeName of LOCALE_NAMES) {
    const locale = readLocale(localeName)

    for (const key of keys) {
      assert.ok(
        Object.prototype.hasOwnProperty.call(locale.translation, key),
        `${localeName}: missing quick-start translation key ${key}`
      )
    }
  }
})

test('Chinese quick start copy does not fall back to English for the guided flow', () => {
  const locale = readLocale('zh')
  const keys = [
    ...QUICK_START_COMPONENT_KEYS,
    ...Object.values(nextStepGuideKeys),
    ...purposeOptions.flatMap((option) => [
      option.titleKey,
      option.descriptionKey,
    ]),
    ...MODEL_TAG_KEYS,
    ...codexDownloadCards.flatMap((card) => [
      card.descriptionKey,
      card.buttonLabelKey,
    ]),
  ]

  for (const key of keys) {
    assert.notEqual(
      locale.translation[key],
      key,
      `zh: quick-start translation falls back to English for ${key}`
    )
  }
})

test('download-page quick start copy does not fall back to English in localized non-Chinese locales', () => {
  const keys = [
    'Codex one-click launcher',
    'Codex one-click setup',
    'Download the Codex one-click launcher and connect it to your Yunbay API key.',
    'Download one-click launcher',
    'What the Windows one-click launcher can do',
    'After downloading and running the installer, open Yunbay Codex and paste your Yunbay API key into Quick Start. It will automatically write a custom API configuration and connect to https://yunbay.xyz/v1. The app also supports model provider management, connectivity testing, balance and usage queries, and Codex session management.',
    'Download and run the Windows installer.',
    'Open Yunbay Codex and paste your Yunbay API key into Quick Start.',
    'Save and enable it, then start Codex, test model connectivity, and manage historical sessions.',
    'Import current setup to CC Switch',
    'Launch CC Switch from your browser with this API and model prefilled.',
    'Configured API',
    'Configured model',
    'Generated API key',
    'One-click import',
    'Generate an API key first',
    'No model selected',
    'CC Switch will import this Codex provider and enable it automatically.',
    'Trying to open CC Switch',
  ] as const

  for (const localeName of FULLY_LOCALIZED_LOCALE_NAMES) {
    const locale = readLocale(localeName)

    for (const key of keys) {
      assert.notEqual(
        locale.translation[key],
        key,
        `${localeName}: download-page quick-start translation falls back to English for ${key}`
      )
    }
  }
})

test('localized locale files keep protected project attribution key obfuscated', () => {
  for (const localeName of LOCALIZED_LOCALE_NAMES) {
    const locale = readLocale(localeName)

    assert.match(
      locale.raw,
      /"footer\.new\\u0061pi\.projectAttributionSuffix":/,
      `${localeName}: protected attribution key must stay serialized as footer.new\\u0061pi.projectAttributionSuffix`
    )
    assert.doesNotMatch(
      locale.raw,
      /"footer\.newapi\.projectAttributionSuffix":/,
      `${localeName}: protected attribution key must not be rewritten as footer.newapi.projectAttributionSuffix`
    )
  }
})
