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
import {
  Download01Icon,
  Mail01Icon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { formatTimestampForInput, parseTimestampFromInput } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { DateRangeKey } from '../types'

interface RangeToolbarProps {
  range: DateRangeKey
  startTime?: number
  endTime?: number
  isChecking: boolean
  onRangeChange: (range: DateRangeKey) => void
  onCustomRangeChange: (startTime?: number, endTime?: number) => void
  onBatchCheck: () => void
  onExportCsv: () => void
}

function inputValueFromTimestamp(value?: number) {
  if (value === undefined || value <= 0) return ''
  return formatTimestampForInput(value)
}

function timestampFromInput(value: string) {
  const timestamp = parseTimestampFromInput(value)
  return timestamp > 0 ? timestamp : undefined
}

export function RangeToolbar({
  range,
  startTime,
  endTime,
  isChecking,
  onRangeChange,
  onCustomRangeChange,
  onBatchCheck,
  onExportCsv,
}: RangeToolbarProps) {
  const { t } = useTranslation()

  return (
    <Card size='sm'>
      <CardContent>
        <div className='flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between'>
          <div className='flex flex-col gap-3 lg:flex-row lg:items-end'>
            <div className='flex flex-col gap-1.5'>
              <div className='text-muted-foreground text-xs font-medium'>
                {t('Order analytics')}
              </div>
              <ToggleGroup
                value={[range]}
                onValueChange={(value) => {
                  const next = value[0]
                  if (next === '7d' || next === '30d' || next === 'custom') {
                    onRangeChange(next)
                  }
                }}
                variant='outline'
                size='sm'
                className='flex-wrap'
              >
                <ToggleGroupItem value='7d'>{t('Last 7 days')}</ToggleGroupItem>
                <ToggleGroupItem value='30d'>
                  {t('Last 30 days')}
                </ToggleGroupItem>
                <ToggleGroupItem value='custom'>
                  {t('Custom range')}
                </ToggleGroupItem>
              </ToggleGroup>
            </div>

            {range === 'custom' && (
              <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:w-[420px]'>
                <div className='flex flex-col gap-1.5'>
                  <label className='text-muted-foreground text-xs font-medium'>
                    {t('Start Time')}
                  </label>
                  <Input
                    type='datetime-local'
                    value={inputValueFromTimestamp(startTime)}
                    onChange={(event) =>
                      onCustomRangeChange(
                        timestampFromInput(event.target.value),
                        endTime
                      )
                    }
                  />
                </div>
                <div className='flex flex-col gap-1.5'>
                  <label className='text-muted-foreground text-xs font-medium'>
                    {t('End Time')}
                  </label>
                  <Input
                    type='datetime-local'
                    value={inputValueFromTimestamp(endTime)}
                    onChange={(event) =>
                      onCustomRangeChange(
                        startTime,
                        timestampFromInput(event.target.value)
                      )
                    }
                  />
                </div>
              </div>
            )}
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={isChecking}
              onClick={onBatchCheck}
            >
              {isChecking ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <HugeiconsIcon icon={Mail01Icon} data-icon='inline-start' />
              )}
              {t('Fetch latest mail')}
            </Button>
            <Button
              type='button'
              size='sm'
              disabled={isChecking}
              onClick={onBatchCheck}
            >
              {isChecking ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
              )}
              {t('Verify unfinished orders now')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={onExportCsv}
            >
              <HugeiconsIcon icon={Download01Icon} data-icon='inline-start' />
              {t('Export CSV')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
