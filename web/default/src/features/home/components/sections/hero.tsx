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
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { publicLandingBrand } from '@/components/layout/config/public-landing-brand.config'
import { getFaceStateForQuota } from '../../point-cloud'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
  userQuota?: number | null
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const faceState = getFaceStateForQuota(props.userQuota)

  return (
    <section
      id='home'
      data-landing-section
      className='relative isolate h-[100dvh] overflow-hidden px-4 pt-24 pb-14 text-white sm:px-6 lg:pt-24'
    >
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_18%_18%,rgba(255,255,255,0.42)_0_1px,transparent_1.5px),radial-gradient(circle_at_82%_64%,rgba(200,216,255,0.32)_0_1px,transparent_1.5px)] bg-[size:138px_138px,212px_212px] opacity-24'
      />

      <div className='mx-auto grid h-[calc(100dvh-7rem)] max-w-7xl grid-cols-1 gap-10 lg:grid-cols-12'>
        <div className='relative z-10 max-w-xl self-start lg:col-span-6 lg:col-start-1'>
          <div className='mb-5 flex flex-wrap items-center gap-x-3 gap-y-2 font-mono text-[10px] font-semibold tracking-[0.18em] text-white/48 uppercase'>
            <span>{publicLandingBrand.slug}</span>
            <span className='h-px w-10 bg-white/20' />
            <span>
              {faceState === 'open' ? t('Balance awake') : t('Balance at zero')}
            </span>
          </div>

          <h1 className='max-w-[7.4em] text-[clamp(2.45rem,5.2vw,5.35rem)] leading-[0.92] font-black tracking-[-0.06em] text-balance text-white'>
            {t(publicLandingBrand.homeHeadline)}
          </h1>

          <p className='mt-5 max-w-md text-[clamp(1rem,1.55vw,1.35rem)] leading-8 font-medium tracking-[-0.02em] text-white/72'>
            {t(publicLandingBrand.homeSubheadline)}
          </p>

          <p className='mt-7 max-w-md text-sm leading-7 text-white/56 sm:text-[15px]'>
            {t(publicLandingBrand.philosophy)}
          </p>

          <p className='mt-4 max-w-md text-xs leading-6 text-white/42'>
            {t(publicLandingBrand.mission)}
          </p>

          <div className='mt-8 flex flex-wrap items-center gap-3'>
            {props.isAuthenticated ? (
              <Button
                className='group h-10 rounded-full bg-white px-4 text-xs font-semibold text-[#030409] shadow-[0_18px_50px_rgba(255,255,255,0.16)] transition-all duration-300 hover:bg-white/88 active:scale-[0.98]'
                render={<Link to='/dashboard' />}
              >
                {t('Go to Dashboard')}
                <ArrowRight className='ml-1.5 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
              </Button>
            ) : (
              <Button
                className='group h-10 rounded-full bg-white px-4 text-xs font-semibold text-[#030409] shadow-[0_18px_50px_rgba(255,255,255,0.16)] transition-all duration-300 hover:bg-white/88 active:scale-[0.98]'
                render={
                  <Link to='/sign-in' search={{ redirect: '/dashboard' }} />
                }
              >
                {t('Get Started')}
                <ArrowRight className='ml-1.5 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
              </Button>
            )}
            <a
              href='#models'
              onClick={(event) => {
                event.preventDefault()
                window.history.pushState(null, '', '#models')
                window.dispatchEvent(
                  new CustomEvent('public-landing:navigate', {
                    detail: { hash: '#models' },
                  })
                )
              }}
              className='inline-flex h-10 items-center rounded-full border border-white/14 bg-white/[0.035] px-4 text-xs font-semibold text-white/72 backdrop-blur-xl transition-all duration-300 hover:border-white/24 hover:text-white active:scale-[0.98]'
            >
              {t('View model routes')}
            </a>
          </div>
        </div>

        <div className='relative z-10 hidden self-end pb-[8vh] lg:col-span-4 lg:col-start-9 lg:block'>
          <div className='space-y-4 border-l border-white/12 pl-5'>
            <div className='font-mono text-[11px] leading-6 text-white/44'>
              <div>{t('Persistent coding route')}</div>
              <div className='text-white/82'>vibe-coding / relay</div>
              <div className='text-white/82'>online / always-on</div>
            </div>
            <p className='max-w-xs text-xs leading-6 text-white/46'>
              {t(
                'Hold and drag the point field. The harbor keeps routes visible while the models move behind it.'
              )}
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}
