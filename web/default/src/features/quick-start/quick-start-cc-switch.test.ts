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
  buildQuickStartCCSwitchImportURL,
  buildQuickStartCCSwitchPromptImportURL,
  buildQuickStartImagePrompt,
  getQuickStartCCSwitchImportState,
  maskQuickStartApiKey,
  normalizeQuickStartApiKey,
  normalizeQuickStartCodexEndpoint,
  QUICK_START_CC_SWITCH_PROMPT_NAME,
} from './quick-start-cc-switch'

test('quick start CC Switch import URL enables a Codex provider with the selected model', () => {
  const url = buildQuickStartCCSwitchImportURL({
    serverAddress: 'https://yunbay.example',
    apiKey: 'abc123',
    model: 'gpt-5.5',
  })
  const parsed = new URL(url)

  assert.equal(parsed.protocol, 'ccswitch:')
  assert.equal(parsed.hostname, 'v1')
  assert.equal(parsed.pathname, '/import')
  assert.equal(parsed.searchParams.get('resource'), 'provider')
  assert.equal(parsed.searchParams.get('app'), 'codex')
  assert.equal(parsed.searchParams.get('name'), 'Yunbay Codex')
  assert.equal(parsed.searchParams.get('endpoint'), 'https://yunbay.example/v1')
  assert.equal(parsed.searchParams.get('apiKey'), 'sk-abc123')
  assert.equal(parsed.searchParams.get('model'), 'gpt-5.5')
  assert.equal(parsed.searchParams.get('homepage'), 'https://yunbay.example')
  assert.equal(parsed.searchParams.get('enabled'), 'true')
})

test('quick start CC Switch import URL preserves an existing sk prefix', () => {
  const url = buildQuickStartCCSwitchImportURL({
    serverAddress: 'https://yunbay.example',
    apiKey: 'sk-live-key',
    model: 'gpt-5.5',
  })
  const parsed = new URL(url)

  assert.equal(parsed.searchParams.get('apiKey'), 'sk-live-key')
})

test('quick start image prompt preserves the imported API key and required routing rules', () => {
  const prompt = buildQuickStartImagePrompt(' test-import-key ')

  assert.equal(
    prompt,
    '当我提出生图请求时，必须直接 POST `https://yunbay.xyz/v1/images/generations`，模型固定使用 `gpt-image-2`。禁止把 `gpt-image-2` 配置为 Codex 主聊天模型，也禁止通过 `/v1/chat/completions` 或 `/v1/responses` 直接调用它。API Key为：sk-test-import-key\n收到图片的 Base64 数据后，先解码并将原图保存到当前工作区的 `outputs/` 目录；需要 4K 时再另外处理，并保留原始图片。'
  )
})

test('quick start CC Switch prompt URL imports and enables the image prompt', () => {
  const parsed = new URL(
    buildQuickStartCCSwitchPromptImportURL({ apiKey: 'sk-test-import-key' })
  )
  const encodedContent = parsed.searchParams.get('content')

  assert.equal(parsed.protocol, 'ccswitch:')
  assert.equal(parsed.hostname, 'v1')
  assert.equal(parsed.pathname, '/import')
  assert.equal(parsed.searchParams.get('resource'), 'prompt')
  assert.equal(parsed.searchParams.get('app'), 'codex')
  assert.equal(
    parsed.searchParams.get('name'),
    QUICK_START_CC_SWITCH_PROMPT_NAME
  )
  assert.ok(encodedContent)
  assert.equal(
    Buffer.from(encodedContent, 'base64').toString('utf8'),
    buildQuickStartImagePrompt('sk-test-import-key')
  )
  assert.equal(parsed.searchParams.get('enabled'), 'true')
  assert.equal(parsed.searchParams.has('apiKey'), false)
  assert.equal(parsed.searchParams.has('endpoint'), false)
  assert.equal(parsed.searchParams.has('model'), false)
})

test('quick start Codex endpoint normalization avoids duplicate v1 suffixes', () => {
  assert.equal(
    normalizeQuickStartCodexEndpoint('https://yunbay.example'),
    'https://yunbay.example/v1'
  )
  assert.equal(
    normalizeQuickStartCodexEndpoint('https://yunbay.example/'),
    'https://yunbay.example/v1'
  )
  assert.equal(
    normalizeQuickStartCodexEndpoint('https://yunbay.example/v1'),
    'https://yunbay.example/v1'
  )
  assert.equal(
    normalizeQuickStartCodexEndpoint('https://yunbay.example/v1/'),
    'https://yunbay.example/v1'
  )
})

test('quick start API key normalization trims whitespace and adds sk when needed', () => {
  assert.equal(normalizeQuickStartApiKey('  abc123  '), 'sk-abc123')
  assert.equal(normalizeQuickStartApiKey(' sk-abc123 '), 'sk-abc123')
})

test('quick start API key masking keeps only a safe preview', () => {
  assert.equal(maskQuickStartApiKey('sk-1234567890abcdef'), 'sk-••••••••cdef')
  assert.equal(maskQuickStartApiKey('abc123'), 'sk-••••••••c123')
  assert.equal(maskQuickStartApiKey(''), '—')
})

test('quick start CC Switch import state reports missing prerequisites', () => {
  assert.deepEqual(
    getQuickStartCCSwitchImportState({ apiKey: '', model: 'gpt-5.5' }),
    { canImport: false, reason: 'api-key' }
  )
  assert.deepEqual(
    getQuickStartCCSwitchImportState({ apiKey: 'sk-abc123', model: '' }),
    { canImport: false, reason: 'model' }
  )
  assert.deepEqual(
    getQuickStartCCSwitchImportState({ apiKey: 'sk-abc123', model: 'gpt-5.5' }),
    { canImport: true, reason: null }
  )
})
