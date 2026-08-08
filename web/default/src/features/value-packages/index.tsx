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
import { useState } from 'react'
import { RefreshCw, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { ResetCardGiftDialog } from '@/components/reset-card-gift-dialog'
import { RechargeChannelNoticeDialog } from '@/features/wallet/components/recharge-channel-notice-dialog'
import { useRedemption } from '@/features/wallet/hooks/use-redemption'
import { ValuePackageCard } from './components/value-package-card'
import { ValuePackagePaymentDialog } from './components/value-package-payment-dialog'
import { ValuePackageStatusBanner } from './components/value-package-status-banner'
import { useValuePackages } from './hooks/use-value-packages'

export function ValuePackages() {
  const { t } = useTranslation()
  const valuePackages = useValuePackages()
  const {
    redeeming,
    redeemCode,
    giftCelebration: redeemGiftCelebration,
    clearGiftCelebration: clearRedeemGiftCelebration,
  } = useRedemption()
  const [redemptionCodes, setRedemptionCodes] = useState<
    Record<number, string>
  >({})
  const [rechargeChannelNoticeOpen, setRechargeChannelNoticeOpen] =
    useState(false)

  const handleRedemptionCodeChange = (planId: number, code: string) => {
    setRedemptionCodes((previous) => ({ ...previous, [planId]: code }))
  }

  const handleRedeemCode = async (planId: number) => {
    const code = redemptionCodes[planId]?.trim() || ''
    const redeemed = await redeemCode(code)
    if (!redeemed) return
    setRedemptionCodes((previous) => ({ ...previous, [planId]: '' }))
    await valuePackages.refresh()
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Value Packages')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            size='sm'
            disabled={valuePackages.refreshing}
            onClick={() => void valuePackages.refresh()}
          >
            <RefreshCw
              data-icon='inline-start'
              className={valuePackages.refreshing ? 'animate-spin' : undefined}
            />
            {t('Refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <div className='border-primary/10 from-primary/10 rounded-2xl border bg-linear-to-br via-transparent to-transparent p-4 sm:p-6'>
              <div className='flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between'>
                <div className='max-w-2xl'>
                  <div className='text-primary mb-2 flex items-center gap-2 text-sm font-semibold'>
                    <Sparkles className='size-4' />
                    {t('Value Packages')}
                  </div>
                  <h1 className='text-2xl font-black tracking-tight sm:text-3xl'>
                    {t(
                      'Daily, weekly, and monthly cards for dedicated model groups'
                    )}
                  </h1>
                  <p className='text-muted-foreground mt-2 text-sm leading-relaxed sm:text-base'>
                    {t(
                      'Buy a package, then press start to switch to its model group. Closing usage switches back to API balance, but the package countdown continues.'
                    )}
                  </p>
                </div>
              </div>
            </div>

            <ValuePackageStatusBanner state={valuePackages.state} />

            {valuePackages.error ? (
              <Alert variant='destructive'>
                <AlertDescription>{valuePackages.error}</AlertDescription>
              </Alert>
            ) : null}

            {valuePackages.loading ? (
              <div className='grid gap-4 lg:grid-cols-3'>
                {Array.from({ length: 3 }).map((_, index) => (
                  <Skeleton key={index} className='h-[520px] rounded-xl' />
                ))}
              </div>
            ) : valuePackages.plans.length > 0 ? (
              <div className='grid gap-4 lg:grid-cols-3'>
                {valuePackages.plans.map((plan) => (
                  <ValuePackageCard
                    key={plan.id}
                    plan={plan}
                    state={valuePackages.state}
                    actionKey={valuePackages.actionKey}
                    redemptionCode={redemptionCodes[plan.id] || ''}
                    redeeming={redeeming}
                    onRedemptionCodeChange={handleRedemptionCodeChange}
                    onRedeemCode={handleRedeemCode}
                    onPurchase={() => setRechargeChannelNoticeOpen(true)}
                    onActivate={valuePackages.activate}
                    onDeactivate={valuePackages.deactivate}
                    onResetQuota={valuePackages.resetQuota}
                  />
                ))}
              </div>
            ) : (
              <Empty className='bg-card'>
                <EmptyHeader>
                  <EmptyMedia variant='icon'>
                    <Sparkles className='size-4' />
                  </EmptyMedia>
                  <EmptyTitle>{t('No value packages available')}</EmptyTitle>
                  <EmptyDescription>
                    {t(
                      'Please contact the administrator to configure value package cards.'
                    )}
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <RechargeChannelNoticeDialog
        open={rechargeChannelNoticeOpen}
        onOpenChange={setRechargeChannelNoticeOpen}
      />

      <ValuePackagePaymentDialog
        sessionResponse={valuePackages.paymentSession}
        loading={valuePackages.paymentLoading}
        error={valuePackages.paymentError}
        onCancel={valuePackages.cancelPayment}
        onClose={valuePackages.resetPayment}
      />

      <ResetCardGiftDialog
        celebration={redeemGiftCelebration ?? valuePackages.giftCelebration}
        onClose={() => {
          clearRedeemGiftCelebration()
          valuePackages.clearGiftCelebration()
        }}
      />
    </>
  )
}
