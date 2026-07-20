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
export type QuickStartPurposeId = 'web-coding' | 'chat' | 'other'
export type QuickStartFullscreenPageId =
  | 'purpose'
  | 'model'
  | 'software'
  | 'account'
  | 'readiness'
export type QuickStartEnterDashboardPath = '/dashboard/overview'

export type QuickStartPurpose = {
  id: QuickStartPurposeId
  titleKey: string
  descriptionKey: string
}

export type CodexDownloadCard = {
  platform: 'macOS' | 'Windows'
  titleKey: string
  descriptionKey: string
  detailKey: string
  buttonLabelKey: string
  downloadHref: string
}

export type QuickStartFullscreenPage = {
  id: QuickStartFullscreenPageId
}

export type QuickStartModelLike = {
  model_name: string
  vendor_name?: string
  tags?: string
  supported_endpoint_types?: string[]
  quota_type?: number
  model_ratio?: number
  completion_ratio?: number
  model_price?: number
  group_ratio?: Record<string, number>
  enable_groups?: string[]
}

export const QUICK_START_DEFAULT_PURPOSE: QuickStartPurposeId = 'web-coding'
export const QUICK_START_PREFERRED_MODEL = 'gpt-5.6-sol'
export const QUICK_START_PREFERRED_MODEL_LABEL_KEY = 'GPT 5.6 Sol'
export const QUICK_START_REASONING_EFFORT = 'xhigh'
export const QUICK_START_REASONING_EFFORT_LABEL_KEY = 'Extreme reasoning'
export const QUICK_START_ENTER_DASHBOARD_PATH: QuickStartEnterDashboardPath =
  '/dashboard/overview'

export const quickStartFullscreenPages: QuickStartFullscreenPage[] = [
  { id: 'purpose' },
  { id: 'model' },
  { id: 'software' },
  { id: 'account' },
  { id: 'readiness' },
]

export const nextStepGuideKeys: Record<
  Exclude<QuickStartFullscreenPageId, 'readiness'>,
  string
> = {
  purpose: 'Next: choose a model and review its price.',
  model: 'Next: download CC Switch for your computer.',
  software: 'Next: prepare your balance and API key.',
  account: 'Next: review your setup and import it to CC Switch.',
}

export const purposeOptions: QuickStartPurpose[] = [
  {
    id: 'web-coding',
    titleKey: 'Web Coding',
    descriptionKey:
      'Use it for code generation, web development, debugging, and project collaboration.',
  },
  {
    id: 'chat',
    titleKey: 'Chat',
    descriptionKey:
      'Use it for daily conversations, writing, summaries, translation, and knowledge Q&A.',
  },
  {
    id: 'other',
    titleKey: 'Image and more',
    descriptionKey:
      'Use it for image generation, multimodal creation, creative exploration, and other model capabilities.',
  },
]

const CC_SWITCH_RELEASE_VERSION = 'v3.17.0'
const CC_SWITCH_RELEASE_BASE_URL = `https://github.com/farion1231/cc-switch/releases/download/${CC_SWITCH_RELEASE_VERSION}`

export const codexDownloadCards: CodexDownloadCard[] = [
  {
    platform: 'macOS',
    titleKey: 'Mac',
    descriptionKey: 'Universal download for Intel and Apple Silicon Macs.',
    detailKey: 'macOS 12 or later · Signed and notarized',
    buttonLabelKey: 'Download for Mac',
    downloadHref: `${CC_SWITCH_RELEASE_BASE_URL}/CC-Switch-${CC_SWITCH_RELEASE_VERSION}-macOS.dmg`,
  },
  {
    platform: 'Windows',
    titleKey: 'Windows',
    descriptionKey: 'Standard 64-bit installer for Windows 10 and Windows 11.',
    detailKey: 'Windows 10/11 · 64-bit',
    buttonLabelKey: 'Download for Windows',
    downloadHref: `${CC_SWITCH_RELEASE_BASE_URL}/CC-Switch-${CC_SWITCH_RELEASE_VERSION}-Windows.msi`,
  },
]

export const fallbackModels: QuickStartModelLike[] = []

function normalizeModelNameForDefault(modelName: string): string {
  return modelName.toLowerCase().replace(/[\s_-]+/g, '')
}

export function isPreferredQuickStartModel(modelName: string): boolean {
  return (
    normalizeModelNameForDefault(modelName) ===
    normalizeModelNameForDefault(QUICK_START_PREFERRED_MODEL)
  )
}

export function getDefaultQuickStartModelName(
  models: QuickStartModelLike[]
): string {
  return (
    models.find((model) => isPreferredQuickStartModel(model.model_name))
      ?.model_name ||
    models[0]?.model_name ||
    ''
  )
}

export function orderQuickStartModels(
  models: QuickStartModelLike[],
  selectedModelName: string
): QuickStartModelLike[] {
  const activeModelName = selectedModelName.trim()
  if (!activeModelName) return models

  const selectedModel = models.find(
    (model) => model.model_name === activeModelName
  )
  if (!selectedModel) return models

  return [
    selectedModel,
    ...models.filter((model) => model.model_name !== activeModelName),
  ]
}

export function getQuickStartModelDisplayName(modelName: string): string {
  return isPreferredQuickStartModel(modelName)
    ? QUICK_START_PREFERRED_MODEL_LABEL_KEY
    : modelName
}

function includesAny(value: string, patterns: RegExp[]): boolean {
  return patterns.some((pattern) => pattern.test(value))
}

export function getModelTags(model: QuickStartModelLike): string[] {
  const name = model.model_name.toLowerCase()
  const rawTags = model.tags?.toLowerCase() ?? ''
  const endpoints = model.supported_endpoint_types ?? []
  const haystack = `${name} ${rawTags} ${endpoints.join(' ')}`
  const tags: string[] = []

  if (includesAny(haystack, [/code/, /coder/, /coding/])) tags.push('Coding')
  if (includesAny(haystack, [/image/, /vision/, /sdxl/, /midjourney/])) {
    tags.push('Image')
  }
  if (includesAny(haystack, [/reasoning/, /thinking/, /deepseek-r\d/, /qwq/])) {
    tags.push('Reasoning')
  }
  if (includesAny(haystack, [/vision/, /multimodal/, /omni/])) {
    tags.push('Vision')
  }
  if (includesAny(haystack, [/audio/, /voice/, /tts/, /whisper/])) {
    tags.push('Audio')
  }
  if (includesAny(haystack, [/video/, /sora/, /veo/])) tags.push('Video')

  if (!tags.includes('Chat') && !endpoints.includes('image-generation')) {
    tags.push('Chat')
  }

  return tags.slice(0, 4)
}

function getMinGroupRatio(model: QuickStartModelLike): number {
  const enableGroups = Array.isArray(model.enable_groups)
    ? model.enable_groups
    : []
  const groupRatio = model.group_ratio || {}
  if (enableGroups.length === 0) return 1

  const ratios = enableGroups
    .map((group) => groupRatio[group])
    .filter((ratio): ratio is number => Number.isFinite(Number(ratio)))

  return ratios.length > 0 ? Math.min(...ratios) : 1
}

function formatUsdRate(value: number): string {
  if (!Number.isFinite(value)) return '$0'
  const digits = Math.abs(value) >= 1 ? 2 : 4
  const normalized = Number.parseFloat(value.toFixed(digits)).toString()
  return `$${normalized}`
}

export function getModelRateLabels(model: QuickStartModelLike): {
  input: string
  output: string
} {
  if (model.quota_type === 1) {
    const requestPrice = Number(model.model_ratio || model.model_price || 0)
    return {
      input: `${formatUsdRate(requestPrice)} / request`,
      output: `${formatUsdRate(requestPrice)} / request`,
    }
  }

  const ratio = getMinGroupRatio(model)
  const input = Number(model.model_ratio || 0) * 2 * ratio
  const output = input * Number(model.completion_ratio || 1)

  return {
    input: `${formatUsdRate(input)} / 百万 tokens`,
    output: `${formatUsdRate(output)} / 百万 tokens`,
  }
}
