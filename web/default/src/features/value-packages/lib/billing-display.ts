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
