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
import { useNavigate } from '@tanstack/react-router'
import { ResetPasswordIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { PasswordInput } from '@/components/password-input'
import { resetPassword } from '@/features/auth/api'
import { resetPasswordConfirmFormSchema } from '@/features/auth/constants'
import { AuthLayout } from '../auth-layout'

export type ResetPasswordSearchParams = {
  email?: string
  token?: string
}

type ResetPasswordConfirmProps = ResetPasswordSearchParams

export function ResetPasswordConfirm({
  email,
  token,
}: ResetPasswordConfirmProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const form = useForm<z.infer<typeof resetPasswordConfirmFormSchema>>({
    resolver: zodResolver(resetPasswordConfirmFormSchema),
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  })

  const isValidResetLink = Boolean(email && token)

  async function onSubmit(
    data: z.infer<typeof resetPasswordConfirmFormSchema>
  ) {
    const resetEmail = email
    const resetToken = token
    if (!isValidResetLink || !resetEmail || !resetToken) {
      toast.error(t('Invalid reset link, please request a new password reset'))
      return
    }

    setLoading(true)
    try {
      const res = await resetPassword({
        email: resetEmail,
        token: resetToken,
        password: data.password,
      })

      if (res?.success) {
        toast.success(t('Password updated successfully'))
        navigate({ to: '/sign-in', replace: true })
      } else {
        toast.error(res?.message || t('Failed to reset password'))
      }
    } catch {
      // Errors handled by global interceptor
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('Reset password')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t('Set a new password for your account.')}
          </p>
        </div>

        <Form {...form}>
          <form
            className='flex flex-col gap-4'
            onSubmit={form.handleSubmit(onSubmit)}
          >
            {!isValidResetLink && (
              <Alert variant='destructive'>
                <AlertDescription>
                  {t(
                    'Invalid reset link, please request a new password reset.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            <div className='grid gap-2'>
              <Label htmlFor='reset-email'>{t('Email')}</Label>
              <Input
                id='reset-email'
                type='email'
                value={email || ''}
                disabled
                placeholder={t('Waiting for email...')}
              />
            </div>

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

            <Button
              type='submit'
              className='w-full'
              disabled={loading || !isValidResetLink}
            >
              {loading ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <HugeiconsIcon
                  icon={ResetPasswordIcon}
                  data-icon='inline-start'
                />
              )}
              {t('Reset password')}
            </Button>

            <Button
              type='button'
              variant='link'
              className='w-full'
              onClick={() => navigate({ to: '/sign-in', replace: true })}
            >
              {t('Back to login')}
            </Button>
          </form>
        </Form>
      </div>
    </AuthLayout>
  )
}
