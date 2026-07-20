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
export const QUICK_START_CC_SWITCH_APP = 'codex'
export const QUICK_START_CC_SWITCH_PROVIDER_NAME = 'Yunbay Codex'
export const QUICK_START_CC_SWITCH_PROMPT_NAME = '云贝生图推荐提示词'

type QuickStartCCSwitchImportURLInput = {
  serverAddress: string
  apiKey: string
  model: string
}

type QuickStartCCSwitchImportStateInput = {
  apiKey: string
  model: string
}

type QuickStartCCSwitchPromptImportURLInput = {
  apiKey: string
}

export type QuickStartCCSwitchImportState =
  | { canImport: true; reason: null }
  | { canImport: false; reason: 'api-key' | 'model' }

export function normalizeQuickStartServerAddress(
  serverAddress: string
): string {
  return serverAddress.trim().replace(/\/+$/, '')
}

export function normalizeQuickStartCodexEndpoint(
  serverAddress: string
): string {
  const normalized = normalizeQuickStartServerAddress(serverAddress)
  return normalized.endsWith('/v1') ? normalized : `${normalized}/v1`
}

export function normalizeQuickStartApiKey(apiKey: string): string {
  const normalized = apiKey.trim()
  if (!normalized) return ''
  return normalized.startsWith('sk-') ? normalized : `sk-${normalized}`
}

export function maskQuickStartApiKey(apiKey: string): string {
  const normalized = normalizeQuickStartApiKey(apiKey)
  if (!normalized) return '—'

  const suffix = normalized.slice(-4)
  return `sk-••••••••${suffix}`
}

export function getQuickStartCCSwitchImportState(
  input: QuickStartCCSwitchImportStateInput
): QuickStartCCSwitchImportState {
  if (!normalizeQuickStartApiKey(input.apiKey)) {
    return { canImport: false, reason: 'api-key' }
  }

  if (!input.model.trim()) {
    return { canImport: false, reason: 'model' }
  }

  return { canImport: true, reason: null }
}

export function buildQuickStartCCSwitchImportURL(
  input: QuickStartCCSwitchImportURLInput
): string {
  const serverAddress = normalizeQuickStartServerAddress(input.serverAddress)
  const params = new URLSearchParams()

  params.set('resource', 'provider')
  params.set('app', QUICK_START_CC_SWITCH_APP)
  params.set('name', QUICK_START_CC_SWITCH_PROVIDER_NAME)
  params.set('endpoint', normalizeQuickStartCodexEndpoint(serverAddress))
  params.set('apiKey', normalizeQuickStartApiKey(input.apiKey))
  params.set('model', input.model.trim())
  params.set('homepage', serverAddress)
  params.set('enabled', 'true')

  return `ccswitch://v1/import?${params.toString()}`
}

export function buildQuickStartImagePrompt(apiKey: string): string {
  const normalizedApiKey = normalizeQuickStartApiKey(apiKey)
  return `文生图请求端点走POST /v1/images/generations；图生图、参考图、局部修改、蒙版请求端点走POST /v1/images/edits。模型固定使用 \`gpt-image-2\`。禁止把 \`gpt-image-2\` 或 \`gpt-image-1.5\` 等生图模型配置为 Codex 主聊天模型，也禁止通过 \`/v1/chat/completions\` 或 \`/v1/responses\` 直接调用它。API Key为：${normalizedApiKey}\n收到图片的 Base64 数据后，先解码并将原图保存到当前工作区的 \`outputs/\` 目录；需要 4K 时再另外处理，并保留原始图片。`
}

export function buildQuickStartImagePromptPreview(apiKey: string): string {
  return buildQuickStartImagePrompt(maskQuickStartApiKey(apiKey))
}

function encodeUtf8Base64(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}

export function buildQuickStartCCSwitchPromptImportURL(
  input: QuickStartCCSwitchPromptImportURLInput
): string {
  const params = new URLSearchParams()

  params.set('resource', 'prompt')
  params.set('app', QUICK_START_CC_SWITCH_APP)
  params.set('name', QUICK_START_CC_SWITCH_PROMPT_NAME)
  params.set(
    'content',
    encodeUtf8Base64(buildQuickStartImagePrompt(input.apiKey))
  )
  params.set('enabled', 'true')

  return `ccswitch://v1/import?${params.toString()}`
}
