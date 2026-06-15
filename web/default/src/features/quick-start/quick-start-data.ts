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
export type QuickStartNextActionPath = '/keys' | '/chat2link' | '/playground'
export type QuickStartFullscreenPageId =
  | 'purpose'
  | 'model'
  | 'balance'
  | 'download'
  | 'finish'
export type QuickStartEnterDashboardPath = '/dashboard/overview'

export type QuickStartPurpose = {
  id: QuickStartPurposeId
  titleKey: string
  descriptionKey: string
  nextActionLabelKey: string
  nextActionPath: QuickStartNextActionPath
}

export type QuickStartDownloadCard = {
  platform: 'macOS' | 'Windows'
  descriptionKey: string
  buttonLabelKey: string
  available: boolean
  downloadHref?: string
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
export const QUICK_START_ENTER_DASHBOARD_PATH: QuickStartEnterDashboardPath =
  '/dashboard/overview'
export const QUICK_START_MINIMUM_QUOTA = 50_000

export const quickStartFullscreenPages: QuickStartFullscreenPage[] = [
  { id: 'purpose' },
  { id: 'model' },
  { id: 'balance' },
  { id: 'download' },
  { id: 'finish' },
]

export const purposeOptions: QuickStartPurpose[] = [
  {
    id: 'web-coding',
    titleKey: 'Web Coding',
    descriptionKey:
      'Use it for code generation, web development, debugging, and project collaboration.',
    nextActionLabelKey: 'Create API Key',
    nextActionPath: '/keys',
  },
  {
    id: 'chat',
    titleKey: 'Chat',
    descriptionKey:
      'Use it for daily conversations, writing, summaries, translation, and knowledge Q&A.',
    nextActionLabelKey: 'Open Chat',
    nextActionPath: '/chat2link',
  },
  {
    id: 'other',
    titleKey: 'Image and more',
    descriptionKey:
      'Use it for image generation, multimodal creation, creative exploration, and other model capabilities.',
    nextActionLabelKey: 'Open Playground',
    nextActionPath: '/playground',
  },
]

export const downloadCards: QuickStartDownloadCard[] = [
  {
    platform: 'macOS',
    descriptionKey: 'For Apple Silicon and Intel Mac',
    buttonLabelKey: 'Download for macOS',
    available: true,
    downloadHref: '/downloads/yunbei-macos.zip',
  },
  {
    platform: 'Windows',
    descriptionKey: 'For Windows 10 / 11',
    buttonLabelKey: 'Download for Windows',
    available: false,
  },
]

export const fallbackModels: QuickStartModelLike[] = [
  {
    model_name: 'openai/gpt-4o-mini',
    vendor_name: 'OpenAI',
    tags: 'chat code vision',
    supported_endpoint_types: ['openai'],
    quota_type: 0,
    model_ratio: 0.075,
    completion_ratio: 4,
    enable_groups: ['default'],
    group_ratio: { default: 1 },
  },
  {
    model_name: 'deepseek/deepseek-chat',
    vendor_name: 'DeepSeek',
    tags: 'chat code',
    supported_endpoint_types: ['openai'],
    quota_type: 0,
    model_ratio: 0.07,
    completion_ratio: 4,
    enable_groups: ['default'],
    group_ratio: { default: 1 },
  },
  {
    model_name: 'stability/sdxl',
    vendor_name: 'Image',
    tags: 'image',
    supported_endpoint_types: ['image-generation'],
    quota_type: 0,
    model_ratio: 0.4,
    completion_ratio: 1,
    enable_groups: ['default'],
    group_ratio: { default: 1 },
  },
]

export function getBalanceState(quota: number | null | undefined): {
  quota: number
  requiredQuota: number
  isEnough: boolean
} {
  const safeQuota = Math.max(Number(quota) || 0, 0)
  return {
    quota: safeQuota,
    requiredQuota: QUICK_START_MINIMUM_QUOTA,
    isEnough: safeQuota >= QUICK_START_MINIMUM_QUOTA,
  }
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
