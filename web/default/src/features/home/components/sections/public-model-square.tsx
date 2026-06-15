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
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { MODEL_SQUARE_COPY } from '../../landing-page-copy'

const MODEL_PROVIDERS = [
  { name: 'OpenAI compatible', icon: 'OpenAI' },
  { name: 'Claude', icon: 'Claude' },
  { name: 'Gemini', icon: 'Gemini' },
  { name: 'DeepSeek', icon: 'DeepSeek' },
  { name: 'Qwen', icon: 'Qwen' },
  { name: 'Llama', icon: 'Meta' },
  { name: 'Mistral', icon: 'Mistral' },
  { name: 'Grok', icon: 'Grok' },
  { name: 'Doubao', icon: 'ByteDance' },
  { name: 'Moonshot', icon: 'Moonshot' },
] as const

export function PublicModelSquare() {
  const { t } = useTranslation()
  const marqueeItems = [...MODEL_PROVIDERS, ...MODEL_PROVIDERS]

  return (
    <section
      id='models'
      data-landing-section
      className='relative h-[100dvh] overflow-hidden px-4 pt-24 pb-10 text-white sm:px-6 lg:pt-24'
    >
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-x-0 bottom-0 h-1/2 bg-[radial-gradient(circle_at_50%_92%,rgba(180,198,255,0.2),transparent_44%)]'
      />

      <div className='mx-auto grid h-[calc(100dvh-7rem)] max-w-7xl grid-cols-1 lg:grid-cols-12'>
        <div className='self-start lg:col-span-5'>
          <div className='mb-5 font-mono text-[10px] font-semibold tracking-[0.18em] text-white/42 uppercase'>
            {t('Model routes')}
          </div>
          <h2 className='max-w-[8.2em] text-[clamp(2rem,3.8vw,3.8rem)] leading-[0.98] font-black tracking-[-0.055em] text-balance'>
            {t(MODEL_SQUARE_COPY.headline)}
          </h2>
          <p className='mt-6 max-w-md text-sm leading-7 text-white/58'>
            {t(MODEL_SQUARE_COPY.promises[0])}
          </p>
          <p className='mt-4 max-w-md font-mono text-xs leading-6 tracking-[0.08em] text-white/42 uppercase'>
            {t(MODEL_SQUARE_COPY.promises[1])}
          </p>
        </div>

        <div className='self-center lg:col-span-5 lg:col-start-8'>
          <div className='max-w-md border-l border-white/12 pl-5'>
            <div className='font-mono text-[10px] font-semibold tracking-[0.18em] text-white/34 uppercase'>
              {t('Model routes')}
            </div>
            <div className='mt-7 grid gap-3'>
              {MODEL_SQUARE_COPY.supportingPoints.map((capability) => (
                <div
                  key={capability}
                  className='grid grid-cols-[auto_1fr] items-center gap-3 text-xs text-white/58'
                >
                  <span className='h-px w-7 bg-white/20' />
                  <span>{t(capability)}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className='pointer-events-none absolute inset-x-0 bottom-8 overflow-hidden'>
        <div className='mb-4 px-4 sm:px-6'>
          <div className='mx-auto max-w-7xl font-mono text-[10px] font-semibold tracking-[0.18em] text-white/34 uppercase'>
            {t('Live model broadcast')}
          </div>
        </div>
        <div className='fade-x'>
          <div className='yunbay-logo-marquee flex w-max gap-3 px-4 sm:px-6'>
            {marqueeItems.map((provider, index) => (
              <div
                key={`${provider.name}-${index}`}
                className='flex h-16 min-w-44 items-center gap-3 rounded-2xl border border-white/10 bg-[#030409]/58 px-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'
              >
                <span className='flex size-9 items-center justify-center rounded-xl border border-white/10 bg-white/[0.04] text-white'>
                  {getLobeIcon(provider.icon, 24)}
                </span>
                <span className='text-sm font-semibold tracking-tight text-white/76'>
                  {t(provider.name)}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
