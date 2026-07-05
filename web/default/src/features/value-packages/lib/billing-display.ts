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
import type { ValuePackageState } from '../types'

export function getActiveValuePackageBillingRatio(
  state: ValuePackageState | null | undefined
): number | undefined {
  const ratio = state?.billing?.active ? state.billing.effective_ratio : undefined
  return typeof ratio === 'number' && Number.isFinite(ratio) ? ratio : undefined
}

export function getActiveValuePackageBillingLabel(
  state: ValuePackageState | null | undefined
): string | null {
  if (!state?.billing?.active) return null
  const title = state.billing.plan_title?.trim()
  const group = state.billing.package_group?.trim()
  if (title && group) return `${title} · ${group}`
  return title || group || null
}
