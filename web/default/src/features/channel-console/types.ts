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

export type ChannelConsoleStatus =
  | 'healthy'
  | 'warning'
  | 'failed'
  | 'disabled'
  | 'unchecked'

export interface ImportPreview {
  provider: string
  provider_label: string
  channel_type: number
  base_url: string
  key_previews: string[]
  is_multi_key: boolean
  multi_key_mode: 'random' | 'polling'
  import_kind: string
  price_source: string
  model_discovery: string
  default_test_model: string
  suggested_name: string
  requires_confirmation: boolean
  warnings?: string[]
}

export interface ImportCommitRequest {
  raw_input: string
  name?: string
  group?: string
  models?: string[]
  multi_key_mode?: 'random' | 'polling'
  markup?: number
}

export interface ImportCommitResult {
  channel_id: number
  name: string
  provider: string
  key_count: number
  model_count: number
  health_status: ChannelConsoleStatus
  price_status: string
}

export interface ManagedChannelSummary {
  id: number
  type: number
  test_model?: string | null
  status: number
  name: string
  weight?: number | null
  created_time: number
  test_time: number
  response_time: number
  base_url?: string | null
  balance: number
  balance_updated_time: number
  models: string
  group: string
  used_quota: number
  priority?: number | null
  auto_ban?: number | null
  tag?: string | null
  remark?: string | null
  channel_info: Record<string, unknown>
}

export interface ChannelConsoleMeta {
  id: number
  channel_id: number
  provider: string
  provider_kind: string
  import_kind: string
  price_source: string
  health_status: ChannelConsoleStatus
  model_sync_status: string
  price_sync_status: string
  last_health_check_at: number
  last_model_sync_at: number
  last_price_sync_at: number
  last_error_code: string
  last_error_message: string
  markup: number
  auto_disable_policy: string
  created_at: number
  updated_at: number
}

export interface ChannelConsoleHealthCheck {
  id: number
  channel_id: number
  key_index?: number | null
  model_name: string
  check_type: string
  status: ChannelConsoleStatus
  response_time_ms: number
  error_code: string
  error_message: string
  checked_at: number
}

export interface ChannelConsoleListItem {
  channel: ManagedChannelSummary
  console: ChannelConsoleMeta
}

export interface ChannelConsoleListResult {
  items: ChannelConsoleListItem[]
  total: number
  page: number
  page_size: number
}

export interface ChannelConsoleBatchDeleteResult {
  requested: number
  deleted: number
  skipped_ids: number[]
}

export interface ChannelConsoleDetail {
  channel: ManagedChannelSummary
  console: ChannelConsoleMeta
  prices: Array<Record<string, unknown>>
  health_checks: ChannelConsoleHealthCheck[]
}

export type CredentialPoolKind = 'third_party_api' | 'oauth_cli'
export type CredentialKind = 'api_key' | 'cliproxy_auth'

export interface CredentialPool {
  id: number
  name: string
  provider: string
  provider_kind: CredentialPoolKind
  base_url: string
  models: string
  default_test_model: string
  price_source: string
  health_status: ChannelConsoleStatus
  model_sync_status: string
  price_sync_status: string
  last_health_check_at: number
  last_model_sync_at: number
  last_error_code: string
  last_error_message: string
  markup: number
  new_api_channel_id: number
  created_at: number
  updated_at: number
}

export interface CredentialPoolCredential {
  id: number
  pool_id: number
  credential_kind: CredentialKind
  display_name: string
  cliproxy_auth_file: string
  status: ChannelConsoleStatus
  status_message: string
  last_health_check_at: number
  last_error_code: string
  last_error_message: string
  last_model_sync_at: number
  last_successful_model: string
  success_count: number
  failure_count: number
  created_at: number
  updated_at: number
}

export interface CredentialPoolListResult {
  items: CredentialPool[]
  total: number
}

export interface CredentialPoolDetail {
  pool: CredentialPool
  credentials: CredentialPoolCredential[]
}

export interface CredentialBatchDeleteResult {
  deleted: number
  failed: number[]
}

export interface ApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}

export interface CliProxyStatus {
  configured: boolean
  reachable: boolean
  base_url: string
  files_count: number
  message: string
}

export interface CliProxyAuthFile {
  id: string
  auth_index: string
  name: string
  type: string
  provider: string
  status: string
  status_message: string
  disabled: boolean
  unavailable: boolean
  runtime_only: boolean
  source: string
  size: number
  modtime: string
  email: string
  account_type: string
  account: string
  last_refresh: string
  success: number
  failed: number
}

export interface CliProxyAuthFilesResult {
  files: CliProxyAuthFile[]
}

export interface CliProxyAuthURLResult {
  status: string
  url: string
  state: string
}
