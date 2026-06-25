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
import {
  getJeepayPaymentSettings as getJeepayPaymentSettingsRequest,
  updateJeepayPaymentSettings as updateJeepayPaymentSettingsRequest,
} from '../api'
import type {
  JeepayPaymentSettings,
  UpdateJeepayPaymentSettingsRequest,
} from '../types'
import { removeTrailingSlash } from './utils'

export const defaultJeepayPaymentSettings: JeepayPaymentSettings = {
  JeepayEnabled: false,
  JeepayAlipayEnabled: false,
  JeepayBaseUrl: '',
  JeepayMchNo: '',
  JeepayAppId: '',
  JeepayAppSecret: '',
  JeepayAppSecretConfigured: false,
  JeepayNotifyUrl: '',
  JeepayReturnUrl: '',
  JeepaySubject: '',
  JeepayBody: '',
  JeepayTimeoutMs: 15000,
  JeepayAliDisplayName: '',
  JeepayAliDisplayColor: '',
}

export function normalizeJeepayPaymentSettings(
  values: JeepayPaymentSettings
): JeepayPaymentSettings {
  return {
    JeepayEnabled: values.JeepayEnabled,
    JeepayAlipayEnabled: values.JeepayAlipayEnabled,
    JeepayBaseUrl: removeTrailingSlash(values.JeepayBaseUrl.trim()),
    JeepayMchNo: values.JeepayMchNo.trim(),
    JeepayAppId: values.JeepayAppId.trim(),
    JeepayAppSecret: values.JeepayAppSecret.trim(),
    JeepayAppSecretConfigured: values.JeepayAppSecretConfigured,
    JeepayNotifyUrl: removeTrailingSlash(values.JeepayNotifyUrl.trim()),
    JeepayReturnUrl: removeTrailingSlash(values.JeepayReturnUrl.trim()),
    JeepaySubject: values.JeepaySubject.trim(),
    JeepayBody: values.JeepayBody.trim(),
    JeepayTimeoutMs: values.JeepayTimeoutMs,
    JeepayAliDisplayName: values.JeepayAliDisplayName.trim(),
    JeepayAliDisplayColor: values.JeepayAliDisplayColor.trim(),
  }
}

export function buildJeepayPaymentSettingsPayload(
  values: JeepayPaymentSettings
): UpdateJeepayPaymentSettingsRequest {
  const normalized = normalizeJeepayPaymentSettings(values)

  const payload: UpdateJeepayPaymentSettingsRequest = {
    JeepayEnabled: normalized.JeepayEnabled,
    JeepayAlipayEnabled: normalized.JeepayAlipayEnabled,
    JeepayBaseUrl: normalized.JeepayBaseUrl,
    JeepayMchNo: normalized.JeepayMchNo,
    JeepayAppId: normalized.JeepayAppId,
    JeepayNotifyUrl: normalized.JeepayNotifyUrl,
    JeepayReturnUrl: normalized.JeepayReturnUrl,
    JeepaySubject: normalized.JeepaySubject,
    JeepayBody: normalized.JeepayBody,
    JeepayTimeoutMs: normalized.JeepayTimeoutMs,
    JeepayAliDisplayName: normalized.JeepayAliDisplayName,
    JeepayAliDisplayColor: normalized.JeepayAliDisplayColor,
  }

  if (normalized.JeepayAppSecret) {
    payload.JeepayAppSecret = normalized.JeepayAppSecret
  }

  return payload
}

export async function getJeepayPaymentSettings() {
  const response = await getJeepayPaymentSettingsRequest()

  return {
    ...response,
    data: {
      ...defaultJeepayPaymentSettings,
      ...(response.data ?? {}),
      JeepayAppSecret: '',
      JeepayAppSecretConfigured:
        response.data?.JeepayAppSecretConfigured ?? false,
    },
  }
}

export async function updateJeepayPaymentSettings(
  request: UpdateJeepayPaymentSettingsRequest
) {
  return updateJeepayPaymentSettingsRequest(request)
}
