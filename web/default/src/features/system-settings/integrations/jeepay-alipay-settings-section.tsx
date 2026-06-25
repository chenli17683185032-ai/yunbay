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
import * as React from 'react'
import * as z from 'zod'
import { Save } from 'lucide-react'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import type { JeepayPaymentSettings } from '../types'
import { safeNumberFieldProps } from '../utils/numeric-field'
import {
  buildJeepayPaymentSettingsPayload,
  defaultJeepayPaymentSettings,
  getJeepayPaymentSettings,
  normalizeJeepayPaymentSettings,
  updateJeepayPaymentSettings,
} from './jeepay-api'

const APP_SECRET_CONFIGURED_HINT = '已配置，留空则不修改'

const createJeepaySchema = (t: (key: string) => string) =>
  z.object({
    JeepayEnabled: z.boolean(),
    JeepayAlipayEnabled: z.boolean(),
    JeepayBaseUrl: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with http:// or https://')),
    JeepayMchNo: z.string(),
    JeepayAppId: z.string(),
    JeepayAppSecret: z.string(),
    JeepayAppSecretConfigured: z.boolean(),
    JeepayNotifyUrl: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with http:// or https://')),
    JeepayReturnUrl: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with http:// or https://')),
    JeepaySubject: z.string(),
    JeepayBody: z.string(),
    JeepayTimeoutMs: z.coerce.number().int().min(0),
    JeepayAliDisplayName: z.string(),
    JeepayAliDisplayColor: z.string(),
  }).superRefine((values, ctx) => {
    if (
      values.JeepayEnabled &&
      !values.JeepayAppSecretConfigured &&
      !values.JeepayAppSecret.trim()
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['JeepayAppSecret'],
        message: t('App secret is required when enabling Jeepay'),
      })
    }
  })

type JeepaySettingsFormValues = z.infer<ReturnType<typeof createJeepaySchema>>

function areJeepaySettingsUnchanged(
  current: JeepayPaymentSettings,
  initial: JeepayPaymentSettings
) {
  const normalizedCurrent = normalizeJeepayPaymentSettings(current)
  const normalizedInitial = normalizeJeepayPaymentSettings(initial)

  return (
    normalizedCurrent.JeepayEnabled === normalizedInitial.JeepayEnabled &&
    normalizedCurrent.JeepayAlipayEnabled ===
      normalizedInitial.JeepayAlipayEnabled &&
    normalizedCurrent.JeepayBaseUrl === normalizedInitial.JeepayBaseUrl &&
    normalizedCurrent.JeepayMchNo === normalizedInitial.JeepayMchNo &&
    normalizedCurrent.JeepayAppId === normalizedInitial.JeepayAppId &&
    normalizedCurrent.JeepayNotifyUrl === normalizedInitial.JeepayNotifyUrl &&
    normalizedCurrent.JeepayReturnUrl === normalizedInitial.JeepayReturnUrl &&
    normalizedCurrent.JeepaySubject === normalizedInitial.JeepaySubject &&
    normalizedCurrent.JeepayBody === normalizedInitial.JeepayBody &&
    normalizedCurrent.JeepayTimeoutMs === normalizedInitial.JeepayTimeoutMs &&
    normalizedCurrent.JeepayAliDisplayName ===
      normalizedInitial.JeepayAliDisplayName &&
    normalizedCurrent.JeepayAliDisplayColor ===
      normalizedInitial.JeepayAliDisplayColor &&
    normalizedCurrent.JeepayAppSecret.length === 0
  )
}

export function JeepayAlipaySettingsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const jeepaySchema = React.useMemo(() => createJeepaySchema(t), [t])
  const hasInitializedRef = React.useRef(false)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['jeepay-payment-settings'],
    queryFn: getJeepayPaymentSettings,
  })

  const queryDefaults = React.useMemo(
    () => data?.data ?? defaultJeepayPaymentSettings,
    [data?.data]
  )
  const [baselineValues, setBaselineValues] =
    React.useState<JeepayPaymentSettings>(defaultJeepayPaymentSettings)

  const form = useForm<JeepaySettingsFormValues>({
    resolver: zodResolver(jeepaySchema) as Resolver<JeepaySettingsFormValues>,
    defaultValues: baselineValues,
  })

  React.useEffect(() => {
    if (!data?.data) return

    if (!hasInitializedRef.current) {
      hasInitializedRef.current = true
      setBaselineValues(queryDefaults)
      form.reset(queryDefaults)
      return
    }

    if (!form.formState.isDirty) {
      setBaselineValues(queryDefaults)
      form.reset(queryDefaults)
    }
  }, [data?.data, form, form.formState.isDirty, queryDefaults])

  const saveMutation = useMutation({
    mutationFn: updateJeepayPaymentSettings,
    onError: (mutationError: Error) => {
      toast.error(mutationError.message || t('Failed to save Jeepay settings'))
    },
  })

  const onSubmit = async (values: JeepaySettingsFormValues) => {
    const current = normalizeJeepayPaymentSettings(values)
    const initial = normalizeJeepayPaymentSettings(baselineValues)

    if (areJeepaySettingsUnchanged(current, initial)) {
      toast.info(t('No changes to save'))
      return
    }

    const body = await saveMutation.mutateAsync(
      buildJeepayPaymentSettingsPayload(current)
    )

    if (!body.success) {
      toast.error(body.message || t('Failed to save Jeepay settings'))
      return
    }

    const nextBaseline: JeepayPaymentSettings = {
      ...current,
      JeepayAppSecret: '',
      JeepayAppSecretConfigured:
        current.JeepayAppSecretConfigured || current.JeepayAppSecret.length > 0,
    }

    setBaselineValues(nextBaseline)
    form.reset(nextBaseline)
    queryClient.invalidateQueries({ queryKey: ['jeepay-payment-settings'] })
    toast.success(t('Jeepay / Alipay recharge settings saved'))
  }

  const appSecretPlaceholder = baselineValues.JeepayAppSecretConfigured
    ? APP_SECRET_CONFIGURED_HINT
    : t('Enter Jeepay app secret')

  const isSaving = saveMutation.isPending || form.formState.isSubmitting

  return (
    <SettingsSection title={t('Jeepay / Alipay Recharge')}>
      <div className='bg-card space-y-4 rounded-xl border p-5 shadow-sm'>
        <div className='space-y-1'>
          <h3 className='text-lg font-medium'>
            {t('Jeepay / Alipay Recharge')}
          </h3>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Configure the dedicated Jeepay + Alipay recharge flow without editing the generic payment methods JSON.'
            )}
          </p>
        </div>

        {isLoading ? (
          <div className='text-muted-foreground text-sm'>
            {t('Loading Jeepay settings...')}
          </div>
        ) : null}

        {isError ? (
          <div className='border-destructive/30 bg-destructive/5 rounded-md border p-4 text-sm'>
            <p className='font-medium'>{t('Failed to load Jeepay settings')}</p>
            <p className='text-muted-foreground mt-1'>
              {error instanceof Error ? error.message : t('Unknown error')}
            </p>
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='mt-3'
              onClick={() => refetch()}
            >
              {t('Retry')}
            </Button>
          </div>
        ) : null}

        {!isLoading && !isError ? (
          <Form {...form}>
            <SettingsForm
              onSubmit={form.handleSubmit(onSubmit)}
              autoComplete='off'
            >
              <FormField
                control={form.control}
                name='JeepayEnabled'
                render={({ field }) => (
                  <SettingsSwitchField
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    label={t('Enable Jeepay recharge')}
                    description={t(
                      'Turn on the dedicated Jeepay recharge entry for backend-managed top-ups.'
                    )}
                  />
                )}
              />

              <FormField
                control={form.control}
                name='JeepayAlipayEnabled'
                render={({ field }) => (
                  <SettingsSwitchField
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    label={t('Enable Alipay under Jeepay')}
                    description={t(
                      'Expose the Alipay payment method inside the Jeepay recharge flow.'
                    )}
                  />
                )}
              />

              <FormField
                control={form.control}
                name='JeepayBaseUrl'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Jeepay base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://jeepay.example.com'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Base URL of your Jeepay deployment')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayMchNo'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Merchant number')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='MCH_xxx'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Merchant number registered in Jeepay')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayAppId'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('App ID')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='APP_xxx'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('App ID bound to the recharge channel in Jeepay')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayAppSecret'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('App secret')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={appSecretPlaceholder}
                        autoComplete='new-password'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {baselineValues.JeepayAppSecretConfigured
                        ? APP_SECRET_CONFIGURED_HINT
                        : t('Required when first configuring Jeepay.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayNotifyUrl'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Notify URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://app.example.com/api/payment/notify'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Server-side webhook callback for payment state sync')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayReturnUrl'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Return URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://app.example.com/topup/result'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Front-end redirect target after payment completes')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepaySubject'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Subject')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Account recharge')}
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Order title shown in the payment sheet')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayBody'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Body')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Recharge balance for your workspace')}
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Extended order description passed to Jeepay')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayTimeoutMs'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Timeout (ms)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step='1000'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Timeout used when calling the Jeepay API')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayAliDisplayName'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Alipay display name')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Alipay Recharge')}
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Label shown to users for the Jeepay Alipay option')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='JeepayAliDisplayColor'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Alipay display color')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='#1677FF'
                        autoComplete='off'
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Optional brand color used when rendering the Alipay recharge entry.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div
                data-settings-form-span='full'
                className='flex justify-end border-t pt-4'
              >
                <Button type='submit' size='sm' disabled={isSaving}>
                  <Save data-icon='inline-start' />
                  <span>
                    {t(
                      isSaving
                        ? 'Saving Jeepay / Alipay recharge settings...'
                        : 'Save Jeepay / Alipay recharge settings'
                    )}
                  </span>
                </Button>
              </div>
            </SettingsForm>
          </Form>
        ) : null}
      </div>
    </SettingsSection>
  )
}
