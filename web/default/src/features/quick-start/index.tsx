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
  useEffect,
  useMemo,
  useRef,
  useState,
  type ElementType,
  type ReactNode,
} from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  Check,
  CheckCircle2,
  Code2,
  Copy,
  Download,
  ImageIcon,
  KeyRound,
  Loader2,
  MessageSquare,
  RotateCcw,
  Sparkles,
  WalletCards,
} from 'lucide-react'
import {
  AnimatePresence,
  LayoutGroup,
  motion,
  useReducedMotion,
} from 'motion/react'
import { useTranslation } from 'react-i18next'
import { FaWindows } from 'react-icons/fa6'
import { SiApple } from 'react-icons/si'
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
  recoverLatestQuickStartApiKey,
} from './quick-start-api-key'
import {
  buildQuickStartCCSwitchImportURL,
  buildQuickStartCCSwitchPromptImportURL,
  getQuickStartCCSwitchImportState,
  maskQuickStartApiKey,
  normalizeQuickStartCodexEndpoint,
  normalizeQuickStartServerAddress,
} from './quick-start-cc-switch'
import {
  QUICK_START_DEFAULT_PURPOSE,
  QUICK_START_ENTER_DASHBOARD_PATH,
  QUICK_START_REASONING_EFFORT_LABEL_KEY,
  codexDownloadCards,
  getDefaultQuickStartModelName,
  getModelRateLabels,
  getModelTags,
  getQuickStartModelDisplayName,
  isPreferredQuickStartModel,
  nextStepGuideKeys,
  orderQuickStartModels,
  purposeOptions,
  quickStartFullscreenPages,
  type CodexDownloadCard,
  type QuickStartEnterDashboardPath,
  type QuickStartFullscreenPageId,
  type QuickStartModelLike,
  type QuickStartPurposeId,
} from './quick-start-data'
import {
  QuickStartSelectionCheck,
  QuickStartSelectionSurface,
  QuickStartStepMarker,
} from './quick-start-motion'
import {
  QUICK_START_REDUCED_TRANSITION,
  QUICK_START_SPRING_TRANSITION,
} from './quick-start-motion-config'
import { redeemQuickStartCode } from './quick-start-redemption'
import {
  readQuickStartSession,
  writeQuickStartSession,
  type QuickStartPlatform,
} from './quick-start-session'

const QUICK_START_API_KEY_COPY_FAILED_MESSAGE =
  'API key was generated but clipboard copy failed. You can copy it again or continue setup.'
const QUICK_START_API_KEY_QUERY_KEY = [
  'quick-start',
  'existing-api-key',
] as const

const PURPOSE_ICONS = {
  'web-coding': Code2,
  chat: MessageSquare,
  other: ImageIcon,
} satisfies Record<QuickStartPurposeId, ElementType>

const QUICK_START_SECTION_IDS = quickStartFullscreenPages.map((page) => page.id)

const COSMIC_AUTH_SURFACE_CLASS =
  'bg-[#030409] text-white [--accent:#121827] [--accent-foreground:#eef4ff] [--background:#030409] [--border:#1e2638] [--card:#070a14] [--card-foreground:#f7fbff] [--foreground:#f7fbff] [--muted:#0c1020] [--muted-foreground:#8f9bb8] [--primary:#eef4ff] [--primary-foreground:#030409] [--secondary:#121827] [--secondary-foreground:#eef4ff]'
const QUICK_START_PRIMARY_ACTION_CLASS =
  'h-11 min-w-28 rounded-full bg-white px-5 text-[#030409] transition-[transform,background-color,box-shadow] duration-300 hover:bg-white/88 hover:shadow-[0_14px_40px_rgba(255,255,255,0.16)] active:scale-[0.98]'
const QUICK_START_SECONDARY_ACTION_CLASS =
  'h-11 min-w-28 rounded-full border-white/14 bg-white/[0.035] px-5 text-white transition-[transform,background-color,border-color] duration-300 hover:bg-white/[0.08] hover:text-white active:scale-[0.98]'

type QuickStartNavigationPath = QuickStartEnterDashboardPath | '/wallet'

function extractQuickStartServerAddress(
  status: Record<string, unknown> | null
): string {
  const fromStatus =
    (status?.server_address as string | undefined) ??
    (status?.serverAddress as string | undefined) ??
    (status?.data as Record<string, unknown> | undefined)?.server_address ??
    (status?.data as Record<string, unknown> | undefined)?.serverAddress

  if (typeof fromStatus === 'string' && fromStatus) return fromStatus
  return typeof window === 'undefined' ? '' : window.location.origin
}

function navigateToQuickStartPage(pageId: QuickStartFullscreenPageId): void {
  window.dispatchEvent(
    new CustomEvent('quick-start:navigate', {
      detail: { hash: `#${pageId}` },
    })
  )
}

export function QuickStart() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const reducedMotion = useReducedMotion()
  const user = useAuthStore((state) => state.auth.user)
  const setUser = useAuthStore((state) => state.auth.setUser)
  const [initialSession] = useState(readQuickStartSession)
  const [selectedPurposeId, setSelectedPurposeId] =
    useState<QuickStartPurposeId>(QUICK_START_DEFAULT_PURPOSE)
  const [selectedModelName, setSelectedModelName] = useState(
    initialSession.modelName
  )
  const [downloadedPlatform, setDownloadedPlatform] =
    useState<QuickStartPlatform | null>(initialSession.platform)
  const [softwareConfirmed, setSoftwareConfirmed] = useState(
    initialSession.softwareConfirmed
  )
  const [importAttempted, setImportAttempted] = useState(
    initialSession.importAttempted
  )
  const [importConfirmed, setImportConfirmed] = useState(
    initialSession.importConfirmed
  )
  const [generatedApiKey, setGeneratedApiKey] = useState('')
  const [generatedApiKeyCopied, setGeneratedApiKeyCopied] = useState<
    boolean | null
  >(null)
  const [isGeneratingApiKey, setIsGeneratingApiKey] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [isRedeemingCode, setIsRedeemingCode] = useState(false)
  const [morphSignal, setMorphSignal] = useState(0)
  const [isExiting, setIsExiting] = useState(false)
  const exitTimerRef = useRef<number | null>(null)
  const exitAnimationRef = useRef<Animation | null>(null)
  const exitSurfaceRef = useRef<HTMLElement>(null)
  const exitStartedRef = useRef(false)
  const navigationCompletedRef = useRef(false)
  const importPanelRef = useRef<HTMLDivElement>(null)
  const importStatusRef = useRef<HTMLDivElement>(null)
  const { status } = useStatus()
  const pricing = usePricingData()

  const modelList = useMemo(
    () => pricing.models as QuickStartModelLike[],
    [pricing.models]
  )
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
  const orderedModelList = useMemo(
    () => orderQuickStartModels(modelList, activeModelName),
    [activeModelName, modelList]
  )
  const selectedModelDisplayName = selectedModel
    ? t(getQuickStartModelDisplayName(selectedModel.model_name))
    : t('No model selected')
  const selectedModelIsPreferred = selectedModel
    ? isPreferredQuickStartModel(selectedModel.model_name)
    : false
  const selectedModelSummary = selectedModelIsPreferred
    ? `${selectedModelDisplayName} · ${t(QUICK_START_REASONING_EFFORT_LABEL_KEY)}`
    : selectedModelDisplayName
  const preferredModelAvailable = modelList.some((model) =>
    isPreferredQuickStartModel(model.model_name)
  )
  let modelPageDescription = t(
    'Your recommended model is pinned first with its live rate.'
  )
  if (!pricing.isLoading && modelList.length > 0 && !preferredModelAvailable) {
    modelPageDescription = t(
      'GPT 5.6 Sol is unavailable. The first enabled model is selected instead.'
    )
  }

  const existingApiKeyQuery = useQuery({
    queryKey: QUICK_START_API_KEY_QUERY_KEY,
    queryFn: () =>
      recoverLatestQuickStartApiKey({ searchApiKeys, fetchTokenKey }),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const effectiveApiKey =
    generatedApiKey || existingApiKeyQuery.data?.fullKey || ''

  const quickStartServerAddress = normalizeQuickStartServerAddress(
    extractQuickStartServerAddress(status as Record<string, unknown> | null)
  )
  const quickStartCodexEndpoint = normalizeQuickStartCodexEndpoint(
    quickStartServerAddress
  )
  const quickStartCCSwitchState = getQuickStartCCSwitchImportState({
    apiKey: effectiveApiKey,
    model: selectedModel?.model_name || '',
  })
  const currentBalance = Math.max(Number(user?.quota) || 0, 0)
  const balanceReady = currentBalance > 0
  const faceState = getFaceStateForQuota(user?.quota)
  const apiKeyActionPending =
    isGeneratingApiKey || existingApiKeyQuery.isLoading
  let apiKeyStepDescription = t(
    'Create one reusable key and copy it automatically.'
  )
  if (effectiveApiKey) {
    if (generatedApiKeyCopied === false) {
      apiKeyStepDescription = t(QUICK_START_API_KEY_COPY_FAILED_MESSAGE)
    } else if (existingApiKeyQuery.data) {
      apiKeyStepDescription = t(
        'Your existing quick-start key was restored securely.'
      )
    } else {
      apiKeyStepDescription = t('Already copied to clipboard')
    }
  }

  let apiKeyActionLabel = t('Generate API key')
  let ApiKeyActionIcon = KeyRound
  if (effectiveApiKey) {
    apiKeyActionLabel = t('Copy API key')
    ApiKeyActionIcon = Copy
  }
  if (apiKeyActionPending) {
    apiKeyActionLabel = t('Preparing...')
    ApiKeyActionIcon = Loader2
  }

  let importStatusTitle = t('Did CC Switch open?')
  let importStatusDescription = t(
    'Confirm only after CC Switch shows the imported Yunbay provider.'
  )
  if (selectedModelIsPreferred) {
    importStatusDescription = t(
      'Confirm after CC Switch shows the Yunbay provider and Extreme reasoning.'
    )
  }
  if (importConfirmed) {
    importStatusTitle = t('Import confirmed')
    importStatusDescription = t(
      'Everything is ready. Enter the console when you are ready.'
    )
  }

  const navigateToPath = useCallback(
    (path: QuickStartNavigationPath) => {
      if (path === '/wallet') {
        navigate({ to: '/wallet' })
        return
      }
      navigate({
        to: '/dashboard/$section',
        params: { section: 'overview' },
      })
    },
    [navigate]
  )

  const completeDashboardNavigation = useCallback(() => {
    if (navigationCompletedRef.current) return
    navigationCompletedRef.current = true
    navigateToPath(QUICK_START_ENTER_DASHBOARD_PATH)
  }, [navigateToPath])

  useEffect(
    () => () => {
      if (exitTimerRef.current !== null) {
        window.clearTimeout(exitTimerRef.current)
      }
      exitAnimationRef.current?.cancel()
    },
    []
  )

  useEffect(() => {
    if (!softwareConfirmed || !effectiveApiKey) return
    const frame = window.requestAnimationFrame(() => {
      const target = importAttempted
        ? importStatusRef.current
        : importPanelRef.current
      target?.scrollIntoView({
        behavior: reducedMotion ? 'auto' : 'smooth',
        block: 'nearest',
      })
    })
    return () => window.cancelAnimationFrame(frame)
  }, [
    effectiveApiKey,
    importAttempted,
    importConfirmed,
    reducedMotion,
    softwareConfirmed,
  ])

  const beginDashboardExit = useCallback(
    (showCompletionPrompt: boolean) => {
      if (exitStartedRef.current) return
      exitStartedRef.current = true
      writeQuickStartSession({
        modelName: selectedModel?.model_name || '',
        platform: downloadedPlatform,
        softwareConfirmed,
        importAttempted,
        importConfirmed,
        completionPromptPending: showCompletionPrompt,
      })
      setMorphSignal((signal) => signal + 1)
      setIsExiting(true)

      const exitSurface = exitSurfaceRef.current
      if (exitSurface?.animate) {
        const keyframes: Keyframe[] = reducedMotion
          ? [{ opacity: 1 }, { opacity: 0 }]
          : [
              {
                clipPath: 'polygon(0 0, 100% 0, 100% 100%, 0 100%)',
                filter: 'blur(0px)',
                opacity: 1,
                transform: 'translate3d(0, 0, 0) scale(1)',
                offset: 0,
                easing: 'cubic-bezier(0.32, 0, 0.24, 1)',
              },
              {
                clipPath:
                  'polygon(0 0, 100% 0, 100% 72%, 88% 77%, 74% 69%, 60% 75%, 46% 68%, 31% 76%, 16% 70%, 0 75%)',
                filter: 'blur(2px)',
                opacity: 0.92,
                transform: 'translate3d(0, 5px, 0) scale(0.995)',
                offset: 0.58,
                easing: 'cubic-bezier(0.16, 1, 0.3, 1)',
              },
              {
                clipPath: 'polygon(0 0, 100% 0, 100% 0, 0 0)',
                filter: 'blur(14px)',
                opacity: 0,
                transform: 'translate3d(0, 24px, 0) scale(0.975)',
                offset: 1,
              },
            ]
        const animation = exitSurface.animate(keyframes, {
          duration: reducedMotion ? 180 : 1050,
          easing: reducedMotion ? 'ease-out' : 'linear',
          fill: 'forwards',
        })
        exitAnimationRef.current = animation
        animation.onfinish = completeDashboardNavigation
      }

      exitTimerRef.current = window.setTimeout(
        completeDashboardNavigation,
        reducedMotion ? 240 : 1250
      )
    },
    [
      completeDashboardNavigation,
      downloadedPlatform,
      importAttempted,
      importConfirmed,
      reducedMotion,
      selectedModel?.model_name,
      softwareConfirmed,
    ]
  )

  const handlePageChange = useCallback(
    (activeIndex: number, previousIndex: number) => {
      setMorphSignal((signal) =>
        getNextMorphSignal(signal, previousIndex, activeIndex)
      )
    },
    []
  )

  const handleSelectModel = (modelName: string) => {
    setSelectedModelName(modelName)
    setImportAttempted(false)
    setImportConfirmed(false)
    writeQuickStartSession({
      modelName,
      importAttempted: false,
      importConfirmed: false,
    })
  }

  const handleDownload = (card: CodexDownloadCard) => {
    setDownloadedPlatform(card.platform)
    setSoftwareConfirmed(false)
    setImportAttempted(false)
    setImportConfirmed(false)
    writeQuickStartSession({
      modelName: selectedModel?.model_name || '',
      platform: card.platform,
      softwareConfirmed: false,
      importAttempted: false,
      importConfirmed: false,
    })
  }

  const handleGenerateApiKey = async () => {
    if (effectiveApiKey) {
      const copied = await copyToClipboard(effectiveApiKey)
      setGeneratedApiKeyCopied(copied)
      if (copied) {
        toast.success(t('Already copied to clipboard'))
      } else {
        toast.warning(t(QUICK_START_API_KEY_COPY_FAILED_MESSAGE))
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
      setGeneratedApiKeyCopied(result.copied)
      queryClient.setQueryData(QUICK_START_API_KEY_QUERY_KEY, result)
      if (result.copied) {
        toast.success(t('Already copied to clipboard'))
      } else {
        toast.warning(t(QUICK_START_API_KEY_COPY_FAILED_MESSAGE))
      }
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
        redeemTopupCode: (request) =>
          redeemTopupCode(request, {
            skipBusinessError: true,
            skipErrorHandler: true,
          }),
        refreshSelf: async () => {
          const response = await getSelf()
          if (response?.success && response.data) setUser(response.data)
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

  const handleConfirmSoftware = () => {
    setSoftwareConfirmed(true)
    writeQuickStartSession({
      modelName: selectedModel?.model_name || '',
      platform: downloadedPlatform,
      softwareConfirmed: true,
    })
  }

  const handleReturnToSoftware = () => {
    setSoftwareConfirmed(false)
    setImportAttempted(false)
    setImportConfirmed(false)
    writeQuickStartSession({
      softwareConfirmed: false,
      importAttempted: false,
      importConfirmed: false,
    })
    navigateToQuickStartPage('software')
  }

  const handleImportToCCSwitch = () => {
    if (!quickStartCCSwitchState.canImport || !selectedModel?.model_name) {
      const message =
        quickStartCCSwitchState.reason === 'api-key'
          ? t('Generate an API key first')
          : t('No model selected')
      toast.warning(message)
      return
    }

    setImportAttempted(true)
    setImportConfirmed(false)
    writeQuickStartSession({
      modelName: selectedModel.model_name,
      platform: downloadedPlatform,
      softwareConfirmed,
      importAttempted: true,
      importConfirmed: false,
    })
    const url = buildQuickStartCCSwitchImportURL({
      serverAddress: quickStartServerAddress,
      apiKey: effectiveApiKey,
      model: selectedModel.model_name,
    })
    window.open(url, '_blank')
    toast.message(t('Trying to open CC Switch'))
  }

  const handleImportPromptToCCSwitch = () => {
    if (!effectiveApiKey) {
      toast.warning(t('Generate an API key first'))
      return
    }

    const url = buildQuickStartCCSwitchPromptImportURL({
      apiKey: effectiveApiKey,
    })
    window.open(url, '_blank')
    toast.message(t('Trying to open the recommended prompt in CC Switch'))
  }

  const handleConfirmImport = () => {
    setImportConfirmed(true)
    writeQuickStartSession({
      modelName: selectedModel?.model_name || '',
      platform: downloadedPlatform,
      softwareConfirmed,
      importAttempted: true,
      importConfirmed: true,
    })
    toast.success(t('CC Switch setup confirmed'))
  }

  const QuickStartControlsComponent = useCallback(
    (api: LandingSnapControlsApi) => (
      <QuickStartControls
        api={api}
        canFinish={importConfirmed}
        disabled={isExiting}
        reducedMotion={Boolean(reducedMotion)}
        onEnterDashboard={() => beginDashboardExit(true)}
        onSkip={() => beginDashboardExit(false)}
      />
    ),
    [beginDashboardExit, importConfirmed, isExiting, reducedMotion]
  )

  return (
    <div className='fixed inset-0 h-[100dvh] w-full overflow-hidden bg-[#030409]'>
      <main
        ref={exitSurfaceRef}
        className={`${COSMIC_AUTH_SURFACE_CLASS} relative h-[100dvh] overflow-hidden`}
      >
        <PointCloudMorphCanvas
          faceState={faceState}
          variant='background'
          pointSize={2.55}
          morphSignal={morphSignal}
          className='absolute z-0'
        />
        <div className='pointer-events-none fixed inset-0 z-[1] bg-black/20' />
        <div className='absolute top-5 left-4 z-30 flex items-center gap-3 sm:left-6'>
          <YunbayLogo />
          <div>
            <div className='text-sm font-semibold text-white'>
              {t('Quick Start Yunbay')}
            </div>
            <div className='font-mono text-[10px] text-white/42 uppercase'>
              {t('Quick Start')}
            </div>
          </div>
        </div>

        <LandingSnapFrame
          sectionIds={QUICK_START_SECTION_IDS}
          navigateEventName='quick-start:navigate'
          className='relative z-10'
          allowContentScroll
          onActiveIndexChange={handlePageChange}
          controlsComponent={QuickStartControlsComponent}
        >
          <QuickStartPage
            eyebrow={t('Quick Start')}
            title={t('Choose how you will use AI')}
            description={t(
              'This helps Yunbay recommend a practical first path.'
            )}
            nextGuide={t(nextStepGuideKeys.purpose)}
          >
            <LayoutGroup id='quick-start-purpose-selection'>
              <div className='grid gap-3 md:grid-cols-3'>
                {purposeOptions.map((purpose) => {
                  const Icon = PURPOSE_ICONS[purpose.id]
                  const selected = selectedPurposeId === purpose.id
                  return (
                    <motion.button
                      key={purpose.id}
                      data-quick-start-purpose={purpose.id}
                      layout={reducedMotion ? false : 'position'}
                      type='button'
                      aria-pressed={selected}
                      onClick={() => setSelectedPurposeId(purpose.id)}
                      whileTap={reducedMotion ? undefined : { scale: 0.985 }}
                      transition={
                        reducedMotion
                          ? QUICK_START_REDUCED_TRANSITION
                          : QUICK_START_SPRING_TRANSITION
                      }
                      className={cn(
                        'relative isolate min-h-40 overflow-hidden rounded-[1.5rem] border p-5 text-left backdrop-blur-2xl transition-[border-color,color,background-color] duration-300 md:min-h-44',
                        selected
                          ? 'border-transparent bg-transparent text-white'
                          : 'border-white/10 bg-[#030409]/50 text-white/72 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] hover:border-white/22 hover:bg-white/[0.07] hover:text-white'
                      )}
                    >
                      {selected ? (
                        <QuickStartSelectionSurface
                          layoutId='quick-start-purpose-selected'
                          reducedMotion={Boolean(reducedMotion)}
                        />
                      ) : null}
                      <div className='relative z-10'>
                        <div className='flex items-center justify-between gap-3'>
                          <span className='flex size-11 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.05]'>
                            <Icon className='size-5' aria-hidden='true' />
                          </span>
                          <QuickStartSelectionCheck
                            visible={selected}
                            reducedMotion={Boolean(reducedMotion)}
                            className='size-5'
                          />
                        </div>
                        <div className='mt-5 text-lg font-semibold'>
                          {t(purpose.titleKey)}
                        </div>
                        <p className='mt-2 text-sm leading-6 text-white/54'>
                          {t(purpose.descriptionKey)}
                        </p>
                      </div>
                    </motion.button>
                  )
                })}
              </div>
            </LayoutGroup>
          </QuickStartPage>

          <QuickStartPage
            eyebrow={t('Model routes')}
            title={t('Choose a model')}
            description={modelPageDescription}
            nextGuide={t(nextStepGuideKeys.model)}
          >
            {pricing.isLoading ? (
              <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
                {Array.from({ length: 6 }).map((_, index) => (
                  <Skeleton
                    key={index}
                    className='h-36 rounded-[1.5rem] bg-white/10'
                  />
                ))}
              </div>
            ) : modelList.length === 0 ? (
              <div className='rounded-[1.5rem] border border-white/10 bg-[#030409]/50 p-6 text-sm leading-7 text-white/54 backdrop-blur-2xl'>
                {t(
                  'No models are currently enabled in the model square. Configure backend channels and model access first.'
                )}
              </div>
            ) : (
              <LayoutGroup id='quick-start-model-selection'>
                <div className='grid max-h-[52vh] gap-3 overflow-y-auto pr-1 md:grid-cols-2 xl:grid-cols-3'>
                  {orderedModelList.map((model) => {
                    const selected = activeModelName === model.model_name
                    const preferred = isPreferredQuickStartModel(
                      model.model_name
                    )
                    const rate = getModelRateLabels(model)
                    return (
                      <motion.button
                        key={model.model_name}
                        data-quick-start-model={model.model_name}
                        layout={reducedMotion ? false : 'position'}
                        type='button'
                        aria-pressed={selected}
                        onClick={() => handleSelectModel(model.model_name)}
                        whileTap={reducedMotion ? undefined : { scale: 0.985 }}
                        transition={
                          reducedMotion
                            ? QUICK_START_REDUCED_TRANSITION
                            : QUICK_START_SPRING_TRANSITION
                        }
                        className={cn(
                          'relative isolate overflow-hidden rounded-[1.5rem] border p-4 text-left backdrop-blur-2xl transition-[border-color,color,background-color] duration-300',
                          selected
                            ? 'border-transparent bg-transparent text-white'
                            : 'border-white/10 bg-[#030409]/50 text-white/72 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] hover:border-white/22 hover:bg-white/[0.07] hover:text-white'
                        )}
                      >
                        {selected ? (
                          <QuickStartSelectionSurface
                            layoutId='quick-start-model-selected'
                            reducedMotion={Boolean(reducedMotion)}
                          />
                        ) : null}
                        <div className='relative z-10'>
                          <div className='flex items-start justify-between gap-3'>
                            <div className='min-w-0'>
                              <div className='truncate font-semibold'>
                                {t(
                                  getQuickStartModelDisplayName(
                                    model.model_name
                                  )
                                )}
                              </div>
                              <div className='mt-1 text-xs text-white/42'>
                                {model.vendor_name || t('Model provider')}
                              </div>
                            </div>
                            <QuickStartSelectionCheck
                              visible={selected}
                              reducedMotion={Boolean(reducedMotion)}
                              className='size-4 text-white'
                            />
                          </div>
                          <div className='mt-3 flex flex-wrap gap-1.5'>
                            {preferred ? (
                              <Badge className='border-white/20 bg-white text-[#030409]'>
                                {t('Recommended')}
                              </Badge>
                            ) : null}
                            {preferred ? (
                              <Badge
                                variant='outline'
                                className='border-white/18 bg-white/[0.05] text-white/72'
                              >
                                {t(QUICK_START_REASONING_EFFORT_LABEL_KEY)}
                              </Badge>
                            ) : null}
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
                        </div>
                      </motion.button>
                    )
                  })}
                </div>
              </LayoutGroup>
            )}
          </QuickStartPage>

          <QuickStartPage
            eyebrow='CC SWITCH'
            title={t('Download CC Switch')}
            description={t(
              'Choose your computer. The official installer opens directly from GitHub.'
            )}
            nextGuide={t(nextStepGuideKeys.software)}
          >
            <LayoutGroup id='quick-start-software-selection'>
              <div className='flex flex-col gap-3'>
                {codexDownloadCards.map((card) => (
                  <SoftwareDownloadRow
                    key={card.platform}
                    card={card}
                    selected={downloadedPlatform === card.platform}
                    reducedMotion={Boolean(reducedMotion)}
                    onDownload={handleDownload}
                  />
                ))}
              </div>
            </LayoutGroup>
            <div className='mt-4 flex items-start gap-3 rounded-[1.25rem] border border-white/10 bg-white/[0.035] p-4 text-sm leading-6 text-white/56 backdrop-blur-xl'>
              <CheckCircle2
                className='mt-0.5 size-4 shrink-0 text-white/64'
                aria-hidden='true'
              />
              <span>
                {t(
                  'Your browser keeps this guide open while GitHub starts the download in a new tab.'
                )}
              </span>
            </div>
          </QuickStartPage>

          <QuickStartPage
            eyebrow={t('Account setup')}
            title={t('Prepare your account')}
            description={t(
              'Add balance if needed, then create or reuse one API key.'
            )}
            nextGuide={t(nextStepGuideKeys.account)}
          >
            <div className='grid gap-3 sm:grid-cols-3'>
              <Metric
                label={t('Current Balance')}
                value={formatQuota(currentBalance)}
              />
              <Metric
                label={t('Selected model')}
                value={selectedModelSummary}
              />
              <Metric
                label={t('API key')}
                value={effectiveApiKey ? t('Ready') : t('Not ready')}
              />
            </div>
            <div className='mt-4 flex flex-col gap-3'>
              <AccountStepCard
                step='01'
                title={t('Add balance or redeem a code')}
                description={
                  balanceReady
                    ? t(
                        'Your balance is ready. You can still add more at any time.'
                      )
                    : t(
                        'Top up or use a redemption code before your first request.'
                      )
                }
                complete={balanceReady}
                reducedMotion={Boolean(reducedMotion)}
              >
                <div className='grid w-full gap-2 sm:grid-cols-[11rem_minmax(0,1fr)_11rem]'>
                  <Button
                    className={QUICK_START_PRIMARY_ACTION_CLASS}
                    onClick={() => navigateToPath('/wallet')}
                  >
                    <WalletCards data-icon='inline-start' />
                    {t('Top up')}
                  </Button>
                  <Input
                    value={redemptionCode}
                    onChange={(event) => setRedemptionCode(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' && !isRedeemingCode) {
                        void handleRedeemCode()
                      }
                    }}
                    placeholder={t('Enter your redemption code')}
                    className='h-11 rounded-full border-white/14 bg-white/[0.035] px-4 text-white placeholder:text-white/34 focus-visible:border-white/28 focus-visible:ring-white/18'
                  />
                  <Button
                    variant='outline'
                    className={QUICK_START_SECONDARY_ACTION_CLASS}
                    disabled={isRedeemingCode}
                    onClick={handleRedeemCode}
                  >
                    {isRedeemingCode ? (
                      <Loader2
                        data-icon='inline-start'
                        className='animate-spin'
                      />
                    ) : (
                      <Sparkles data-icon='inline-start' />
                    )}
                    {t('Redeem')}
                  </Button>
                </div>
              </AccountStepCard>

              <AccountStepCard
                step='02'
                title={
                  effectiveApiKey
                    ? t('API key is ready')
                    : t('Create your API key')
                }
                description={apiKeyStepDescription}
                complete={Boolean(effectiveApiKey)}
                reducedMotion={Boolean(reducedMotion)}
                inlineActions
              >
                <div className='flex justify-end'>
                  <Button
                    className={cn(
                      QUICK_START_PRIMARY_ACTION_CLASS,
                      'w-full sm:w-auto'
                    )}
                    disabled={apiKeyActionPending}
                    onClick={handleGenerateApiKey}
                  >
                    <ApiKeyActionIcon
                      data-icon='inline-start'
                      className={
                        apiKeyActionPending ? 'animate-spin' : undefined
                      }
                    />
                    {apiKeyActionLabel}
                  </Button>
                </div>
              </AccountStepCard>
            </div>
          </QuickStartPage>

          <QuickStartPage
            eyebrow={t('Ready check')}
            title={t('Review and import')}
            description={t(
              'Confirm the last device step, then import your prepared setup.'
            )}
          >
            <div className='flex flex-col gap-3'>
              <ReadinessRow
                step='01'
                title={t('Model selected')}
                description={selectedModelSummary}
                complete={Boolean(selectedModel)}
                reducedMotion={Boolean(reducedMotion)}
              />
              <ReadinessRow
                step='02'
                title={t('CC Switch installed')}
                description={
                  softwareConfirmed
                    ? t('Installation confirmed on this device.')
                    : t('Have you finished installing CC Switch?')
                }
                complete={softwareConfirmed}
                reducedMotion={Boolean(reducedMotion)}
              >
                {!softwareConfirmed ? (
                  <div className='flex flex-wrap gap-2'>
                    <Button
                      className={QUICK_START_PRIMARY_ACTION_CLASS}
                      onClick={handleConfirmSoftware}
                    >
                      <Check data-icon='inline-start' />
                      {t('Already installed')}
                    </Button>
                    <Button
                      variant='outline'
                      className={QUICK_START_SECONDARY_ACTION_CLASS}
                      onClick={handleReturnToSoftware}
                    >
                      {t('Not yet')}
                    </Button>
                  </div>
                ) : null}
              </ReadinessRow>
              <ReadinessRow
                step='03'
                title={t('API key ready')}
                description={
                  effectiveApiKey
                    ? maskQuickStartApiKey(effectiveApiKey)
                    : t('Generate an API key before importing.')
                }
                complete={Boolean(effectiveApiKey)}
                reducedMotion={Boolean(reducedMotion)}
              >
                {!effectiveApiKey ? (
                  <Button
                    variant='outline'
                    className={QUICK_START_SECONDARY_ACTION_CLASS}
                    onClick={() => navigateToQuickStartPage('account')}
                  >
                    {t('Return to account setup')}
                  </Button>
                ) : null}
              </ReadinessRow>
            </div>

            <AnimatePresence initial={false}>
              {softwareConfirmed && effectiveApiKey ? (
                <motion.div
                  ref={importPanelRef}
                  initial={{ opacity: 0, y: 12 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: 8 }}
                  transition={{ duration: 0.32, ease: [0.16, 1, 0.3, 1] }}
                  className='mt-3 scroll-mb-4 sm:scroll-mb-[8rem]'
                >
                  <CCSwitchImportPanel
                    endpoint={quickStartCodexEndpoint}
                    modelName={selectedModelDisplayName}
                    reasoningTarget={
                      selectedModelIsPreferred
                        ? t(QUICK_START_REASONING_EFFORT_LABEL_KEY)
                        : null
                    }
                    apiKey={effectiveApiKey}
                    providerImportAttempted={importAttempted}
                    onImportProvider={handleImportToCCSwitch}
                    onImportPrompt={handleImportPromptToCCSwitch}
                  />
                </motion.div>
              ) : null}
            </AnimatePresence>

            <AnimatePresence initial={false}>
              {importAttempted ? (
                <motion.div
                  ref={importStatusRef}
                  initial={{ opacity: 0, y: 12 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: 8 }}
                  transition={{ duration: 0.32, ease: [0.16, 1, 0.3, 1] }}
                  className='mt-3 scroll-mb-4 rounded-[1.25rem] border border-white/12 bg-white/[0.055] p-4 backdrop-blur-xl sm:scroll-mb-[8rem]'
                >
                  <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
                    <div>
                      <div className='font-semibold text-white'>
                        {importStatusTitle}
                      </div>
                      <p className='mt-1 text-sm leading-6 text-white/54'>
                        {importStatusDescription}
                      </p>
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {!importConfirmed ? (
                        <Button
                          className={QUICK_START_PRIMARY_ACTION_CLASS}
                          onClick={handleConfirmImport}
                        >
                          <Check data-icon='inline-start' />
                          {t('It opened')}
                        </Button>
                      ) : null}
                      <Button
                        variant='outline'
                        className={QUICK_START_SECONDARY_ACTION_CLASS}
                        onClick={handleImportToCCSwitch}
                      >
                        <RotateCcw data-icon='inline-start' />
                        {t('Try again')}
                      </Button>
                    </div>
                  </div>
                </motion.div>
              ) : null}
            </AnimatePresence>
          </QuickStartPage>
        </LandingSnapFrame>
      </main>
    </div>
  )
}

function SoftwareDownloadRow(props: {
  card: CodexDownloadCard
  selected: boolean
  reducedMotion: boolean
  onDownload: (card: CodexDownloadCard) => void
}) {
  const { t } = useTranslation()
  const Icon = props.card.platform === 'macOS' ? SiApple : FaWindows

  return (
    <motion.div
      data-quick-start-platform={props.card.platform}
      layout={props.reducedMotion ? false : 'position'}
      transition={
        props.reducedMotion
          ? QUICK_START_REDUCED_TRANSITION
          : QUICK_START_SPRING_TRANSITION
      }
      className={cn(
        'relative isolate grid grid-cols-1 gap-5 overflow-hidden rounded-[1.5rem] border p-5 backdrop-blur-2xl transition-colors duration-300 sm:grid-cols-[minmax(0,1fr)_14rem] sm:items-center',
        props.selected
          ? 'border-transparent bg-transparent'
          : 'border-white/10 bg-[#030409]/54 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]'
      )}
    >
      {props.selected ? (
        <QuickStartSelectionSurface
          layoutId='quick-start-software-selected'
          reducedMotion={props.reducedMotion}
        />
      ) : null}
      <div className='relative z-10 flex min-w-0 items-start gap-4'>
        <span className='flex size-12 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.05]'>
          <Icon className='size-5 text-white/82' aria-hidden='true' />
        </span>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <h2 className='text-lg font-semibold text-white'>
              {t(props.card.titleKey)}
            </h2>
            <AnimatePresence initial={false}>
              {props.selected ? (
                <motion.span
                  key='download-opened'
                  initial={
                    props.reducedMotion
                      ? { opacity: 0 }
                      : { opacity: 0, scale: 0.86, x: -5 }
                  }
                  animate={{ opacity: 1, scale: 1, x: 0 }}
                  exit={
                    props.reducedMotion
                      ? { opacity: 0 }
                      : { opacity: 0, scale: 0.9, x: 4 }
                  }
                  transition={
                    props.reducedMotion
                      ? QUICK_START_REDUCED_TRANSITION
                      : QUICK_START_SPRING_TRANSITION
                  }
                >
                  <Badge className='border-white/20 bg-white text-[#030409]'>
                    {t('Download opened')}
                  </Badge>
                </motion.span>
              ) : null}
            </AnimatePresence>
          </div>
          <p className='mt-1 text-sm leading-6 text-white/56'>
            {t(props.card.descriptionKey)}
          </p>
          <p className='mt-2 font-mono text-[11px] text-white/38'>
            {t(props.card.detailKey)}
          </p>
        </div>
      </div>
      <Button
        className='relative z-10 h-12 w-full rounded-full bg-white px-5 text-[#030409] transition-[transform,background-color,box-shadow] duration-300 hover:bg-white/88 hover:shadow-[0_14px_40px_rgba(255,255,255,0.16)] active:scale-[0.98]'
        render={
          <a
            href={props.card.downloadHref}
            target='_blank'
            rel='noopener noreferrer'
          />
        }
        onClick={() => props.onDownload(props.card)}
      >
        <Download data-icon='inline-start' />
        {t(props.card.buttonLabelKey)}
      </Button>
    </motion.div>
  )
}

function AccountStepCard(props: {
  step: string
  title: string
  description: string
  complete: boolean
  reducedMotion: boolean
  inlineActions?: boolean
  children: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <div className='rounded-[1.5rem] border border-white/10 bg-[#030409]/54 p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl'>
      <div
        className={cn(
          'flex flex-col gap-5',
          props.inlineActions &&
            'sm:flex-row sm:items-center sm:justify-between'
        )}
      >
        <div className='flex min-w-0 items-start gap-4'>
          <QuickStartStepMarker
            step={props.step}
            complete={props.complete}
            reducedMotion={props.reducedMotion}
            className='size-10 rounded-2xl text-xs'
          />
          <div>
            <div className='flex flex-wrap items-center gap-2'>
              <h2 className='font-semibold text-white'>{props.title}</h2>
              {props.complete ? (
                <span className='text-xs text-white/52'>{t('Complete')}</span>
              ) : null}
            </div>
            <p className='mt-1 text-sm leading-6 text-white/52'>
              {props.description}
            </p>
          </div>
        </div>
        <div
          className={cn(
            'w-full',
            props.inlineActions && 'sm:w-auto sm:shrink-0'
          )}
        >
          {props.children}
        </div>
      </div>
    </div>
  )
}

function ReadinessRow(props: {
  step: string
  title: string
  description: string
  complete: boolean
  reducedMotion: boolean
  children?: ReactNode
}) {
  return (
    <div className='rounded-[1.25rem] border border-white/10 bg-[#030409]/54 p-4 backdrop-blur-xl'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex min-w-0 items-start gap-3'>
          <QuickStartStepMarker
            step={props.step}
            complete={props.complete}
            reducedMotion={props.reducedMotion}
            className='size-9 rounded-xl text-[11px]'
          />
          <div className='min-w-0'>
            <div className='font-semibold text-white'>{props.title}</div>
            <p className='mt-1 text-sm leading-6 break-words text-white/52'>
              {props.description}
            </p>
          </div>
        </div>
        {props.children ? (
          <div className='shrink-0'>{props.children}</div>
        ) : null}
      </div>
    </div>
  )
}

function CCSwitchImportPanel(props: {
  endpoint: string
  modelName: string
  reasoningTarget: string | null
  apiKey: string
  providerImportAttempted: boolean
  onImportProvider: () => void
  onImportPrompt: () => void
}) {
  const { t } = useTranslation()
  const handleImport = props.providerImportAttempted
    ? props.onImportPrompt
    : props.onImportProvider
  const importLabel = props.providerImportAttempted
    ? t('Continue importing recommended prompt')
    : t('One-click import')
  const ImportIcon = props.providerImportAttempted ? Sparkles : ArrowUpRight

  return (
    <div className='overflow-hidden rounded-[1.25rem] border border-white/12 bg-white/[0.045] shadow-[0_24px_80px_rgba(0,0,0,0.28),inset_0_1px_0_rgba(255,255,255,0.08)]'>
      <div className='flex items-center justify-between gap-3 border-b border-white/10 px-5 py-3'>
        <div className='flex items-center gap-2'>
          <span className='size-2.5 rounded-full bg-[#ff5f57]' />
          <span className='size-2.5 rounded-full bg-[#febc2e]' />
          <span className='size-2.5 rounded-full bg-[#28c840]' />
        </div>
        <div className='font-mono text-[10px] text-white/36 uppercase'>
          CC Switch
        </div>
      </div>
      <div className='grid gap-4 p-5 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)] lg:items-center'>
        <div className='min-w-0'>
          <h2 className='text-lg font-semibold text-white'>
            {t('Import current setup to CC Switch')}
          </h2>
          <p className='mt-1 text-sm leading-6 text-white/52'>
            {t('The API, model, and key are prepared for one-click import.')}
          </p>
          <div className='mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-4'>
            <Metric
              label={t('Configured API')}
              value={props.endpoint}
              compact
            />
            <Metric
              label={t('Configured model')}
              value={props.modelName}
              compact
            />
            <Metric
              label={t('Generated API key')}
              value={maskQuickStartApiKey(props.apiKey)}
              compact
            />
            {props.reasoningTarget ? (
              <Metric
                label={t('Reasoning target')}
                value={props.reasoningTarget}
                compact
              />
            ) : null}
          </div>
        </div>
        <Button
          className='h-12 w-full min-w-0 rounded-full bg-white px-4 text-center leading-tight whitespace-normal text-[#030409] transition-[transform,background-color,box-shadow] duration-300 hover:bg-white/88 hover:shadow-[0_14px_40px_rgba(255,255,255,0.16)] active:scale-[0.98]'
          onClick={handleImport}
        >
          <ImportIcon data-icon='inline-start' />
          <span className='min-w-0 text-balance'>{importLabel}</span>
        </Button>
      </div>
    </div>
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
    <section className='relative h-[100dvh] overflow-hidden text-white'>
      <div
        data-landing-snap-scroll
        className='h-[calc(100dvh-7.75rem-env(safe-area-inset-bottom))] overflow-y-auto overscroll-contain px-4 pt-24 pb-6 sm:h-full sm:px-6 sm:pb-[calc(9.5rem+env(safe-area-inset-bottom))] lg:pt-28'
      >
        <div className='mx-auto grid min-h-full max-w-7xl grid-cols-1 content-center gap-8 py-4 lg:grid-cols-12 lg:items-center'>
          <div className='lg:col-span-5'>
            <div className='mb-4 font-mono text-[10px] font-semibold text-white/42 uppercase'>
              {props.eyebrow}
            </div>
            <h1 className='max-w-[9em] text-4xl leading-none font-black tracking-normal text-balance text-white sm:text-5xl lg:text-7xl'>
              {props.title}
            </h1>
            <p className='mt-5 max-w-md text-sm leading-7 text-white/58 sm:text-base sm:leading-8'>
              {props.description}
            </p>
            {props.nextGuide ? (
              <div className='mt-5 flex max-w-md items-start gap-3 rounded-[1.25rem] border border-white/10 bg-white/[0.045] p-4 text-sm leading-6 text-white/66 backdrop-blur-xl'>
                <ArrowRight
                  className='mt-0.5 size-4 shrink-0 text-white/48'
                  aria-hidden='true'
                />
                <span>{props.nextGuide}</span>
              </div>
            ) : null}
          </div>
          <div className='min-h-0 lg:col-span-6 lg:col-start-7'>
            {props.children}
          </div>
        </div>
      </div>
    </section>
  )
}

function QuickStartControls(props: {
  api: LandingSnapControlsApi
  canFinish: boolean
  disabled: boolean
  reducedMotion: boolean
  onEnterDashboard: () => void
  onSkip: () => void
}) {
  const { t } = useTranslation()
  const isFinalPage = !props.api.canGoNext
  const primaryDisabled = props.disabled || (isFinalPage && !props.canFinish)
  const handlePrimary = isFinalPage ? props.onEnterDashboard : props.api.goNext
  let secondaryControl: ReactNode = null
  if (!isFinalPage) {
    secondaryControl = (
      <motion.button
        type='button'
        onClick={props.onSkip}
        disabled={props.disabled}
        whileTap={props.reducedMotion ? undefined : { scale: 0.98 }}
        transition={
          props.reducedMotion
            ? QUICK_START_REDUCED_TRANSITION
            : QUICK_START_SPRING_TRANSITION
        }
        className='rounded-full px-4 py-1 text-xs font-medium text-white/44 transition-colors hover:text-white/76 disabled:pointer-events-none disabled:opacity-30'
      >
        {t('Set up later and enter dashboard')}
      </motion.button>
    )
  } else if (!props.canFinish) {
    secondaryControl = (
      <div className='rounded-full px-4 py-1 text-xs font-medium text-white/44'>
        {t('Confirm the CC Switch import to continue')}
      </div>
    )
  }

  return (
    <div
      data-point-cloud-ignore
      className='absolute bottom-[calc(1rem+env(safe-area-inset-bottom))] left-1/2 z-30 flex w-[min(calc(100%-1.5rem),34rem)] -translate-x-1/2 flex-col items-center gap-2'
    >
      <div
        className={cn(
          'grid w-full items-center gap-1.5 rounded-full border border-white/12 bg-[#030409]/72 p-1.5 shadow-[0_20px_60px_rgba(0,0,0,0.34),inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-2xl',
          isFinalPage
            ? 'grid-cols-[3rem_auto_minmax(0,1fr)] sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]'
            : 'grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]'
        )}
      >
        <motion.button
          type='button'
          aria-label={t('Previous')}
          onClick={props.api.goPrevious}
          disabled={!props.api.canGoPrevious || props.disabled}
          whileTap={props.reducedMotion ? undefined : { scale: 0.96 }}
          transition={
            props.reducedMotion
              ? QUICK_START_REDUCED_TRANSITION
              : QUICK_START_SPRING_TRANSITION
          }
          className='flex h-12 min-w-12 items-center justify-center gap-2 justify-self-start rounded-full border border-transparent px-3 text-sm font-semibold text-white/72 transition-[border-color,background-color,color] duration-300 hover:border-white/12 hover:bg-white/[0.06] hover:text-white disabled:pointer-events-none disabled:opacity-28 sm:min-w-28'
        >
          <ArrowLeft className='size-4' aria-hidden='true' />
          <span className='hidden sm:inline'>{t('Previous')}</span>
        </motion.button>
        <div className='min-w-16 text-center font-mono text-[10px] font-semibold text-white/44 tabular-nums'>
          {String(props.api.activeIndex + 1).padStart(2, '0')} /{' '}
          {String(props.api.totalPages).padStart(2, '0')}
        </div>
        <motion.button
          type='button'
          onClick={handlePrimary}
          disabled={primaryDisabled}
          whileHover={props.reducedMotion ? undefined : { y: -2 }}
          whileTap={props.reducedMotion ? undefined : { scale: 0.97, y: 0 }}
          transition={
            props.reducedMotion
              ? QUICK_START_REDUCED_TRANSITION
              : QUICK_START_SPRING_TRANSITION
          }
          className={cn(
            'flex h-12 max-w-full min-w-32 items-center justify-center gap-2 justify-self-end rounded-full bg-white px-3 text-xs leading-tight font-black whitespace-normal text-[#030409] shadow-[0_16px_48px_rgba(255,255,255,0.2)] ring-1 ring-white/30 transition-[background-color,color,box-shadow] duration-300 hover:shadow-[0_20px_58px_rgba(255,255,255,0.28)] disabled:pointer-events-none disabled:bg-white/20 disabled:text-white/42 disabled:shadow-none sm:min-w-40 sm:px-4 sm:text-sm sm:whitespace-nowrap',
            isFinalPage && 'w-full sm:w-auto'
          )}
        >
          <span className='min-w-0 text-balance'>
            {isFinalPage ? t('Enter dashboard') : t('Next')}
          </span>
          <ArrowRight className='size-4' aria-hidden='true' />
        </motion.button>
      </div>
      {secondaryControl}
    </div>
  )
}

function Metric(props: { label: string; value: string; compact?: boolean }) {
  return (
    <div
      className={cn(
        'min-w-0 rounded-[1.25rem] border border-white/10 bg-white/[0.045] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]',
        props.compact ? 'px-3 py-2' : 'p-4'
      )}
    >
      <div className='font-mono text-[10px] font-semibold text-white/38 uppercase'>
        {props.label}
      </div>
      <div
        className={cn(
          'truncate font-semibold text-white/82',
          props.compact ? 'mt-1 text-xs' : 'mt-3 text-sm'
        )}
        title={props.value}
      >
        {props.value}
      </div>
    </div>
  )
}
