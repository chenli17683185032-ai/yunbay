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
export type QuickStartPlatform = 'macOS' | 'Windows'

export type QuickStartSessionState = {
  modelName: string
  platform: QuickStartPlatform | null
  softwareConfirmed: boolean
  importAttempted: boolean
  importConfirmed: boolean
  completionPromptPending: boolean
}

export const QUICK_START_SESSION_STORAGE_KEY = 'yunbay_quick_start_session_v1'

export const EMPTY_QUICK_START_SESSION: QuickStartSessionState = {
  modelName: '',
  platform: null,
  softwareConfirmed: false,
  importAttempted: false,
  importConfirmed: false,
  completionPromptPending: false,
}

function isQuickStartPlatform(value: unknown): value is QuickStartPlatform {
  return value === 'macOS' || value === 'Windows'
}

export function normalizeQuickStartSession(
  value: unknown
): QuickStartSessionState {
  if (!value || typeof value !== 'object') return EMPTY_QUICK_START_SESSION
  const candidate = value as Record<string, unknown>

  return {
    modelName:
      typeof candidate.modelName === 'string' ? candidate.modelName : '',
    platform: isQuickStartPlatform(candidate.platform)
      ? candidate.platform
      : null,
    softwareConfirmed: candidate.softwareConfirmed === true,
    importAttempted: candidate.importAttempted === true,
    importConfirmed: candidate.importConfirmed === true,
    completionPromptPending: candidate.completionPromptPending === true,
  }
}

export function readQuickStartSession(): QuickStartSessionState {
  if (typeof window === 'undefined') return EMPTY_QUICK_START_SESSION
  try {
    const raw = window.sessionStorage.getItem(QUICK_START_SESSION_STORAGE_KEY)
    return raw
      ? normalizeQuickStartSession(JSON.parse(raw))
      : EMPTY_QUICK_START_SESSION
  } catch {
    return EMPTY_QUICK_START_SESSION
  }
}

export function writeQuickStartSession(
  updates: Partial<QuickStartSessionState>
): QuickStartSessionState {
  const nextState = normalizeQuickStartSession({
    ...readQuickStartSession(),
    ...updates,
  })
  if (typeof window !== 'undefined') {
    window.sessionStorage.setItem(
      QUICK_START_SESSION_STORAGE_KEY,
      JSON.stringify(nextState)
    )
  }
  return nextState
}
