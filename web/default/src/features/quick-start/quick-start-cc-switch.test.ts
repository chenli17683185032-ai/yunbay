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
  getQuickStartCCSwitchImportState,
  maskQuickStartApiKey,
  normalizeQuickStartApiKey,
  normalizeQuickStartCodexEndpoint,
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
