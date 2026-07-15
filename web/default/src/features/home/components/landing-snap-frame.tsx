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
  Children,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentType,
  type ReactNode,
} from 'react'
import { flushSync } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  LANDING_SECTION_IDS,
  getNextPageIndex,
  getSectionHash,
  normalizeSectionHash,
  type PageDirection,
} from '../landing-page-snap'

type LandingSnapFrameProps = {
  children: ReactNode
  className?: string
  sectionIds?: readonly string[]
  navigateEventName?: string
  showControls?: boolean
  allowContentScroll?: boolean
  onActiveIndexChange?: (activeIndex: number, previousIndex: number) => void
  controlsComponent?: ComponentType<LandingSnapControlsApi>
}

type LandingNavigateEvent = CustomEvent<{
  hash: string
}>

export type LandingSnapControlsApi = {
  activeIndex: number
  totalPages: number
  canGoPrevious: boolean
  canGoNext: boolean
  goPrevious: () => void
  goNext: () => void
  goToIndex: (index: number) => void
}

const PAGE_TRANSITION_LOCK_MS = 980
const TOUCH_PAGE_STEP_THRESHOLD = 44
const WHEEL_PAGE_STEP_THRESHOLD = 16
const DEFAULT_NAVIGATE_EVENT = 'public-landing:navigate'

function useReducedMotionPreference() {
  const [reduced, setReduced] = useState(false)

  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = () => setReduced(query.matches)
    update()
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [])

  return reduced
}

function clampLandingIndex(index: number, totalPages: number): number {
  return Math.max(0, Math.min(Math.max(totalPages - 1, 0), index))
}

function getInitialLandingIndex(sectionIds: readonly string[]): number {
  if (typeof window === 'undefined') return 0
  return normalizeSectionHash(sectionIds, window.location.href) ?? 0
}

function getLandingScrollContainer(
  target: EventTarget | null,
  root: HTMLDivElement
): HTMLElement | null {
  if (!(target instanceof Element)) return null
  const container = target.closest<HTMLElement>('[data-landing-snap-scroll]')
  return container && root.contains(container) ? container : null
}

function canScrollLandingContent(
  element: HTMLElement,
  deltaY: number
): boolean {
  const maxScrollTop = element.scrollHeight - element.clientHeight
  if (maxScrollTop <= 1) return false
  if (deltaY > 0) return element.scrollTop < maxScrollTop - 1
  if (deltaY < 0) return element.scrollTop > 1
  return false
}

function isInteractiveLandingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false
  return Boolean(
    target.closest(
      'a, button, input, textarea, select, [contenteditable="true"], [role="button"]'
    )
  )
}

export function LandingSnapFrame(props: LandingSnapFrameProps) {
  const { t } = useTranslation()
  const sectionIds = props.sectionIds ?? LANDING_SECTION_IDS
  const navigateEventName = props.navigateEventName ?? DEFAULT_NAVIGATE_EVENT
  const onActiveIndexChange = props.onActiveIndexChange
  const ControlsComponent = props.controlsComponent
  const showControls = props.showControls !== false
  const reducedMotion = useReducedMotionPreference()
  const pages = Children.toArray(props.children)
  const totalPages = pages.length
  const [activeIndex, setActiveIndex] = useState(() =>
    getInitialLandingIndex(sectionIds)
  )
  const rootRef = useRef<HTMLDivElement>(null)
  const activeIndexRef = useRef(activeIndex)
  const transitionLockedRef = useRef(false)
  const transitionTimerRef = useRef<number | null>(null)
  const touchStartYRef = useRef<number | null>(null)
  const touchScrollStateRef = useRef<{
    element: HTMLElement
    scrollTop: number
  } | null>(null)

  const navigateToIndex = useCallback(
    (
      nextIndex: number,
      options: { replace?: boolean; updateUrl?: boolean } = {}
    ) => {
      const previousIndex = activeIndexRef.current
      const clampedIndex = clampLandingIndex(nextIndex, totalPages)
      if (previousIndex === clampedIndex) return
      if (rootRef.current) {
        rootRef.current.scrollTop = 0
      }
      activeIndexRef.current = clampedIndex
      flushSync(() => {
        setActiveIndex(clampedIndex)
      })
      onActiveIndexChange?.(clampedIndex, previousIndex)

      if (options.updateUrl === false) return

      const nextHash = getSectionHash(sectionIds, clampedIndex)
      if (window.location.hash === nextHash) return

      if (options.replace) {
        window.history.replaceState(null, '', nextHash)
      } else {
        window.history.pushState(null, '', nextHash)
      }
    },
    [onActiveIndexChange, sectionIds, totalPages]
  )

  const lockTransition = useCallback(() => {
    if (transitionTimerRef.current !== null) {
      window.clearTimeout(transitionTimerRef.current)
    }
    transitionLockedRef.current = true
    transitionTimerRef.current = window.setTimeout(
      () => {
        transitionLockedRef.current = false
        transitionTimerRef.current = null
      },
      reducedMotion ? 120 : PAGE_TRANSITION_LOCK_MS
    )
  }, [reducedMotion])

  const stepByDelta = useCallback(
    (deltaY: number) => {
      if (Math.abs(deltaY) < WHEEL_PAGE_STEP_THRESHOLD) return
      const nextIndex = getNextPageIndex(
        activeIndexRef.current,
        deltaY > 0 ? 'next' : 'previous',
        totalPages
      )
      if (nextIndex === activeIndexRef.current) return
      lockTransition()
      navigateToIndex(nextIndex)
    },
    [lockTransition, navigateToIndex, totalPages]
  )

  const stepByDirection = useCallback(
    (direction: PageDirection) => {
      const nextIndex = getNextPageIndex(
        activeIndexRef.current,
        direction,
        totalPages
      )
      if (nextIndex === activeIndexRef.current) return
      lockTransition()
      navigateToIndex(nextIndex)
    },
    [lockTransition, navigateToIndex, totalPages]
  )

  useEffect(() => {
    const syncFromLocation = () => {
      const nextIndex = normalizeSectionHash(sectionIds, window.location.href)
      if (nextIndex !== null) {
        navigateToIndex(nextIndex, { updateUrl: false })
      }
    }

    const syncFromNavigationEvent = (event: Event) => {
      const nextIndex = normalizeSectionHash(
        sectionIds,
        (event as LandingNavigateEvent).detail.hash
      )
      if (nextIndex !== null) {
        navigateToIndex(nextIndex)
      }
    }

    const initialSyncId = window.setTimeout(syncFromLocation, 0)

    const onKeyDown = (event: KeyboardEvent) => {
      if (isInteractiveLandingTarget(event.target)) return
      if (
        event.key !== 'ArrowDown' &&
        event.key !== 'PageDown' &&
        event.key !== 'ArrowRight' &&
        event.key !== ' ' &&
        event.key !== 'ArrowUp' &&
        event.key !== 'PageUp' &&
        event.key !== 'ArrowLeft'
      ) {
        return
      }

      event.preventDefault()
      if (transitionLockedRef.current) return
      stepByDirection(
        event.key === 'ArrowDown' ||
          event.key === 'PageDown' ||
          event.key === 'ArrowRight' ||
          event.key === ' '
          ? 'next'
          : 'previous'
      )
    }

    window.addEventListener('hashchange', syncFromLocation)
    window.addEventListener('popstate', syncFromLocation)
    window.addEventListener(navigateEventName, syncFromNavigationEvent)
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('hashchange', syncFromLocation)
      window.removeEventListener('popstate', syncFromLocation)
      window.removeEventListener(navigateEventName, syncFromNavigationEvent)
      window.removeEventListener('keydown', onKeyDown)
      window.clearTimeout(initialSyncId)
      if (transitionTimerRef.current !== null) {
        window.clearTimeout(transitionTimerRef.current)
      }
    }
  }, [navigateEventName, navigateToIndex, sectionIds, stepByDirection])

  const goPrevious = useCallback(
    () => stepByDirection('previous'),
    [stepByDirection]
  )
  const goNext = useCallback(() => stepByDirection('next'), [stepByDirection])
  const canGoPrevious = activeIndex > 0
  const canGoNext = activeIndex < totalPages - 1
  const controlsApi = useMemo<LandingSnapControlsApi>(
    () => ({
      activeIndex,
      totalPages,
      canGoPrevious,
      canGoNext,
      goPrevious,
      goNext,
      goToIndex: navigateToIndex,
    }),
    [
      activeIndex,
      canGoNext,
      canGoPrevious,
      goNext,
      goPrevious,
      navigateToIndex,
      totalPages,
    ]
  )

  return (
    <div
      ref={rootRef}
      data-landing-snap-root
      className={cn(
        'relative h-[100dvh] overflow-hidden focus:outline-none',
        props.className
      )}
      onScroll={(event) => {
        event.currentTarget.scrollTop = 0
      }}
      onWheelCapture={(event) => {
        const scrollContainer = props.allowContentScroll
          ? getLandingScrollContainer(event.target, event.currentTarget)
          : null
        if (
          scrollContainer &&
          canScrollLandingContent(scrollContainer, event.deltaY)
        ) {
          return
        }
        event.preventDefault()
        event.currentTarget.scrollTop = 0
        if (transitionLockedRef.current) return
        stepByDelta(event.deltaY)
      }}
      onTouchStart={(event) => {
        touchStartYRef.current = event.touches[0]?.clientY ?? null
        const scrollContainer = props.allowContentScroll
          ? getLandingScrollContainer(event.target, event.currentTarget)
          : null
        touchScrollStateRef.current = scrollContainer
          ? { element: scrollContainer, scrollTop: scrollContainer.scrollTop }
          : null
      }}
      onTouchEnd={(event) => {
        const startY = touchStartYRef.current
        touchStartYRef.current = null
        const touchScrollState = touchScrollStateRef.current
        touchScrollStateRef.current = null
        const endY = event.changedTouches[0]?.clientY
        if (startY == null || endY == null) return
        const deltaY = startY - endY
        if (Math.abs(deltaY) < TOUCH_PAGE_STEP_THRESHOLD) return
        if (touchScrollState) {
          const maxScrollTop =
            touchScrollState.element.scrollHeight -
            touchScrollState.element.clientHeight
          if (
            (deltaY > 0 && touchScrollState.scrollTop < maxScrollTop - 1) ||
            (deltaY < 0 && touchScrollState.scrollTop > 1)
          ) {
            return
          }
        }
        if (transitionLockedRef.current) return
        stepByDelta(deltaY)
      }}
      tabIndex={-1}
    >
      <div
        className={cn(
          'absolute inset-0 h-[100dvh] will-change-transform',
          reducedMotion
            ? 'transition-none'
            : 'transition-transform duration-[900ms] ease-[cubic-bezier(0.16,1,0.3,1)]'
        )}
        style={{
          transform: `translate3d(0, -${activeIndex * 100}dvh, 0)`,
        }}
      >
        {pages.map((page, index) => (
          <div
            key={index}
            aria-hidden={index !== activeIndex}
            className='absolute inset-0'
            style={{
              transform: `translate3d(0, ${index * 100}dvh, 0)`,
            }}
          >
            {page}
          </div>
        ))}
      </div>
      {showControls &&
        (ControlsComponent ? (
          <ControlsComponent {...controlsApi} />
        ) : (
          <div
            data-point-cloud-ignore
            className='absolute right-4 bottom-5 left-4 z-20 flex items-center justify-between gap-3 sm:right-6 sm:left-auto'
          >
            <button
              type='button'
              onClick={goPrevious}
              disabled={!canGoPrevious}
              className='h-10 rounded-full border border-white/12 bg-[#030409]/58 px-4 text-xs font-semibold text-white/72 backdrop-blur-xl transition-all duration-300 hover:border-white/24 hover:text-white active:scale-[0.98] disabled:pointer-events-none disabled:opacity-35'
            >
              {t('Previous')}
            </button>
            <div className='rounded-full border border-white/10 bg-[#030409]/58 px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.16em] text-white/48 backdrop-blur-xl'>
              {String(activeIndex + 1).padStart(2, '0')} /{' '}
              {String(totalPages).padStart(2, '0')}
            </div>
            <button
              type='button'
              onClick={goNext}
              disabled={!canGoNext}
              className='h-10 rounded-full bg-white px-4 text-xs font-semibold text-[#030409] shadow-[0_18px_50px_rgba(255,255,255,0.14)] transition-all duration-300 hover:bg-white/88 active:scale-[0.98] disabled:pointer-events-none disabled:opacity-35'
            >
              {t('Next')}
            </button>
          </div>
        ))}
    </div>
  )
}
