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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { listChannelConsoleChannels } from './api'
import { ChannelConsoleTable } from './components/channel-console-table'
import { ChannelDetailDrawer } from './components/channel-detail-drawer'
import { ImportPanel } from './components/import-panel'
import type { ChannelConsoleListItem } from './types'

export function ChannelConsole() {
  const { t } = useTranslation()
  const [items, setItems] = useState<ChannelConsoleListItem[]>([])
  const [selected, setSelected] = useState<ChannelConsoleListItem | null>(null)

  async function loadChannels() {
    const res = await listChannelConsoleChannels({ page_size: 100 })
    setItems(res.data?.items || [])
  }

  useEffect(() => {
    void loadChannels()
  }, [])

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Unified Channel Console')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid h-full min-h-0 gap-4 overflow-auto xl:grid-cols-[1fr_380px]'>
          <ChannelConsoleTable items={items} onOpen={setSelected} />
          <ImportPanel onImported={loadChannels} />
        </div>
        <ChannelDetailDrawer
          item={selected}
          onChecked={loadChannels}
          onOpenChange={(open) => !open && setSelected(null)}
          open={Boolean(selected)}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
