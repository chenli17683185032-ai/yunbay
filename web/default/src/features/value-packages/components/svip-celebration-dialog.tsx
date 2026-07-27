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
import { CrownIcon, SparklesIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'

interface SvipCelebrationDialogProps {
  open: boolean
  onClose: () => void
}

export function SvipCelebrationDialog(props: SvipCelebrationDialogProps) {
  const { t } = useTranslation()
  const shouldReduceMotion = useReducedMotion()

  return (
    <Dialog open={props.open} onOpenChange={(next) => !next && props.onClose()}>
      <DialogContent
        className='max-w-md overflow-hidden border-0 bg-transparent p-0 shadow-none'
        showCloseButton={false}
      >
        <AnimatePresence>
          {props.open && (
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
              className='yunbay-svip-dialog-card relative overflow-hidden rounded-3xl border p-6 shadow-2xl sm:p-7'
            >
              <div className='yunbay-svip-card-shine pointer-events-none absolute inset-0' />
              <div className='yunbay-svip-top-glow pointer-events-none absolute inset-x-0 top-0 h-28' />
              <div className='relative flex flex-col gap-5'>
                <div className='flex items-start justify-between gap-4'>
                  <div className='flex items-center gap-3'>
                    <motion.div
                      initial={
                        shouldReduceMotion ? false : { y: -12, scale: 0.5 }
                      }
                      animate={{ y: 0, scale: 1 }}
                      transition={
                        shouldReduceMotion
                          ? { duration: 0 }
                          : { delay: 0.15, type: 'spring', damping: 12 }
                      }
                      className='yunbay-svip-emblem flex size-12 items-center justify-center rounded-2xl shadow-lg'
                    >
                      <HugeiconsIcon
                        icon={CrownIcon}
                        className='size-6'
                        aria-hidden='true'
                      />
                    </motion.div>
                    <div>
                      <div className='yunbay-svip-kicker text-xs font-semibold uppercase'>
                        SVIP
                      </div>
                      <div className='yunbay-svip-title text-lg font-black'>
                        {t('Yunbei SVIP Black Gold Card')}
                      </div>
                    </div>
                  </div>
                  <HugeiconsIcon
                    icon={SparklesIcon}
                    className='yunbay-svip-accent size-5'
                    aria-hidden='true'
                  />
                </div>

                <div className='flex flex-col gap-2 py-1'>
                  <DialogTitle className='yunbay-svip-title text-2xl leading-tight font-black sm:text-3xl'>
                    {t('Congratulations on reaching SVIP status')}
                  </DialogTitle>
                  <DialogDescription className='yunbay-svip-copy text-sm leading-relaxed'>
                    {t(
                      'Your cumulative valid top-ups have reached 200 CNY. The exclusive SVIP black-gold glow is now active for your account.'
                    )}
                  </DialogDescription>
                </div>

                <Alert className='yunbay-svip-perk-alert'>
                  <HugeiconsIcon icon={SparklesIcon} aria-hidden='true' />
                  <AlertTitle>
                    {t('SVIP perk: enjoy an extra 25% off on top-ups')}
                  </AlertTitle>
                  <AlertDescription className='text-xs leading-relaxed'>
                    {t(
                      'After you top up, the admin will verify it and credit the discount to your account.'
                    )}
                  </AlertDescription>
                </Alert>

                <Button
                  type='button'
                  className='yunbay-svip-action w-full font-semibold'
                  onClick={props.onClose}
                >
                  <HugeiconsIcon
                    icon={CrownIcon}
                    data-icon='inline-start'
                    aria-hidden='true'
                  />
                  {t('Start the SVIP experience')}
                </Button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </DialogContent>
    </Dialog>
  )
}
