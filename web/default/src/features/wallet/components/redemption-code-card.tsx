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
import { ExternalLink, Gift, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TitledCard } from '@/components/ui/titled-card'

interface RedemptionCodeCardProps {
  redemptionCode: string
  onRedemptionCodeChange: (code: string) => void
  onRedeem: () => void
  redeeming: boolean
  topupLink?: string
  redemptionEnabled?: boolean
}

export function RedemptionCodeCard({
  redemptionCode,
  onRedemptionCodeChange,
  onRedeem,
  redeeming,
  topupLink,
  redemptionEnabled = true,
}: RedemptionCodeCardProps) {
  const { t } = useTranslation()

  return (
    <TitledCard
      title={t('Redemption code')}
      description={t('Redeem balance with a code or card key.')}
      icon={<Gift className='h-4 w-4' />}
      iconClassName='bg-primary/10 text-primary ring-1 ring-primary/15'
      contentClassName='flex flex-col gap-3'
    >
      <div id='wallet-redemption-code' className='scroll-mt-6'>
        {redemptionEnabled ? (
          <div className='flex flex-col gap-3'>
            <Label
              htmlFor='redemption-code'
              className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
            >
              {t('Have a code or card?')}
            </Label>
            <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
              <Input
                id='redemption-code'
                value={redemptionCode}
                onChange={(e) => onRedemptionCodeChange(e.target.value)}
                placeholder={t('Enter your redemption code or card key')}
                className='h-10 min-w-0'
              />
              <Button
                onClick={onRedeem}
                disabled={redeeming}
                variant='outline'
                className='h-10 px-4'
              >
                {redeeming && (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                )}
                {t('Redeem')}
              </Button>
            </div>
            {topupLink ? (
              <p className='text-muted-foreground text-xs'>
                {t('Need a redemption code or card?')}{' '}
                <a
                  href={topupLink}
                  target='_blank'
                  rel='noopener noreferrer'
                  className='inline-flex items-center gap-1 underline-offset-4 hover:underline'
                >
                  {t('Get one here')}
                  <ExternalLink className='h-3 w-3' />
                </a>
              </p>
            ) : null}
          </div>
        ) : (
          <Alert>
            <AlertDescription>
              {t(
                'Redemption codes are disabled until the administrator confirms compliance terms.'
              )}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </TitledCard>
  )
}
