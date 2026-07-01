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
import { Badge } from '@/components/ui/badge'
import { getMailStatusLabelKey, isMailStatusError } from '../lib/format'
import type { MailCheckStatus } from '../types'

export function MailCheckStatusBadge({
  status,
}: {
  status: MailCheckStatus | string
}) {
  const { t } = useTranslation()
  const label = t(getMailStatusLabelKey(status))

  if (status === 'verified') {
    return <Badge variant='default'>{label}</Badge>
  }

  if (status === 'checking') {
    return <Badge variant='secondary'>{label}</Badge>
  }

  if (isMailStatusError(status)) {
    return <Badge variant='destructive'>{label}</Badge>
  }

  return <Badge variant='outline'>{label}</Badge>
}
