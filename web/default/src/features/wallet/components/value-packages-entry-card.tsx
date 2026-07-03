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
import { Link } from '@tanstack/react-router'
import { ArrowRight, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'

export function ValuePackagesEntryCard() {
  const { t } = useTranslation()

  return (
    <TitledCard
      title={t('Value Packages')}
      description={t(
        'Daily, weekly, and monthly cards are now managed on a dedicated page.'
      )}
      icon={<Sparkles className='size-4' />}
      action={
        <Button
          render={<Link to='/value-packages' />}
          className='w-full sm:w-auto'
        >
          {t('View value packages')}
          <ArrowRight data-icon='inline-end' />
        </Button>
      }
      className='border-primary/15 from-primary/10 bg-linear-to-br via-transparent to-transparent'
      iconClassName='bg-primary/10 text-primary ring-1 ring-primary/15'
    >
      <p className='text-muted-foreground text-sm leading-relaxed'>
        {t(
          'Start with package cards first, then return here for API balance top-up, redemption codes, and invitations.'
        )}
      </p>
    </TitledCard>
  )
}
