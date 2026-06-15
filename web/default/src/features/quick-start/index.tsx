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
import {
  useCallback,
  useMemo,
  useState,
  type ElementType,
  type ReactNode,
} from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  CheckCircle2,
  Code2,
  Download,
  ImageIcon,
  MessageSquare,
  Play,
  Rocket,
  Sparkles,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { YunbayLogo } from '@/components/layout/components/yunbay-logo'
import {
  LandingSnapFrame,
  type LandingSnapControlsApi,
} from '@/features/home/components'
import { getNextMorphSignal } from '@/features/home/landing-page-snap'
import {
  PointCloudMorphCanvas,
  getFaceStateForQuota,
} from '@/features/home/point-cloud'
import { usePricingData } from '@/features/pricing/hooks'
import {
  QUICK_START_DEFAULT_PURPOSE,
  QUICK_START_ENTER_DASHBOARD_PATH,
  downloadCards,
  fallbackModels,
  getBalanceState,
  getModelRateLabels,
  getModelTags,
  purposeOptions,
  quickStartFullscreenPages,
  type QuickStartEnterDashboardPath,
  type QuickStartNextActionPath,
  type QuickStartDownloadCard,
  type QuickStartModelLike,
  type QuickStartPurposeId,
} from './quick-start-data'

const PURPOSE_ICONS = {
  'web-coding': Code2,
  chat: MessageSquare,
  other: ImageIcon,
} satisfies Record<QuickStartPurposeId, ElementType>

const QUICK_START_SECTION_IDS = quickStartFullscreenPages.map((page) => page.id)

const COSMIC_AUTH_SURFACE_CLASS =
  'bg-[#030409] text-white [--accent:#121827] [--accent-foreground:#eef4ff] [--background:#030409] [--border:#1e2638] [--card:#070a14] [--card-foreground:#f7fbff] [--foreground:#f7fbff] [--muted:#0c1020] [--muted-foreground:#8f9bb8] [--primary:#eef4ff] [--primary-foreground:#030409] [--secondary:#121827] [--secondary-foreground:#eef4ff]'

type QuickStartNavigationPath =
  | QuickStartNextActionPath
  | QuickStartEnterDashboardPath
  | '/wallet'
  | '/wallet?section=redeem'

function toModelList(models: QuickStartModelLike[]): QuickStartModelLike[] {
  return models.length > 0 ? models : fallbackModels
}

export function QuickStart() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.auth.user)
  const [selectedPurposeId, setSelectedPurposeId] =
    useState<QuickStartPurposeId>(QUICK_START_DEFAULT_PURPOSE)
  const [selectedModelName, setSelectedModelName] = useState<string>('')
  const [morphSignal, setMorphSignal] = useState(0)
  const pricing = usePricingData()

  const modelList = useMemo(
    () => toModelList(pricing.models as QuickStartModelLike[]),
    [pricing.models]
  )

  const selectedPurpose =
    purposeOptions.find((item) => item.id === selectedPurposeId) ||
    purposeOptions[0]
  const activeModelName = selectedModelName || modelList[0]?.model_name || ''
  const selectedModel =
    modelList.find((model) => model.model_name === activeModelName) ||
    modelList[0]
  const balance = getBalanceState(user?.quota)
  const faceState = getFaceStateForQuota(user?.quota)

  const handlePageChange = useCallback(
    (activeIndex: number, previousIndex: number) => {
      setMorphSignal((signal) =>
        getNextMorphSignal(signal, previousIndex, activeIndex)
      )
    },
    []
  )

  const navigateToPath = useCallback(
    (path: QuickStartNavigationPath) => {
      switch (path) {
        case '/chat2link':
          navigate({ to: '/chat2link' })
          return
        case '/keys':
          navigate({ to: '/keys' })
          return
        case '/playground':
          navigate({ to: '/playground' })
          return
        case QUICK_START_ENTER_DASHBOARD_PATH:
          navigate({
            to: '/dashboard/$section',
            params: { section: 'overview' },
          })
          return
        case '/wallet':
          navigate({ to: '/wallet' })
          return
        case '/wallet?section=redeem':
          navigate({ to: '/wallet', search: { section: 'redeem' } })
          return
      }
    },
    [navigate]
  )

  const enterDashboard = useCallback(() => {
    navigateToPath(QUICK_START_ENTER_DASHBOARD_PATH)
  }, [navigateToPath])

  const QuickStartControlsComponent = useCallback(
    (api: LandingSnapControlsApi) => (
      <QuickStartControls api={api} onEnterDashboard={enterDashboard} />
    ),
    [enterDashboard]
  )

  const handleDownload = (card: QuickStartDownloadCard) => {
    if (card.available && card.downloadHref) {
      window.location.href = card.downloadHref
      return
    }

    toast.info(
      t('Download resources are being configured. Please check back later.')
    )
  }

  return (
    <main
      className={`${COSMIC_AUTH_SURFACE_CLASS} relative h-[100dvh] overflow-hidden`}
    >
      <PointCloudMorphCanvas
        faceState={faceState}
        variant='background'
        pointSize={2.55}
        morphSignal={morphSignal}
        className='z-0'
      />
      <div className='pointer-events-none fixed inset-0 z-[1] bg-[linear-gradient(180deg,rgba(3,4,9,0)_0%,rgba(3,4,9,0.2)_42%,rgba(3,4,9,0.9)_100%)]' />
      <div className='absolute top-5 left-4 z-30 flex items-center gap-3 sm:left-6'>
        <YunbayLogo />
        <div>
          <div className='text-sm font-semibold tracking-tight text-white'>
            {t('Quick Start Yunbay')}
          </div>
          <div className='font-mono text-[10px] tracking-[0.14em] text-white/42 uppercase'>
            {t('Quick Start')}
          </div>
        </div>
      </div>

      <LandingSnapFrame
        sectionIds={QUICK_START_SECTION_IDS}
        navigateEventName='quick-start:navigate'
        className='relative z-10'
        onActiveIndexChange={handlePageChange}
        controlsComponent={QuickStartControlsComponent}
      >
        <QuickStartPage
          eyebrow={t('Quick Start')}
          title={t('Choose how you will use AI')}
          description={t('This helps Yunbay recommend a practical first path.')}
        >
          <div className='grid gap-3 md:grid-cols-3'>
            {purposeOptions.map((purpose) => {
              const Icon = PURPOSE_ICONS[purpose.id]
              const selected = selectedPurposeId === purpose.id
              return (
                <button
                  key={purpose.id}
                  type='button'
                  onClick={() => setSelectedPurposeId(purpose.id)}
                  className={cn(
                    'min-h-44 rounded-[1.5rem] border p-5 text-left shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl transition-all duration-300 active:scale-[0.98]',
                    selected
                      ? 'border-white/32 bg-white/[0.12] text-white'
                      : 'border-white/10 bg-[#030409]/50 text-white/72 hover:border-white/22 hover:bg-white/[0.07] hover:text-white'
                  )}
                >
                  <div className='flex items-center justify-between gap-3'>
                    <span className='flex size-11 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.05]'>
                      <Icon className='size-5' />
                    </span>
                    {selected ? <CheckCircle2 className='size-5' /> : null}
                  </div>
                  <div className='mt-6 text-lg font-semibold tracking-tight'>
                    {t(purpose.titleKey)}
                  </div>
                  <p className='mt-3 text-sm leading-7 text-white/54'>
                    {t(purpose.descriptionKey)}
                  </p>
                </button>
              )
            })}
          </div>
        </QuickStartPage>

        <QuickStartPage
          eyebrow={t('Model routes')}
          title={t('Choose a model')}
          description={t(
            'All supported models are listed with OpenRouter-style rates.'
          )}
        >
          {pricing.isLoading ? (
            <div className='grid max-h-[52vh] gap-3 overflow-hidden md:grid-cols-2 xl:grid-cols-3'>
              {Array.from({ length: 6 }).map((_, index) => (
                <Skeleton
                  key={index}
                  className='h-36 rounded-[1.5rem] bg-white/10'
                />
              ))}
            </div>
          ) : (
            <div className='grid max-h-[52vh] gap-3 overflow-y-auto pr-1 md:grid-cols-2 xl:grid-cols-3'>
              {modelList.map((model) => {
                const selected = activeModelName === model.model_name
                const rate = getModelRateLabels(model)
                return (
                  <button
                    key={model.model_name}
                    type='button'
                    onClick={() => setSelectedModelName(model.model_name)}
                    className={cn(
                      'rounded-[1.5rem] border p-4 text-left shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl transition-all duration-300 active:scale-[0.98]',
                      selected
                        ? 'border-white/30 bg-white/[0.12] text-white'
                        : 'border-white/10 bg-[#030409]/50 text-white/72 hover:border-white/22 hover:bg-white/[0.07] hover:text-white'
                    )}
                  >
                    <div className='flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <div className='truncate font-semibold tracking-tight'>
                          {model.model_name}
                        </div>
                        <div className='mt-1 text-xs text-white/42'>
                          {model.vendor_name || t('Model provider')}
                        </div>
                      </div>
                      {selected ? (
                        <CheckCircle2 className='size-4 shrink-0 text-white' />
                      ) : null}
                    </div>
                    <div className='mt-3 flex flex-wrap gap-1.5'>
                      {getModelTags(model).map((tag) => (
                        <Badge
                          key={tag}
                          variant='outline'
                          className='border-white/12 bg-white/[0.03] text-white/62'
                        >
                          {t(tag)}
                        </Badge>
                      ))}
                    </div>
                    <div className='mt-4 grid gap-1 font-mono text-[11px] text-white/46'>
                      <span>
                        {t('Input')}: {rate.input}
                      </span>
                      <span>
                        {t('Output')}: {rate.output}
                      </span>
                    </div>
                  </button>
                )
              })}
            </div>
          )}
        </QuickStartPage>

        <QuickStartPage
          eyebrow={t('Current Balance')}
          title={t('Check account balance')}
          description={t(
            'Yunbay checks whether your balance is enough to start safely.'
          )}
        >
          <div className='grid gap-3 lg:grid-cols-3'>
            <Metric
              label={t('Current Balance')}
              value={formatQuota(balance.quota)}
            />
            <Metric
              label={t('Minimum to start')}
              value={formatQuota(balance.requiredQuota)}
            />
            <Metric
              label={t('Selected model')}
              value={selectedModel?.model_name || '-'}
            />
          </div>
          <div className='mt-5 rounded-[1.5rem] border border-white/10 bg-[#030409]/54 p-5 backdrop-blur-2xl'>
            <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
              <p className='max-w-2xl text-sm leading-7 text-white/62'>
                {balance.isEnough
                  ? t('Your balance is enough. You can continue directly.')
                  : t(
                      'Your balance is low. Please top up or redeem a code first.'
                    )}
              </p>
              <div className='flex flex-wrap gap-2'>
                {!balance.isEnough && (
                  <>
                    <Button
                      className='gap-2 rounded-full bg-white text-[#030409] hover:bg-white/88'
                      onClick={() => navigateToPath('/wallet')}
                    >
                      <WalletCards className='size-4' />
                      {t('Top up')}
                    </Button>
                    <Button
                      variant='outline'
                      className='gap-2 rounded-full border-white/14 bg-white/[0.035] text-white hover:bg-white/[0.08] hover:text-white'
                      onClick={() => navigateToPath('/wallet?section=redeem')}
                    >
                      <Sparkles className='size-4' />
                      {t('Use redemption code')}
                    </Button>
                  </>
                )}
              </div>
            </div>
          </div>
        </QuickStartPage>

        <QuickStartPage
          eyebrow={t('Download')}
          title={t('Download Yunbay to start easily')}
          description={t(
            'The macOS package is ready. Windows package will be connected later.'
          )}
        >
          <div className='grid gap-3 md:grid-cols-2'>
            {downloadCards.map((card) => (
              <div
                key={card.platform}
                className='rounded-[1.5rem] border border-white/10 bg-[#030409]/54 p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'
              >
                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <div className='text-lg font-semibold tracking-tight'>
                      {card.platform}
                    </div>
                    <p className='mt-2 text-sm text-white/54'>
                      {t(card.descriptionKey)}
                    </p>
                  </div>
                  <Download className='size-5 text-white/44' />
                </div>
                <Button
                  variant='outline'
                  className='mt-6 w-full gap-2 rounded-full border-white/14 bg-white/[0.035] text-white hover:bg-white/[0.08] hover:text-white'
                  onClick={() => handleDownload(card)}
                >
                  <Download className='size-4' />
                  {t(card.buttonLabelKey)}
                </Button>
              </div>
            ))}
          </div>
        </QuickStartPage>

        <QuickStartPage
          eyebrow={t('Finish and launch')}
          title={t('Finish and launch')}
          description={t(
            'Use the selected path to start your first Yunbay workflow.'
          )}
        >
          <div className='rounded-[1.75rem] border border-white/10 bg-[#030409]/58 p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
            <div className='grid gap-3 lg:grid-cols-3'>
              <Metric
                label={t('Quick Start')}
                value={t(selectedPurpose.titleKey)}
              />
              <Metric
                label={t('Selected model')}
                value={selectedModel?.model_name || '-'}
              />
              <Metric
                label={t('Current Balance')}
                value={formatQuota(balance.quota)}
              />
            </div>
            <div className='mt-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
              <p className='max-w-xl text-sm leading-7 text-white/56'>
                {t(
                  'Use the selected path to start your first Yunbay workflow.'
                )}
              </p>
              <div className='flex flex-wrap gap-2'>
                <Button
                  className='gap-2 rounded-full bg-white text-[#030409] hover:bg-white/88'
                  onClick={() => navigateToPath(selectedPurpose.nextActionPath)}
                >
                  <Rocket className='size-4' />
                  {t(selectedPurpose.nextActionLabelKey)}
                </Button>
                <Button
                  variant='outline'
                  className='gap-2 rounded-full border-white/14 bg-white/[0.035] text-white hover:bg-white/[0.08] hover:text-white'
                  onClick={enterDashboard}
                >
                  <Play className='size-4' />
                  {t('Enter dashboard')}
                </Button>
              </div>
            </div>
          </div>
        </QuickStartPage>
      </LandingSnapFrame>
    </main>
  )
}

function QuickStartPage(props: {
  eyebrow: string
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <section className='relative h-[100dvh] overflow-hidden px-4 pt-24 pb-24 text-white sm:px-6 lg:pt-28'>
      <div className='mx-auto grid h-full max-w-7xl grid-cols-1 gap-8 lg:grid-cols-12 lg:items-center'>
        <div className='lg:col-span-5'>
          <div className='mb-5 font-mono text-[10px] font-semibold tracking-[0.18em] text-white/42 uppercase'>
            {props.eyebrow}
          </div>
          <h1 className='max-w-[8.4em] text-[clamp(2.25rem,4.8vw,4.9rem)] leading-[0.95] font-black tracking-[-0.055em] text-balance text-white'>
            {props.title}
          </h1>
          <p className='mt-6 max-w-md text-sm leading-7 text-white/58 sm:text-base sm:leading-8'>
            {props.description}
          </p>
        </div>
        <div className='min-h-0 lg:col-span-6 lg:col-start-7'>
          {props.children}
        </div>
      </div>
    </section>
  )
}

function QuickStartControls(props: {
  api: LandingSnapControlsApi
  onEnterDashboard: () => void
}) {
  const { t } = useTranslation()
  const nextLabel = props.api.canGoNext ? t('Next') : t('Enter dashboard')
  const handleNext = props.api.canGoNext
    ? props.api.goNext
    : props.onEnterDashboard

  return (
    <div
      data-point-cloud-ignore
      className='absolute right-4 bottom-5 left-4 z-30 flex flex-wrap items-center justify-between gap-3 sm:right-6 sm:left-6 lg:left-auto'
    >
      <button
        type='button'
        onClick={props.api.goPrevious}
        disabled={!props.api.canGoPrevious}
        className='h-10 rounded-full border border-white/12 bg-[#030409]/58 px-4 text-xs font-semibold text-white/72 backdrop-blur-xl transition-all duration-300 hover:border-white/24 hover:text-white active:scale-[0.98] disabled:pointer-events-none disabled:opacity-35'
      >
        {t('Previous')}
      </button>
      <div className='rounded-full border border-white/10 bg-[#030409]/58 px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.16em] text-white/48 backdrop-blur-xl'>
        {String(props.api.activeIndex + 1).padStart(2, '0')} /{' '}
        {String(props.api.totalPages).padStart(2, '0')}
      </div>
      <button
        type='button'
        onClick={props.onEnterDashboard}
        className='h-10 rounded-full border border-white/12 bg-[#030409]/58 px-4 text-xs font-semibold text-white/72 backdrop-blur-xl transition-all duration-300 hover:border-white/24 hover:text-white active:scale-[0.98]'
      >
        {t('Enter dashboard')}
      </button>
      <button
        type='button'
        onClick={handleNext}
        className='h-10 rounded-full bg-white px-4 text-xs font-semibold text-[#030409] shadow-[0_18px_50px_rgba(255,255,255,0.14)] transition-all duration-300 hover:bg-white/88 active:scale-[0.98]'
      >
        {nextLabel}
      </button>
    </div>
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='min-w-0 rounded-[1.25rem] border border-white/10 bg-white/[0.045] p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]'>
      <div className='font-mono text-[10px] font-semibold tracking-[0.16em] text-white/38 uppercase'>
        {props.label}
      </div>
      <div className='mt-3 truncate text-sm font-semibold text-white/82'>
        {props.value}
      </div>
    </div>
  )
}
