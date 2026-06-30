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
import {
  cancelLdxpTopupSession,
  createLdxpTopupSession,
  getLdxpTopupSession,
  isApiSuccess,
} from '../api'
import { isLdxpTerminalStatus } from '../lib/ldxp-topup'
import type { LdxpTopupSession } from '../types'

interface UseLdxpTopupOptions {
  onSuccess?: () => Promise<void> | void
}

export function useLdxpTopup(options: UseLdxpTopupOptions = {}) {
  const [session, setSession] = useState<LdxpTopupSession | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pollAttempt, setPollAttempt] = useState(0)
  const handledSuccessSessionIdRef = useRef<string | null>(null)
  const operationSeqRef = useRef(0)
  const activeSessionIdRef = useRef<string | null>(null)

  const start = useCallback(async (amount: number) => {
    const operationSeq = operationSeqRef.current + 1
    operationSeqRef.current = operationSeq
    activeSessionIdRef.current = null
    setLoading(true)
    setError(null)
    setSession(null)
    setPollAttempt(0)
    handledSuccessSessionIdRef.current = null

    try {
      const response = await createLdxpTopupSession(amount)

      if (operationSeqRef.current !== operationSeq) {
        return false
      }

      if (!isApiSuccess(response) || !response.data) {
        setError(response.message || 'Failed to create recharge session')
        return false
      }

      activeSessionIdRef.current = response.data.session_id
      setSession(response.data)
      return true
    } catch (_error) {
      if (operationSeqRef.current === operationSeq) {
        setError('Failed to create recharge session')
      }
      return false
    } finally {
      if (operationSeqRef.current === operationSeq) {
        setLoading(false)
      }
    }
  }, [])

  const cancel = useCallback(async () => {
    if (!session) {
      return false
    }

    const sessionId = session.session_id
    if (activeSessionIdRef.current !== sessionId) {
      return false
    }

    const operationSeq = operationSeqRef.current + 1
    operationSeqRef.current = operationSeq
    setLoading(true)
    setError(null)

    try {
      const response = await cancelLdxpTopupSession(sessionId)

      if (
        operationSeqRef.current !== operationSeq ||
        activeSessionIdRef.current !== sessionId
      ) {
        return false
      }

      if (!isApiSuccess(response)) {
        setError(response.message || 'Failed to cancel recharge session')
        setPollAttempt((attempt) => attempt + 1)
        return false
      }

      const canceledSession: LdxpTopupSession =
        response.data?.session_id === sessionId
          ? response.data
          : { ...session, status: 'canceled' }
      activeSessionIdRef.current = canceledSession.session_id
      setSession((current) =>
        current?.session_id === sessionId ? canceledSession : current
      )
      return true
    } catch (_error) {
      if (
        operationSeqRef.current === operationSeq &&
        activeSessionIdRef.current === sessionId
      ) {
        setError('Failed to cancel recharge session')
        setPollAttempt((attempt) => attempt + 1)
      }
      return false
    } finally {
      if (
        operationSeqRef.current === operationSeq &&
        activeSessionIdRef.current === sessionId
      ) {
        setLoading(false)
      }
    }
  }, [session])

  const reset = useCallback(() => {
    operationSeqRef.current += 1
    activeSessionIdRef.current = null
    handledSuccessSessionIdRef.current = null
    setSession(null)
    setError(null)
    setPollAttempt(0)
    setLoading(false)
  }, [])

  useEffect(() => {
    if (!session || isLdxpTerminalStatus(session.status)) {
      return
    }

    const operationSeq = operationSeqRef.current
    const sessionId = session.session_id
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
          setError(response.message || 'Failed to refresh recharge session')
          setPollAttempt((attempt) => attempt + 1)
          return
        }

        if (response.data.session_id !== sessionId) {
          setError('Failed to refresh recharge session')
          setPollAttempt((attempt) => attempt + 1)
          return
        }

        activeSessionIdRef.current = response.data.session_id
        setError(null)
        setSession(response.data)
      } catch (_error) {
        if (
          active &&
          operationSeqRef.current === operationSeq &&
          activeSessionIdRef.current === sessionId
        ) {
          setError('Failed to refresh recharge session')
          setPollAttempt((attempt) => attempt + 1)
        }
      }
    }, session.poll_interval_ms || 2000)

    return () => {
      active = false
      window.clearTimeout(timeoutId)
    }
  }, [pollAttempt, session])

  useEffect(() => {
    if (!session || session.status !== 'success') {
      return
    }

    if (handledSuccessSessionIdRef.current === session.session_id) {
      return
    }

    handledSuccessSessionIdRef.current = session.session_id
    Promise.resolve(options.onSuccess?.()).catch(() => {
      /* caller owns refresh error handling */
    })
  }, [options, session])

  return {
    session,
    loading,
    error,
    start,
    cancel,
    reset,
  }
}
