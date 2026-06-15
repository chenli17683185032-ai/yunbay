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
const ADMIN_ROLE = 10
const USER_DEFAULT_PATH = '/quick-start'
const ADMIN_DEFAULT_PATH = '/dashboard/overview'
const USER_REDEMPTION_PATH = '/wallet?section=redeem'

const ORDINARY_USER_ALLOWED_PREFIXES = [
  '/quick-start',
  '/playground',
  '/chat',
  '/chat2link',
  '/keys',
  '/usage-logs',
  '/wallet',
  '/profile',
  '/pricing',
]

const ORDINARY_USER_MANAGEMENT_PREFIXES = [
  '/dashboard',
  '/channels',
  '/models',
  '/users',
  '/redemption-codes',
  '/subscriptions',
  '/system-settings',
]

export type RoleLikeUser = { role?: number | null } | null | undefined

export function isAdminRole(role?: number | null): boolean {
  return typeof role === 'number' && role >= ADMIN_ROLE
}

function isRootRelativePath(path: string): boolean {
  return path.startsWith('/') && !path.startsWith('//')
}

function getPathname(path: string): string {
  const queryIndex = path.indexOf('?')
  const hashIndex = path.indexOf('#')
  const endIndexes = [queryIndex, hashIndex].filter((index) => index >= 0)
  if (endIndexes.length === 0) return path
  return path.slice(0, Math.min(...endIndexes))
}

function matchesPrefix(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`)
}

export function isOrdinaryUserAllowedPath(path: string): boolean {
  if (!isRootRelativePath(path)) return false

  const pathname = getPathname(path)
  if (
    ORDINARY_USER_MANAGEMENT_PREFIXES.some((prefix) =>
      matchesPrefix(pathname, prefix)
    )
  ) {
    return false
  }

  return ORDINARY_USER_ALLOWED_PREFIXES.some((prefix) =>
    matchesPrefix(pathname, prefix)
  )
}

export function getPostLoginPath(
  user?: RoleLikeUser,
  redirectTo?: string
): string {
  if (isAdminRole(user?.role)) {
    return redirectTo && isRootRelativePath(redirectTo)
      ? redirectTo
      : ADMIN_DEFAULT_PATH
  }

  if (!redirectTo || !isRootRelativePath(redirectTo)) {
    return USER_DEFAULT_PATH
  }

  const pathname = getPathname(redirectTo)
  if (matchesPrefix(pathname, '/redemption-codes')) {
    return USER_REDEMPTION_PATH
  }

  return isOrdinaryUserAllowedPath(redirectTo) ? redirectTo : USER_DEFAULT_PATH
}

export function getPublicHeaderAuthedTarget(user?: RoleLikeUser): string {
  return isAdminRole(user?.role) ? ADMIN_DEFAULT_PATH : USER_DEFAULT_PATH
}
