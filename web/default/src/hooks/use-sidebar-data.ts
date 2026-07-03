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
import {
  Activity,
  Box,
  CreditCard,
  FileText,
  FlaskConical,
  Key,
  LayoutDashboard,
  ListTodo,
  MessageSquare,
  Radio,
  Rocket,
  Settings,
  Sparkles,
  Ticket,
  User,
  Users,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { type SidebarData } from '@/components/layout/types'
import { buildSidebarData, type SidebarIconMap } from './sidebar-data-model'

const SIDEBAR_ICONS: SidebarIconMap = {
  activity: Activity,
  box: Box,
  creditCard: CreditCard,
  fileText: FileText,
  flask: FlaskConical,
  key: Key,
  layoutDashboard: LayoutDashboard,
  listTodo: ListTodo,
  messageSquare: MessageSquare,
  radio: Radio,
  rocket: Rocket,
  settings: Settings,
  ticket: Ticket,
  user: User,
  valuePackages: Sparkles,
  users: Users,
  wallet: Wallet,
}

/**
 * Root navigation groups for the application sidebar.
 *
 * These are shown when the URL does not match any nested sidebar view
 * registered in `layout/lib/sidebar-view-registry.ts`.
 */
export function useSidebarData(): SidebarData {
  const { t } = useTranslation()
  const userRole = useAuthStore((state) => state.auth.user?.role)

  return buildSidebarData(t, userRole, SIDEBAR_ICONS)
}
