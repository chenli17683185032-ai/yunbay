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
  ArrowRight01Icon,
  CheckmarkCircle02Icon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import {
  QUICK_START_REASONING_EFFORT_LABEL_KEY,
  getQuickStartModelDisplayName,
  isPreferredQuickStartModel,
} from './quick-start-data'
import type { QuickStartSessionState } from './quick-start-session'

type QuickStartCompletionDialogProps = {
  open: boolean
  session: QuickStartSessionState
  onReview: () => void
  onStart: () => void
}

type CompletionDetail = {
  label: string
  value: string
}

export function QuickStartCompletionDialog(
  props: QuickStartCompletionDialogProps
) {
  const { t } = useTranslation()
  const modelName = props.session.modelName
    ? t(getQuickStartModelDisplayName(props.session.modelName))
    : t('Selected')
  const platform = props.session.platform || t('Current device')
  const details: CompletionDetail[] = [
    { label: t('Model'), value: modelName },
    { label: t('CC Switch'), value: `${platform} · ${t('Installed')}` },
    { label: t('API key'), value: t('Created or restored') },
    { label: t('Provider import'), value: t('Confirmed') },
  ]
  if (isPreferredQuickStartModel(props.session.modelName)) {
    details.splice(1, 0, {
      label: t('Reasoning target'),
      value: t(QUICK_START_REASONING_EFFORT_LABEL_KEY),
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={() => undefined}
      title={t('Your setup is ready')}
      description={t(
        'Review the final setup summary, or start using the console now.'
      )}
      showCloseButton={false}
      contentClassName='max-w-[calc(100%-1.5rem)] rounded-xl border-border/70 bg-background/92 shadow-2xl backdrop-blur-2xl sm:max-w-lg'
      titleClassName='text-xl font-semibold tracking-normal'
      descriptionClassName='leading-6'
      bodyClassName='py-0'
      footerClassName='bg-muted/35 sm:flex-row'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            className='h-11 w-full sm:w-auto'
            onClick={props.onReview}
          >
            <HugeiconsIcon
              icon={RefreshIcon}
              data-icon='inline-start'
              strokeWidth={2}
            />
            {t('I need to review it again')}
          </Button>
          <Button
            type='button'
            className='h-11 w-full sm:w-auto'
            onClick={props.onStart}
          >
            {t('No, start now')}
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              data-icon='inline-end'
              strokeWidth={2}
            />
          </Button>
        </>
      }
    >
      <div className='divide-border/70 border-border/70 divide-y border-y'>
        {details.map((detail) => (
          <div
            key={detail.label}
            className='flex min-h-12 items-center gap-3 py-3'
          >
            <HugeiconsIcon
              icon={CheckmarkCircle02Icon}
              className='size-5 shrink-0 text-emerald-600 dark:text-emerald-400'
              strokeWidth={2}
              aria-hidden='true'
            />
            <span className='text-muted-foreground min-w-0 flex-1 text-sm'>
              {detail.label}
            </span>
            <span className='text-foreground max-w-[55%] text-right text-sm font-medium break-words'>
              {detail.value}
            </span>
          </div>
        ))}
      </div>
    </Dialog>
  )
}
