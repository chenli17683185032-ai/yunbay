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
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { AlertTriangle } from 'lucide-react'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { RedemptionsDialogs } from './components/redemptions-dialogs'
import { RedemptionsPrimaryButtons } from './components/redemptions-primary-buttons'
import {
  RedemptionsProvider,
  useRedemptions,
} from './components/redemptions-provider'
import { RedemptionsTable } from './components/redemptions-table'

function RedemptionComplianceNotice() {
  const { t } = useTranslation()
  const { complianceConfirmed } = useRedemptions()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const canOpenPaymentSettings = userRole === ROLE.SUPER_ADMIN

  if (complianceConfirmed) return null

  return (
    <Alert className='mb-4 border-amber-500/30 bg-amber-500/10'>
      <AlertTriangle className='h-4 w-4' />
      <AlertTitle>{t('Redemption code creation is not enabled')}</AlertTitle>
      <AlertDescription className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <span>
          {canOpenPaymentSettings
            ? t(
                'Confirm compliance terms in Payment Gateway settings before creating redemption codes.'
              )
            : t(
                'Contact the root administrator to confirm compliance terms in Payment Gateway settings before creating redemption codes.'
              )}
        </span>
        {canOpenPaymentSettings ? (
          <Button
            size='sm'
            variant='outline'
            render={
              <Link
                to='/system-settings/billing/$section'
                params={{ section: 'payment' }}
              />
            }
          >
            {t('Go to Payment Gateway settings')}
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  )
}

export function Redemptions() {
  const { t } = useTranslation()
  return (
    <RedemptionsProvider>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Redemption Codes')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <RedemptionsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <RedemptionComplianceNotice />
          <RedemptionsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <RedemptionsDialogs />
    </RedemptionsProvider>
  )
}
