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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Edit, Plus, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { quotaUnitsToDollars } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { getAdminPlans } from '../api'
import { VALUE_PACKAGE_TYPES } from '../constants'
import type { PlanRecord, SubscriptionPlan } from '../types'
import { useSubscriptions } from './subscriptions-provider'

function formatMoney(amount: number, currency = 'USD'): string {
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
      minimumFractionDigits: Number.isInteger(amount) ? 0 : 2,
      maximumFractionDigits: 2,
    }).format(amount)
  } catch {
    return `${currency} ${amount.toFixed(Number.isInteger(amount) ? 0 : 2)}`
  }
}

function hasPaymentConfig(plan: SubscriptionPlan): boolean {
  return Boolean(
    plan.ldxp_product_url &&
    plan.ldxp_product_name &&
    Number(plan.ldxp_product_amount || 0) > 0
  )
}

function createTemplatePlan(
  type: (typeof VALUE_PACKAGE_TYPES)[number]
): SubscriptionPlan {
  return {
    id: 0,
    title: type.labelKey,
    subtitle: '',
    price_amount: 0,
    currency: 'USD',
    duration_unit: type.durationUnit,
    duration_value: type.durationValue,
    custom_seconds: 0,
    quota_reset_period: 'never',
    quota_reset_custom_seconds: 0,
    enabled: true,
    sort_order: 0,
    allow_balance_pay: false,
    max_purchase_per_user: 0,
    total_amount: 0,
    upgrade_group: '',
    stripe_price_id: '',
    creem_product_id: '',
    waffo_pancake_product_id: '',
    plan_kind: 'value_package',
    package_type: type.value,
    package_level: type.level,
    model_group: '',
    concurrency_limit: 1,
    limit_5h_amount: 0,
    limit_7d_amount: 0,
    benefits: '',
    ldxp_product_url: '',
    ldxp_product_name: '',
    ldxp_product_amount: 0,
    ldxp_product_ref: '',
    ldxp_session_ttl_seconds: 0,
  }
}

export function ValuePackageAdminCards() {
  const { t } = useTranslation()
  const { refreshTrigger, setCurrentRow, setOpen, complianceConfirmed } =
    useSubscriptions()

  const { data, isLoading } = useQuery({
    queryKey: [
      'admin-subscription-plans',
      'value-package-cards',
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getAdminPlans()
      return result.data || []
    },
    placeholderData: (prev) => prev,
  })

  const valuePackagePlans = useMemo(() => {
    return (data || []).filter(
      (record) => record.plan.plan_kind === 'value_package'
    )
  }, [data])

  const planByType = useMemo(() => {
    const map = new Map<string, PlanRecord>()
    for (const record of valuePackagePlans) {
      const type = record.plan.package_type
      if (type && !map.has(type)) {
        map.set(type, record)
      }
    }
    return map
  }, [valuePackagePlans])

  const openPlan = (record: PlanRecord) => {
    setCurrentRow(record)
    setOpen(record.plan.id > 0 ? 'update' : 'create')
  }

  return (
    <TitledCard
      title={t('Value Package Cards')}
      description={t(
        'Configure the fixed day, week, and month cards shown to ordinary users.'
      )}
      icon={<Sparkles className='size-4' />}
      contentClassName='flex flex-col gap-3'
    >
      <div className='grid gap-3 lg:grid-cols-3'>
        {VALUE_PACKAGE_TYPES.map((type) => {
          const record = planByType.get(type.value)
          const plan = record?.plan
          const paymentConfigured = plan ? hasPaymentConfig(plan) : false

          return (
            <Card key={type.value} className='gap-0 py-0'>
              <CardHeader className='border-b p-4'>
                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <Badge variant='secondary'>{t(type.labelKey)}</Badge>
                    <CardTitle className='mt-2 text-lg'>
                      {plan?.title || t(type.labelKey)}
                    </CardTitle>
                  </div>
                  {plan ? (
                    <Badge
                      variant={paymentConfigured ? 'default' : 'destructive'}
                    >
                      {paymentConfigured
                        ? t('Payment configured')
                        : t('付款未配置')}
                    </Badge>
                  ) : (
                    <Badge variant='secondary'>{t('Not created')}</Badge>
                  )}
                </div>
              </CardHeader>
              <CardContent className='flex flex-col gap-3 p-4 text-sm'>
                {isLoading ? (
                  <>
                    <Skeleton className='h-4 w-24' />
                    <Skeleton className='h-4 w-32' />
                    <Skeleton className='h-4 w-28' />
                  </>
                ) : plan ? (
                  <>
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        {t('Payment amount')}
                      </span>
                      <span className='font-medium'>
                        {formatMoney(
                          Number(
                            plan.ldxp_product_amount || plan.price_amount || 0
                          ),
                          plan.currency || 'USD'
                        )}
                      </span>
                    </div>
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        {t('Duration')}
                      </span>
                      <span className='font-medium'>
                        {plan.duration_value} {t(plan.duration_unit)}
                      </span>
                    </div>
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        {t('Model group')}
                      </span>
                      <span className='font-medium'>
                        {plan.model_group || t('Not configured')}
                      </span>
                    </div>
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        concurrency_limit
                      </span>
                      <span className='font-medium'>
                        {plan.concurrency_limit || 1}
                      </span>
                    </div>
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        limit_5h_amount
                      </span>
                      <span className='font-medium'>
                        {quotaUnitsToDollars(Number(plan.limit_5h_amount || 0))}
                      </span>
                    </div>
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        limit_7d_amount
                      </span>
                      <span className='font-medium'>
                        {quotaUnitsToDollars(Number(plan.limit_7d_amount || 0))}
                      </span>
                    </div>
                    <div className='text-muted-foreground truncate text-xs'>
                      ldxp_product_url:{' '}
                      {plan.ldxp_product_url || t('Not configured')}
                    </div>
                  </>
                ) : (
                  <p className='text-muted-foreground'>
                    {t(
                      'Create this fixed value package card to show it to users.'
                    )}
                  </p>
                )}
              </CardContent>
              <CardFooter className='p-4'>
                <Button
                  type='button'
                  variant={plan ? 'outline' : 'default'}
                  className='w-full'
                  disabled={!complianceConfirmed}
                  onClick={() =>
                    openPlan(record || { plan: createTemplatePlan(type) })
                  }
                >
                  {plan ? (
                    <Edit data-icon='inline-start' />
                  ) : (
                    <Plus data-icon='inline-start' />
                  )}
                  {plan ? t('Edit') : t('Create')}
                </Button>
              </CardFooter>
            </Card>
          )
        })}
      </div>
    </TitledCard>
  )
}
