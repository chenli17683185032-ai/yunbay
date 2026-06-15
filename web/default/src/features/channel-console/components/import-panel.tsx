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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { commitChannelConsoleImport, previewChannelConsoleImport } from '../api'
import type { ImportPreview } from '../types'

export function ImportPanel({ onImported }: { onImported: () => void }) {
  const { t } = useTranslation()
  const [rawInput, setRawInput] = useState('')
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [loading, setLoading] = useState(false)

  async function handlePreview() {
    setLoading(true)
    try {
      const res = await previewChannelConsoleImport(rawInput)
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Preview failed'))
      }
      setPreview(res.data)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Preview failed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleCommit() {
    if (!preview) return
    setLoading(true)
    try {
      const res = await commitChannelConsoleImport({
        raw_input: rawInput,
        name: preview.suggested_name,
        models: preview.default_test_model ? [preview.default_test_model] : [],
        multi_key_mode: preview.multi_key_mode,
        markup: 1.2,
      })
      if (!res.success) {
        throw new Error(res.message || t('Import failed'))
      }
      toast.success(t('Channel imported'))
      setRawInput('')
      setPreview(null)
      onImported()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Import failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Copy to import')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        <Textarea
          className='min-h-36 font-mono text-xs'
          onChange={(event) => setRawInput(event.target.value)}
          placeholder={t(
            'Paste API key, curl, JSON, Base URL + Key, or Authorization header'
          )}
          value={rawInput}
        />
        <div className='flex flex-wrap gap-2'>
          <Button disabled={!rawInput.trim() || loading} onClick={handlePreview}>
            {t('Preview')}
          </Button>
          <Button
            disabled={!preview || loading}
            onClick={handleCommit}
            variant='secondary'
          >
            {t('Save and verify')}
          </Button>
        </div>
        {preview ? (
          <div className='space-y-1 rounded-lg border p-3 text-sm'>
            <div className='font-medium'>{preview.provider_label}</div>
            <div className='text-muted-foreground break-all'>
              {preview.base_url}
            </div>
            <div>
              {t('Keys')}: {preview.key_previews.join(', ')}
            </div>
            <div>
              {t('Price source')}: {preview.price_source}
            </div>
            {preview.warnings?.map((warning) => (
              <div className='text-amber-600' key={warning}>
                {warning}
              </div>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
