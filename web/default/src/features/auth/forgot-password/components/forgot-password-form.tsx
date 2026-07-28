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
import type { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useNavigate } from '@tanstack/react-router'
import { MailSend01Icon, ResetPasswordIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useCountdown } from '@/hooks/use-countdown'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp'
import { Spinner } from '@/components/ui/spinner'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import { resetPassword, sendPasswordResetEmail } from '@/features/auth/api'
import {
  forgotPasswordFormSchema,
  OTP_LENGTH,
  PASSWORD_RESET_COUNTDOWN,
} from '@/features/auth/constants'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'

export function ForgotPasswordForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [isSendingCode, setIsSendingCode] = useState(false)
  const [isResetting, setIsResetting] = useState(false)

  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()
  const {
    secondsLeft,
    isActive,
    start: startCountdown,
  } = useCountdown({ initialSeconds: PASSWORD_RESET_COUNTDOWN })

  const form = useForm<z.infer<typeof forgotPasswordFormSchema>>({
    resolver: zodResolver(forgotPasswordFormSchema),
    defaultValues: {
      email: '',
      code: '',
      password: '',
      confirmPassword: '',
    },
  })
  const emailValue = form.watch('email')
  const turnstileReady = !isTurnstileEnabled || Boolean(turnstileToken)

  async function onSubmit(data: z.infer<typeof forgotPasswordFormSchema>) {
    setIsResetting(true)
    try {
      const res = await resetPassword({
        email: data.email,
        token: data.code,
        password: data.password,
      })
      if (res?.success) {
        toast.success(t('Password updated successfully'))
        navigate({ to: '/sign-in', replace: true })
      } else {
        toast.error(res?.message || t('Failed to reset password'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsResetting(false)
    }
  }

  async function handleSendCode() {
    const emailIsValid = await form.trigger('email')
    if (!emailIsValid || !validateTurnstile()) return

    setIsSendingCode(true)
    try {
      const res = await sendPasswordResetEmail(emailValue, turnstileToken)
      if (res?.success) {
        startCountdown()
        toast.success(t('Verification code sent! Please check your email.'))
      } else {
        toast.error(res?.message || t('Failed to send verification code'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsSendingCode(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Email')}</FormLabel>
              <FormControl>
                <Input
                  type='email'
                  autoComplete='email'
                  placeholder='name@example.com'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='code'
          render={({ field }) => (
            <FormItem>
              <div className='flex items-center justify-between gap-3'>
                <FormLabel>{t('Verification code')}</FormLabel>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={
                    isSendingCode ||
                    isResetting ||
                    isActive ||
                    !emailValue ||
                    !turnstileReady
                  }
                  onClick={handleSendCode}
                >
                  {isSendingCode ? (
                    <Spinner data-icon='inline-start' />
                  ) : (
                    <HugeiconsIcon
                      icon={MailSend01Icon}
                      data-icon='inline-start'
                    />
                  )}
                  {isActive
                    ? t('Resend ({{seconds}}s)', { seconds: secondsLeft })
                    : t('Send code')}
                </Button>
              </div>
              <FormControl>
                <InputOTP
                  maxLength={OTP_LENGTH}
                  autoComplete='one-time-code'
                  containerClassName='w-full'
                  {...field}
                >
                  <InputOTPGroup className='w-full'>
                    <InputOTPSlot className='h-9 flex-1' index={0} />
                    <InputOTPSlot className='h-9 flex-1' index={1} />
                    <InputOTPSlot className='h-9 flex-1' index={2} />
                    <InputOTPSlot className='h-9 flex-1' index={3} />
                    <InputOTPSlot className='h-9 flex-1' index={4} />
                    <InputOTPSlot className='h-9 flex-1' index={5} />
                  </InputOTPGroup>
                </InputOTP>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('New password')}</FormLabel>
              <FormControl>
                <PasswordInput
                  autoComplete='new-password'
                  placeholder={t('Enter password (8-20 characters)')}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='confirmPassword'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Confirm password')}</FormLabel>
              <FormControl>
                <PasswordInput
                  autoComplete='new-password'
                  placeholder={t('Confirm password')}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {isTurnstileEnabled && (
          <div className='mt-2'>
            <Turnstile
              siteKey={turnstileSiteKey}
              onVerify={setTurnstileToken}
            />
          </div>
        )}

        <Button type='submit' className='mt-2 w-full' disabled={isResetting}>
          {isResetting ? (
            <Spinner data-icon='inline-start' />
          ) : (
            <HugeiconsIcon icon={ResetPasswordIcon} data-icon='inline-start' />
          )}
          {t('Reset password')}
        </Button>

        <Button
          type='button'
          variant='link'
          className='w-full'
          render={<Link to='/sign-in' />}
          nativeButton={false}
        >
          {t('Back to login')}
        </Button>
      </form>
    </Form>
  )
}
