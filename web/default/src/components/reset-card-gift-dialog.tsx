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
import { Refresh01Icon, SparklesIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'

export interface ResetCardGiftCelebration {
  /** 张数（>= 1） */
  count: number
  /** 触发来源套餐名，仅展示用 */
  planTitle?: string
  /** 兑换码直接兑换（true）还是开通套餐赠送（false） */
  fromRedemption?: boolean
}

interface ResetCardGiftDialogProps {
  celebration: ResetCardGiftCelebration | null
  onClose: () => void
}

export function ResetCardGiftDialog(props: ResetCardGiftDialogProps) {
  const { t } = useTranslation()
  const shouldReduceMotion = useReducedMotion()
  const open = Boolean(props.celebration)
  const count = props.celebration?.count ?? 0

  return (
    <Dialog open={open} onOpenChange={(next) => !next && props.onClose()}>
      <DialogContent
        className='max-w-sm overflow-hidden p-0'
        showCloseButton={false}
      >
        <AnimatePresence>
          {open && (
            <motion.div
              initial={
                shouldReduceMotion
                  ? { opacity: 0 }
                  : { rotateY: 90, scale: 0.6, opacity: 0 }
              }
              animate={{ rotateY: 0, scale: 1, opacity: 1 }}
              exit={
                shouldReduceMotion ? { opacity: 0 } : { scale: 0.8, opacity: 0 }
              }
              transition={
                shouldReduceMotion
                  ? { duration: 0 }
                  : { type: 'spring', damping: 16, stiffness: 220 }
              }
              style={{ transformPerspective: 900 }}
              className='relative flex flex-col items-center gap-4 px-8 py-10 text-center'
            >
              <motion.div
                initial={shouldReduceMotion ? false : { y: -12, scale: 0.5 }}
                animate={{ y: 0, scale: 1 }}
                transition={
                  shouldReduceMotion
                    ? { duration: 0 }
                    : { delay: 0.15, type: 'spring', damping: 12 }
                }
                className='bg-primary text-primary-foreground flex size-16 items-center justify-center rounded-full shadow-lg'
              >
                <HugeiconsIcon
                  icon={Refresh01Icon}
                  className='size-8'
                  aria-hidden='true'
                />
              </motion.div>
              <DialogTitle className='flex max-w-full items-center justify-center gap-2 text-center text-lg font-semibold'>
                <HugeiconsIcon
                  icon={SparklesIcon}
                  className='text-primary size-4 shrink-0'
                  aria-hidden='true'
                />
                <span className='min-w-0'>
                  {props.celebration?.fromRedemption
                    ? t('You received {{count}} reset card(s)!', { count })
                    : t('Bonus: {{count}} reset card(s)!', { count })}
                </span>
              </DialogTitle>
              <DialogDescription className='text-muted-foreground text-sm'>
                {t(
                  'You can use it to reset your package quota when it runs out.'
                )}
                {props.celebration?.planTitle &&
                  !props.celebration.fromRedemption && (
                    <span className='mt-1 block'>
                      {t('From plan: {{plan}}', {
                        plan: props.celebration.planTitle,
                      })}
                    </span>
                  )}
              </DialogDescription>
              <Button
                type='button'
                className='mt-2 w-full'
                onClick={props.onClose}
              >
                {t('Done')}
              </Button>
            </motion.div>
          )}
        </AnimatePresence>
      </DialogContent>
    </Dialog>
  )
}
