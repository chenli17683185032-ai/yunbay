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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { publicLandingBrand } from '@/components/layout/config/public-landing-brand.config'
import { ABOUT_COPY } from '../../landing-page-copy'

type PublicAboutProps = {
  footer?: ReactNode
}

export function PublicAbout(props: PublicAboutProps) {
  const { t } = useTranslation()

  return (
    <section
      id='about'
      data-landing-section
      className='relative h-[100dvh] overflow-hidden px-4 pt-24 pb-10 text-white sm:px-6 lg:pt-24'
    >
      <div className='mx-auto grid h-[calc(100dvh-7rem)] max-w-7xl grid-cols-1 gap-10 lg:grid-cols-12 lg:items-center'>
        <div className='lg:col-span-6'>
          <div className='mb-5 font-mono text-[10px] font-semibold tracking-[0.18em] text-white/42 uppercase'>
            {t('About yunbay')}
          </div>
          <h2 className='max-w-[8.8em] text-[clamp(2rem,3.7vw,3.8rem)] leading-[0.98] font-black tracking-[-0.055em] text-balance'>
            {t(ABOUT_COPY.headline)}
          </h2>
          <p className='mt-7 max-w-xl text-sm leading-7 text-white/62'>
            {t(publicLandingBrand.harborMeaning)}
          </p>
          <p className='mt-4 max-w-xl text-sm leading-7 text-white/52'>
            {t(ABOUT_COPY.description)}
          </p>
        </div>

        <div className='lg:col-span-5 lg:col-start-8'>
          <div className='space-y-6'>
            {ABOUT_COPY.points.map((point) => (
              <p
                key={point}
                className='border-l border-white/14 pl-5 text-sm leading-7 text-white/58'
              >
                {t(point)}
              </p>
            ))}
          </div>
        </div>
      </div>

      <div className='absolute inset-x-0 bottom-6 px-4 sm:px-6'>
        {props.footer ?? (
          <div className='mx-auto flex max-w-7xl flex-col gap-2 border-t border-white/10 pt-4 text-xs leading-6 text-white/34 sm:flex-row sm:items-center sm:justify-between'>
            <span>
              {t('Powered by New API with QuantumNous attribution preserved.')}
            </span>
            <span>{t('Designed for AI API relay operations.')}</span>
          </div>
        )}
      </div>
    </section>
  )
}
