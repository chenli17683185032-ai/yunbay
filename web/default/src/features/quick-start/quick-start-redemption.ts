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

type QuickStartRedemptionResponse = {
  success?: boolean
  message?: string
  data?: number
}

type QuickStartRedemptionDependencies = {
  redeemTopupCode: (request: {
    key: string
  }) => Promise<QuickStartRedemptionResponse>
  refreshSelf: () => Promise<void>
}

type QuickStartRedemptionResult = {
  quotaAdded: number
  refreshed: boolean
}

export async function redeemQuickStartCode(
  code: string,
  dependencies: QuickStartRedemptionDependencies
): Promise<QuickStartRedemptionResult> {
  const key = code.trim()
  if (!key) {
    throw new Error('Please enter a redemption code')
  }

  const response = await dependencies.redeemTopupCode({ key })
  if (response.success !== true) {
    throw new Error(response.message || 'Redemption failed')
  }

  let refreshed = false
  try {
    await dependencies.refreshSelf()
    refreshed = true
  } catch {
    refreshed = false
  }

  return {
    quotaAdded: Number(response.data || 0),
    refreshed,
  }
}
