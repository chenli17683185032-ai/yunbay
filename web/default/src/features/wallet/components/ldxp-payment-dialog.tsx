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
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import {
  getSafeLdxpQrCodeSrc,
  getLdxpStatusMessageKey,
  isLdxpTerminalStatus,
} from '../lib/ldxp-topup'
import type { LdxpTopupSession } from '../types'

interface LdxpPaymentDialogProps {
  session: LdxpTopupSession | null
  loading?: boolean
  error?: string | null
  onCancel: () => void
  onClose: () => void
}

const FAILURE_STATUSES = new Set<LdxpTopupSession['status']>([
  'canceled',
  'expired',
  'worker_failed',
  'mail_timeout',
  'verify_failed',
  'redeem_failed',
])

const NON_CANCELABLE_IN_PROGRESS_STATUSES = new Set<LdxpTopupSession['status']>(
  ['worker_paid', 'verified', 'redeemed']
)

function getRemainingSeconds(expiresAt: number, nowMs: number): number {
  const expiresAtMs = expiresAt > 1000000000000 ? expiresAt : expiresAt * 1000
  return Math.max(0, Math.ceil((expiresAtMs - nowMs) / 1000))
}

function formatCountdown(seconds: number): string {
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
}

export function LdxpPaymentDialog({
  session,
  loading,
  error,
  onCancel,
  onClose,
}: LdxpPaymentDialogProps) {
  const { t } = useTranslation()
  const [nowMs, setNowMs] = useState(() => Date.now())

  useEffect(() => {
    if (!session || isLdxpTerminalStatus(session.status)) {
      return
    }

    const intervalId = window.setInterval(() => setNowMs(Date.now()), 1000)
    return () => window.clearInterval(intervalId)
  }, [session])

  const remainingSeconds = useMemo(() => {
    if (!session) {
      return 0
    }
    return getRemainingSeconds(session.expires_at, nowMs)
  }, [nowMs, session])

  if (!session) {
    return null
  }

  const isSuccess = session.status === 'success'
  const isFailure = FAILURE_STATUSES.has(session.status)
  const isTerminal = isLdxpTerminalStatus(session.status)
  const canCancel =
    !isTerminal &&
    !NON_CANCELABLE_IN_PROGRESS_STATUSES.has(session.status)
  const statusMessage = isSuccess
    ? t('Recharge successful')
    : t(getLdxpStatusMessageKey(session.status))
  const safeErrorMessage = session.error_message || error
  const safeQrCodeSrc = getSafeLdxpQrCodeSrc(session.qr_code)
  const hasInvalidQrCode =
    session.status === 'qr_ready' && Boolean(session.qr_code) && !safeQrCodeSrc

  return (
    <Dialog
      open={true}
      onOpenChange={(open) => {
        if (!open && isTerminal) {
          onClose()
        }
      }}
      title={t('Alipay Auto Top-up')}
      description={t('Scan the QR code before it expires to complete payment.')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-[425px]'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      contentHeight='auto'
      bodyClassName='flex flex-col gap-4'
      showCloseButton={isTerminal}
      footer={
        canCancel || isTerminal ? (
          <>
            {canCancel ? (
              <Button variant='outline' onClick={onCancel} disabled={loading}>
                {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {t('Cancel')}
              </Button>
            ) : null}
            {isTerminal ? (
              <Button onClick={onClose}>{t('Close')}</Button>
            ) : null}
          </>
        ) : undefined
      }
    >
      <div className='flex flex-col gap-4 py-3 sm:py-4'>
        {session.status === 'qr_ready' && safeQrCodeSrc ? (
          <div className='bg-muted flex justify-center rounded-lg p-4'>
            <img
              src={safeQrCodeSrc}
              alt={t('Alipay payment QR code')}
              className='size-48 rounded-md bg-background object-contain'
            />
          </div>
        ) : null}

        {hasInvalidQrCode ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('QR code is not configured. Please contact support.')}
            </AlertDescription>
          </Alert>
        ) : null}

        <div className='flex flex-col gap-3'>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground text-sm'>{t('Amount')}</span>
            <span className='text-lg font-semibold'>{session.amount}</span>
          </div>

          {session.worker_order_no ? (
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground text-sm'>
                {t('Order No.')}
              </span>
              <span className='font-mono text-sm'>
                {session.worker_order_no}
              </span>
            </div>
          ) : null}

          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground text-sm'>
              {t('Expires in')}
            </span>
            <span className='font-mono text-sm'>
              {formatCountdown(remainingSeconds)}
            </span>
          </div>

          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground text-sm'>{t('Status')}</span>
            <Badge variant={isFailure ? 'destructive' : 'secondary'}>
              {statusMessage}
            </Badge>
          </div>
        </div>

        {loading && !isTerminal ? (
          <div className='text-muted-foreground flex items-center gap-2 text-sm'>
            <Loader2 className='h-4 w-4 animate-spin' />
            {t('Refreshing payment status')}
          </div>
        ) : null}

        {(isFailure || !isTerminal) && safeErrorMessage ? (
          <Alert variant='destructive'>
            <AlertDescription>{safeErrorMessage}</AlertDescription>
          </Alert>
        ) : null}
      </div>
    </Dialog>
  )
}
