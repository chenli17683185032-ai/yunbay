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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Markdown } from '@/components/ui/markdown'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { YunbayLogo } from '@/components/layout/components/yunbay-logo'
import { publicLandingBrand } from '@/components/layout/config/public-landing-brand.config'
import { publicLandingNavLinks } from '@/components/layout/config/public-landing-nav.config'
import {
  Hero,
  LandingSnapFrame,
  PublicAbout,
  PublicModelSquare,
} from './components'
import { useHomePageContent } from './hooks'
import { getNextMorphSignal } from './landing-page-snap'
import { PointCloudMorphCanvas, getFaceStateForQuota } from './point-cloud'

const COSMIC_PUBLIC_SURFACE_CLASS =
  'bg-[#030409] text-white [--accent:#121827] [--accent-foreground:#eef4ff] [--background:#030409] [--border:#1e2638] [--card:#070a14] [--card-foreground:#f7fbff] [--foreground:#f7fbff] [--muted:#0c1020] [--muted-foreground:#8f9bb8] [--primary:#eef4ff] [--primary-foreground:#030409] [--secondary:#121827] [--secondary-foreground:#eef4ff]'

export function Home() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const [morphSignal, setMorphSignal] = useState(0)
  const isAuthenticated = !!auth.user
  const faceState = getFaceStateForQuota(auth.user?.quota)
  const { content, isLoaded, isUrl } = useHomePageContent()
  const handleLandingPageChange = useCallback(
    (activeIndex: number, previousIndex: number) => {
      setMorphSignal((signal) =>
        getNextMorphSignal(signal, previousIndex, activeIndex)
      )
    },
    []
  )

  if (!isLoaded) {
    return (
      <PublicLayout
        showMainContainer={false}
        navLinks={publicLandingNavLinks}
        logo={<YunbayLogo />}
        siteName={publicLandingBrand.displayName}
        headerProps={{ className: COSMIC_PUBLIC_SURFACE_CLASS }}
      >
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='overflow-x-hidden'>
          {isUrl ? (
            <iframe
              src={content}
              className='h-screen w-full border-none'
              title={t('Custom Home Page')}
            />
          ) : (
            <div className='container mx-auto py-8'>
              <Markdown className='custom-home-content'>{content}</Markdown>
            </div>
          )}
        </main>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      navLinks={publicLandingNavLinks}
      logo={<YunbayLogo />}
      siteName={publicLandingBrand.displayName}
      headerProps={{ className: COSMIC_PUBLIC_SURFACE_CLASS }}
    >
      <div className={`${COSMIC_PUBLIC_SURFACE_CLASS} relative isolate`}>
        <PointCloudMorphCanvas
          faceState={faceState}
          variant='background'
          pointSize={2.55}
          morphSignal={morphSignal}
          className='z-0'
        />
        <div className='pointer-events-none fixed inset-0 z-[1] bg-[linear-gradient(180deg,rgba(3,4,9,0)_0%,rgba(3,4,9,0.18)_42%,rgba(3,4,9,0.88)_100%)]' />
        <LandingSnapFrame
          className='relative z-10'
          onActiveIndexChange={handleLandingPageChange}
        >
          <Hero
            isAuthenticated={isAuthenticated}
            userQuota={auth.user?.quota}
          />
          <PublicModelSquare />
          <PublicAbout
            footer={
              <Footer
                attributionOnly
                name={publicLandingBrand.displayName}
                className='mx-auto max-w-7xl border-white/10 bg-transparent text-white [--border:rgba(255,255,255,0.1)] [--foreground:#f7fbff] [--muted-foreground:rgba(255,255,255,0.55)] [&_a]:text-white/68 [&_span]:text-white/40'
              />
            }
          />
        </LandingSnapFrame>
      </div>
    </PublicLayout>
  )
}
