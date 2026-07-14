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
import type { ApiKey } from '../types'

type ApiKeyDisplayGroupInput = Pick<ApiKey, 'group'>

export function getApiKeyDisplayGroup(
  apiKey: ApiKeyDisplayGroupInput,
  groupRatios: Record<string, number>,
  packageBillingActive = false
): { group: string; ratio?: number; isEffective: boolean } {
  const storedGroup = apiKey.group?.trim() ?? ''
  const group = storedGroup
  const isEffective = packageBillingActive

  if (group === 'auto') {
    return { group, isEffective }
  }

  return { group, ratio: groupRatios[group], isEffective }
}
