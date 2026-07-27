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
import { Receipt, ShieldCheck, Sparkles, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import {
  LDXP_TOPUP_AMOUNTS,
  getLdxpDiscountLabel,
  getLdxpPricing,
} from '../lib/ldxp-topup'
import type { TopupInfo } from '../types'
import { SvipTopupPerkAlert } from './svip-topup-perk-alert'

interface LdxpTopupCardProps {
  topupInfo: TopupInfo | null
  loading?: boolean
  disabled?: boolean
  error?: string | null
  onStart: (amount: number) => void
  onOpenBilling?: () => void
}

function getVisibleLdxpAmounts(topupInfo: TopupInfo | null): number[] {
  const backendAmounts = topupInfo?.ldxp_amount_options
  if (!Array.isArray(backendAmounts) || backendAmounts.length === 0) {
    return [...LDXP_TOPUP_AMOUNTS]
  }

  const allowed = new Set(backendAmounts)
  return LDXP_TOPUP_AMOUNTS.filter((amount) => allowed.has(amount))
}

function formatLdxpMoney(amount: number): string {
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: Number.isInteger(amount) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(amount)
}

export function LdxpTopupCard({
  topupInfo,
  loading,
  disabled,
  error,
  onStart,
  onOpenBilling,
}: LdxpTopupCardProps) {
  const { t, i18n } = useTranslation()

  if (loading || topupInfo?.enable_ldxp_topup !== true) {
    return null
  }

  const amounts = getVisibleLdxpAmounts(topupInfo)
  if (amounts.length === 0) {
    return null
  }

  return (
    <TitledCard
      title={t('Top up now')}
      description={t(
        'Choose an amount, scan the QR code, and funds will arrive automatically.'
      )}
      icon={<WalletCards className='h-4 w-4' />}
      action={
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end'>
          <Badge variant='secondary' className='justify-center gap-1.5'>
            <Sparkles className='size-3' />
            {t('Alipay QR payment')}
          </Badge>
          {onOpenBilling ? (
            <Button
              variant='outline'
              size='sm'
              onClick={onOpenBilling}
              className='w-full gap-2 sm:w-auto'
            >
              <Receipt className='h-4 w-4' />
              {t('Order History')}
            </Button>
          ) : null}
        </div>
      }
      className='border-primary/10 bg-[radial-gradient(circle_at_10%_0%,color-mix(in_oklch,var(--primary)_10%,transparent),transparent_34%),var(--card)]'
      iconClassName='bg-primary/10 text-primary ring-1 ring-primary/15'
      contentClassName='flex flex-col gap-4'
    >
      {error ? (
        <Alert variant='destructive'>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <SvipTopupPerkAlert />

      <div className='grid grid-cols-2 gap-2.5 sm:grid-cols-3 xl:grid-cols-6'>
        {amounts.map((amount) => {
          const pricing = getLdxpPricing(amount)
          const discountLabel = getLdxpDiscountLabel(
            pricing.discount,
            i18n.language
          )

          return (
            <Button
              key={amount}
              variant='outline'
              disabled={disabled}
              onClick={() => onStart(amount)}
              className='group border-primary/12 bg-background/70 hover:border-primary/28 hover:bg-primary/[0.045] relative h-auto min-h-32 overflow-hidden rounded-2xl px-3 py-4 text-left shadow-[inset_0_1px_0_color-mix(in_oklch,var(--primary)_12%,transparent)] transition-all duration-300 hover:-translate-y-0.5 hover:shadow-[0_18px_50px_color-mix(in_oklch,var(--primary)_12%,transparent)] active:scale-[0.98] disabled:hover:translate-y-0 sm:min-h-36 sm:px-4'
            >
              <span className='via-primary/35 pointer-events-none absolute inset-x-4 top-0 h-px bg-gradient-to-r from-transparent to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100' />
              <span className='flex w-full flex-col items-start gap-3'>
                <span className='flex w-full items-start justify-between gap-2'>
                  <span className='text-2xl leading-none font-black tracking-[-0.04em] tabular-nums sm:text-3xl'>
                    ¥{formatLdxpMoney(pricing.payable)}
                  </span>
                  <Badge
                    variant={pricing.hasDiscount ? 'default' : 'secondary'}
                    className='shrink-0 rounded-full px-2 text-[10px] font-black tracking-[0.08em] uppercase'
                  >
                    {discountLabel}
                  </Badge>
                </span>
                <span className='flex min-h-10 flex-col gap-1 text-xs'>
                  {pricing.hasDiscount ? (
                    <>
                      <span className='text-muted-foreground'>
                        {t('Original price')}{' '}
                        <span className='line-through'>
                          ¥{formatLdxpMoney(pricing.amount)}
                        </span>
                      </span>
                      <span className='text-primary font-semibold'>
                        {t('Saved {{amount}}', {
                          amount: `¥${formatLdxpMoney(pricing.saved)}`,
                        })}
                      </span>
                    </>
                  ) : (
                    <span className='text-muted-foreground'>
                      {t('Standard price')}
                    </span>
                  )}
                </span>
                <span className='text-muted-foreground flex items-center gap-1.5 text-[11px] leading-4'>
                  <ShieldCheck className='size-3.5 shrink-0' />
                  {t('Payment platform fees are borne by the user.')}
                </span>
              </span>
            </Button>
          )
        })}
      </div>
    </TitledCard>
  )
}
