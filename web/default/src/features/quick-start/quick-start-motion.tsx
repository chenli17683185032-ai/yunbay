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
import { Check, CheckCircle2 } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { cn } from '@/lib/utils'
import {
  QUICK_START_APPLE_EASE,
  QUICK_START_REDUCED_TRANSITION,
  QUICK_START_SPRING_TRANSITION,
} from './quick-start-motion-config'

export function QuickStartSelectionSurface(props: {
  layoutId: string
  reducedMotion: boolean
}) {
  return (
    <motion.span
      aria-hidden='true'
      data-quick-start-selection={props.layoutId}
      layoutId={props.reducedMotion ? undefined : props.layoutId}
      initial={props.reducedMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={
        props.reducedMotion
          ? QUICK_START_REDUCED_TRANSITION
          : QUICK_START_SPRING_TRANSITION
      }
      className='pointer-events-none absolute inset-0 z-0 rounded-[inherit] border border-white/30 bg-white/[0.1] shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_18px_56px_rgba(0,0,0,0.18)]'
    />
  )
}

export function QuickStartSelectionCheck(props: {
  visible: boolean
  reducedMotion: boolean
  className?: string
}) {
  return (
    <AnimatePresence initial={false}>
      {props.visible ? (
        <motion.span
          key='selected'
          aria-hidden='true'
          initial={
            props.reducedMotion
              ? { opacity: 0 }
              : { opacity: 0, scale: 0.72, y: 4 }
          }
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={
            props.reducedMotion
              ? { opacity: 0 }
              : { opacity: 0, scale: 0.82, y: -2 }
          }
          transition={
            props.reducedMotion
              ? QUICK_START_REDUCED_TRANSITION
              : QUICK_START_SPRING_TRANSITION
          }
          className={cn(
            'flex shrink-0 items-center justify-center',
            props.className
          )}
        >
          <CheckCircle2 className='size-full' />
        </motion.span>
      ) : null}
    </AnimatePresence>
  )
}

export function QuickStartStepMarker(props: {
  step: string
  complete: boolean
  reducedMotion: boolean
  className?: string
}) {
  return (
    <motion.span
      data-quick-start-step-marker={props.step}
      layout={props.reducedMotion ? false : 'position'}
      transition={
        props.reducedMotion
          ? QUICK_START_REDUCED_TRANSITION
          : QUICK_START_SPRING_TRANSITION
      }
      className={cn(
        'relative flex shrink-0 items-center justify-center overflow-hidden border font-mono font-semibold transition-colors duration-300',
        props.complete
          ? 'border-white/24 bg-white text-[#030409]'
          : 'border-white/12 bg-white/[0.04] text-white/52',
        props.className
      )}
    >
      <AnimatePresence initial={false} mode='wait'>
        <motion.span
          key={props.complete ? 'complete' : props.step}
          initial={
            props.reducedMotion
              ? { opacity: 0 }
              : { opacity: 0, scale: 0.68, y: 5 }
          }
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={
            props.reducedMotion
              ? { opacity: 0 }
              : { opacity: 0, scale: 0.78, y: -4 }
          }
          transition={
            props.reducedMotion
              ? QUICK_START_REDUCED_TRANSITION
              : { duration: 0.22, ease: QUICK_START_APPLE_EASE }
          }
          className='absolute inset-0 flex items-center justify-center'
        >
          {props.complete ? (
            <Check className='size-4' aria-hidden='true' />
          ) : (
            props.step
          )}
        </motion.span>
      </AnimatePresence>
    </motion.span>
  )
}
