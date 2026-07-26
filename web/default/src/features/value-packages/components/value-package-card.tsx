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
  AlertTriangle,
  Clock,
  Gauge,
  Loader2,
  PauseCircle,
  Play,
  RotateCcw,
  Shield,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  shouldExposeValuePackage7dPeriodLimit,
  VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY,
  VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY,
} from '@/features/subscriptions/lib/value-package-limit-labels'
import { getValuePackagePeriodLimits } from '../lib/period-limits'
import {
  getPackageCardState,
  getPackageLevelLabel,
  type ValuePackageCardStateKind,
} from '../lib/rules'
import type { ValuePackagePlan, ValuePackageState } from '../types'
import { ValuePackagePeriodList } from './value-package-period-list'

interface ValuePackageCardProps {
  plan: ValuePackagePlan
  state: ValuePackageState | null
  actionKey?: string | null
  redemptionCode: string
  redeeming?: boolean
  onRedemptionCodeChange: (planId: number, code: string) => void
  onRedeemCode: (planId: number) => void
  onPurchase: (plan: ValuePackagePlan) => void
  onActivate: (userSubscriptionId: number) => void
  onDeactivate: () => void
  onResetQuota: (userSubscriptionId?: number) => void
}

function getValuePackageDisplayCurrency(currencyOverride?: string): string {
  const normalized = currencyOverride?.trim()
  return normalized || 'CNY'
}

function formatMoney(amount: number, currencyOverride?: string): string {
  const normalizedCurrency = getValuePackageDisplayCurrency(currencyOverride)
  const locale = normalizedCurrency === 'CNY' ? 'zh-CN' : undefined
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: normalizedCurrency,
    minimumFractionDigits: Number.isInteger(amount) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(amount)
}

function formatLimitAmount(amount: number, t: (key: string) => string): string {
  if (!amount || amount <= 0) {
    return t('Unlimited')
  }

  return new Intl.NumberFormat().format(amount)
}

function getPlanDurationLabel(
  plan: ValuePackagePlan,
  t: (key: string) => string
) {
  const value = plan.duration_value || 1
  switch (plan.duration_unit) {
    case 'hour':
      return `${value} ${t('hours')}`
    case 'day':
      return `${value} ${t('days')}`
    case 'month':
      return `${value} ${t('months')}`
    case 'year':
      return `${value} ${t('years')}`
    default:
      return t('Custom duration')
  }
}

function getBenefits(
  plan: ValuePackagePlan,
  t: (key: string) => string
): string[] {
  const benefits = (plan.benefits || '')
    .split(/\n|；|;|，|,/)
    .map((item) => item.trim())
    .filter(Boolean)

  if (benefits.length > 0) {
    return benefits.slice(0, 4)
  }

  return [
    t('Dedicated package model group'),
    t('Independent concurrency control'),
    t('Package total limit and 5-hour protection'),
  ]
}

function getActionLabel(
  kind: ValuePackageCardStateKind,
  t: (key: string) => string
) {
  switch (kind) {
    case 'start':
      return `▶ ${t('Start using')}`
    case 'running':
      return t('Close package usage')
    case 'expired':
      return t('Purchase again')
    case 'disabled':
      return t('Not available yet')
    default:
      return t('Purchase')
  }
}

export function ValuePackageCard({
  plan,
  state,
  actionKey,
  redemptionCode,
  redeeming = false,
  onRedemptionCodeChange,
  onRedeemCode,
  onPurchase,
  onActivate,
  onDeactivate,
  onResetQuota,
}: ValuePackageCardProps) {
  const { t } = useTranslation()
  const cardState = getPackageCardState(plan, state)
  const hasPaymentConfig = Boolean(
    plan.ldxp_product_url &&
    plan.ldxp_product_name &&
    Number(plan.ldxp_product_amount) > 0
  )
  const resetTargetSubscriptionId = cardState.userSubscriptionId || undefined
  const resetBusy =
    actionKey === `reset-quota-${resetTargetSubscriptionId || 'active'}`
  const mainActionBusy =
    actionKey === `purchase-${plan.id}` ||
    actionKey === `activate-${cardState.userSubscriptionId || 0}` ||
    actionKey === 'deactivate' ||
    actionKey === `redeem-${plan.id}`
  const isBusy = mainActionBusy || resetBusy
  const requiresPayment =
    cardState.kind === 'purchase' || cardState.kind === 'expired'
  const disabled =
    cardState.kind === 'disabled' ||
    isBusy ||
    (requiresPayment && !hasPaymentConfig) ||
    (cardState.kind === 'start' && !cardState.userSubscriptionId)
  const actionLabel =
    requiresPayment && !hasPaymentConfig
      ? t('Not available yet')
      : getActionLabel(cardState.kind, t)
  const packageLabel = t(getPackageLevelLabel(plan.package_type))
  const show7dPeriodLimit = shouldExposeValuePackage7dPeriodLimit(
    plan.package_type
  )
  const benefits = getBenefits(plan, t)
  const displayPrice = Number(
    plan.ldxp_product_amount || plan.price_amount || 0
  )
  const usage = state?.subscription?.plan_id === plan.id ? state.usage : null
  const usagePeriods = getValuePackagePeriodLimits(usage, plan.package_type)
  const exhaustedMessage =
    usage?.exhausted_message ||
    t(
      'Current quota is used up. Consider pausing, using the wallet API billing, or waiting for the time window to pass.'
    )
  const resetCount = Number(state?.preference?.reset_count || 0)
  const canShowResetQuota =
    cardState.kind === 'running' && state?.preference?.enabled === true
  const resetDisabled =
    !canShowResetQuota ||
    resetBusy ||
    mainActionBusy ||
    resetCount <= 0 ||
    !resetTargetSubscriptionId

  const handleRedeemCode = () => {
    onRedeemCode(plan.id)
  }

  const handleRedemptionKeyDown = (
    event: React.KeyboardEvent<HTMLInputElement>
  ) => {
    if (event.key === 'Enter') {
      event.preventDefault()
      handleRedeemCode()
    }
  }

  const handleAction = () => {
    if (disabled) {
      return
    }

    if (cardState.kind === 'start' && cardState.userSubscriptionId) {
      onActivate(cardState.userSubscriptionId)
      return
    }

    if (cardState.kind === 'running') {
      onDeactivate()
      return
    }

    if (cardState.kind === 'purchase' || cardState.kind === 'expired') {
      onPurchase(plan)
    }
  }

  const handleResetQuota = () => {
    if (resetDisabled) {
      return
    }

    const confirmed = window.confirm(t(VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY))
    if (!confirmed) {
      return
    }

    onResetQuota(resetTargetSubscriptionId)
  }

  return (
    <Card
      className={cn(
        'relative gap-0 overflow-hidden py-0 transition-all duration-300',
        cardState.kind === 'running'
          ? 'ring-primary/30 shadow-[0_18px_60px_color-mix(in_oklch,var(--primary)_12%,transparent)] ring-2'
          : 'hover:ring-primary/20 hover:shadow-md'
      )}
    >
      <div className='from-primary/12 pointer-events-none absolute inset-x-0 top-0 h-1 bg-gradient-to-r via-transparent to-transparent' />
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0'>
            <Badge variant='secondary' className='mb-3 rounded-full'>
              {packageLabel}
            </Badge>
            <CardTitle className='text-xl font-black tracking-tight sm:text-2xl'>
              {plan.title || packageLabel}
            </CardTitle>
            {plan.subtitle ? (
              <p className='text-muted-foreground mt-1 text-sm leading-relaxed'>
                {plan.subtitle}
              </p>
            ) : null}
          </div>
          <div className='text-right'>
            <div className='text-2xl font-black tracking-[-0.04em] sm:text-3xl'>
              {formatMoney(displayPrice, 'CNY')}
            </div>
            <div className='text-muted-foreground mt-1 text-xs'>
              {getPlanDurationLabel(plan, t)}
            </div>
          </div>
        </div>
      </CardHeader>

      <CardContent className='flex flex-1 flex-col gap-4 p-4 sm:p-5'>
        <div className='grid grid-cols-3 gap-2 text-xs'>
          <div className='bg-muted/60 rounded-lg p-2.5'>
            <div className='text-muted-foreground flex items-center gap-1.5'>
              <Shield className='size-3.5' />
              {t('Model group')}
            </div>
            <div className='mt-1 truncate font-semibold'>
              {plan.model_group || t('Not configured')}
            </div>
          </div>
          <div className='bg-muted/60 rounded-lg p-2.5'>
            <div className='text-muted-foreground flex items-center gap-1.5'>
              <Gauge className='size-3.5' />
              {t('Concurrency')}
            </div>
            <div className='mt-1 font-semibold'>
              {plan.concurrency_limit || 1}
            </div>
          </div>
          <div className='bg-muted/60 rounded-lg p-2.5'>
            <div className='text-muted-foreground flex items-center gap-1.5'>
              <Clock className='size-3.5' />
              {t('Countdown')}
            </div>
            <div className='mt-1 font-semibold'>{t('No pause')}</div>
          </div>
        </div>

        {Number(plan.gift_reset_count || 0) > 0 ? (
          <div className='border-primary/30 bg-primary/5 text-primary flex items-center gap-2 rounded-lg border border-dashed px-3 py-2 text-xs font-medium'>
            <RotateCcw className='size-3.5 shrink-0' aria-hidden='true' />
            {t('Each activation gifts {{count}} reset card(s)', {
              count: Number(plan.gift_reset_count || 0),
            })}
          </div>
        ) : null}

        <div className='grid grid-cols-1 gap-2 text-sm sm:grid-cols-2'>
          <div className='rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('5-hour limit')}
            </div>
            <div className='mt-1 font-semibold tabular-nums'>
              {formatLimitAmount(Number(plan.limit_5h_amount || 0), t)}
            </div>
          </div>
          {show7dPeriodLimit && Number(plan.limit_7d_amount || 0) > 0 ? (
            <div className='rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs font-medium'>
                {t(VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY)}
              </div>
              <div className='mt-1 font-semibold tabular-nums'>
                {formatLimitAmount(Number(plan.limit_7d_amount || 0), t)}
              </div>
            </div>
          ) : null}
        </div>

        <div className='rounded-lg border p-3'>
          <ValuePackagePeriodList periods={usagePeriods} />
        </div>

        <Separator />

        <ul className='flex flex-col gap-2 text-sm'>
          {benefits.map((benefit) => (
            <li key={benefit} className='flex items-start gap-2'>
              <span className='bg-primary mt-2 size-1.5 shrink-0 rounded-full' />
              <span>{benefit}</span>
            </li>
          ))}
        </ul>

        <div className='rounded-lg border p-3'>
          <Label
            htmlFor={`value-package-redemption-${plan.id}`}
            className='text-muted-foreground text-xs font-medium'
          >
            {t('Redeem Code')}
          </Label>
          <div className='mt-2 grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
            <Input
              id={`value-package-redemption-${plan.id}`}
              value={redemptionCode}
              onChange={(event) =>
                onRedemptionCodeChange(plan.id, event.target.value)
              }
              onKeyDown={handleRedemptionKeyDown}
              placeholder={t('Enter your redemption code or card key')}
              disabled={redeeming}
              className='h-9 min-w-0'
            />
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={redeeming || redemptionCode.trim() === ''}
              onClick={handleRedeemCode}
            >
              {redeeming ? (
                <Loader2 className='animate-spin' data-icon='inline-start' />
              ) : null}
              {t('Redeem')}
            </Button>
          </div>
        </div>

        {cardState.kind === 'running' || cardState.kind === 'start' ? (
          <Alert className='mt-auto'>
            <PauseCircle className='size-4' />
            <AlertDescription>
              {t('Closing package usage does not pause its countdown.')}
            </AlertDescription>
          </Alert>
        ) : null}

        {cardState.kind === 'running' && usage?.exhausted ? (
          <Alert variant='destructive'>
            <AlertTriangle className='size-4' />
            <AlertDescription>{exhaustedMessage}</AlertDescription>
          </Alert>
        ) : null}
      </CardContent>

      <CardFooter className='bg-muted/35 flex flex-col gap-2 p-4 sm:p-5'>
        <Button
          className='w-full'
          variant={cardState.kind === 'running' ? 'outline' : 'default'}
          disabled={disabled}
          onClick={handleAction}
        >
          {mainActionBusy ? (
            <Loader2 className='animate-spin' data-icon='inline-start' />
          ) : null}
          {cardState.kind === 'start' && !mainActionBusy ? (
            <Play data-icon='inline-start' />
          ) : null}
          {actionLabel}
        </Button>
        {canShowResetQuota ? (
          <>
            <Button
              type='button'
              className='w-full'
              variant='outline'
              disabled={resetDisabled}
              onClick={handleResetQuota}
            >
              {resetBusy ? (
                <Loader2 className='animate-spin' data-icon='inline-start' />
              ) : (
                <RotateCcw data-icon='inline-start' />
              )}
              {t('Reset quota')}
            </Button>
            <div className='text-muted-foreground text-center text-xs'>
              {t('Remaining reset count')}: {resetCount}
            </div>
          </>
        ) : null}
      </CardFooter>
    </Card>
  )
}
