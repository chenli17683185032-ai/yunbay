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
import type { HeaderNavModules } from '@/lib/nav-modules'
import type { TopNavLink } from '@/components/layout/types'

export function buildVisibleTopNavLinks(args: {
  modules: HeaderNavModules | null
  isAuthed: boolean
  t: (key: string) => string
}): TopNavLink[] {
  const { modules, isAuthed, t } = args
  const links: TopNavLink[] = []

  if (modules?.home !== false) {
    links.push({ title: t('Home'), href: '/' })
  }

  if (modules?.console !== false) {
    links.push({ title: t('Console'), href: '/dashboard' })
  }

  const pricing = modules?.pricing
  if (pricing && typeof pricing === 'object' && pricing.enabled) {
    links.push({
      title: t('Model Square'),
      href: '/pricing',
      requiresAuth: pricing.requireAuth && !isAuthed,
    })
  }

  return links
}
