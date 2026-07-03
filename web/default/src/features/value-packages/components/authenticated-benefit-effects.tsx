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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Crown, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import {
  getValuePackageSelf,
  markVipUpgradeModalSeen,
} from '../api'
import {
  getBenefitGlowMode,
  isVipUserGroup,
  shouldShowVipCelebration,
  withVipUpgradeModalSeen,
  type BenefitGlowMode,
} from '../lib/benefit-effects'
import { shouldShowPackageGlow } from '../lib/rules'
import { valuePackageSelfQueryKey } from '../query-keys'

function currentUnixSeconds(): number {
  return Math.floor(Date.now() / 1000)
}

function useNowSeconds(enabled: boolean): number {
  const [now, setNow] = useState(currentUnixSeconds)

  useEffect(() => {
    if (!enabled) {
      return
    }

    const intervalId = window.setInterval(
      () => setNow(currentUnixSeconds()),
      1000
    )
    return () => window.clearInterval(intervalId)
  }, [enabled])

  return now
}

function ViewportBenefitGlow({ mode }: { mode: BenefitGlowMode }) {
  if (mode === 'none') {
    return null
  }

  return (
    <div
      aria-hidden='true'
      className={cn(
        'yunbay-viewport-benefit-glow',
        mode === 'package' && 'yunbay-viewport-benefit-glow--package',
        mode === 'vip' && 'yunbay-viewport-benefit-glow--vip'
      )}
    />
  )
}

export function AuthenticatedBenefitEffects() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const setUser = useAuthStore((state) => state.auth.setUser)
  const [vipDialogDismissedUserId, setVipDialogDismissedUserId] = useState<
    number | null
  >(null)
  const markSeenInFlightUserIdRef = useRef<number | null>(null)

  const { data: valuePackageState } = useQuery({
    queryKey: valuePackageSelfQueryKey,
    enabled: Boolean(user),
    staleTime: 10_000,
    refetchOnWindowFocus: true,
    queryFn: async () => {
      const response = await getValuePackageSelf()
      return response.success ? response.data || null : null
    },
  })

  const isVipUser = isVipUserGroup(user?.group)
  const now = useNowSeconds(Boolean(valuePackageState))
  const packageGlow = shouldShowPackageGlow(valuePackageState || null, now)
  const mode = getBenefitGlowMode({ packageGlow, isVipUser })
  const showVipDialog =
    vipDialogDismissedUserId !== user?.id &&
    shouldShowVipCelebration({
      group: user?.group,
      setting: user?.setting,
    })

  const markVipSeen = useCallback(() => {
    if (!user || markSeenInFlightUserIdRef.current === user.id) {
      return
    }

    markSeenInFlightUserIdRef.current = user.id
    setVipDialogDismissedUserId(user.id)
    setUser({
      ...user,
      setting: withVipUpgradeModalSeen(user.setting),
    })

    void markVipUpgradeModalSeen().catch(() => {
      if (markSeenInFlightUserIdRef.current === user.id) {
        markSeenInFlightUserIdRef.current = null
      }
    })
  }, [setUser, user])

  return (
    <>
      <ViewportBenefitGlow mode={mode} />
      <Dialog
        open={showVipDialog}
        onOpenChange={(open) => {
          if (!open) {
            markVipSeen()
          }
        }}
        title={t('VIP upgrade celebration')}
        description={t('VIP membership benefits are now active.')}
        contentClassName='overflow-hidden border-0 p-0 sm:max-w-[460px]'
        headerClassName='sr-only'
        bodyClassName='p-0'
        showCloseButton
      >
        <div className='from-card via-card to-muted relative overflow-hidden bg-linear-to-br p-6 sm:p-7'>
          <div className='yunbay-vip-card-shine pointer-events-none absolute inset-0' />
          <div className='border-warning/30 bg-background/70 relative overflow-hidden rounded-3xl border p-5 shadow-2xl backdrop-blur'>
            <div className='from-warning/25 via-primary/10 absolute inset-x-0 top-0 h-24 bg-linear-to-b to-transparent' />
            <div className='relative flex flex-col gap-5'>
              <div className='flex items-start justify-between gap-4'>
                <div className='flex items-center gap-3'>
                  <div className='bg-warning/15 text-warning flex size-12 items-center justify-center rounded-2xl'>
                    <Crown className='size-6' />
                  </div>
                  <div>
                    <div className='text-muted-foreground text-xs font-semibold tracking-[0.28em] uppercase'>
                      VIP
                    </div>
                    <div className='text-lg font-black'>
                      {t('Yunbei VIP Card')}
                    </div>
                  </div>
                </div>
                <Sparkles className='text-warning size-5' />
              </div>

              <div className='flex flex-col gap-2 py-2'>
                <div className='text-2xl leading-tight font-black tracking-tight sm:text-3xl'>
                  {t('恭喜你获得会员权益')}
                </div>
                <p className='text-muted-foreground text-sm leading-relaxed'>
                  {t(
                    'Your valid payments have unlocked VIP benefits. The exclusive VIP border glow will stay active for your account.'
                  )}
                </p>
              </div>

              <div className='grid grid-cols-3 gap-2 text-center text-xs'>
                <div className='bg-muted/70 rounded-2xl p-3'>
                  <div className='font-semibold'>{t('VIP group')}</div>
                  <div className='text-muted-foreground mt-1'>
                    {t('Active')}
                  </div>
                </div>
                <div className='bg-muted/70 rounded-2xl p-3'>
                  <div className='font-semibold'>{t('Plus rate')}</div>
                  <div className='text-muted-foreground mt-1'>
                    {t('Unlocked')}
                  </div>
                </div>
                <div className='bg-muted/70 rounded-2xl p-3'>
                  <div className='font-semibold'>{t('VIP glow')}</div>
                  <div className='text-muted-foreground mt-1'>
                    {t('Always on')}
                  </div>
                </div>
              </div>

              <Button className='w-full' onClick={markVipSeen}>
                <Sparkles data-icon='inline-start' />
                {t('Start VIP experience')}
              </Button>
            </div>
          </div>
        </div>
      </Dialog>
    </>
  )
}
