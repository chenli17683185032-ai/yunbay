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

export function useLdxpTopup(options: UseLdxpTopupOptions) {
  const [session, setSession] = useState<LdxpTopupSession | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const handledSuccessSessionIdRef = useRef<string | null>(null)

  const start = useCallback(async (amount: number) => {
    setLoading(true)
    setError(null)
    setSession(null)
    handledSuccessSessionIdRef.current = null

    try {
      const response = await createLdxpTopupSession(amount)

      if (!isApiSuccess(response) || !response.data) {
        setError(response.message || 'Failed to create recharge session')
        return false
      }

      setSession(response.data)
      return true
    } catch (_error) {
      setError('Failed to create recharge session')
      return false
    } finally {
      setLoading(false)
    }
  }, [])

  const cancel = useCallback(async () => {
    if (!session) {
      return false
    }

    setLoading(true)
    setError(null)

    try {
      const response = await cancelLdxpTopupSession(session.session_id)

      if (!isApiSuccess(response)) {
        setError(response.message || 'Failed to cancel recharge session')
        return false
      }

      setSession((current) =>
        current?.session_id === session.session_id
          ? { ...current, status: 'canceled' }
          : current
      )
      return true
    } catch (_error) {
      setError('Failed to cancel recharge session')
      return false
    } finally {
      setLoading(false)
    }
  }, [session])

  const reset = useCallback(() => {
    handledSuccessSessionIdRef.current = null
    setSession(null)
    setError(null)
    setLoading(false)
  }, [])

  useEffect(() => {
    if (!session || isLdxpTerminalStatus(session.status)) {
      return
    }

    let active = true
    const timeoutId = window.setTimeout(async () => {
      try {
        const response = await getLdxpTopupSession(session.session_id)

        if (!active) {
          return
        }

        if (!isApiSuccess(response) || !response.data) {
          setError(response.message || 'Failed to refresh recharge session')
          return
        }

        setSession(response.data)
      } catch (_error) {
        if (active) {
          setError('Failed to refresh recharge session')
        }
      }
    }, session.poll_interval_ms || 2000)

    return () => {
      active = false
      window.clearTimeout(timeoutId)
    }
  }, [session])

  useEffect(() => {
    if (!session || session.status !== 'success') {
      return
    }

    if (handledSuccessSessionIdRef.current === session.session_id) {
      return
    }

    handledSuccessSessionIdRef.current = session.session_id
    void options.onSuccess?.()
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
