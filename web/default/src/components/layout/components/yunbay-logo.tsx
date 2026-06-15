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
import { cn } from '@/lib/utils'

type YunbayLogoProps = {
  className?: string
}

export function YunbayLogo(props: YunbayLogoProps) {
  return (
    <span
      aria-hidden='true'
      className={cn(
        'relative inline-flex size-8 items-center justify-center overflow-hidden rounded-[1rem] border border-white/18 bg-[radial-gradient(circle_at_50%_0%,rgba(238,244,255,0.92),rgba(163,186,255,0.22)_34%,rgba(4,7,18,0.96)_70%),linear-gradient(145deg,rgba(255,255,255,0.09),rgba(255,255,255,0.02))] text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.34),inset_0_-1px_0_rgba(255,255,255,0.08),0_16px_42px_rgba(110,139,225,0.22)]',
        props.className
      )}
    >
      <span className='pointer-events-none absolute inset-[3px] rounded-[0.78rem] border border-white/8' />
      <svg
        viewBox='0 0 32 32'
        className='relative size-[25px] drop-shadow-[0_0_12px_rgba(219,230,255,0.42)]'
      >
        <path
          d='M7.8 15.5c.2-2.45 2.05-4.13 4.28-4.13.88-1.92 2.88-2.9 4.95-2.34 2.34.64 3.62 2.54 3.23 4.63 2.12.14 3.55 1.42 3.55 3.05 0 1.78-1.45 2.88-3.55 2.88H9.45'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeLinejoin='round'
          strokeWidth='1.42'
          opacity='0.88'
        />
        <path
          d='M6.1 23.1c4.42-3.54 15.38-3.54 19.8 0'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeWidth='1.34'
          opacity='0.7'
        />
        <path
          d='M9.45 21.32c3.2-1.92 9.9-1.92 13.1 0'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeWidth='1.18'
          opacity='0.42'
        />
        <path
          d='M16 20.55v-6.3'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeWidth='1.3'
          opacity='0.82'
        />
        <path
          d='M13.82 17.35 16 14.25l2.18 3.1'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeLinejoin='round'
          strokeWidth='1.04'
          opacity='0.44'
        />
        <circle
          cx='16'
          cy='13.1'
          r='1.15'
          fill='currentColor'
          opacity='0.92'
        />
      </svg>
    </span>
  )
}
