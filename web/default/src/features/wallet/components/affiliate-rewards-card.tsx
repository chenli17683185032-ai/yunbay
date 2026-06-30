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
import { Share2, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { CopyButton } from '@/components/copy-button'
import type { AffiliateSummary, UserWalletData } from '../types'

interface AffiliateRewardsCardProps {
  user: UserWalletData | null
  affiliateLink: string
  affiliateSummary: AffiliateSummary | null
  onTransfer: () => void
  onWithdraw: () => void
  complianceConfirmed?: boolean
  loading?: boolean
}

export function AffiliateRewardsCard(props: AffiliateRewardsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <Card className='bg-muted/20 py-0'>
        <CardContent className='grid gap-4 p-3 sm:p-4 lg:grid-cols-[minmax(220px,1fr)_minmax(220px,0.72fr)_minmax(320px,1.15fr)] lg:items-center'>
          <div>
            <Skeleton className='h-5 w-32' />
            <Skeleton className='mt-2 h-4 w-48' />
          </div>
          <Skeleton className='h-14 rounded-lg' />
          <Skeleton className='h-10 rounded-lg' />
        </CardContent>
      </Card>
    )
  }

  const hasLegacyRewards = (props.user?.aff_quota ?? 0) > 0
  const availableMoney = props.affiliateSummary?.available_money ?? 0
  const hasWithdrawalRewards = availableMoney > 0
  const inviteCount =
    props.affiliateSummary?.invite_count ?? props.user?.aff_count ?? 0
  const commissionRate = Math.round((props.affiliateSummary?.rate ?? 0) * 100)

  const monetaryStats = [
    [
      t('Available Rewards'),
      formatCurrencyFromUSD(props.affiliateSummary?.available_money ?? 0),
    ],
    [
      t('Frozen Rewards'),
      formatCurrencyFromUSD(props.affiliateSummary?.frozen_money ?? 0),
    ],
    [
      t('Withdrawn Rewards'),
      formatCurrencyFromUSD(props.affiliateSummary?.withdrawn_money ?? 0),
    ],
    [
      t('Commission Rate'),
      props.affiliateSummary ? `${commissionRate}%` : '-',
    ],
  ]

  return (
    <Card className='bg-muted/20 py-0'>
      <CardContent className='grid gap-3 p-3 sm:gap-4 sm:p-4 xl:grid-cols-[minmax(220px,0.9fr)_minmax(360px,1.15fr)_minmax(320px,1fr)] xl:items-center'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <div className='bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border'>
            <Share2 className='text-muted-foreground size-4' />
          </div>
          <div className='min-w-0'>
            <h3 className='truncate text-sm font-semibold'>
              {t('Referral Program')}
            </h3>
            <p className='text-muted-foreground line-clamp-1 text-xs'>
              {t(
                'Earn 15% monetary rewards when invited users complete paid top-ups.'
              )}
            </p>
          </div>
        </div>

        <div className='grid grid-cols-2 gap-2 text-center sm:grid-cols-4'>
          {monetaryStats.map(([label, value]) => (
            <div
              key={label}
              className='bg-background/70 rounded-lg border px-2 py-2'
            >
              <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-0.5 truncate text-sm font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className='grid grid-cols-3 gap-1.5 text-center xl:col-span-1'>
          {[
            [t('Pending'), formatQuota(props.user?.aff_quota ?? 0)],
            [t('Total Earned'), formatQuota(props.user?.aff_history_quota ?? 0)],
            [t('Invites'), String(inviteCount)],
          ].map(([label, value]) => (
            <div key={label}>
              <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-0.5 truncate text-sm font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className='flex flex-col gap-2 xl:col-span-3 xl:flex-row xl:items-center'>
          <Input
            value={props.affiliateLink}
            readOnly
            className='border-muted bg-background/70 h-9 min-w-0 flex-1 font-mono text-xs'
          />
          <div className='flex flex-wrap items-center gap-2'>
            <CopyButton
              value={props.affiliateLink}
              variant='outline'
              className='bg-background size-9 shrink-0'
              iconClassName='size-4'
              tooltip={t('Copy referral link')}
              aria-label={t('Copy referral link')}
            />
            <Badge variant='secondary' className='h-9 rounded-lg px-3'>
              <WalletCards data-icon='inline-start' />
              {t('Total Rewards')}{' '}
              {formatCurrencyFromUSD(props.affiliateSummary?.total_money ?? 0)}
            </Badge>
            <Button
              onClick={props.onWithdraw}
              disabled={!props.complianceConfirmed || !hasWithdrawalRewards}
              className='h-9 shrink-0 px-3'
              size='sm'
            >
              {t('Apply for Withdrawal')}
            </Button>
            {hasLegacyRewards ? (
              <Button
                onClick={props.onTransfer}
                disabled={!props.complianceConfirmed}
                className='h-9 shrink-0 px-3'
                size='sm'
                variant='outline'
              >
                {t('Transfer to Balance')}
              </Button>
            ) : null}
          </div>
        </div>
        {!props.complianceConfirmed ? (
          <p className='text-muted-foreground text-xs xl:col-span-3'>
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
