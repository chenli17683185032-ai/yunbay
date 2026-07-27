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

export type BenefitGlowMode = 'none' | 'package' | 'vip' | 'svip'
export type UserSettingInput =
  | Record<string, unknown>
  | string
  | null
  | undefined

/** SVIP 有效充值阈值（分），与后端 model/svip.go 保持一致 */
export const SVIP_THRESHOLD_CENTS = 20000

export function isVipUserGroup(group?: string | null): boolean {
  return (group || '').trim().toLowerCase() === 'vip'
}

export function isSvipByValidTopupCents(cents?: number | null): boolean {
  return typeof cents === 'number' && cents >= SVIP_THRESHOLD_CENTS
}

export function isSvipUser(
  user?: {
    is_svip?: boolean
    valid_topup_cents?: number
  } | null
): boolean {
  if (!user) {
    return false
  }
  return (
    user.is_svip === true || isSvipByValidTopupCents(user.valid_topup_cents)
  )
}

export function getBenefitGlowMode({
  packageGlow,
  isVipUser,
  isSvipUser = false,
}: {
  packageGlow: boolean
  isVipUser: boolean
  isSvipUser?: boolean
}): BenefitGlowMode {
  if (isSvipUser) {
    return 'svip'
  }
  if (packageGlow) {
    return 'package'
  }
  return isVipUser ? 'vip' : 'none'
}

export function parseUserSettingRecord(
  setting: UserSettingInput
): Record<string, unknown> {
  if (!setting) {
    return {}
  }

  if (typeof setting === 'object') {
    return { ...setting }
  }

  try {
    const parsed = JSON.parse(setting)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? { ...(parsed as Record<string, unknown>) }
      : {}
  } catch {
    return {}
  }
}

export function hasVipUpgradeModalSeen(setting: UserSettingInput): boolean {
  return parseUserSettingRecord(setting).vip_upgrade_modal_seen === true
}

export function shouldShowVipCelebration({
  group,
  setting,
}: {
  group?: string | null
  setting: UserSettingInput
}): boolean {
  return isVipUserGroup(group) && !hasVipUpgradeModalSeen(setting)
}

export function withVipUpgradeModalSeen(
  setting: UserSettingInput
): Record<string, unknown> {
  return {
    ...parseUserSettingRecord(setting),
    vip_upgrade_modal_seen: true,
  }
}

export function hasSvipCelebrationSeen(setting: UserSettingInput): boolean {
  return parseUserSettingRecord(setting).svip_celebration_seen === true
}

export function shouldShowSvipCelebration({
  isSvip,
  setting,
}: {
  isSvip: boolean
  setting: UserSettingInput
}): boolean {
  return isSvip && !hasSvipCelebrationSeen(setting)
}

export function withSvipCelebrationSeen(
  setting: UserSettingInput
): Record<string, unknown> {
  return {
    ...parseUserSettingRecord(setting),
    svip_celebration_seen: true,
  }
}
