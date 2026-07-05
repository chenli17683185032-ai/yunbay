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
import { useEffect, useMemo, useRef, useState } from 'react'
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
  const [count, setCount] = useState('1')
  const [submitting, setSubmitting] = useState(false)
  const [codes, setCodes] = useState<string[]>([])
  const submittingRef = useRef(false)
  const requestTokenRef = useRef(0)
  const currentPlanIdRef = useRef<number | null>(null)
  const openRef = useRef(open)
  const activePlanIdRef = useRef<number | null>(record?.plan.id ?? null)
  const { copyToClipboard } = useCopyToClipboard()

  openRef.current = open
  activePlanIdRef.current = record?.plan.id ?? null

  const defaultName = useMemo(() => {
    return record
      ? t('{{plan}} Redemption Code', { plan: record.plan.title })
      : ''
  }, [record, t])

  const resetState = () => {
    requestTokenRef.current += 1
    setName('')
    setCount('1')
    setCodes([])
    submittingRef.current = false
    setSubmitting(false)
  }

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      if (submittingRef.current) return
      resetState()
    }
    onOpenChange(next)
  }

  useEffect(() => {
    if (!open) {
      currentPlanIdRef.current = null
      resetState()
      return
    }

    const planId = record?.plan.id ?? null
    if (currentPlanIdRef.current !== planId) {
      currentPlanIdRef.current = planId
      resetState()
    }
  }, [open, record?.plan.id])

  const submit = async () => {
    if (!record || submittingRef.current) return

    const parsedCount = Number(count)
    if (
      !Number.isInteger(parsedCount) ||
      parsedCount < 1 ||
      parsedCount > 100
    ) {
      toast.error(t('Count must be an integer between 1 and 100'))
      return
    }

    const redemptionName = (name.trim() || defaultName).trim()
    const nameLength = [...redemptionName].length
    if (nameLength < 1 || nameLength > 20) {
      toast.error(t('Name must be 1-20 characters'))
      return
    }

    const planId = record.plan.id
    const requestToken = requestTokenRef.current + 1
    requestTokenRef.current = requestToken
    submittingRef.current = true
    setSubmitting(true)
    try {
      const response = await createSubscriptionRedemptions(
        planId,
        {
          name: redemptionName,
          count: parsedCount,
          expired_time: 0,
        },
        { skipBusinessError: true }
      )

      if (
        requestToken !== requestTokenRef.current ||
        !openRef.current ||
        activePlanIdRef.current !== planId
      ) {
        return
      }

      if (response.success && Array.isArray(response.data)) {
        setCodes(response.data)
        toast.success(
          t('Successfully created {{count}} redemption codes', {
            count: response.data.length,
          })
        )
        return
      }
      toast.error(response.message || t('Failed to create redemption code'))
    } catch {
      // Global API error handling already owns network/HTTP error toasts.
    } finally {
      if (requestToken === requestTokenRef.current) {
        submittingRef.current = false
        setSubmitting(false)
      }
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Generate Redemption Codes')}
      description={
        record
          ? t('Generate redemption codes for {{plan}}', {
              plan: record.plan.title,
            })
          : ''
      }
      showCloseButton={!submitting}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={submitting}
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
        </>
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
            maxLength={20}
            onChange={(event) => setName(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label>{t('Count')}</Label>
          <Input
            type='number'
            min={1}
            max={100}
            step={1}
            value={count}
            onChange={(event) => setCount(event.target.value)}
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
                <Copy data-icon='inline-start' />
                {t('Copy All')}
              </Button>
            </div>
            <pre className='bg-muted max-h-48 overflow-auto rounded-md p-3 text-xs'>
              {codes.join('\n')}
            </pre>
          </div>
        )}
      </div>
    </Dialog>
  )
}
