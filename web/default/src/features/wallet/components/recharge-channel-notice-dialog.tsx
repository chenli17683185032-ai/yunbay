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
import { AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import rechargeGroupQrCode from '../assets/wechat-recharge-group-20260815.jpg'

export const RECHARGE_CHANNEL_NOTICE =
  '因充值渠道出问题，请加微信群聊联系管理员进行充值。'

interface RechargeChannelNoticeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function RechargeChannelNoticeDialog({
  open,
  onOpenChange,
}: RechargeChannelNoticeDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title='充值渠道临时公告'
      description={RECHARGE_CHANNEL_NOTICE}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-[460px]'
      descriptionClassName='text-foreground text-sm leading-relaxed'
      bodyClassName='flex flex-col items-center gap-4'
      showCloseButton
      footer={
        <Button
          className='w-full sm:w-auto'
          onClick={() => onOpenChange(false)}
        >
          我知道了
        </Button>
      }
    >
      <div className='border-destructive/25 bg-destructive/5 text-destructive flex w-full items-start gap-2.5 rounded-xl border px-3 py-3 text-sm leading-relaxed'>
        <AlertTriangle className='mt-0.5 size-4 shrink-0' aria-hidden='true' />
        <span>{RECHARGE_CHANNEL_NOTICE}</span>
      </div>
      <div className='bg-background w-full max-w-[320px] overflow-hidden rounded-2xl border p-2 shadow-sm'>
        <img
          src={rechargeGroupQrCode}
          alt='云贝技术交流3群微信群二维码'
          className='h-auto w-full rounded-xl object-contain'
        />
      </div>
      <p className='text-muted-foreground text-center text-xs leading-relaxed'>
        请使用微信扫码加入群聊，并联系群内管理员处理充值。
      </p>
    </Dialog>
  )
}
