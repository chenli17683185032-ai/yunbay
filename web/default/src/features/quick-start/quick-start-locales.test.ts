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
const LOCALE_DIR = join(import.meta.dirname, '../../i18n/locales')

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
  'Generate your first API key',
  'Create a ready-to-use key with one click. Yunbay copies it automatically.',
  'Selected purpose',
  'API key is ready',
  'One-click API key',
  'Already copied to clipboard',
  'Failed to copy API key',
  'Failed to create API key',
  'Click generate. The new API key will be copied to your clipboard.',
  'Generating...',
  'Copy API key again',
  'Generate API key',
  'Official Codex',
  'Download Codex',
  'Choose your operating system and continue to the official OpenAI Codex download.',
  'The download opens the official OpenAI installer for the selected platform.',
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
