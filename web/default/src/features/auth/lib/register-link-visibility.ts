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
export type RegisterPromptStatus = {
  self_use_mode_enabled?: boolean
  register_enabled?: boolean
}

export type RegisterPromptState =
  | {
      kind: 'link'
      textKey: "Don't have an account?"
      linkTextKey: 'Sign up now'
    }
  | {
      kind: 'closed'
      textKey: 'Registration is currently closed'
    }

export function getRegisterPromptState(
  status?: RegisterPromptStatus | null
): RegisterPromptState {
  if (status?.register_enabled === false) {
    return {
      kind: 'closed',
      textKey: 'Registration is currently closed',
    }
  }

  return {
    kind: 'link',
    textKey: "Don't have an account?",
    linkTextKey: 'Sign up now',
  }
}
