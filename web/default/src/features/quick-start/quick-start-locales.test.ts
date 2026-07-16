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
  QUICK_START_PREFERRED_MODEL_LABEL_KEY,
  QUICK_START_REASONING_EFFORT_LABEL_KEY,
  codexDownloadCards,
  nextStepGuideKeys,
  purposeOptions,
} from './quick-start-data'

const LOCALE_NAMES = ['en', 'zh', 'fr', 'ru', 'ja', 'vi'] as const
const LOCALIZED_LOCALE_NAMES = ['zh', 'fr', 'ru', 'ja', 'vi'] as const
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
  'API key',
  'API key is ready',
  'API key ready',
  'Account setup',
  'Add balance if needed, then create or reuse one API key.',
  'Add balance or redeem a code',
  'Already copied to clipboard',
  'Already installed',
  'CC Switch',
  'CC Switch installed',
  'CC Switch setup confirmed',
  'Choose a model',
  'Quick Start Yunbay',
  'Quick Start',
  'Choose how you will use AI',
  'This helps Yunbay recommend a practical first path.',
  'Choose your computer. The official installer opens directly from GitHub.',
  'Complete',
  'Configured API',
  'Configured model',
  'Confirm after CC Switch shows the Yunbay provider and Extreme reasoning.',
  'Confirm only after CC Switch shows the imported Yunbay provider.',
  'Confirm the CC Switch import to continue',
  'Confirm the last device step, then import your prepared setup.',
  'Confirmed',
  'Copy API key',
  'Create one reusable key and copy it automatically.',
  'Create your API key',
  'Created or restored',
  'Current Balance',
  'Current device',
  'Did CC Switch open?',
  'Download CC Switch',
  'Download opened',
  'Enter dashboard',
  'Enter your redemption code',
  'Image generation settings',
  'Failed to create API key',
  'Generate API key',
  'Generate an API key before importing.',
  'Generate an API key first',
  'Generated API key',
  'Have you finished installing CC Switch?',
  'I need to review it again',
  'Import again',
  'Import confirmed',
  'Import current setup to CC Switch',
  'Input',
  'Installation confirmed on this device.',
  'Installed',
  'It opened',
  'Model',
  'Model routes',
  'Model provider',
  'Model selected',
  'Next',
  'No model selected',
  'No models are currently enabled in the model square. Configure backend channels and model access first.',
  'No, start now',
  'Not ready',
  'Not yet',
  'One-click import image settings',
  'One-click import',
  'Output',
  'Prepare your account',
  'Preparing...',
  'Previous',
  'Provider import',
  'Provider imported. Continue with the image settings.',
  'Ready',
  'Ready check',
  'Reasoning effort',
  'Reasoning target',
  'Recommended',
  'Redeem',
  'Redemption successful! Added: {{quota}}',
  'Redemption failed',
  'Return to account setup',
  'Review and import',
  'Review the final setup summary, or start using the console now.',
  'Selected',
  'Selected model',
  'Set up later and enter dashboard',
  'The API, model, and key are prepared for one-click import.',
  'Top up',
  'Top up or use a redemption code before your first request.',
  'Try again',
  'Trying to open CC Switch',
  'Trying to open image settings in CC Switch',
  'Your balance is ready. You can still add more at any time.',
  'Your browser keeps this guide open while GitHub starts the download in a new tab.',
  'Your existing quick-start key was restored securely.',
  'GPT 5.6 Sol is unavailable. The first enabled model is selected instead.',
  'Your recommended model is pinned first with its live rate.',
  'Your setup is ready',
  'API key was generated but clipboard copy failed. You can copy it again or continue setup.',
  'No available group for the new API key',
] as const

const CHINESE_LITERAL_KEYS = new Set([
  'API key',
  'CC Switch',
  'GPT 5.6 Sol',
  'Mac',
  'Windows',
])

function getQuickStartTranslationKeys(): string[] {
  return [
    ...QUICK_START_COMPONENT_KEYS,
    QUICK_START_PREFERRED_MODEL_LABEL_KEY,
    QUICK_START_REASONING_EFFORT_LABEL_KEY,
    ...Object.values(nextStepGuideKeys),
    ...purposeOptions.flatMap((option) => [
      option.titleKey,
      option.descriptionKey,
    ]),
    ...MODEL_TAG_KEYS,
    ...codexDownloadCards.flatMap((card) => [
      card.titleKey,
      card.descriptionKey,
      card.detailKey,
      card.buttonLabelKey,
    ]),
  ]
}

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
  const keys = getQuickStartTranslationKeys()

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
  const keys = getQuickStartTranslationKeys()

  for (const key of keys) {
    if (CHINESE_LITERAL_KEYS.has(key)) continue
    assert.notEqual(
      locale.translation[key],
      key,
      `zh: quick-start translation falls back to English for ${key}`
    )
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
