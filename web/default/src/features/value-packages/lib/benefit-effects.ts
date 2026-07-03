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

export type BenefitGlowMode = 'none' | 'package' | 'vip'
export type UserSettingInput = Record<string, unknown> | string | null | undefined

export function isVipUserGroup(group?: string | null): boolean {
  return (group || '').trim().toLowerCase() === 'vip'
}

export function getBenefitGlowMode({
  packageGlow,
  isVipUser,
}: {
  packageGlow: boolean
  isVipUser: boolean
}): BenefitGlowMode {
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
