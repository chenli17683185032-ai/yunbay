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
import { CrownIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { isSvipUser } from '@/features/value-packages/lib/benefit-effects'

/** 充值卡上的 SVIP 权益提示：仅在用户已达 SVIP 时展示 */
export function SvipTopupPerkAlert() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)

  if (!isSvipUser(user)) {
    return null
  }

  return (
    <Alert className='yunbay-svip-alert'>
      <div className='yunbay-svip-card-shine pointer-events-none absolute inset-0' />
      <HugeiconsIcon icon={CrownIcon} aria-hidden='true' />
      <AlertTitle className='relative'>
        {t('SVIP reached — top up now for an extra 25% off')}
      </AlertTitle>
      <AlertDescription className='relative text-xs'>
        {t(
          'After you top up, the admin will verify it and credit the discount to your account.'
        )}
      </AlertDescription>
    </Alert>
  )
}
