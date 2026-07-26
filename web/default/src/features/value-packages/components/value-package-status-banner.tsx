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
import { useEffect, useMemo, useState } from 'react'
import { Clock, PauseCircle, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { getPackageLevelLabel } from '../lib/rules'
import type { ValuePackageState } from '../types'

function formatRemaining(seconds: number): string {
  if (seconds <= 0) {
    return '00:00:00'
  }

  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = seconds % 60
  const time = [hours, minutes, rest]
    .map((value) => value.toString().padStart(2, '0'))
    .join(':')

  return days > 0 ? `${days}d ${time}` : time
}

export function ValuePackageStatusBanner({
  state,
}: {
  state: ValuePackageState | null
}) {
  const { t } = useTranslation()
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000))

  useEffect(() => {
    const intervalId = window.setInterval(
      () => setNow(Math.floor(Date.now() / 1000)),
      1000
    )
    return () => window.clearInterval(intervalId)
  }, [])

  const remainingSeconds = useMemo(() => {
    const endTime = state?.subscription?.end_time || 0
    return Math.max(0, endTime - now)
  }, [now, state?.subscription?.end_time])

  if (!state?.subscription || !state.plan) {
    return null
  }

  const enabled = state.preference.enabled === true
  const packageLabel = t(getPackageLevelLabel(state.plan.package_type))

  return (
    <Alert className='border-primary/20 bg-primary/5'>
      <Sparkles className='size-4' />
      <AlertTitle className='flex flex-wrap items-center gap-2'>
        {enabled ? t('Value package is running') : t('Value package is ready')}
        <Badge variant={enabled ? 'default' : 'secondary'}>
          {packageLabel}
        </Badge>
      </AlertTitle>
      <AlertDescription className='flex flex-col gap-2'>
        <span className='flex items-center gap-2'>
          <Clock className='size-4' />
          {t('Remaining time')}: {formatRemaining(remainingSeconds)}
        </span>
        <span className='text-muted-foreground flex items-center gap-2'>
          <PauseCircle className='size-4' />
          {t('Closing package usage does not pause its countdown.')}
        </span>
      </AlertDescription>
    </Alert>
  )
}
