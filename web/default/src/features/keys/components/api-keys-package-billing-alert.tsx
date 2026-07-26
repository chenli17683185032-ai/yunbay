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
import { useQuery } from '@tanstack/react-query'
import { Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { GroupBadge } from '@/components/group-badge'
import { getValuePackageSelf } from '@/features/value-packages/api'
import { getPackageLevelLabel } from '@/features/value-packages/lib/rules'
import { valuePackageSelfQueryKey } from '@/features/value-packages/query-keys'

export function ApiKeysPackageBillingAlert() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const userGroup = user?.group?.trim() || t('original billing group')

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

  const currentPlan = valuePackageState?.plan
  const currentSubscription = valuePackageState?.subscription
  const preference = valuePackageState?.preference
  const currentPackageLabel = t(getPackageLevelLabel(currentPlan?.package_type))
  const currentModelGroup = currentPlan
    ? currentPlan.model_group?.trim() || ''
    : ''
  const isActivePackageBilling = Boolean(
    preference &&
    preference.enabled &&
    currentPlan &&
    currentSubscription &&
    currentModelGroup
  )

  if (!isActivePackageBilling) {
    return null
  }

  return (
    <Alert className='border-info/25 bg-info/5 text-info'>
      <Info className='size-4' />
      <AlertTitle className='text-foreground'>
        {t('Package billing is active')}
      </AlertTitle>
      <AlertDescription className='text-foreground/80 flex flex-col gap-2 text-sm'>
        <span>
          {t('API requests are currently billed through {{packageName}}.', {
            packageName: currentPackageLabel,
          })}{' '}
          <span className='inline-flex align-middle'>
            <GroupBadge group={currentModelGroup} />
          </span>
        </span>
        <span>
          {t(
            'Personal profile shows your user group; this page shows the effective API billing/model group.'
          )}
        </span>
        <span>
          {t(
            'When you close package usage, API keys return to {{group}} billing.',
            { group: userGroup }
          )}
        </span>
      </AlertDescription>
    </Alert>
  )
}
