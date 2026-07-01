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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import type { AffiliateWithdrawalStatus } from '../types'

interface WithdrawalActionsProps {
  withdrawalId: number
  status: AffiliateWithdrawalStatus | string
  onPaid: (remark: string) => Promise<void>
  onReject: (remark: string) => Promise<void>
}

type ActionType = 'paid' | 'reject'

export function WithdrawalActions({
  status,
  onPaid,
  onReject,
}: WithdrawalActionsProps) {
  const { t } = useTranslation()
  const [action, setAction] = useState<ActionType | null>(null)
  const [remark, setRemark] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (status !== 'pending') return null

  const closeDialog = () => {
    setAction(null)
    setRemark('')
  }

  const submit = async () => {
    if (!action) return
    const trimmedRemark = remark.trim()
    if (action === 'reject' && !trimmedRemark) {
      toast.error(t('Please enter an admin remark'))
      return
    }

    setIsSubmitting(true)
    try {
      if (action === 'paid') {
        await onPaid(trimmedRemark)
      } else {
        await onReject(trimmedRemark)
      }
      closeDialog()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className='flex flex-wrap justify-end gap-2'>
      <Button
        type='button'
        size='sm'
        variant='outline'
        onClick={() => setAction('paid')}
      >
        {t('Mark as paid')}
      </Button>
      <Button
        type='button'
        size='sm'
        variant='destructive'
        onClick={() => setAction('reject')}
      >
        {t('Reject withdrawal')}
      </Button>

      <Dialog
        open={action !== null}
        onOpenChange={(open) => !open && closeDialog()}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {action === 'paid' ? t('Mark as paid') : t('Reject withdrawal')}
            </DialogTitle>
            <DialogDescription>{t('Admin remark')}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={remark}
            onChange={(event) => setRemark(event.target.value)}
            placeholder={t('Admin remark')}
            disabled={isSubmitting}
          />
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={isSubmitting}
              onClick={closeDialog}
            >
              {t('Cancel')}
            </Button>
            <Button type='button' disabled={isSubmitting} onClick={submit}>
              {action === 'paid' ? t('Mark as paid') : t('Reject withdrawal')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
