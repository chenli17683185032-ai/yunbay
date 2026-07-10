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
import type {
  GroupRatioOptionsResponse,
  GroupRatioOptionsSnapshot,
  UpdateGroupRatioOptionsRequest,
  UpdateOptionRequest,
  UpdateOptionResponse,
} from '../types'

export type { GroupRatioOptionsResponse } from '../types'

export type GroupRatioFormValues = {
  GroupRatio: string
  TopupGroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  AutoGroups: string
  DefaultUseAutoGroup: boolean
  GroupSpecialUsableGroup: string
}

export type GroupRatioOverrideRow = {
  userGroup: string
  overrides: Array<{ targetGroup: string; ratio: number }>
  isPackageGroup: boolean
  isVirtual: boolean
}

type PackageGroupQueryState = {
  isPending: boolean
  isError: boolean
  packageGroups?: string[]
}

export type PackageGroupDisplayState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; packageGroups: string[] }

export const GROUP_RATIO_REQUEST_CONFIG = {
  skipBusinessError: true,
  skipErrorHandler: true,
} as const

type GroupRatioSaveDependencies = {
  updateGroupRatioOptions: (
    request: UpdateGroupRatioOptionsRequest
  ) => Promise<GroupRatioOptionsResponse>
  updateSystemOption: (
    request: UpdateOptionRequest
  ) => Promise<UpdateOptionResponse>
  commitBaseline?: (baseline: GroupRatioFormValues) => void
}

export function getPackageGroupDisplayState(
  state: PackageGroupQueryState
): PackageGroupDisplayState {
  if (state.isPending) return { status: 'loading' }
  if (state.isError || !state.packageGroups) return { status: 'error' }
  return { status: 'ready', packageGroups: state.packageGroups }
}

const JSON_FIELDS: Array<
  Exclude<keyof GroupRatioFormValues, 'DefaultUseAutoGroup'>
> = [
  'GroupRatio',
  'TopupGroupRatio',
  'UserUsableGroups',
  'GroupGroupRatio',
  'AutoGroups',
  'GroupSpecialUsableGroup',
]

const GENERIC_OPTION_KEYS: Array<
  Exclude<keyof GroupRatioFormValues, 'GroupRatio' | 'GroupGroupRatio'>
> = [
  'TopupGroupRatio',
  'UserUsableGroups',
  'AutoGroups',
  'DefaultUseAutoGroup',
  'GroupSpecialUsableGroup',
]

const API_KEY_BY_FORM_KEY: Partial<Record<keyof GroupRatioFormValues, string>> =
  {
    GroupSpecialUsableGroup: 'group_ratio_setting.group_special_usable_group',
  }

function normalizeJson(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''

  try {
    return JSON.stringify(JSON.parse(trimmed))
  } catch {
    return trimmed
  }
}

function normalizeGroupRatioJson(value: string): string {
  const normalized = normalizeJson(value)
  if (!normalized) return normalized

  const parsed = JSON.parse(normalized) as unknown
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    return normalized
  }

  const result: Record<string, unknown> = {}
  for (const [rawGroup, ratio] of Object.entries(parsed)) {
    const group = rawGroup.trim()
    if (!group) throw new Error('Group ratio name must not be empty')
    if (Object.prototype.hasOwnProperty.call(result, group)) {
      throw new Error(`Group ratio name conflicts after trimming: ${group}`)
    }
    result[group] = ratio
  }
  return JSON.stringify(result)
}

function normalizeGroupGroupRatioJson(value: string): string {
  const normalized = normalizeJson(value)
  if (!normalized) return normalized

  const parsed = JSON.parse(normalized) as unknown
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    return normalized
  }

  const result: Record<string, Record<string, unknown>> = {}
  const seenParents = new Set<string>()
  for (const [rawParent, rawChildren] of Object.entries(parsed)) {
    const parent = rawParent.trim()
    if (!parent) throw new Error('Group group ratio parent must not be empty')
    if (seenParents.has(parent)) {
      throw new Error(
        `Group group ratio parent conflicts after trimming: ${parent}`
      )
    }
    seenParents.add(parent)
    if (
      !rawChildren ||
      Array.isArray(rawChildren) ||
      typeof rawChildren !== 'object'
    ) {
      return normalized
    }

    const children: Record<string, unknown> = {}
    for (const [rawChild, ratio] of Object.entries(rawChildren)) {
      const child = rawChild.trim()
      if (!child) {
        throw new Error(`Group group ratio child must not be empty: ${parent}`)
      }
      if (Object.prototype.hasOwnProperty.call(children, child)) {
        throw new Error(
          `Group group ratio child conflicts after trimming: ${parent}/${child}`
        )
      }
      children[child] = ratio
    }
    if (Object.keys(children).length > 0) result[parent] = children
  }
  return JSON.stringify(result)
}

function normalizeFormValues(
  values: GroupRatioFormValues
): GroupRatioFormValues {
  const normalized = { ...values }
  for (const key of JSON_FIELDS) {
    normalized[key] = normalizeJson(values[key])
  }
  normalized.GroupRatio = normalizeGroupRatioJson(values.GroupRatio)
  normalized.GroupGroupRatio = normalizeGroupGroupRatioJson(
    values.GroupGroupRatio
  )
  return normalized
}

function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalize)
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, child]) => [key, canonicalize(child)])
    )
  }
  return value
}

function jsonMapsMatch(left: string, right: string): boolean {
  return (
    JSON.stringify(canonicalize(JSON.parse(left))) ===
    JSON.stringify(canonicalize(JSON.parse(right)))
  )
}

export function requireSuccessfulOptionResponse<
  T extends { success: boolean; message?: string },
>(response: T): T {
  if (!response.success) {
    throw new Error(response.message || 'Option request failed')
  }
  return response
}

export function requireGroupRatioOptionsData(
  response: GroupRatioOptionsResponse
): asserts response is GroupRatioOptionsResponse & {
  data: GroupRatioOptionsSnapshot
} {
  requireSuccessfulOptionResponse(response)
  if (!response.data) {
    throw new Error('Missing group ratio snapshot')
  }
}

export async function saveGroupRatioChanges(
  values: GroupRatioFormValues,
  baseline: GroupRatioFormValues,
  dependencies: GroupRatioSaveDependencies
): Promise<{ changed: boolean; baseline: GroupRatioFormValues }> {
  const submitted = normalizeFormValues(values)
  const normalizedBaseline = normalizeFormValues(baseline)
  const pairChanged =
    submitted.GroupRatio !== normalizedBaseline.GroupRatio ||
    submitted.GroupGroupRatio !== normalizedBaseline.GroupGroupRatio
  const genericChanges = GENERIC_OPTION_KEYS.filter(
    (key) => submitted[key] !== normalizedBaseline[key]
  )

  if (!pairChanged && genericChanges.length === 0) {
    return { changed: false, baseline }
  }

  const nextBaseline = { ...normalizedBaseline }

  if (pairChanged) {
    const response = await dependencies.updateGroupRatioOptions({
      group_ratio: submitted.GroupRatio,
      group_group_ratio: submitted.GroupGroupRatio,
    })
    requireGroupRatioOptionsData(response)

    const serverGroupRatio = normalizeJson(response.data.group_ratio)
    const serverGroupGroupRatio = normalizeJson(response.data.group_group_ratio)
    if (
      !jsonMapsMatch(serverGroupRatio, submitted.GroupRatio) ||
      !jsonMapsMatch(serverGroupGroupRatio, submitted.GroupGroupRatio)
    ) {
      throw new Error(
        'Group ratio server readback does not match normalized submission'
      )
    }

    nextBaseline.GroupRatio = serverGroupRatio
    nextBaseline.GroupGroupRatio = serverGroupGroupRatio
  }

  for (const key of genericChanges) {
    const response = await dependencies.updateSystemOption({
      key: API_KEY_BY_FORM_KEY[key] || key,
      value: submitted[key],
    })
    requireSuccessfulOptionResponse(response)
    Object.assign(nextBaseline, { [key]: submitted[key] })
  }

  dependencies.commitBaseline?.(nextBaseline)
  return { changed: true, baseline: nextBaseline }
}

export function buildGroupRatioOverrideRows(
  value: string,
  packageGroups: string[]
): GroupRatioOverrideRow[] {
  const parsed = (() => {
    try {
      return JSON.parse(value) as Record<string, Record<string, number>>
    } catch {
      return {}
    }
  })()

  const normalizedPackageGroups = new Set(
    packageGroups.map((group) => group.trim()).filter(Boolean)
  )
  const rows = Object.entries(parsed).map(([userGroup, overrides]) => ({
    userGroup,
    overrides: Object.entries(overrides || {}).map(([targetGroup, ratio]) => ({
      targetGroup,
      ratio,
    })),
    isPackageGroup: normalizedPackageGroups.has(userGroup),
    isVirtual: false,
  }))

  for (const userGroup of normalizedPackageGroups) {
    if (Object.prototype.hasOwnProperty.call(parsed, userGroup)) continue
    rows.push({
      userGroup,
      overrides: [],
      isPackageGroup: true,
      isVirtual: true,
    })
  }

  return rows
}

export function serializeGroupRatioOverrideRows(
  rows: GroupRatioOverrideRow[]
): string {
  const serialized: Record<string, Record<string, number>> = {}

  for (const row of rows) {
    const userGroup = row.userGroup.trim()
    if (!userGroup) continue
    if (row.overrides.length === 0) {
      if (!row.isVirtual) serialized[userGroup] = {}
      continue
    }

    const overrides: Record<string, number> = {}
    for (const override of row.overrides) {
      const targetGroup = override.targetGroup.trim()
      if (!targetGroup) continue
      overrides[targetGroup] = override.ratio
    }
    if (Object.keys(overrides).length > 0) {
      serialized[userGroup] = overrides
    }
  }

  return JSON.stringify(serialized, null, 2)
}
