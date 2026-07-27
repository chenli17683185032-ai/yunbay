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
import { api, type ApiRequestConfig } from '@/lib/api'
import type {
  ApiResponse,
  ValuePackageLdxpSessionResponse,
  ValuePackagePlansResponse,
  ValuePackagePurchaseIntent,
  ValuePackageState,
} from './types'

export async function getValuePackagePlans(): Promise<
  ApiResponse<ValuePackagePlansResponse>
> {
  const res = await api.get('/api/value-packages/plans')
  return res.data
}

export async function getValuePackageSelf(): Promise<
  ApiResponse<ValuePackageState>
> {
  const res = await api.get('/api/value-packages/self')
  return res.data
}

export async function getValuePackagePurchaseIntent(
  planId: number,
  confirmedCover = false
): Promise<ApiResponse<ValuePackagePurchaseIntent>> {
  const res = await api.get(
    `/api/value-packages/plans/${planId}/purchase-intent`,
    {
      params: { confirmed_cover: confirmedCover },
      skipBusinessError: true,
    } satisfies ApiRequestConfig
  )
  return res.data
}

export async function createValuePackageLdxpSession(
  planId: number,
  confirmedCover: boolean
): Promise<ApiResponse<ValuePackageLdxpSessionResponse>> {
  const res = await api.post(
    `/api/value-packages/plans/${planId}/ldxp/session`,
    { confirmed_cover: confirmedCover },
    { skipBusinessError: true } satisfies ApiRequestConfig
  )
  return res.data
}

export async function activateValuePackage(
  userSubscriptionId: number
): Promise<ApiResponse<ValuePackageState>> {
  const res = await api.post('/api/value-packages/activate', {
    user_subscription_id: userSubscriptionId,
  })
  return res.data
}

export async function deactivateValuePackage(): Promise<
  ApiResponse<ValuePackageState>
> {
  const res = await api.post('/api/value-packages/deactivate')
  return res.data
}

export async function updateValuePackageWalletFallback(
  enabled: boolean
): Promise<ApiResponse<ValuePackageState>> {
  const res = await api.put('/api/value-packages/wallet-fallback', { enabled })
  return res.data
}

export async function resetValuePackageQuota(
  userSubscriptionId?: number
): Promise<ApiResponse<ValuePackageState>> {
  const payload =
    userSubscriptionId && userSubscriptionId > 0
      ? { user_subscription_id: userSubscriptionId }
      : {}
  const res = await api.post('/api/value-packages/reset-quota', payload)
  return res.data
}

export async function markVipUpgradeModalSeen(): Promise<
  ApiResponse<{ vip_upgrade_modal_seen: boolean }>
> {
  const res = await api.post('/api/user/vip-upgrade-modal/seen')
  return res.data
}

export async function markSvipCelebrationSeen(): Promise<
  ApiResponse<{ svip_celebration_seen: boolean }>
> {
  const res = await api.post('/api/user/svip-celebration/seen')
  return res.data
}
