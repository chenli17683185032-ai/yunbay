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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleQuestionMarkIcon, ZapIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getValuePackageSelf,
  updateValuePackageWalletFallback,
} from '@/features/value-packages/api'
import { valuePackageSelfQueryKey } from '@/features/value-packages/query-keys'

const toggleId = 'value-package-wallet-fallback'

export function ApiKeysWalletFallbackToggle() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: valuePackageSelfQueryKey,
    staleTime: 10_000,
    refetchOnWindowFocus: true,
    queryFn: async () => {
      const response = await getValuePackageSelf()
      return response.success ? response.data || null : null
    },
  })
  const mutation = useMutation({
    mutationFn: updateValuePackageWalletFallback,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to update setting'))
        return
      }
      queryClient.setQueryData(valuePackageSelfQueryKey, response.data)
      toast.success(t('Setting saved'))
    },
    onError: () => toast.error(t('Failed to update setting')),
  })

  const savedEnabled = data?.preference?.wallet_fallback_enabled !== false
  const checked = mutation.isPending ? mutation.variables : savedEnabled
  const description = t(
    'Use value package quota first, then wallet balance at your VIP rate, and switch back automatically when package quota recovers.'
  )

  return (
    <div className='bg-background flex h-9 shrink-0 items-center gap-2 rounded-md border px-2.5'>
      <HugeiconsIcon
        icon={ZapIcon}
        className='text-primary size-4'
        aria-hidden='true'
      />
      <label
        htmlFor={toggleId}
        className='cursor-pointer text-sm font-medium whitespace-nowrap'
      >
        {t('Uninterrupted Boost')}
      </label>
      <div className='hidden sm:block'>
        <TooltipProvider delay={100}>
          <Tooltip>
            <TooltipTrigger
              render={
                <button
                  type='button'
                  className='text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex size-6 items-center justify-center rounded-sm outline-none focus-visible:ring-2'
                  aria-label={description}
                />
              }
            >
              <HugeiconsIcon
                icon={CircleQuestionMarkIcon}
                className='size-4'
                aria-hidden='true'
              />
            </TooltipTrigger>
            <TooltipContent className='max-w-72 text-xs'>
              {description}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <div className='sm:hidden'>
        <Popover>
          <PopoverTrigger
            render={
              <button
                type='button'
                className='text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex size-6 items-center justify-center rounded-sm outline-none focus-visible:ring-2'
                aria-label={description}
              />
            }
          >
            <HugeiconsIcon
              icon={CircleQuestionMarkIcon}
              className='size-4'
              aria-hidden='true'
            />
          </PopoverTrigger>
          <PopoverContent
            className='w-[min(18rem,calc(100vw-2rem))]'
            collisionPadding={8}
          >
            <PopoverDescription className='text-xs leading-relaxed'>
              {description}
            </PopoverDescription>
          </PopoverContent>
        </Popover>
      </div>
      <Switch
        id={toggleId}
        size='sm'
        checked={checked}
        disabled={isLoading || mutation.isPending}
        onCheckedChange={(enabled) => mutation.mutate(enabled)}
        aria-label={t('Uninterrupted Boost')}
      />
    </div>
  )
}
