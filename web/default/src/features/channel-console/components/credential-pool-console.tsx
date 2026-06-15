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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import {
  addCliProxyCredential,
  addThirdPartyCredential,
  batchDeleteCredentials,
  createCredentialPool,
  getCliProxyAuthURL,
  getCredentialPoolDetail,
  listCredentialPools,
} from '../api'
import type {
  ChannelConsoleStatus,
  CredentialPool,
  CredentialPoolCredential,
  CredentialPoolDetail,
  CredentialPoolKind,
} from '../types'

const oauthProviders = [
  { provider: 'codex', label: '官方 GPT / Codex OAuth' },
  { provider: 'claude', label: 'Claude OAuth' },
  { provider: 'gemini', label: 'Gemini CLI OAuth' },
  { provider: 'antigravity', label: 'Antigravity OAuth' },
]

function statusVariant(status: ChannelConsoleStatus | string) {
  if (status === 'healthy') return 'default'
  if (status === 'failed' || status === 'disabled') return 'destructive'
  if (status === 'warning') return 'secondary'
  return 'outline'
}

function isBadCredential(credential: CredentialPoolCredential) {
  const status = `${credential.status || ''} ${
    credential.status_message || ''
  } ${credential.last_error_message || ''}`.toLowerCase()
  return (
    credential.status === 'failed' ||
    credential.status === 'disabled' ||
    status.includes('invalid') ||
    status.includes('error') ||
    status.includes('fail') ||
    status.includes('unavailable')
  )
}

function countModels(models: string) {
  return models
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean).length
}

function safeFileStem(value: string) {
  return value
    .trim()
    .replace(/\.json$/i, '')
    .replace(/[^a-zA-Z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function withJsonExtension(name: string) {
  return name.toLowerCase().endsWith('.json') ? name : `${name}.json`
}

function parseAuthJSON(rawInput: string, prefix: string) {
  const raw = rawInput.trim()
  if (!raw) return []

  let parsedValues: unknown[] = []
  try {
    const parsed = JSON.parse(raw)
    parsedValues = Array.isArray(parsed) ? parsed : [parsed]
  } catch {
    parsedValues = parseConcatenatedJSONObjects(raw)
  }

  return parsedValues
    .filter((item) => item && typeof item === 'object')
    .map((item, index, all) => {
      const record = item as Record<string, unknown>
      const suggested =
        [record.email, record.account, record.username, record.name].find(
          (value): value is string =>
            typeof value === 'string' && value.trim() !== ''
        ) || `oauth-${index + 1}`
      const stem = safeFileStem(prefix) || safeFileStem(suggested)
      const suffix =
        all.length > 1 ? `-${String(index + 1).padStart(3, '0')}` : ''
      return {
        name: withJsonExtension(`${stem}${suffix}`),
        raw: JSON.stringify(item),
      }
    })
}

function parseConcatenatedJSONObjects(raw: string) {
  const values: unknown[] = []
  let depth = 0
  let start = -1
  let inString = false
  let escaped = false

  for (let index = 0; index < raw.length; index += 1) {
    const char = raw[index]
    if (inString) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === '"') {
        inString = false
      }
      continue
    }

    if (char === '"') {
      inString = true
      continue
    }
    if (char === '{') {
      if (depth === 0) start = index
      depth += 1
      continue
    }
    if (char === '}') {
      depth -= 1
      if (depth === 0 && start >= 0) {
        const segment = raw.slice(start, index + 1)
        try {
          values.push(JSON.parse(segment))
        } catch {
          // Ignore non-JSON fragments in pasted text.
        }
        start = -1
      }
    }
  }

  return values
}

export function CredentialPoolConsole() {
  const { t } = useTranslation()
  const [pools, setPools] = useState<CredentialPool[]>([])
  const [selectedPoolID, setSelectedPoolID] = useState<number | null>(null)
  const [detail, setDetail] = useState<CredentialPoolDetail | null>(null)
  const [selectedCredentialIDs, setSelectedCredentialIDs] = useState<number[]>(
    []
  )
  const [newPoolName, setNewPoolName] = useState('')
  const [newPoolKind, setNewPoolKind] =
    useState<CredentialPoolKind>('third_party_api')
  const [newPoolBaseURL, setNewPoolBaseURL] = useState('')
  const [apiKeys, setApiKeys] = useState('')
  const [oauthName, setOauthName] = useState('')
  const [oauthJSON, setOauthJSON] = useState('')
  const [loading, setLoading] = useState(false)
  const selectedCredentialSet = useMemo(
    () => new Set(selectedCredentialIDs),
    [selectedCredentialIDs]
  )
  const selectedPool =
    detail?.pool || pools.find((pool) => pool.id === selectedPoolID) || null
  const credentials = detail?.credentials || []
  const allCredentialsSelected =
    credentials.length > 0 &&
    credentials.every((credential) => selectedCredentialSet.has(credential.id))

  async function loadPools(nextSelectedID?: number) {
    const res = await listCredentialPools()
    if (!res.success || !res.data) {
      throw new Error(res.message || t('Failed to load credential pools'))
    }
    setPools(res.data.items || [])
    const wantedID = nextSelectedID || selectedPoolID || res.data.items[0]?.id
    if (wantedID) {
      setSelectedPoolID(wantedID)
    }
  }

  async function loadDetail(poolID: number) {
    const res = await getCredentialPoolDetail(poolID)
    if (!res.success || !res.data) {
      throw new Error(res.message || t('Failed to load credential pool'))
    }
    setDetail(res.data)
  }

  async function refresh(poolID = selectedPoolID) {
    setLoading(true)
    try {
      await loadPools(poolID || undefined)
      if (poolID) {
        await loadDetail(poolID)
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!selectedPoolID) {
      setDetail(null)
      return
    }
    void loadDetail(selectedPoolID).catch((error) => {
      toast.error(
        error instanceof Error ? error.message : t('Failed to load detail')
      )
    })
    setSelectedCredentialIDs([])
  }, [selectedPoolID, t])

  async function handleCreatePool() {
    setLoading(true)
    try {
      const res = await createCredentialPool({
        name: newPoolName,
        provider_kind: newPoolKind,
        base_url:
          newPoolKind === 'third_party_api' ? newPoolBaseURL.trim() : '',
      })
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Create channel failed'))
      }
      toast.success(t('Channel created'))
      setNewPoolName('')
      setNewPoolBaseURL('')
      await loadPools(res.data.id)
      await loadDetail(res.data.id)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Create channel failed')
      )
    } finally {
      setLoading(false)
    }
  }

  async function handleAddAPIKeys() {
    if (!selectedPool) return
    const keys = apiKeys
      .split(/\r?\n/)
      .map((item) => item.trim())
      .filter(Boolean)
    if (keys.length === 0) {
      toast.error(t('Please enter API key'))
      return
    }
    setLoading(true)
    try {
      let added = 0
      for (const key of keys) {
        const res = await addThirdPartyCredential(selectedPool.id, {
          api_key: key,
        })
        if (!res.success) {
          throw new Error(res.message || t('Add API key failed'))
        }
        added += 1
      }
      toast.success(t('Added {{count}} API keys', { count: added }))
      setApiKeys('')
      await refresh(selectedPool.id)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Add API key failed')
      )
    } finally {
      setLoading(false)
    }
  }

  async function handleAddOAuthFiles() {
    if (!selectedPool) return
    const authFiles = parseAuthJSON(oauthJSON, oauthName)
    if (authFiles.length === 0) {
      toast.error(t('Please paste valid OAuth/auth JSON first'))
      return
    }
    setLoading(true)
    try {
      let added = 0
      for (const authFile of authFiles) {
        const res = await addCliProxyCredential(selectedPool.id, {
          name: authFile.name,
          raw_credential: authFile.raw,
        })
        if (!res.success) {
          throw new Error(res.message || t('Add OAuth credential failed'))
        }
        added += 1
      }
      toast.success(t('Added {{count}} OAuth credentials', { count: added }))
      setOauthName('')
      setOauthJSON('')
      await refresh(selectedPool.id)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Add OAuth credential failed')
      )
    } finally {
      setLoading(false)
    }
  }

  function toggleCredential(id: number, checked: boolean) {
    setSelectedCredentialIDs((current) => {
      if (checked) return current.includes(id) ? current : [...current, id]
      return current.filter((item) => item !== id)
    })
  }

  function toggleAllCredentials(checked: boolean) {
    setSelectedCredentialIDs(
      checked ? credentials.map((credential) => credential.id) : []
    )
  }

  async function handleDeleteCredentials() {
    if (!selectedPool || selectedCredentialIDs.length === 0) return
    if (!window.confirm(t('Delete selected credentials?'))) return

    setLoading(true)
    try {
      const res = await batchDeleteCredentials(selectedCredentialIDs)
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Delete failed'))
      }
      toast.success(
        t('Deleted {{count}} credentials', { count: res.data.deleted })
      )
      setSelectedCredentialIDs([])
      await refresh(selectedPool.id)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Delete failed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleOpenOAuthURL(provider: string) {
    try {
      const res = await getCliProxyAuthURL(provider)
      if (!res.success || !res.data?.url) {
        throw new Error(res.message || t('Failed to create OAuth URL'))
      }
      window.open(res.data.url, '_blank', 'noopener,noreferrer')
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to create OAuth URL')
      )
    }
  }

  return (
    <div className='grid h-full min-h-0 gap-4 overflow-auto xl:grid-cols-[320px_1fr]'>
      <div className='space-y-4'>
        <Card>
          <CardHeader>
            <CardTitle>{t('Credential channels')}</CardTitle>
            <CardDescription>
              {t('Create a channel first, then add API keys or OAuth files.')}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-3'>
            <div className='space-y-1.5'>
              <Label>{t('Channel name')}</Label>
              <Input
                onChange={(event) => setNewPoolName(event.target.value)}
                placeholder={t('OpenRouter main pool')}
                value={newPoolName}
              />
            </div>
            <div className='space-y-1.5'>
              <Label>{t('Channel type')}</Label>
              <NativeSelect
                className='w-full'
                onChange={(event) =>
                  setNewPoolKind(event.target.value as CredentialPoolKind)
                }
                value={newPoolKind}
              >
                <NativeSelectOption value='third_party_api'>
                  {t('Third-party API')}
                </NativeSelectOption>
                <NativeSelectOption value='oauth_cli'>
                  {t('OAuth / CliProxy')}
                </NativeSelectOption>
              </NativeSelect>
            </div>
            {newPoolKind === 'third_party_api' ? (
              <div className='space-y-1.5'>
                <Label>{t('API URL')}</Label>
                <Input
                  onChange={(event) => setNewPoolBaseURL(event.target.value)}
                  placeholder='https://api.example.com/v1'
                  value={newPoolBaseURL}
                />
              </div>
            ) : null}
            <Button
              className='w-full'
              disabled={
                loading ||
                !newPoolName.trim() ||
                (newPoolKind === 'third_party_api' &&
                  !newPoolBaseURL.trim())
              }
              onClick={handleCreatePool}
            >
              {t('Create channel')}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('Channel list')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-2'>
            {pools.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border p-4 text-sm'>
                {t('No credential channels yet')}
              </div>
            ) : (
              pools.map((pool) => (
                <button
                  className={`w-full rounded-lg border p-3 text-left transition ${
                    pool.id === selectedPoolID
                      ? 'border-primary bg-primary/5'
                      : 'hover:bg-muted/50'
                  }`}
                  key={pool.id}
                  onClick={() => setSelectedPoolID(pool.id)}
                  type='button'
                >
                  <div className='flex items-center justify-between gap-2'>
                    <div className='truncate font-medium'>{pool.name}</div>
                    <Badge variant={statusVariant(pool.health_status)}>
                      {pool.health_status || 'unchecked'}
                    </Badge>
                  </div>
                  <div className='text-muted-foreground mt-1 truncate text-xs'>
                    {pool.provider_kind === 'third_party_api'
                      ? pool.base_url
                      : t('OAuth / CliProxy')}
                  </div>
                  <div className='text-muted-foreground mt-1 text-xs'>
                    {t('Models')}: {countModels(pool.models)} · New API #
                    {pool.new_api_channel_id || '-'}
                  </div>
                </button>
              ))
            )}
          </CardContent>
        </Card>
      </div>

      <div className='space-y-4'>
        <Card>
          <CardHeader>
            <CardTitle>
              {selectedPool ? selectedPool.name : t('Select a channel')}
            </CardTitle>
            <CardDescription>
              {selectedPool
                ? selectedPool.provider_kind === 'third_party_api'
                  ? t(
                      'Only API URL and API Key are needed. Models are discovered from the API website.'
                    )
                  : t('OAuth auth files are stored in CliProxy and shown here.')
                : t('Create or select a channel on the left.')}
            </CardDescription>
          </CardHeader>
          {selectedPool ? (
            <CardContent className='grid gap-3 md:grid-cols-4'>
              <div className='rounded-lg border p-3'>
                <div className='text-muted-foreground text-xs'>
                  {t('Type')}
                </div>
                <div className='mt-1 font-medium'>
                  {selectedPool.provider_kind === 'third_party_api'
                    ? t('Third-party API')
                    : t('OAuth / CliProxy')}
                </div>
              </div>
              <div className='rounded-lg border p-3'>
                <div className='text-muted-foreground text-xs'>
                  {t('Models')}
                </div>
                <div className='mt-1 font-medium'>
                  {countModels(selectedPool.models)}
                </div>
              </div>
              <div className='rounded-lg border p-3'>
                <div className='text-muted-foreground text-xs'>
                  {t('Default model')}
                </div>
                <div className='mt-1 truncate font-medium'>
                  {selectedPool.default_test_model || '-'}
                </div>
              </div>
              <div className='rounded-lg border p-3'>
                <div className='text-muted-foreground text-xs'>
                  {t('Price source')}
                </div>
                <div className='mt-1 truncate font-medium'>
                  {selectedPool.price_source || '-'}
                </div>
              </div>
            </CardContent>
          ) : null}
        </Card>

        {selectedPool ? (
          <Card>
            <CardHeader>
              <CardTitle>{t('Add credential to selected channel')}</CardTitle>
              <CardDescription>
                {t('The selected channel is required before any credential enters the pool.')}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-3'>
              {selectedPool.provider_kind === 'third_party_api' ? (
                <>
                  <div className='rounded-lg border p-3 text-sm'>
                    <div className='text-muted-foreground text-xs'>
                      {t('API URL')}
                    </div>
                    <div className='break-all font-mono text-xs'>
                      {selectedPool.base_url}
                    </div>
                  </div>
                  <Textarea
                    className='min-h-28 font-mono text-xs'
                    onChange={(event) => setApiKeys(event.target.value)}
                    placeholder={t('Paste API Key here. One key per line is supported.')}
                    value={apiKeys}
                  />
                  <Button
                    disabled={loading || !apiKeys.trim()}
                    onClick={handleAddAPIKeys}
                  >
                    {t('Add API key and auto-discover models')}
                  </Button>
                </>
              ) : (
                <>
                  <div className='flex flex-wrap gap-2'>
                    {oauthProviders.map((item) => (
                      <Button
                        disabled={loading}
                        key={item.provider}
                        onClick={() => handleOpenOAuthURL(item.provider)}
                        size='sm'
                        variant='outline'
                      >
                        {t(item.label)}
                      </Button>
                    ))}
                  </div>
                  <Input
                    onChange={(event) => setOauthName(event.target.value)}
                    placeholder={t('auth file name or batch prefix')}
                    value={oauthName}
                  />
                  <Textarea
                    className='min-h-32 font-mono text-xs'
                    onChange={(event) => setOauthJSON(event.target.value)}
                    placeholder={t('Paste OAuth/auth JSON here')}
                    value={oauthJSON}
                  />
                  <Button
                    disabled={loading || !oauthJSON.trim()}
                    onClick={handleAddOAuthFiles}
                  >
                    {t('Add OAuth credential')}
                  </Button>
                </>
              )}
            </CardContent>
          </Card>
        ) : null}

        <Card>
          <CardHeader>
            <CardTitle>{t('Credentials under this channel')}</CardTitle>
            <CardDescription>
              {t('Failed credentials are marked red and can be batch deleted.')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className='overflow-hidden rounded-lg border'>
              <div className='flex items-center justify-between gap-3 border-b px-3 py-2'>
                <div className='text-muted-foreground text-sm'>
                  {selectedCredentialIDs.length > 0
                    ? t('{{count}} credentials selected', {
                        count: selectedCredentialIDs.length,
                      })
                    : t('Select credentials for batch operation')}
                </div>
                <Button
                  disabled={selectedCredentialIDs.length === 0 || loading}
                  onClick={handleDeleteCredentials}
                  size='sm'
                  variant='destructive'
                >
                  {t('Batch delete')}
                </Button>
              </div>
              <table className='w-full text-sm'>
                <thead className='bg-muted/50 text-left'>
                  <tr>
                    <th className='w-10 p-3'>
                      <Checkbox
                        aria-label={t('Select all credentials')}
                        checked={allCredentialsSelected}
                        onCheckedChange={(value) =>
                          toggleAllCredentials(Boolean(value))
                        }
                      />
                    </th>
                    <th className='p-3'>{t('Credential')}</th>
                    <th className='p-3'>{t('Kind')}</th>
                    <th className='p-3'>{t('Status')}</th>
                    <th className='p-3'>{t('Last model')}</th>
                    <th className='p-3'>{t('Stats')}</th>
                  </tr>
                </thead>
                <tbody>
                  {credentials.length === 0 ? (
                    <tr>
                      <td
                        className='text-muted-foreground p-6 text-center'
                        colSpan={6}
                      >
                        {selectedPool
                          ? t('No credentials in this channel yet')
                          : t('Select a channel first')}
                      </td>
                    </tr>
                  ) : (
                    credentials.map((credential) => (
                      <tr
                        className={
                          isBadCredential(credential)
                            ? 'bg-destructive/5 border-t'
                            : 'border-t'
                        }
                        key={credential.id}
                      >
                        <td className='p-3'>
                          <Checkbox
                            aria-label={t('Select credential')}
                            checked={selectedCredentialSet.has(credential.id)}
                            onCheckedChange={(value) =>
                              toggleCredential(credential.id, Boolean(value))
                            }
                          />
                        </td>
                        <td className='max-w-64 p-3'>
                          <div className='truncate font-medium'>
                            {credential.display_name ||
                              credential.cliproxy_auth_file ||
                              `#${credential.id}`}
                          </div>
                          <div className='text-muted-foreground text-xs'>
                            #{credential.id}
                          </div>
                        </td>
                        <td className='p-3'>
                          {credential.credential_kind === 'api_key'
                            ? t('API Key')
                            : t('OAuth auth file')}
                        </td>
                        <td className='p-3'>
                          <Badge variant={statusVariant(credential.status)}>
                            {credential.status || 'unchecked'}
                          </Badge>
                          {credential.status_message ||
                          credential.last_error_message ? (
                            <div className='text-muted-foreground mt-1 max-w-72 truncate text-xs'>
                              {credential.status_message ||
                                credential.last_error_message}
                            </div>
                          ) : null}
                        </td>
                        <td className='max-w-56 truncate p-3'>
                          {credential.last_successful_model || '-'}
                        </td>
                        <td className='p-3 text-xs'>
                          <div>
                            {t('OK')}: {credential.success_count || 0}
                          </div>
                          <div>
                            {t('Failed')}: {credential.failure_count || 0}
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
