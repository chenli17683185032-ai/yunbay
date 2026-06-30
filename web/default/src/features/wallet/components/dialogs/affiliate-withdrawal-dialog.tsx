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
import { useTranslation } from 'react-i18next'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { Dialog } from '@/components/dialog'
import {
  normalizeAffiliateWithdrawalAmount,
  validateAffiliateWithdrawalInput,
} from '../../lib'

interface AffiliateWithdrawalDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  availableMoney: number
  submitting: boolean
  onConfirm: (
    amount: number,
    contact: string,
    remark?: string
  ) => Promise<boolean>
}

export function AffiliateWithdrawalDialog(
  props: AffiliateWithdrawalDialogProps
) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState(0)
  const [contact, setContact] = useState('')
  const [remark, setRemark] = useState('')
  const [errorKey, setErrorKey] = useState<string | null>(null)

  useEffect(() => {
    if (props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAmount(normalizeAffiliateWithdrawalAmount(props.availableMoney))
      setContact('')
      setRemark('')
      setErrorKey(null)
    }
  }, [props.availableMoney, props.open])

  const handleConfirm = async () => {
    const validationError = validateAffiliateWithdrawalInput(
      amount,
      props.availableMoney,
      contact
    )
    if (validationError) {
      setErrorKey(validationError)
      return
    }

    const success = await props.onConfirm(amount, contact, remark)
    if (success) {
      props.onOpenChange(false)
    }
  }

  const amountErrorKey =
    errorKey === 'Withdrawal amount must be greater than 0' ||
    errorKey === 'Withdrawal amount exceeds available rewards'
      ? errorKey
      : null
  const contactErrorKey =
    errorKey === 'Withdrawal contact is required' ? errorKey : null

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Apply for Withdrawal')}
      description={t(
        'Submit a withdrawal request for your referral rewards. The administrator will contact you for manual payout.'
      )}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      titleClassName='text-xl font-semibold'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      contentHeight='auto'
      bodyClassName='flex flex-col gap-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.submitting}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={props.submitting}>
            {props.submitting ? <Spinner data-icon='inline-start' /> : null}
            {t('Submit Request')}
          </Button>
        </>
      }
    >
      <Alert>
        <AlertDescription>
          {t('Available Rewards')}: {formatCurrencyFromUSD(props.availableMoney)}
          . {t('After submitting, this amount will be frozen')}
        </AlertDescription>
      </Alert>

      <FieldGroup>
        <Field data-invalid={Boolean(amountErrorKey)}>
          <FieldLabel htmlFor='affiliate-withdrawal-amount'>
            {t('Withdrawal Amount')}
          </FieldLabel>
          <Input
            id='affiliate-withdrawal-amount'
            type='number'
            min={0}
            max={props.availableMoney}
            step='0.01'
            value={amount}
            onChange={(event) => {
              setAmount(Number(event.target.value))
              setErrorKey(null)
            }}
            aria-invalid={Boolean(amountErrorKey)}
            className='font-mono text-lg'
          />
          <FieldDescription>
            {t('Maximum available')}: {formatCurrencyFromUSD(props.availableMoney)}
          </FieldDescription>
          <FieldError>{amountErrorKey ? t(amountErrorKey) : null}</FieldError>
        </Field>

        <Field data-invalid={Boolean(contactErrorKey)}>
          <FieldLabel htmlFor='affiliate-withdrawal-contact'>
            {t('Contact information')}
          </FieldLabel>
          <Input
            id='affiliate-withdrawal-contact'
            value={contact}
            onChange={(event) => {
              setContact(event.target.value)
              setErrorKey(null)
            }}
            placeholder={t('Alipay, WeChat, email, or other payout contact')}
            aria-invalid={Boolean(contactErrorKey)}
          />
          <FieldDescription>
            {t('The administrator will use this contact to process the payout.')}
          </FieldDescription>
          <FieldError>{contactErrorKey ? t(contactErrorKey) : null}</FieldError>
        </Field>

        <Field>
          <FieldLabel htmlFor='affiliate-withdrawal-remark'>
            {t('Remark')}
          </FieldLabel>
          <Textarea
            id='affiliate-withdrawal-remark'
            value={remark}
            onChange={(event) => setRemark(event.target.value)}
            placeholder={t('Optional note for the administrator')}
          />
        </Field>
      </FieldGroup>
    </Dialog>
  )
}
