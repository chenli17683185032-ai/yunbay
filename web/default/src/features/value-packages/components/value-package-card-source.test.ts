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

const sourcePath = new URL('./value-package-card.tsx', import.meta.url)

test('value package card source keeps required user controls and limit copy', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /▶/)
  assert.match(source, /Close package usage|关闭使用/)
  assert.match(source, /5-hour limit|5 小时限额/)
  assert.match(source, /7-day limit|7 天限额/)
  assert.match(source, /Progress/)
  assert.match(source, /Package total limit/)
  assert.match(source, /formatUsageAmount/)
  assert.match(source, /getProgressToneClass/)
  assert.match(source, /formatValuePackageResetLine/)
  assert.match(source, /resetSeconds\?: number/)
  assert.match(source, /limited\?: boolean/)
  assert.match(source, /reset_seconds_5h/)
  assert.match(source, /reset_seconds_7d/)
  assert.match(source, /limited_5h/)
  assert.match(source, /limited_7d/)
  assert.match(source, /!Number\.isFinite\(limit\) \|\| limit <= 0/)
  assert.match(source, /hasUsageProgress/)
  assert.match(source, /Math\.round\(amount \|\| 0\)/)
  assert.match(
    source,
    /label=\{t\('Package total limit'\)\}[\s\S]*?percent=\{usage\.total_percent\}/
  )
  assert.match(source, /getValuePackageDisplayCurrency/)
  assert.match(source, /currencyOverride/)
  assert.match(source, /CNY/)
  assert.match(source, /zh-CN/)
  assert.doesNotMatch(source, /t=\{t\}/)
  assert.match(
    source,
    /当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用/
  )
  assert.match(
    source,
    /Closing package usage does not pause its countdown|停止使用.*继续计时|关闭.*不会暂停/
  )
})

test('value package reset quota button is rendered directly below the main action button', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /onResetQuota/)
  assert.match(source, /Reset quota/)
  assert.match(source, /Remaining reset count/)
  assert.match(
    source,
    /cardState\.kind === 'running' && state\?\.preference\?\.enabled === true/
  )
  const resetQuotaGate =
    source.match(
      /const canShowResetQuota =[\s\S]*?const resetDisabled =/
    )?.[0] ?? ''
  assert.doesNotMatch(resetQuotaGate, /cardState\.kind === 'start'/)
  assert.match(
    source,
    /<CardFooter[\s\S]*<Button[\s\S]*onClick=\{handleAction\}[\s\S]*\{actionLabel\}[\s\S]*<Button[\s\S]*onClick=\{handleResetQuota\}[\s\S]*\{t\('Reset quota'\)\}[\s\S]*<\/CardFooter>/
  )
})
