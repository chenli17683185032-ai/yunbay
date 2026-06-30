import type { WorkerConfig } from './config.js'

export interface ClaimedSession {
  session_id: string
  amount: number
  money: number
  product_url: string
  product_name: string
  contact_email: string
}

export interface WorkerQrPayload {
  worker_order_no: string
  worker_amount: number
  worker_product_name: string
  qr_code: string
  qr_page_url?: string
}

export interface WorkerResultPayload {
  worker_order_no: string
  worker_amount: number
  worker_product_name: string
  worker_card_key: string
  worker_status_text?: string
  worker_success_url?: string
}

export interface WorkerErrorPayload {
  error_code: string
  error_message: string
  snapshot_path?: string
}

export interface WorkerMailEventPayload {
  message_id?: string
  imap_uid?: string
  raw_hash?: string
  from?: string
  to?: string
  subject?: string
  received_time?: number
  order_no?: string
  amount?: number
  product_name?: string
  card_key?: string
  paid_time?: number
  body_excerpt?: string
}

interface ApiResponse<T> {
  success?: boolean
  message?: string
  error?: string
  data?: T
}

export async function claimSession(config: WorkerConfig, signal?: AbortSignal): Promise<ClaimedSession | null> {
  const response = await postJson<ClaimedSession>(config, '/api/ldxp/worker/sessions/claim', {
    worker_id: config.workerId,
  }, { allowNotFound: true, signal })

  if (response.status === 404 || response.data == null || isEmptyObject(response.data)) {
    return null
  }

  return response.data
}

export async function postQr(config: WorkerConfig, sessionId: string, payload: WorkerQrPayload): Promise<void> {
  await postJson(config, `/api/ldxp/worker/sessions/${encodeURIComponent(sessionId)}/qr`, {
    worker_id: config.workerId,
    ...payload,
  })
}

export async function postResult(config: WorkerConfig, sessionId: string, payload: WorkerResultPayload): Promise<void> {
  await postJson(config, `/api/ldxp/worker/sessions/${encodeURIComponent(sessionId)}/result`, {
    worker_id: config.workerId,
    ...payload,
  })
}

export async function postError(config: WorkerConfig, sessionId: string, payload: WorkerErrorPayload): Promise<void> {
  await postJson(config, `/api/ldxp/worker/sessions/${encodeURIComponent(sessionId)}/error`, {
    worker_id: config.workerId,
    ...payload,
  })
}

export async function postMailEvent(config: WorkerConfig, payload: WorkerMailEventPayload): Promise<void> {
  await postJson(config, '/api/ldxp/worker/mail-events', payload)
}

async function postJson<T>(
  config: WorkerConfig,
  path: string,
  body: unknown,
  options: { allowNotFound?: boolean; signal?: AbortSignal } = {},
): Promise<{ status: number; data: T | null }> {
  const response = await fetch(`${config.backendBaseUrl}${path}`, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      'x-ldxp-worker-token': config.workerToken,
    },
    body: JSON.stringify(body),
    signal: options.signal,
  })

  if (response.status === 404 && options.allowNotFound) {
    return { status: response.status, data: null }
  }

  const parsed = await parseApiResponse<T>(response)
  if (!response.ok) {
    throw new Error(`Backend request failed with HTTP ${response.status}: ${apiMessage(parsed)}`)
  }

  if (parsed.success === false) {
    throw new Error(apiMessage(parsed))
  }

  return { status: response.status, data: parsed.data ?? null }
}

async function parseApiResponse<T>(response: Response): Promise<ApiResponse<T>> {
  const text = await response.text()
  if (!text) {
    return {}
  }

  try {
    return JSON.parse(text) as ApiResponse<T>
  } catch {
    if (!response.ok) {
      return { success: false, message: text }
    }
    throw new Error('Backend returned invalid JSON response')
  }
}

function apiMessage(response: ApiResponse<unknown>): string {
  return response.message ?? response.error ?? 'backend request failed'
}

function isEmptyObject(value: unknown): boolean {
  return typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).length === 0
}
