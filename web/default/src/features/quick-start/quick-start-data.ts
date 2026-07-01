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
  | 'wallet'
  | 'api-key'
  | 'codex'
export type QuickStartEnterDashboardPath = '/dashboard/overview'

export type QuickStartPurpose = {
  id: QuickStartPurposeId
  titleKey: string
  descriptionKey: string
}

export type CodexDownloadCard = {
  platform: 'macOS' | 'Windows'
  descriptionKey: string
  buttonLabelKey: string
  downloadHref: string
  guideTitleKey?: string
  guideDescriptionKey?: string
  guideStepKeys?: string[]
  quarantineFixCommand?: string
  terminalInstallCommand?: string
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

export const quickStartFullscreenPages: QuickStartFullscreenPage[] = [
  { id: 'purpose' },
  { id: 'model' },
  { id: 'wallet' },
  { id: 'api-key' },
  { id: 'codex' },
]

export const nextStepGuideKeys: Record<
  Exclude<QuickStartFullscreenPageId, 'codex'>,
  string
> = {
  purpose: 'Next: choose a model and review its price.',
  model: 'Next: check your wallet or redeem a code.',
  wallet: 'Next: generate an API key and copy it automatically.',
  'api-key': 'Next: download the official Codex app for your computer.',
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

const YUNBAY_CODEX_MACOS_DOWNLOAD_HREF =
  '/downloads/yunbay-codex-macos-20260624-174731-53933cc047c3.zip'
const YUNBAY_CODEX_MACOS_DOWNLOAD_URL = `https://yunbay.xyz${YUNBAY_CODEX_MACOS_DOWNLOAD_HREF}`
const YUNBAY_CODEX_MACOS_APP_DOWNLOAD_PATH = '$HOME/Downloads/Yunbay Codex.app'
const YUNBAY_CODEX_WINDOWS_DOWNLOAD_HREF =
  '/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe'

export const codexDownloadCards: CodexDownloadCard[] = [
  {
    platform: 'macOS',
    descriptionKey: 'Download starts now.',
    buttonLabelKey: 'Download one-click launcher',
    downloadHref: YUNBAY_CODEX_MACOS_DOWNLOAD_HREF,
    quarantineFixCommand: `xattr -dr com.apple.quarantine "${YUNBAY_CODEX_MACOS_APP_DOWNLOAD_PATH}" && open "${YUNBAY_CODEX_MACOS_APP_DOWNLOAD_PATH}"`,
    terminalInstallCommand: `curl -L "${YUNBAY_CODEX_MACOS_DOWNLOAD_URL}" -o /tmp/yunbay-codex.zip && rm -rf "${YUNBAY_CODEX_MACOS_APP_DOWNLOAD_PATH}" && ditto -x -k /tmp/yunbay-codex.zip "$HOME/Downloads" && xattr -dr com.apple.quarantine "${YUNBAY_CODEX_MACOS_APP_DOWNLOAD_PATH}" && open "${YUNBAY_CODEX_MACOS_APP_DOWNLOAD_PATH}"`,
  },
  {
    platform: 'Windows',
    descriptionKey: 'Download starts now.',
    buttonLabelKey: 'Download one-click launcher',
    downloadHref: YUNBAY_CODEX_WINDOWS_DOWNLOAD_HREF,
    guideTitleKey: 'What the Windows one-click launcher can do',
    guideDescriptionKey:
      'After downloading and running the installer, open Yunbay Codex and paste your Yunbay API key into Quick Start. It will automatically write a custom API configuration and connect to https://yunbay.xyz/v1. The app also supports model provider management, connectivity testing, balance and usage queries, and Codex session management.',
    guideStepKeys: [
      'Download and run the Windows installer.',
      'Open Yunbay Codex and paste your Yunbay API key into Quick Start.',
      'Save and enable it, then start Codex, test model connectivity, and manage historical sessions.',
    ],
  },
]

export const fallbackModels: QuickStartModelLike[] = []

function normalizeModelNameForDefault(modelName: string): string {
  return modelName.toLowerCase().replace(/[\s_-]+/g, '')
}

export function isPreferredQuickStartModel(modelName: string): boolean {
  return normalizeModelNameForDefault(modelName).includes('gpt5.5')
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
