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
import { useMemo, useState } from 'react'
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog } from '@/components/dialog'
import { createSubscriptionRedemptions } from '../../api'
import type { PlanRecord } from '../../types'

export function SubscriptionRedemptionsDialog({
  open,
  onOpenChange,
  record,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  record: PlanRecord | null
}) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [count, setCount] = useState(1)
  const [submitting, setSubmitting] = useState(false)
  const [codes, setCodes] = useState<string[]>([])
  const { copyToClipboard } = useCopyToClipboard()

  const defaultName = useMemo(() => {
    return record ? `${record.plan.title}${t('Redemption Code')}` : ''
  }, [record, t])

  const submit = async () => {
    if (!record) return
    const safeCount = Math.max(1, Math.min(100, Number(count || 1)))
    setSubmitting(true)
    try {
      const response = await createSubscriptionRedemptions(record.plan.id, {
        name: name.trim() || defaultName,
        count: safeCount,
        expired_time: 0,
      })
      if (response.success && response.data) {
        setCodes(response.data)
        toast.success(
          t('Successfully created {{count}} redemption codes', {
            count: response.data.length,
          })
        )
        return
      }
      toast.error(response.message || t('Failed to create redemption code'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setName('')
          setCount(1)
          setCodes([])
        }
        onOpenChange(next)
      }}
      title={t('Generate Redemption Codes')}
      description={
        record
          ? t('Generate redemption codes for {{plan}}', {
              plan: record.plan.title,
            })
          : ''
      }
    >
      <div className='space-y-4'>
        <div className='space-y-2'>
          <Label>{t('Plan')}</Label>
          <Input value={record?.plan.title || ''} disabled />
        </div>
        <div className='space-y-2'>
          <Label>{t('Name')}</Label>
          <Input
            value={name}
            placeholder={defaultName}
            onChange={(event) => setName(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label>{t('Count')}</Label>
          <Input
            type='number'
            min={1}
            max={100}
            value={count}
            onChange={(event) => setCount(Number(event.target.value))}
          />
        </div>
        {codes.length > 0 && (
          <div className='space-y-2'>
            <div className='flex items-center justify-between'>
              <Label>{t('Generated Codes')}</Label>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => copyToClipboard(codes.join('\n'))}
              >
                <Copy className='mr-1 h-3 w-3' />
                {t('Copy All')}
              </Button>
            </div>
            <pre className='bg-muted max-h-48 overflow-auto rounded-md p-3 text-xs'>
              {codes.join('\n')}
            </pre>
          </div>
        )}
        <div className='flex justify-end gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={submit}
            disabled={submitting || !record}
          >
            {submitting ? t('Processing...') : t('Generate')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
