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
import { z } from 'zod'
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import {
  getValuePackageDuration,
  getValuePackageLevel,
} from '../constants'
import type { SubscriptionPlan, PlanPayload } from '../types'

export function getPlanFormSchema(t: TFunction) {
  return z.object({
    title: z.string().min(1, t('Please enter plan title')),
    subtitle: z.string().optional(),
    price_amount: z.coerce.number().min(0, t('Please enter amount')),
    duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
    duration_value: z.coerce.number().min(1),
    custom_seconds: z.coerce.number().min(0).optional(),
    quota_reset_period: z.enum([
      'never',
      'daily',
      'weekly',
      'monthly',
      'custom',
    ]),
    quota_reset_custom_seconds: z.coerce.number().min(0).optional(),
    enabled: z.boolean(),
    sort_order: z.coerce.number(),
    allow_balance_pay: z.boolean(),
    max_purchase_per_user: z.coerce.number().min(0),
    total_amount: z.coerce.number().min(0),
    upgrade_group: z.string().optional(),
    stripe_price_id: z.string().optional(),
    creem_product_id: z.string().optional(),
    waffo_pancake_product_id: z.string().optional(),
    plan_kind: z
      .enum(['subscription', 'value_package'])
      .default('subscription'),
    package_type: z.enum(['day', 'week', 'month']).optional(),
    package_level: z.coerce.number().min(0).optional(),
    model_group: z.string().optional(),
    concurrency_limit: z.coerce.number().min(1).max(2).optional(),
    limit_5h_amount: z.coerce.number().min(0).optional(),
    limit_7d_amount: z.coerce.number().min(0).optional(),
    benefits: z.string().optional(),
    ldxp_product_url: z.string().optional(),
    ldxp_product_name: z.string().optional(),
    ldxp_product_amount: z.coerce.number().min(0).optional(),
    ldxp_product_ref: z.string().optional(),
    ldxp_session_ttl_seconds: z.coerce.number().min(0).optional(),
  })
}

export type PlanFormValues = z.infer<ReturnType<typeof getPlanFormSchema>>

export const PLAN_FORM_DEFAULTS: PlanFormValues = {
  title: '',
  subtitle: '',
  price_amount: 0,
  duration_unit: 'month',
  duration_value: 1,
  custom_seconds: 0,
  quota_reset_period: 'never',
  quota_reset_custom_seconds: 0,
  enabled: true,
  sort_order: 0,
  allow_balance_pay: true,
  max_purchase_per_user: 0,
  total_amount: 0,
  upgrade_group: '',
  stripe_price_id: '',
  creem_product_id: '',
  waffo_pancake_product_id: '',
  plan_kind: 'subscription',
  package_type: undefined,
  package_level: 0,
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

export function planToFormValues(plan: SubscriptionPlan): PlanFormValues {
  return {
    title: plan.title || '',
    subtitle: plan.subtitle || '',
    price_amount: Number(plan.price_amount || 0),
    duration_unit: plan.duration_unit || 'month',
    duration_value: Number(plan.duration_value || 1),
    custom_seconds: Number(plan.custom_seconds || 0),
    quota_reset_period: plan.quota_reset_period || 'never',
    quota_reset_custom_seconds: Number(plan.quota_reset_custom_seconds || 0),
    enabled: plan.enabled !== false,
    sort_order: Number(plan.sort_order || 0),
    allow_balance_pay: plan.allow_balance_pay !== false,
    max_purchase_per_user: Number(plan.max_purchase_per_user || 0),
    total_amount: quotaUnitsToDollars(Number(plan.total_amount || 0)),
    upgrade_group: plan.upgrade_group || '',
    stripe_price_id: plan.stripe_price_id || '',
    creem_product_id: plan.creem_product_id || '',
    waffo_pancake_product_id: plan.waffo_pancake_product_id || '',
    plan_kind: plan.plan_kind || 'subscription',
    package_type: plan.package_type,
    package_level: Number(plan.package_level || 0),
    model_group: plan.model_group || '',
    concurrency_limit: Number(plan.concurrency_limit || 1),
    limit_5h_amount: quotaUnitsToDollars(Number(plan.limit_5h_amount || 0)),
    limit_7d_amount: quotaUnitsToDollars(Number(plan.limit_7d_amount || 0)),
    benefits: plan.benefits || '',
    ldxp_product_url: plan.ldxp_product_url || '',
    ldxp_product_name: plan.ldxp_product_name || '',
    ldxp_product_amount: Number(plan.ldxp_product_amount || 0),
    ldxp_product_ref: plan.ldxp_product_ref || '',
    ldxp_session_ttl_seconds: Number(plan.ldxp_session_ttl_seconds || 0),
  }
}

export function formValuesToPlanPayload(values: PlanFormValues): PlanPayload {
  const isValuePackage = values.plan_kind === 'value_package'
  const packageLevel = isValuePackage
    ? getValuePackageLevel(values.package_type)
    : Number(values.package_level || 0)
  const packageDuration = isValuePackage
    ? getValuePackageDuration(values.package_type)
    : null

  return {
    plan: {
      ...values,
      price_amount: Number(values.price_amount || 0),
      currency: 'USD',
      duration_unit: packageDuration?.duration_unit || values.duration_unit,
      duration_value:
        packageDuration?.duration_value ?? Number(values.duration_value || 0),
      custom_seconds:
        packageDuration?.custom_seconds ?? Number(values.custom_seconds || 0),
      quota_reset_period: values.quota_reset_period || 'never',
      quota_reset_custom_seconds:
        values.quota_reset_period === 'custom'
          ? Number(values.quota_reset_custom_seconds || 0)
          : 0,
      sort_order: Number(values.sort_order || 0),
      max_purchase_per_user: Number(values.max_purchase_per_user || 0),
      total_amount: parseQuotaFromDollars(Number(values.total_amount || 0)),
      plan_kind: values.plan_kind || 'subscription',
      package_type: isValuePackage ? values.package_type : undefined,
      package_level: packageLevel,
      model_group: values.model_group || '',
      concurrency_limit: Number(values.concurrency_limit || 1),
      limit_5h_amount: parseQuotaFromDollars(
        Number(values.limit_5h_amount || 0)
      ),
      limit_7d_amount: parseQuotaFromDollars(
        Number(values.limit_7d_amount || 0)
      ),
      benefits: values.benefits || '',
      ldxp_product_url: values.ldxp_product_url || '',
      ldxp_product_name: values.ldxp_product_name || '',
      ldxp_product_amount: Number(values.ldxp_product_amount || 0),
      ldxp_product_ref: values.ldxp_product_ref || '',
      ldxp_session_ttl_seconds: Number(values.ldxp_session_ttl_seconds || 0),
      allow_balance_pay: isValuePackage ? false : values.allow_balance_pay,
      upgrade_group: isValuePackage ? '' : values.upgrade_group || '',
    },
  }
}
