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
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const sourcePath = new URL('./subscriptions-mutate-drawer.tsx', import.meta.url)

const requiredFields = [
  'plan_kind',
  'package_type',
  'package_level',
  'model_group',
  'concurrency_limit',
  'limit_5h_amount',
  'limit_7d_amount',
  'benefits',
  'ldxp_product_url',
  'ldxp_product_name',
  'ldxp_product_amount',
  'ldxp_product_ref',
  'ldxp_session_ttl_seconds',
]

test('mutate drawer source exposes complete value package fields', async () => {
  const source = await readFile(sourcePath, 'utf8')

  for (const field of requiredFields) {
    assert.match(source, new RegExp(field))
  }
  assert.match(source, /getValuePackageDuration/)
  assert.match(source, /duration_unit/)
  assert.match(source, /duration_value/)
  assert.match(source, /custom_seconds/)
  assert.match(source, /disabled=\{isValuePackage\}/)
  assert.match(
    source,
    /保存后用户购买将直接调用现有联动小铺支付系统创建付款会话|existing LDXP payment system/
  )
})

test('mutate drawer source uses dynamic value package total and month-only 7d period labels', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /getValuePackageTotalLimitLabelKey/)
  assert.match(source, /getValuePackageTotalLimitDescriptionKey/)
  assert.match(source, /shouldExposeValuePackage7dPeriodLimit/)
  assert.match(source, /VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY/)
  assert.doesNotMatch(source, /<FormLabel>limit_7d_amount<\/FormLabel>/)
})

test('mutate drawer source uses a positive integer input for custom concurrency', async () => {
  const source = await readFile(sourcePath, 'utf8')
  const fieldStart = source.indexOf("name='concurrency_limit'")
  const nextFieldStart = source.indexOf("name='limit_5h_amount'", fieldStart)
  const concurrencyField = source.slice(fieldStart, nextFieldStart)

  assert.ok(fieldStart >= 0)
  assert.ok(nextFieldStart > fieldStart)
  assert.match(concurrencyField, /<Input/)
  assert.match(concurrencyField, /type='number'/)
  assert.match(concurrencyField, /min=\{1\}/)
  assert.match(concurrencyField, /step=\{1\}/)
  assert.doesNotMatch(concurrencyField, /<Select/)
})
