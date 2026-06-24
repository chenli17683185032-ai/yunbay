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
  ArrowRight,
  ArrowUpRight,
  CheckCircle2,
  Code2,
  Copy,
  Download,
  ImageIcon,
  KeyRound,
  Loader2,
  MessageSquare,
  MonitorCog,
  Sparkles,
  Terminal,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf, getUserGroups } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import { createApiKey, fetchTokenKey, searchApiKeys } from '@/features/keys/api'
import { usePricingData } from '@/features/pricing/hooks'
import { redeemTopupCode } from '@/features/wallet/api'
import {
  generateAndCopyQuickStartApiKey,
  getQuickStartApiKeyGroup,
} from './quick-start-api-key'
import {
  buildQuickStartCCSwitchImportURL,
  getQuickStartCCSwitchImportState,
  maskQuickStartApiKey,
  normalizeQuickStartCodexEndpoint,
  normalizeQuickStartServerAddress,
} from './quick-start-cc-switch'
import {
  QUICK_START_DEFAULT_PURPOSE,
  QUICK_START_ENTER_DASHBOARD_PATH,
  codexDownloadCards,
  getDefaultQuickStartModelName,
  getModelRateLabels,
  getModelTags,
  nextStepGuideKeys,
  purposeOptions,
  quickStartFullscreenPages,
  type CodexDownloadCard,
  type QuickStartEnterDashboardPath,
  type QuickStartModelLike,
  type QuickStartPurposeId,
} from './quick-start-data'
import { redeemQuickStartCode } from './quick-start-redemption'

const PURPOSE_ICONS = {
  'web-coding': Code2,
  chat: MessageSquare,
  other: ImageIcon,
} satisfies Record<QuickStartPurposeId, ElementType>

const QUICK_START_SECTION_IDS = quickStartFullscreenPages.map((page) => page.id)

const COSMIC_AUTH_SURFACE_CLASS =
  'bg-[#030409] text-white [--accent:#121827] [--accent-foreground:#eef4ff] [--background:#030409] [--border:#1e2638] [--card:#070a14] [--card-foreground:#f7fbff] [--foreground:#f7fbff] [--muted:#0c1020] [--muted-foreground:#8f9bb8] [--primary:#eef4ff] [--primary-foreground:#030409] [--secondary:#121827] [--secondary-foreground:#eef4ff]'

type QuickStartNavigationPath = QuickStartEnterDashboardPath | '/wallet'

function extractQuickStartServerAddress(
  status: Record<string, unknown> | null
): string {
  const fromStatus =
    (status?.server_address as string | undefined) ??
    (status?.serverAddress as string | undefined) ??
    (status?.data as Record<string, unknown> | undefined)?.server_address ??
    (status?.data as Record<string, unknown> | undefined)?.serverAddress

  if (typeof fromStatus === 'string' && fromStatus) {
    return fromStatus
  }

  return window.location.origin
}

export function QuickStart() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.auth.user)
  const setUser = useAuthStore((state) => state.auth.setUser)
  const [selectedPurposeId, setSelectedPurposeId] =
    useState<QuickStartPurposeId>(QUICK_START_DEFAULT_PURPOSE)
  const [selectedModelName, setSelectedModelName] = useState<string>('')
  const [generatedApiKey, setGeneratedApiKey] = useState('')
  const [isGeneratingApiKey, setIsGeneratingApiKey] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [isRedeemingCode, setIsRedeemingCode] = useState(false)
  const [morphSignal, setMorphSignal] = useState(0)
  const { status } = useStatus()
  const pricing = usePricingData()

  const modelList = useMemo(
    () => pricing.models as QuickStartModelLike[],
    [pricing.models]
  )

  const selectedPurpose =
    purposeOptions.find((item) => item.id === selectedPurposeId) ||
    purposeOptions[0]
  const defaultModelName = useMemo(
    () => getDefaultQuickStartModelName(modelList),
    [modelList]
  )
  const activeModelName =
    selectedModelName &&
    modelList.some((model) => model.model_name === selectedModelName)
      ? selectedModelName
      : defaultModelName
  const selectedModel =
    modelList.find((model) => model.model_name === activeModelName) ||
    modelList[0]
  const quickStartServerAddress = normalizeQuickStartServerAddress(
    extractQuickStartServerAddress(status as Record<string, unknown> | null)
  )
  const quickStartCodexEndpoint = normalizeQuickStartCodexEndpoint(
    quickStartServerAddress
  )
  const quickStartCCSwitchState = getQuickStartCCSwitchImportState({
    apiKey: generatedApiKey,
    model: selectedModel?.model_name || '',
  })
  const quickStartCCSwitchDisabledReason =
    quickStartCCSwitchState.reason === 'api-key'
      ? t('Generate an API key first')
      : quickStartCCSwitchState.reason === 'model'
        ? t('No model selected')
        : null
  const currentBalance = Math.max(Number(user?.quota) || 0, 0)
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
        case QUICK_START_ENTER_DASHBOARD_PATH:
          navigate({
            to: '/dashboard/$section',
            params: { section: 'overview' },
          })
          return
        case '/wallet':
          navigate({ to: '/wallet' })
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

  const handleGenerateApiKey = async () => {
    if (generatedApiKey) {
      const copied = await copyToClipboard(generatedApiKey)
      if (copied) {
        toast.success(t('Already copied to clipboard'))
      } else {
        toast.error(t('Failed to copy API key'))
      }
      return
    }

    setIsGeneratingApiKey(true)
    try {
      let preferredGroup = user?.group
      const selfResponse = await getSelf()
      if (selfResponse?.success && selfResponse.data) {
        setUser(selfResponse.data)
        preferredGroup = selfResponse.data.group || preferredGroup
      }

      const groupsResponse = await getUserGroups()
      if (!groupsResponse.success) {
        throw new Error(groupsResponse.message || t('Failed to create API key'))
      }
      const quickStartGroup = getQuickStartApiKeyGroup({
        defaultUseAutoGroup: status?.default_use_auto_group === true,
        availableGroups: Object.keys(groupsResponse.data || {}),
        preferredGroup,
      })
      const result = await generateAndCopyQuickStartApiKey({
        createApiKey,
        searchApiKeys,
        fetchTokenKey,
        copyToClipboard,
        defaultGroup: quickStartGroup.group,
        crossGroupRetry: quickStartGroup.crossGroupRetry,
      })
      setGeneratedApiKey(result.fullKey)
      toast.success(t('Already copied to clipboard'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to create API key')
      )
    } finally {
      setIsGeneratingApiKey(false)
    }
  }

  const handleRedeemCode = async () => {
    setIsRedeemingCode(true)
    try {
      const result = await redeemQuickStartCode(redemptionCode, {
        redeemTopupCode,
        refreshSelf: async () => {
          const response = await getSelf()
          if (response?.success && response.data) {
            setUser(response.data)
          }
        },
      })

      toast.success(
        t('Redemption successful! Added: {{quota}}', {
          quota: formatQuota(result.quotaAdded),
        })
      )
      setRedemptionCode('')
    } catch (error) {
      toast.error(
        error instanceof Error ? t(error.message) : t('Redemption failed')
      )
    } finally {
      setIsRedeemingCode(false)
    }
  }

  const handleDownload = (card: CodexDownloadCard) => {
    window.location.assign(card.downloadHref)
  }

  const handleImportToCCSwitch = () => {
    if (!quickStartCCSwitchState.canImport || !selectedModel?.model_name) {
      toast.warning(quickStartCCSwitchDisabledReason || t('No model selected'))
      return
    }

    const url = buildQuickStartCCSwitchImportURL({
      serverAddress: quickStartServerAddress,
      apiKey: generatedApiKey,
      model: selectedModel.model_name,
    })

    window.open(url, '_blank')
    toast.message(t('Trying to open CC Switch'))
  }

  const handleCopyCommand = async (command: string) => {
    const copied = await copyToClipboard(command)
    if (copied) {
      toast.success(t('Terminal command copied'))
    } else {
      toast.error(t('Failed to copy terminal command'))
    }
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
          nextGuide={t(nextStepGuideKeys.purpose)}
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
          nextGuide={t(nextStepGuideKeys.model)}
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
          ) : modelList.length === 0 ? (
            <div className='rounded-[1.5rem] border border-white/10 bg-[#030409]/50 p-6 text-sm leading-7 text-white/54 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
              {t(
                'No models are currently enabled in the model square. Configure backend channels and model access first.'
              )}
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
                    aria-pressed={selected}
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
          eyebrow={t('Wallet')}
          title={t('Wallet and redemption code')}
          description={t(
            'Add balance in the wallet or redeem a code before you begin.'
          )}
          nextGuide={t(nextStepGuideKeys.wallet)}
        >
          <div className='grid gap-3 md:grid-cols-2'>
            <Metric
              label={t('Current Balance')}
              value={formatQuota(currentBalance)}
            />
            <Metric
              label={t('Selected model')}
              value={selectedModel?.model_name || '-'}
            />
          </div>
          <div className='mt-5 grid gap-3 md:grid-cols-2'>
            <div className='rounded-[1.5rem] border border-white/10 bg-[#030409]/54 p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
              <WalletCards className='size-5 text-white/50' />
              <h2 className='mt-5 text-lg font-semibold tracking-tight'>
                {t('Open wallet')}
              </h2>
              <p className='mt-2 text-sm leading-7 text-white/54'>
                {t('View your balance and choose a top-up method.')}
              </p>
              <Button
                className='mt-6 w-full gap-2 rounded-full bg-white text-[#030409] hover:bg-white/88'
                onClick={() => navigateToPath('/wallet')}
              >
                <WalletCards className='size-4' />
                {t('Top up')}
              </Button>
            </div>
            <div className='rounded-[1.5rem] border border-white/10 bg-[#030409]/54 p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
              <Sparkles className='size-5 text-white/50' />
              <h2 className='mt-5 text-lg font-semibold tracking-tight'>
                {t('Redeem a code')}
              </h2>
              <p className='mt-2 text-sm leading-7 text-white/54'>
                {t('Use a redemption code to add balance to your account.')}
              </p>
              <div className='mt-6 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]'>
                <Input
                  value={redemptionCode}
                  onChange={(event) => setRedemptionCode(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' && !isRedeemingCode) {
                      void handleRedeemCode()
                    }
                  }}
                  placeholder={t('Enter your redemption code')}
                  className='h-10 rounded-full border-white/14 bg-white/[0.035] px-4 text-white placeholder:text-white/34 focus-visible:border-white/28 focus-visible:ring-white/18'
                />
                <Button
                  variant='outline'
                  className='gap-2 rounded-full border-white/14 bg-white/[0.035] text-white hover:bg-white/[0.08] hover:text-white'
                  disabled={isRedeemingCode}
                  onClick={handleRedeemCode}
                >
                  {isRedeemingCode ? (
                    <Loader2 className='size-4 animate-spin' />
                  ) : (
                    <Sparkles className='size-4' />
                  )}
                  {t('Use redemption code')}
                </Button>
              </div>
            </div>
          </div>
        </QuickStartPage>

        <QuickStartPage
          eyebrow='API KEY'
          title={t('Generate your first API key')}
          description={t(
            'Create a ready-to-use key with one click. Yunbay copies it automatically.'
          )}
          nextGuide={t(nextStepGuideKeys['api-key'])}
        >
          <div className='rounded-[1.75rem] border border-white/10 bg-[#030409]/58 p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
            <div className='grid gap-3 md:grid-cols-2'>
              <Metric
                label={t('Selected purpose')}
                value={t(selectedPurpose.titleKey)}
              />
              <Metric
                label={t('Selected model')}
                value={selectedModel?.model_name || '-'}
              />
            </div>
            <div className='mt-5 flex flex-col gap-5 rounded-[1.5rem] border border-white/10 bg-white/[0.035] p-5 sm:flex-row sm:items-center sm:justify-between'>
              <div className='flex min-w-0 items-start gap-4'>
                <span className='flex size-11 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.05]'>
                  {generatedApiKey ? (
                    <CheckCircle2 className='size-5 text-emerald-300' />
                  ) : (
                    <KeyRound className='size-5 text-white/72' />
                  )}
                </span>
                <div className='min-w-0'>
                  <h2 className='font-semibold tracking-tight text-white'>
                    {generatedApiKey
                      ? t('API key is ready')
                      : t('One-click API key')}
                  </h2>
                  <p className='mt-2 text-sm leading-7 text-white/54'>
                    {generatedApiKey
                      ? t('Already copied to clipboard')
                      : t(
                          'Click generate. The new API key will be copied to your clipboard.'
                        )}
                  </p>
                </div>
              </div>
              <Button
                className='shrink-0 gap-2 rounded-full bg-white text-[#030409] hover:bg-white/88'
                disabled={isGeneratingApiKey}
                onClick={handleGenerateApiKey}
              >
                {isGeneratingApiKey ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : generatedApiKey ? (
                  <Copy className='size-4' />
                ) : (
                  <KeyRound className='size-4' />
                )}
                {isGeneratingApiKey
                  ? t('Generating...')
                  : generatedApiKey
                    ? t('Copy API key again')
                    : t('Generate API key')}
              </Button>
            </div>
          </div>
        </QuickStartPage>

        <QuickStartPage
          eyebrow={t('Official Codex')}
          title={t('Download Codex')}
          description={t(
            'Download Yunbay Codex and connect it to your Yunbay API key.'
          )}
        >
          <div className='grid gap-3 md:grid-cols-2'>
            {codexDownloadCards.map((card) => (
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
                {card.platform === 'macOS' && card.quarantineFixCommand ? (
                  <div className='mt-4 rounded-2xl border border-amber-300/18 bg-amber-300/[0.055] p-4 text-xs leading-6 text-amber-50/74'>
                    <div className='font-semibold text-amber-50/90'>
                      {t('If macOS says the app is damaged')}
                    </div>
                    <p className='mt-1'>
                      {t(
                        'This build is not notarized by Apple yet. If Gatekeeper blocks it, run the terminal command below after downloading.'
                      )}
                    </p>
                    <code className='mt-3 block overflow-x-auto rounded-xl border border-white/10 bg-black/36 px-3 py-2 font-mono text-[11px] leading-5 text-white/78'>
                      {card.quarantineFixCommand}
                    </code>
                    <div className='mt-3 grid gap-2'>
                      <Button
                        variant='outline'
                        size='sm'
                        className='gap-2 rounded-full border-white/14 bg-white/[0.035] text-white hover:bg-white/[0.08] hover:text-white'
                        onClick={() =>
                          handleCopyCommand(card.quarantineFixCommand || '')
                        }
                      >
                        <Copy className='size-3.5' />
                        {t('Copy repair command')}
                      </Button>
                      {card.terminalInstallCommand ? (
                        <Button
                          variant='outline'
                          size='sm'
                          className='gap-2 rounded-full border-white/14 bg-white/[0.035] text-white hover:bg-white/[0.08] hover:text-white'
                          onClick={() =>
                            handleCopyCommand(card.terminalInstallCommand || '')
                          }
                        >
                          <Terminal className='size-3.5' />
                          {t('Copy one-line terminal install')}
                        </Button>
                      ) : null}
                    </div>
                  </div>
                ) : null}
              </div>
            ))}
          </div>
          <div className='mt-4 overflow-hidden rounded-[1.75rem] border border-white/10 bg-white/[0.045] shadow-[0_24px_80px_rgba(0,0,0,0.28),inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
            <div className='flex items-center justify-between gap-3 border-b border-white/10 px-5 py-3'>
              <div className='flex items-center gap-2'>
                <span className='size-2.5 rounded-full bg-[#ff5f57]' />
                <span className='size-2.5 rounded-full bg-[#febc2e]' />
                <span className='size-2.5 rounded-full bg-[#28c840]' />
              </div>
              <div className='font-mono text-[10px] font-semibold tracking-[0.18em] text-white/36 uppercase'>
                CC Switch
              </div>
            </div>
            <div className='grid gap-4 p-5 lg:grid-cols-[1.15fr_0.85fr] lg:items-center'>
              <div className='min-w-0'>
                <div className='flex items-center gap-3'>
                  <div className='grid size-10 place-items-center rounded-2xl border border-white/10 bg-white/[0.07]'>
                    <MonitorCog className='size-5 text-white/72' />
                  </div>
                  <div className='min-w-0'>
                    <h2 className='text-lg font-semibold tracking-tight text-white'>
                      {t('Import current setup to CC Switch')}
                    </h2>
                    <p className='mt-1 text-sm leading-6 text-white/52'>
                      {t(
                        'Launch CC Switch from your browser with this API and model prefilled.'
                      )}
                    </p>
                  </div>
                </div>

                <div className='mt-5 grid gap-2 sm:grid-cols-3'>
                  <QuickStartConfigPill
                    label={t('Configured API')}
                    value={quickStartCodexEndpoint}
                  />
                  <QuickStartConfigPill
                    label={t('Configured model')}
                    value={selectedModel?.model_name || t('No model selected')}
                  />
                  <QuickStartConfigPill
                    label={t('Generated API key')}
                    value={maskQuickStartApiKey(generatedApiKey)}
                  />
                </div>
              </div>

              <div className='rounded-2xl border border-white/10 bg-black/20 p-4'>
                <div className='flex items-start gap-3 text-xs leading-6 text-white/52'>
                  <CheckCircle2 className='mt-0.5 size-4 shrink-0 text-white/58' />
                  <p>
                    {t(
                      'CC Switch will import this Codex provider and enable it automatically.'
                    )}
                  </p>
                </div>
                <Button
                  className='mt-4 w-full gap-2 rounded-full bg-white text-[#030409] hover:bg-white/88 disabled:bg-white/22 disabled:text-white/42'
                  disabled={!quickStartCCSwitchState.canImport}
                  onClick={handleImportToCCSwitch}
                >
                  <ArrowUpRight className='size-4' />
                  {quickStartCCSwitchDisabledReason || t('One-click import')}
                </Button>
              </div>
            </div>
          </div>
          <p className='mt-4 text-xs leading-6 text-white/42'>
            {t(
              'The macOS download is a Yunbay Codex build. The Windows button opens the official Microsoft Store installer.'
            )}
          </p>
        </QuickStartPage>
      </LandingSnapFrame>
    </main>
  )
}

function QuickStartPage(props: {
  eyebrow: string
  title: string
  description: string
  nextGuide?: string
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
          {props.nextGuide ? (
            <div className='mt-6 flex max-w-md items-start gap-3 rounded-2xl border border-white/10 bg-white/[0.045] p-4 text-sm leading-6 text-white/66 backdrop-blur-xl'>
              <ArrowRight className='mt-0.5 size-4 shrink-0 text-white/48' />
              <span>{props.nextGuide}</span>
            </div>
          ) : null}
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

function QuickStartConfigPill(props: { label: string; value: string }) {
  return (
    <div className='min-w-0 rounded-2xl border border-white/10 bg-black/18 px-3 py-2'>
      <div className='font-mono text-[10px] font-semibold tracking-[0.14em] text-white/34 uppercase'>
        {props.label}
      </div>
      <div className='mt-1 truncate text-sm font-medium text-white/76'>
        {props.value}
      </div>
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
