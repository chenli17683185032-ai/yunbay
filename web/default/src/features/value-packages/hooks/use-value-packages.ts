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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import {
  cancelLdxpTopupSession,
  getLdxpTopupSession,
  isApiSuccess,
} from '@/features/wallet/api'
import { isLdxpTerminalStatus } from '@/features/wallet/lib/ldxp-topup'
import type { LdxpTopupSession, LdxpTopupStatus } from '@/features/wallet/types'
import {
  activateValuePackage,
  createValuePackageLdxpSession,
  deactivateValuePackage,
  getValuePackagePlans,
  getValuePackagePurchaseIntent,
  resetValuePackageQuota,
} from '../api'
import type {
  ValuePackageLdxpSessionResponse,
  ValuePackagePlan,
  ValuePackageState,
} from '../types'
import { valuePackageSelfQueryKey } from '../query-keys'

function getErrorMessage(
  responseMessage: string | undefined,
  fallback: string
) {
  return responseMessage && responseMessage !== 'success'
    ? responseMessage
    : fallback
}

function toValuePackageSessionResponse(
  current: ValuePackageLdxpSessionResponse,
  session: LdxpTopupSession
): ValuePackageLdxpSessionResponse {
  return {
    ...current,
    session: {
      ...current.session,
      ...session,
    },
  }
}

function isTerminalValuePackageSession(
  session: ValuePackageLdxpSessionResponse | null
): boolean {
  if (!session) {
    return true
  }
  return isLdxpTerminalStatus(session.session.status as LdxpTopupStatus)
}

export function useValuePackages() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const setUser = useAuthStore((store) => store.auth.setUser)
  const [plans, setPlans] = useState<ValuePackagePlan[]>([])
  const [state, setState] = useState<ValuePackageState | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [actionKey, setActionKey] = useState<string | null>(null)
  const [paymentSession, setPaymentSession] =
    useState<ValuePackageLdxpSessionResponse | null>(null)
  const [paymentLoading, setPaymentLoading] = useState(false)
  const [paymentError, setPaymentError] = useState<string | null>(null)
  const [pollAttempt, setPollAttempt] = useState(0)
  const activeSessionIdRef = useRef<string | null>(null)
  const operationSeqRef = useRef(0)
  const handledSuccessSessionIdRef = useRef<string | null>(null)

  const syncGlobalState = useCallback(
    (nextState: ValuePackageState | null) => {
      queryClient.setQueryData(valuePackageSelfQueryKey, nextState)
    },
    [queryClient]
  )

  const refreshSelf = useCallback(async () => {
    const response = await getSelf().catch(() => null)
    if (response?.success && response.data) {
      setUser(response.data)
    }
  }, [setUser])

  const refresh = useCallback(async () => {
    setRefreshing(true)
    setError(null)
    try {
      const response = await getValuePackagePlans()
      if (!isApiSuccess(response) || !response.data) {
        const message = getErrorMessage(
          response.message,
          t('Failed to load value packages')
        )
        setError(message)
        setPlans([])
        setState(null)
        return false
      }

      setPlans(response.data.plans || [])
      const nextState = response.data.state || null
      setState(nextState)
      syncGlobalState(nextState)
      return true
    } catch (_error) {
      setError(t('Failed to load value packages'))
      setPlans([])
      setState(null)
      syncGlobalState(null)
      return false
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [syncGlobalState, t])

  useEffect(() => {
    const timeoutId = window.setTimeout(() => {
      void refresh()
    }, 0)

    return () => window.clearTimeout(timeoutId)
  }, [refresh])

  const purchase = useCallback(
    async (plan: ValuePackagePlan) => {
      const operationSeq = operationSeqRef.current + 1
      operationSeqRef.current = operationSeq
      activeSessionIdRef.current = null
      handledSuccessSessionIdRef.current = null
      setActionKey(`purchase-${plan.id}`)
      setPaymentLoading(true)
      setPaymentError(null)
      setPaymentSession(null)
      setPollAttempt(0)

      try {
        const intentResponse = await getValuePackagePurchaseIntent(
          plan.id,
          false
        )
        if (operationSeqRef.current !== operationSeq) {
          return false
        }

        if (!isApiSuccess(intentResponse) || !intentResponse.data) {
          const message = getErrorMessage(
            intentResponse.message,
            t('Failed to create value package order')
          )
          setPaymentError(message)
          toast.error(message)
          return false
        }

        const requiresConfirmation =
          intentResponse.data.requires_confirmation === true
        const confirmedCover = requiresConfirmation
          ? window.confirm(
              intentResponse.data.message ||
                t(
                  'Buying this package will cover the current lower package. Continue?'
                )
            )
          : false

        if (requiresConfirmation && !confirmedCover) {
          return false
        }

        const sessionResponse = await createValuePackageLdxpSession(
          plan.id,
          confirmedCover
        )
        if (operationSeqRef.current !== operationSeq) {
          return false
        }

        if (!isApiSuccess(sessionResponse) || !sessionResponse.data) {
          const message = getErrorMessage(
            sessionResponse.message,
            t('Failed to create value package payment session')
          )
          setPaymentError(message)
          toast.error(message)
          return false
        }

        activeSessionIdRef.current = sessionResponse.data.session.session_id
        setPaymentSession(sessionResponse.data)
        return true
      } catch (_error) {
        if (operationSeqRef.current === operationSeq) {
          const message = t('Failed to create value package payment session')
          setPaymentError(message)
          toast.error(message)
        }
        return false
      } finally {
        if (operationSeqRef.current === operationSeq) {
          setPaymentLoading(false)
          setActionKey(null)
        }
      }
    },
    [t]
  )

  const activate = useCallback(
    async (userSubscriptionId: number) => {
      setActionKey(`activate-${userSubscriptionId}`)
      try {
        const response = await activateValuePackage(userSubscriptionId)
        if (!isApiSuccess(response) || !response.data) {
          const message = getErrorMessage(
            response.message,
            t('Failed to start value package')
          )
          toast.error(message)
          return false
        }

        setState(response.data)
        syncGlobalState(response.data)
        toast.success(t('Value package started'))
        return true
      } catch (_error) {
        toast.error(t('Failed to start value package'))
        return false
      } finally {
        setActionKey(null)
      }
    },
    [syncGlobalState, t]
  )

  const deactivate = useCallback(async () => {
    setActionKey('deactivate')
    try {
      const response = await deactivateValuePackage()
      if (!isApiSuccess(response) || !response.data) {
        const message = getErrorMessage(
          response.message,
          t('Failed to close package usage')
        )
        toast.error(message)
        return false
      }

      setState(response.data)
      syncGlobalState(response.data)
      toast.success(t('Package usage closed'))
      return true
    } catch (_error) {
      toast.error(t('Failed to close package usage'))
      return false
    } finally {
      setActionKey(null)
    }
  }, [syncGlobalState, t])


  const resetQuota = useCallback(
    async (userSubscriptionId?: number) => {
      const actionSubscriptionId = userSubscriptionId || 0
      setActionKey(`reset-quota-${actionSubscriptionId || 'active'}`)
      try {
        const response = await resetValuePackageQuota(userSubscriptionId)
        if (!isApiSuccess(response) || !response.data) {
          const message = getErrorMessage(
            response.message,
            t('Failed to reset value package quota')
          )
          toast.error(message)
          return false
        }

        setState(response.data)
        syncGlobalState(response.data)
        toast.success(t('Value package quota reset'))
        return true
      } catch (_error) {
        toast.error(t('Failed to reset value package quota'))
        return false
      } finally {
        setActionKey(null)
      }
    },
    [syncGlobalState, t]
  )

  const cancelPayment = useCallback(async () => {
    const sessionId = paymentSession?.session.session_id
    if (!sessionId || activeSessionIdRef.current !== sessionId) {
      return false
    }

    const operationSeq = operationSeqRef.current + 1
    operationSeqRef.current = operationSeq
    setPaymentLoading(true)
    setPaymentError(null)

    try {
      const response = await cancelLdxpTopupSession(sessionId)
      if (
        operationSeqRef.current !== operationSeq ||
        activeSessionIdRef.current !== sessionId
      ) {
        return false
      }

      if (!isApiSuccess(response) || !response.data) {
        const message = getErrorMessage(
          response.message,
          t('Failed to cancel value package payment')
        )
        setPaymentError(message)
        setPollAttempt((attempt) => attempt + 1)
        return false
      }

      const canceledSession = response.data
      setPaymentSession((current) => {
        if (!current || current.session.session_id !== sessionId) {
          return current
        }
        return toValuePackageSessionResponse(current, canceledSession)
      })
      return true
    } catch (_error) {
      if (
        operationSeqRef.current === operationSeq &&
        activeSessionIdRef.current === sessionId
      ) {
        setPaymentError(t('Failed to cancel value package payment'))
        setPollAttempt((attempt) => attempt + 1)
      }
      return false
    } finally {
      if (
        operationSeqRef.current === operationSeq &&
        activeSessionIdRef.current === sessionId
      ) {
        setPaymentLoading(false)
      }
    }
  }, [paymentSession, t])

  const resetPayment = useCallback(() => {
    operationSeqRef.current += 1
    activeSessionIdRef.current = null
    handledSuccessSessionIdRef.current = null
    setPaymentSession(null)
    setPaymentError(null)
    setPollAttempt(0)
    setPaymentLoading(false)
  }, [])

  useEffect(() => {
    if (!paymentSession || isTerminalValuePackageSession(paymentSession)) {
      return
    }

    const operationSeq = operationSeqRef.current
    const sessionId = paymentSession.session.session_id
    if (activeSessionIdRef.current !== sessionId) {
      return
    }

    let active = true
    const timeoutId = window.setTimeout(async () => {
      try {
        const response = await getLdxpTopupSession(sessionId)
        if (
          !active ||
          operationSeqRef.current !== operationSeq ||
          activeSessionIdRef.current !== sessionId
        ) {
          return
        }

        if (!isApiSuccess(response) || !response.data) {
          setPaymentError(
            getErrorMessage(
              response.message,
              t('Failed to refresh value package payment')
            )
          )
          setPollAttempt((attempt) => attempt + 1)
          return
        }

        const refreshedSession = response.data
        setPaymentError(null)
        setPaymentSession((current) => {
          if (!current || current.session.session_id !== sessionId) {
            return current
          }
          return toValuePackageSessionResponse(current, refreshedSession)
        })
      } catch (_error) {
        if (
          active &&
          operationSeqRef.current === operationSeq &&
          activeSessionIdRef.current === sessionId
        ) {
          setPaymentError(t('Failed to refresh value package payment'))
          setPollAttempt((attempt) => attempt + 1)
        }
      }
    }, paymentSession.session.poll_interval_ms || 2000)

    return () => {
      active = false
      window.clearTimeout(timeoutId)
    }
  }, [paymentSession, pollAttempt, t])

  useEffect(() => {
    const session = paymentSession?.session
    if (!session || session.status !== 'success') {
      return
    }

    if (handledSuccessSessionIdRef.current === session.session_id) {
      return
    }

    handledSuccessSessionIdRef.current = session.session_id
    void refresh()
    void refreshSelf()
  }, [paymentSession, refresh, refreshSelf])

  return {
    plans,
    state,
    loading,
    refreshing,
    error,
    actionKey,
    paymentSession,
    paymentLoading,
    paymentError,
    refresh,
    purchase,
    activate,
    deactivate,
    resetQuota,
    cancelPayment,
    resetPayment,
  }
}
