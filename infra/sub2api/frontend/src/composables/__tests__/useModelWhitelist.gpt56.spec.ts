import { describe, expect, it } from 'vitest'

if (typeof localStorage === 'undefined') {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem: () => null,
      setItem: () => undefined
    }
  })
}

if (typeof navigator === 'undefined') {
  Object.defineProperty(globalThis, 'navigator', {
    configurable: true,
    value: { language: 'en' }
  })
} else if (!navigator.language) {
  Object.defineProperty(navigator, 'language', {
    configurable: true,
    value: 'en'
  })
}

const { getModelsByPlatform, getPresetMappingsByPlatform } = await import('../useModelWhitelist')

describe('GPT-5.6 OpenAI model whitelist', () => {
  it('exposes only the four exact GPT-5.6 model identifiers', () => {
    const gpt56Models = getModelsByPlatform('openai')
      .filter(model => model.startsWith('gpt-5.6'))

    expect(gpt56Models).toEqual([
      'gpt-5.6',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6-luna'
    ])
    expect(gpt56Models).not.toContain('gpt-5.6-pro')
    expect(gpt56Models).not.toContain('gpt-5.6-unknown')
    expect(gpt56Models).not.toContain('gpt-5.6-preview')
    expect(gpt56Models).not.toContain('gpt-5.60')
  })

  it('provides the GPT-5.6 alias and exact variant presets without unknown variants', () => {
    const gpt56Presets = getPresetMappingsByPlatform('openai')
      .filter(({ from }) => from.startsWith('gpt-5.6'))

    expect(gpt56Presets).toEqual(expect.arrayContaining([
      expect.objectContaining({ from: 'gpt-5.6', to: 'gpt-5.6-sol' }),
      expect.objectContaining({ from: 'gpt-5.6-sol', to: 'gpt-5.6-sol' }),
      expect.objectContaining({ from: 'gpt-5.6-terra', to: 'gpt-5.6-terra' }),
      expect.objectContaining({ from: 'gpt-5.6-luna', to: 'gpt-5.6-luna' })
    ]))
    expect(gpt56Presets.map(({ from }) => from)).toEqual([
      'gpt-5.6',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6-luna'
    ])
  })

  it('keeps GPT-5.5 available with its existing self-mapping default', () => {
    expect(getModelsByPlatform('openai')).toContain('gpt-5.5')
    expect(getPresetMappingsByPlatform('openai')).toContainEqual(
      expect.objectContaining({ from: 'gpt-5.5', to: 'gpt-5.5' })
    )
  })
})
