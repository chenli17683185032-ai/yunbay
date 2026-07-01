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
import type { SidebarData, NavItem } from '@/components/layout/types'

type Translate = (value: string) => string

type SidebarIcon = NonNullable<NavItem['icon']>

export type SidebarIconMap = Partial<{
  activity: SidebarIcon
  box: SidebarIcon
  creditCard: SidebarIcon
  fileText: SidebarIcon
  flask: SidebarIcon
  key: SidebarIcon
  layoutDashboard: SidebarIcon
  listTodo: SidebarIcon
  messageSquare: SidebarIcon
  paintbrush: SidebarIcon
  rocket: SidebarIcon
  radio: SidebarIcon
  settings: SidebarIcon
  ticket: SidebarIcon
  user: SidebarIcon
  users: SidebarIcon
  wallet: SidebarIcon
}>

const ADMIN_ROLE = 10

function isAdminRole(role?: number): boolean {
  return typeof role === 'number' && role >= ADMIN_ROLE
}

export function buildSidebarData(
  t: Translate,
  role?: number,
  icons: SidebarIconMap = {}
): SidebarData {
  if (!isAdminRole(role)) {
    return {
      navGroups: [
        {
          id: 'start',
          title: t('Getting Started'),
          items: [
            {
              title: t('Quick Start'),
              url: '/quick-start',
              icon: icons.rocket,
            },
          ],
        },
        {
          id: 'ai-usage',
          title: t('AI Usage'),
          items: [
            {
              title: t('Playground'),
              url: '/playground',
              icon: icons.flask,
            },
            {
              title: t('Chat'),
              icon: icons.messageSquare,
              type: 'chat-presets',
            },
          ],
        },
        {
          id: 'api',
          title: 'API',
          items: [
            {
              title: t('API Keys'),
              url: '/keys',
              icon: icons.key,
            },
            {
              title: t('Usage Logs'),
              url: '/usage-logs/common',
              icon: icons.fileText,
            },
          ],
        },
        {
          id: 'wallet',
          title: t('Wallet'),
          items: [
            {
              title: t('Wallet / Top up'),
              url: '/wallet',
              icon: icons.wallet,
            },
          ],
        },
        {
          id: 'account',
          title: t('Account'),
          items: [
            {
              title: t('Profile'),
              url: '/profile',
              icon: icons.user,
            },
          ],
        },
      ],
    }
  }

  return {
    navGroups: [
      {
        id: 'chat',
        title: t('Chat'),
        items: [
          {
            title: t('Playground'),
            url: '/playground',
            icon: icons.flask,
          },
          {
            title: t('Chat'),
            icon: icons.messageSquare,
            type: 'chat-presets',
          },
        ],
      },
      {
        id: 'general',
        title: t('General'),
        items: [
          {
            title: t('Overview'),
            url: '/dashboard/overview',
            icon: icons.activity,
          },
          {
            title: t('Dashboard'),
            url: '/dashboard/models',
            icon: icons.layoutDashboard,
          },
          {
            title: t('API Keys'),
            url: '/keys',
            icon: icons.key,
          },
          {
            title: t('Usage Logs'),
            url: '/usage-logs/common',
            icon: icons.fileText,
          },
          {
            title: t('Task Logs'),
            url: '/usage-logs/task',
            activeUrls: ['/usage-logs/drawing'],
            configUrls: ['/usage-logs/drawing', '/usage-logs/task'],
            icon: icons.listTodo,
          },
        ],
      },
      {
        id: 'personal',
        title: t('Personal'),
        items: [
          {
            title: t('Wallet'),
            url: '/wallet',
            icon: icons.wallet,
          },
          {
            title: t('Profile'),
            url: '/profile',
            icon: icons.user,
          },
        ],
      },
      {
        id: 'admin',
        title: t('Admin'),
        items: [
          {
            title: t('Channels'),
            url: '/channels',
            icon: icons.radio,
          },
          {
            title: t('Models'),
            url: '/models/metadata',
            icon: icons.box,
          },
          {
            title: t('Users'),
            url: '/users',
            icon: icons.users,
          },
          {
            title: t('Redemption Codes'),
            url: '/redemption-codes',
            icon: icons.ticket,
          },
          {
            title: t('Subscription Management'),
            url: '/subscriptions',
            icon: icons.creditCard,
          },
          {
            title: t('System Settings'),
            url: '/system-settings/site',
            activeUrls: ['/system-settings'],
            icon: icons.settings,
          },
        ],
      },
    ],
  }
}
