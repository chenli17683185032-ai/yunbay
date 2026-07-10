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
const periodListSourcePath = new URL(
  './value-package-period-list.tsx',
  import.meta.url
)
const staleDefaultBenefitCopy = [
  '5-hour limit and 7-day limit ',
  'protection',
].join('')

test('value package card uses the shared usage period renderer', async () => {
  const source = await readFile(sourcePath, 'utf8')
  const periodListSource = await readFile(periodListSourcePath, 'utf8')

  assert.match(source, /▶/)
  assert.match(source, /Close package usage|关闭使用/)
  assert.match(source, /5-hour limit|5 小时限额/)
  assert.match(source, /VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY/)
  assert.match(source, /Package total limit and 5-hour protection/)
  assert.equal(source.includes(staleDefaultBenefitCopy), false)
  assert.match(source, /shouldExposeValuePackage7dPeriodLimit/)
  assert.match(source, /VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY/)
  assert.match(source, /getValuePackagePeriodLimits/)
  assert.match(source, /ValuePackagePeriodList/)
  assert.match(
    source,
    /getValuePackagePeriodLimits\(usage, plan\.package_type\)/
  )
  assert.match(source, /<ValuePackagePeriodList periods=\{usagePeriods\} \/>/)
  assert.doesNotMatch(source, /LimitProgressRow/)
  assert.doesNotMatch(source, /plan\.total_amount/)
  assert.match(
    source,
    /show7dPeriodLimit && Number\(plan\.limit_7d_amount \|\| 0\) > 0/
  )
  assert.match(periodListSource, /formatQuota/)
  assert.match(periodListSource, /Progress/)
  assert.match(periodListSource, /formatValuePackageResetLine/)
  assert.match(periodListSource, /5-hour remaining/)
  assert.match(periodListSource, /Current 7-day stage remaining/)
  assert.match(periodListSource, /1-day total remaining/)
  assert.match(periodListSource, /7-day total remaining/)
  assert.match(periodListSource, /30-day total remaining/)
  assert.match(periodListSource, /Does not refresh/)
  assert.match(periodListSource, /Number\.isFinite\(resetAt\)/)
  assert.match(periodListSource, /Math\.max\(0,/)
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
