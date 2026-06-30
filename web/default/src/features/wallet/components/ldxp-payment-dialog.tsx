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
import { Loader2, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import {
  LDXP_QR_CREATION_ANIMATION_SECONDS,
  getSafeLdxpQrCodeSrc,
  getLdxpStatusMessageKey,
  isLdxpTerminalStatus,
  shouldShowLdxpQrCreationHint,
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
    !isTerminal && !NON_CANCELABLE_IN_PROGRESS_STATUSES.has(session.status)
  const statusMessage = isSuccess
    ? t('Recharge successful')
    : t(getLdxpStatusMessageKey(session.status))
  const safeErrorMessage = session.error_message || error
  const safeQrCodeSrc = getSafeLdxpQrCodeSrc(session.qr_code)
  const hasInvalidQrCode =
    session.status === 'qr_ready' && Boolean(session.qr_code) && !safeQrCodeSrc
  const showQrCreationHint = shouldShowLdxpQrCreationHint(session.status)

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
              className='bg-background size-48 rounded-md object-contain'
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

        {showQrCreationHint ? (
          <div
            className='border-primary/15 from-primary/10 via-background to-muted/70 flex flex-col items-center gap-3 rounded-2xl border bg-linear-to-b px-4 py-5 text-center shadow-sm'
            style={
              {
                '--ldxp-qr-creation-duration': `${LDXP_QR_CREATION_ANIMATION_SECONDS}s`,
              } as React.CSSProperties
            }
          >
            <div className='ldxp-qr-creation-pop relative flex size-24 items-center justify-center rounded-full'>
              <span className='ldxp-qr-creation-pulse bg-primary/15 absolute inset-0 rounded-full' />
              <span className='ldxp-qr-creation-progress-ring absolute inset-1 rounded-full' />
              <span className='border-primary/20 bg-background absolute inset-3 rounded-full border shadow-lg' />
              <Loader2 className='ldxp-qr-creation-spinner text-primary relative size-11' />
            </div>
            <div className='space-y-1.5'>
              <div className='text-sm font-medium'>
                {t('Creating payment QR code')}
              </div>
              <div className='text-muted-foreground text-xs leading-relaxed'>
                {t(
                  'The payment QR code usually appears in about 20 seconds. Please wait.'
                )}
              </div>
            </div>
          </div>
        ) : null}

        <div className='flex flex-col gap-3'>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground text-sm'>{t('Amount')}</span>
            {showQrCreationHint ? (
              <span
                className='ring-primary/20 relative isolate inline-flex overflow-hidden rounded-full px-3 py-1 text-lg font-semibold shadow-sm ring-1'
                style={
                  {
                    '--ldxp-qr-creation-duration': `${LDXP_QR_CREATION_ANIMATION_SECONDS}s`,
                  } as React.CSSProperties
                }
              >
                <span className='bg-primary/10 absolute inset-0 -z-10' />
                <span className='ldxp-qr-creation-amount-fill bg-primary/20 absolute inset-y-0 left-0 -z-10 rounded-full' />
                <span className='ldxp-qr-creation-amount-shine via-primary/25 absolute inset-y-0 -left-1/2 -z-10 w-1/2 bg-gradient-to-r from-transparent to-transparent' />
                {session.amount}
              </span>
            ) : (
              <span className='text-lg font-semibold'>{session.amount}</span>
            )}
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

          {showQrCreationHint ? (
            <div className='text-muted-foreground bg-muted/60 flex items-start gap-2 rounded-lg px-3 py-2 text-xs leading-relaxed'>
              <Sparkles className='text-primary mt-0.5 h-3.5 w-3.5 shrink-0' />
              <span>
                {t(
                  'The payment QR code usually appears in about 20 seconds. Please wait.'
                )}
              </span>
            </div>
          ) : null}
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
