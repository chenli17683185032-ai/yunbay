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
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const pageSource = readFileSync(resolve(currentDir, 'index.tsx'), 'utf8')
const ccSwitchSource = readFileSync(
  resolve(currentDir, 'quick-start-cc-switch.ts'),
  'utf8'
)
const dataSource = readFileSync(
  resolve(currentDir, 'quick-start-data.ts'),
  'utf8'
)
const completionDialogSource = readFileSync(
  resolve(currentDir, 'quick-start-completion-dialog.tsx'),
  'utf8'
)
const appHeaderSource = readFileSync(
  resolve(currentDir, '../../components/layout/components/app-header.tsx'),
  'utf8'
)

test('quick start controls are centered, enlarged, and keep one primary action', () => {
  assert.match(pageSource, /left-1\/2/)
  assert.match(pageSource, /-translate-x-1\/2/)
  assert.match(pageSource, /h-12 min-w-12/)
  assert.match(pageSource, /h-12 min-w-32/)
  assert.match(pageSource, /shadow-\[0_16px_48px_rgba\(255,255,255,0\.2\)\]/)
  assert.match(pageSource, /ring-1 ring-white\/30/)
  assert.match(pageSource, /font-black/)
  assert.match(pageSource, /hover:-translate-y-0\.5/)
  assert.match(pageSource, /isFinalPage\s*\?\s*props\.onEnterDashboard/)
  assert.equal(pageSource.match(/t\('Enter dashboard'\)/g)?.length, 1)
})

test('quick start keeps DOM content above the WebGL background in filtered app shells', () => {
  assert.match(pageSource, /fixed inset-0 h-\[100dvh\] w-full overflow-hidden/)
  assert.match(
    pageSource,
    /PointCloudMorphCanvas[\s\S]*className='absolute z-0'/
  )
  assert.match(pageSource, /LandingSnapFrame[\s\S]*className='relative z-10'/)
})

test('quick start keeps skip as a low-priority text action', () => {
  assert.match(pageSource, /Set up later and enter dashboard/)
  assert.match(pageSource, /text-white\/44/)
  assert.doesNotMatch(pageSource, /secondary dashboard CTA/)
})

test('quick start keeps generated API key when clipboard copy fails', () => {
  assert.match(pageSource, /generatedApiKeyCopied/)
  assert.match(
    pageSource,
    /API key was generated but clipboard copy failed\. You can copy it again or continue setup\./
  )
  assert.match(pageSource, /setGeneratedApiKey\(result\.fullKey\)/)
  assert.match(pageSource, /setGeneratedApiKeyCopied\(result\.copied\)/)
})

test('quick start uses the corrected model slug and keeps reasoning as a separate setting', () => {
  assert.match(dataSource, /QUICK_START_PREFERRED_MODEL = 'gpt-5\.6-sol'/)
  assert.match(dataSource, /QUICK_START_REASONING_EFFORT = 'xhigh'/)
  assert.doesNotMatch(dataSource, /gpt-5\.6-sol-thinking/)
  assert.match(pageSource, /QUICK_START_REASONING_EFFORT_LABEL_KEY/)
})

test('quick start software and account steps match the new five-page flow', () => {
  assert.match(pageSource, /Download CC Switch/)
  assert.match(pageSource, /codexDownloadCards\.map/)
  assert.match(
    dataSource,
    /CC-Switch-\$\{CC_SWITCH_RELEASE_VERSION\}-macOS\.dmg/
  )
  assert.match(
    dataSource,
    /CC-Switch-\$\{CC_SWITCH_RELEASE_VERSION\}-Windows\.msi/
  )
  assert.match(pageSource, /Add balance or redeem a code/)
  assert.match(pageSource, /Create your API key/)
  assert.match(pageSource, /recoverLatestQuickStartApiKey/)
  assert.match(pageSource, /queryClient\.setQueryData/)
  assert.match(pageSource, /const balanceReady = currentBalance > 0/)
})

test('quick start final page confirms prerequisites before one-click import and guarded exit', () => {
  assert.match(pageSource, /Have you finished installing CC Switch\?/)
  assert.match(pageSource, /Import current setup to CC Switch/)
  assert.match(pageSource, /buildQuickStartCCSwitchImportURL/)
  assert.match(pageSource, /handleImportToCCSwitch/)
  assert.match(pageSource, /Configured API/)
  assert.match(pageSource, /Configured model/)
  assert.match(pageSource, /Generated API key/)
  assert.match(pageSource, /importConfirmed/)
  assert.match(pageSource, /navigationCompletedRef/)
  assert.match(pageSource, /importAttempted\s*\?\s*importStatusRef\.current/)
  assert.match(pageSource, /target\?\.scrollIntoView/)
  assert.match(pageSource, /exitSurface\.animate\(keyframes/)
  assert.match(pageSource, /fill: 'forwards'/)
  assert.match(pageSource, /scroll-mb-4 sm:scroll-mb-\[8rem\]/)
  assert.match(
    pageSource,
    /h-\[calc\(100dvh-7\.75rem-env\(safe-area-inset-bottom\)\)\][\s\S]*sm:h-full/
  )
  assert.match(pageSource, /window\.setTimeout/)
  assert.match(pageSource, /clipPath/)
  assert.match(ccSwitchSource, /ccswitch:\/\/v1\/import/)
})

test('console completion guide waits for required announcements and offers both outcomes', () => {
  assert.match(appHeaderSource, /quickStartSession\.completionPromptPending/)
  assert.match(appHeaderSource, /!notifications\.loading/)
  assert.match(appHeaderSource, /!notifications\.requiredDialogOpen/)
  assert.match(appHeaderSource, /to: '\/quick-start', hash: 'readiness'/)
  assert.match(completionDialogSource, /I need to review it again/)
  assert.match(completionDialogSource, /No, start now/)
  assert.match(completionDialogSource, /showCloseButton=\{false\}/)
})
