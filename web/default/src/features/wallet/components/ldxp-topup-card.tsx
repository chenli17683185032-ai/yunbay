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
import { WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import { LDXP_TOPUP_AMOUNTS } from '../lib/ldxp-topup'
import type { TopupInfo } from '../types'

interface LdxpTopupCardProps {
  topupInfo: TopupInfo | null
  loading?: boolean
  disabled?: boolean
  error?: string | null
  onStart: (amount: number) => void
}

function getVisibleLdxpAmounts(topupInfo: TopupInfo | null): number[] {
  const backendAmounts = topupInfo?.ldxp_amount_options
  if (!Array.isArray(backendAmounts) || backendAmounts.length === 0) {
    return [...LDXP_TOPUP_AMOUNTS]
  }

  const allowed = new Set(backendAmounts)
  return LDXP_TOPUP_AMOUNTS.filter((amount) => allowed.has(amount))
}

export function LdxpTopupCard({
  topupInfo,
  loading,
  disabled,
  error,
  onStart,
}: LdxpTopupCardProps) {
  const { t } = useTranslation()

  if (loading || topupInfo?.enable_ldxp_topup !== true) {
    return null
  }

  const amounts = getVisibleLdxpAmounts(topupInfo)
  if (amounts.length === 0) {
    return null
  }

  return (
    <TitledCard
      title={t('Alipay Auto Top-up')}
      description={t('Choose a fixed amount and scan the QR code to pay')}
      icon={<WalletCards className='h-4 w-4' />}
      action={<Badge variant='secondary'>{t('Alipay')}</Badge>}
      contentClassName='flex flex-col gap-3'
    >
      {error ? (
        <Alert variant='destructive'>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <div className='grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-6'>
        {amounts.map((amount) => (
          <Button
            key={amount}
            variant='outline'
            disabled={disabled}
            onClick={() => onStart(amount)}
          >
            {amount}
          </Button>
        ))}
      </div>
    </TitledCard>
  )
}
