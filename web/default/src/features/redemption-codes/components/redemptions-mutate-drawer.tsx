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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { addTimeToDate } from '@/lib/time'
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { createRedemption, updateRedemption, getRedemption } from '../api'
import {
  REDEMPTION_KINDS,
  REDEMPTION_SOURCES,
  SUCCESS_MESSAGES,
  getRedemptionKindOptions,
  getRedemptionSourceOptions,
} from '../constants'
import {
  getRedemptionFormSchema,
  type RedemptionFormValues,
  REDEMPTION_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
} from '../lib'
import { redemptionKindSchema, type Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type RedemptionsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Redemption
}

export function RedemptionsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: RedemptionsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useRedemptions()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const redemptionKindOptions = getRedemptionKindOptions(t)
  const redemptionSourceOptions = getRedemptionSourceOptions(t)

  const form = useForm<RedemptionFormValues>({
    resolver: zodResolver(getRedemptionFormSchema(t)),
    defaultValues: REDEMPTION_FORM_DEFAULT_VALUES,
  })

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      getRedemption(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformRedemptionToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(REDEMPTION_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  const onSubmit = async (data: RedemptionFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateRedemption({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.REDEMPTION_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t('Failed to update redemption code'))
        }
      } else {
        // Create mode
        const result = await createRedemption(basePayload)
        if (result.success) {
          const count = result.data?.length || 0
          const message =
            count > 1
              ? t('Successfully created {{count}} redemption codes', {
                  count,
                })
              : t(SUCCESS_MESSAGES.REDEMPTION_CREATED)
          toast.success(
            result.batch_id
              ? `${message} (${t('Batch ID')}: ${result.batch_id})`
              : message
          )
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t('Failed to create redemption code'))
        }
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    const newDate = addTimeToDate(months, days, hours)
    form.setValue('expired_time', newDate)
  }

  const handleKindChange = (value: string | null) => {
    const kind = redemptionKindSchema.parse(
      value ?? REDEMPTION_KINDS.PROMO_CREDIT
    )
    form.setValue('kind', kind, { shouldValidate: true })

    if (kind === REDEMPTION_KINDS.PAID_TOPUP) {
      form.setValue('source', REDEMPTION_SOURCES.LIANDONG, {
        shouldValidate: true,
      })
      form.setValue('count_as_topup', true, { shouldValidate: true })
      return
    }

    if (kind === REDEMPTION_KINDS.PROMO_CREDIT) {
      form.setValue('source', REDEMPTION_SOURCES.PROMO, {
        shouldValidate: true,
      })
      form.setValue('count_as_topup', false, { shouldValidate: true })
      return
    }

    form.setValue('source', REDEMPTION_SOURCES.MANUAL, { shouldValidate: true })
    form.setValue('count_as_topup', false, { shouldValidate: true })
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[600px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate
              ? t('Update Redemption Code')
              : t('Create Redemption Code')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the redemption code by providing necessary info.')
              : t(
                  'Add new redemption code(s) by providing necessary info.'
                )}{' '}
            {t('Click save when you&apos;re done.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='redemption-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='kind'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Card type')}</FormLabel>
                    <Select
                      items={redemptionKindOptions}
                      value={field.value}
                      onValueChange={handleKindChange}
                      disabled={isUpdate}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {redemptionKindOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {isUpdate
                        ? t('Type metadata cannot be changed after creation.')
                        : t(
                            'Choose how this redemption card should be billed.'
                          )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='source'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Source')}</FormLabel>
                    <Select
                      items={redemptionSourceOptions}
                      value={field.value}
                      onValueChange={(value) =>
                        value !== null && field.onChange(value)
                      }
                      disabled={isUpdate}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {redemptionSourceOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t('Where this redemption batch comes from.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='batch_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Batch ID')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('Leave empty to generate automatically')}
                        disabled={isUpdate}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Use the same batch ID for cards from one card shop export.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='amount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Face amount')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          step='0.01'
                          placeholder={t('Face amount')}
                          disabled={isUpdate}
                          onChange={(e) =>
                            field.onChange(parseFloat(e.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='money'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Paid money')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          step='0.01'
                          placeholder={t('Paid money')}
                          disabled={isUpdate}
                          onChange={(e) =>
                            field.onChange(parseFloat(e.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='count_as_topup'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                    <div className='flex flex-col gap-1'>
                      <FormLabel>{t('Count as paid top-up')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Paid top-up cards must be counted in wallet top-up statistics.'
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={isUpdate}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormDescription>
                      {t('Name for this redemption code (1-20 characters)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='quota_dollars'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{quotaLabel}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        step={tokensOnly ? 1 : 0.01}
                        placeholder={quotaPlaceholder}
                        onChange={(e) =>
                          field.onChange(parseFloat(e.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {tokensOnly
                        ? t('Enter the quota amount in tokens')
                        : t('Enter the quota amount in {{currency}}', {
                            currency: currencyLabel,
                          })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='expired_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <div className='flex flex-col gap-2'>
                      <FormControl>
                        <DateTimePicker
                          value={field.value}
                          onChange={field.onChange}
                          placeholder={t('Never expires')}
                        />
                      </FormControl>
                      <div className='grid grid-cols-4 gap-1.5 sm:flex sm:gap-2'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(0, 0, 0)}
                        >
                          {t('Never')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(1, 0, 0)}
                        >
                          {t('1M')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(0, 7, 0)}
                        >
                          {t('1W')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => handleSetExpiry(0, 1, 0)}
                        >
                          {t('1 Day')}
                        </Button>
                      </div>
                    </div>
                    <FormDescription>
                      {t('Leave empty for never expires')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && (
                <FormField
                  control={form.control}
                  name='count'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quantity')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          max='100'
                          placeholder={t('Number of codes to create')}
                          onChange={(e) =>
                            field.onChange(parseInt(e.target.value, 10) || 1)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Create multiple redemption codes at once (1-100)')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button form='redemption-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
