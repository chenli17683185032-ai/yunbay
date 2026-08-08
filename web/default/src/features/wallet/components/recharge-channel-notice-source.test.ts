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
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const dialogSource = readFileSync(
  resolve(currentDir, 'recharge-channel-notice-dialog.tsx'),
  'utf8'
)
const walletSource = readFileSync(resolve(currentDir, '../index.tsx'), 'utf8')
const valuePackagesSource = readFileSync(
  resolve(currentDir, '../../value-packages/index.tsx'),
  'utf8'
)
const qrCodePath = resolve(
  currentDir,
  '../assets/wechat-recharge-group-20260815.jpg'
)

const notice = '因充值渠道出问题，请加微信群聊联系管理员进行充值。'

test('recharge channel notice contains the required message and QR code', () => {
  assert.match(dialogSource, new RegExp(notice))
  assert.match(dialogSource, /wechat-recharge-group-20260815\.jpg/)
  assert.match(dialogSource, /云贝技术交流3群微信群二维码/)
  assert.equal(existsSync(qrCodePath), true)
})

test('wallet amount cards open the notice without starting an LDXP session', () => {
  assert.match(walletSource, /onStart=\{showRechargeChannelNotice\}/)
  assert.match(walletSource, /<RechargeChannelNoticeDialog/)
  assert.doesNotMatch(walletSource, /onStart=\{ldxpTopup\.start\}/)
})

test('value package purchase cards open the same notice without purchasing', () => {
  assert.match(
    valuePackagesSource,
    /onPurchase=\{\(\) => setRechargeChannelNoticeOpen\(true\)\}/
  )
  assert.match(valuePackagesSource, /<RechargeChannelNoticeDialog/)
  assert.doesNotMatch(
    valuePackagesSource,
    /onPurchase=\{valuePackages\.purchase\}/
  )
})
