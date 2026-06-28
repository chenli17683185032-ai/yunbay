import { setTimeout as sleep } from 'node:timers/promises'
import { ImapFlow, type FetchMessageObject } from 'imapflow'
import { simpleParser } from 'mailparser'
import { postMailEvent, type WorkerMailEventPayload } from './backend.js'
import type { WorkerConfig } from './config.js'
import { makeBodyExcerpt, hashRawMail, parseLdxpMailBody, type ParsedMailEvent } from './mail-parser.js'
import { redactValue } from './redact.js'

const defaultMailbox = 'INBOX'
const defaultMailMatchWindowSeconds = 6 * 60 * 60

interface ParsedMailLike {
  messageId?: string
  from?: AddressLike
  to?: AddressLike | AddressLike[]
  subject?: string
  date?: Date
  text?: string
  html?: string | false
}

interface AddressLike {
  text?: string
}

export async function pollMailboxOnce(config: WorkerConfig): Promise<number> {
  assertImapConfig(config)

  const client = new ImapFlow({
    host: config.imapHost,
    port: config.imapPort ?? 993,
    secure: true,
    auth: {
      user: config.imapUser,
      pass: config.imapPassword,
    },
    logger: false,
  })

  try {
    await client.connect()
    await client.mailboxOpen(defaultMailbox)

    const uids = await findCandidateUids(client)
    if (uids.length === 0) {
      return 0
    }

    let postedCount = 0

    for await (const message of client.fetch(uids, { uid: true, source: true, internalDate: true, envelope: true }, { uid: true })) {
      if (!message.source) {
        continue
      }

      const accepted = await parseAndPostMessage(config, message)
      if (accepted) {
        postedCount += 1
        await markSeen(client, message.uid)
      }
    }

    return postedCount
  } finally {
    await safeLogout(client)
  }
}

export async function runMailPoller(config: WorkerConfig, signal: AbortSignal): Promise<void> {
  while (!signal.aborted) {
    try {
      await pollMailboxOnce(config)
    } catch (error) {
      console.warn(`LDXP mail poll failed: ${errorMessage(error)}`)
    }

    try {
      await sleep(config.pollIntervalMs, undefined, { signal })
    } catch (error) {
      if (signal.aborted || isAbortError(error)) {
        return
      }
      throw error
    }
  }
}

async function findCandidateUids(client: ImapFlow): Promise<number[]> {
  const unseen = await client.search({ seen: false }, { uid: true })
  if (Array.isArray(unseen) && unseen.length > 0) {
    return unseen
  }

  const since = new Date(Date.now() - defaultMailMatchWindowSeconds * 1000)
  const recent = await client.search({ since }, { uid: true })
  return Array.isArray(recent) ? recent : []
}

async function parseAndPostMessage(config: WorkerConfig, message: FetchMessageObject): Promise<boolean> {
  const source = message.source
  if (!source) {
    return false
  }

  let event: ParsedMailEvent
  let subject = message.envelope?.subject ?? ''
  let mailFrom = message.envelope?.from?.map((address) => address.address).filter(Boolean).join(', ') ?? ''

  try {
    const parsed = await simpleParser(source) as ParsedMailLike
    subject = parsed.subject ?? subject
    mailFrom = parsed.from?.text ?? mailFrom
    const body = parsed.text ?? (typeof parsed.html === 'string' ? parsed.html : '')
    const extracted = parseLdxpMailBody(body)
    event = {
      message_id: parsed.messageId ?? message.envelope?.messageId ?? '',
      imap_uid: String(message.uid),
      raw_hash: hashRawMail(source),
      mail_from: mailFrom,
      mail_to: addressText(parsed.to),
      subject,
      received_time: unixSeconds(parsed.date ?? toDate(message.internalDate) ?? new Date()),
      paid_time: extractPaidTime(body),
      body_excerpt: makeBodyExcerpt(body),
      ...extracted,
    }
  } catch (error) {
    console.warn(`Skipping unparsable LDXP mail subject=${redactForLog(subject)} from=${redactForLog(mailFrom)} reason=${errorMessage(error)}`)
    return false
  }

  await postMailEvent(config, toBackendPayload(event))
  return true
}

function toBackendPayload(event: ParsedMailEvent): WorkerMailEventPayload {
  return {
    message_id: event.message_id,
    imap_uid: event.imap_uid,
    raw_hash: event.raw_hash,
    from: event.mail_from,
    to: event.mail_to,
    subject: event.subject,
    received_time: event.received_time,
    order_no: event.order_no,
    amount: event.amount,
    product_name: event.product_name,
    card_key: event.card_key,
    paid_time: event.paid_time,
    body_excerpt: event.body_excerpt,
  }
}

function extractPaidTime(body: string): number {
  const match = body.match(/付款时间\s*[:：]?\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})/)
  if (!match?.[1]) {
    return 0
  }

  const timestamp = Date.parse(match[1].replace(' ', 'T'))
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : 0
}

function addressText(address: AddressLike | AddressLike[] | undefined): string {
  if (!address) {
    return ''
  }
  if (Array.isArray(address)) {
    return address.map((item) => item.text).filter(Boolean).join(', ')
  }
  return address.text ?? ''
}

function assertImapConfig(config: WorkerConfig): asserts config is WorkerConfig & {
  imapHost: string
  imapPort?: number
  imapUser: string
  imapPassword: string
} {
  const missing: string[] = []
  if (!config.imapHost) missing.push('imapHost')
  if (!config.imapUser) missing.push('imapUser')
  if (!config.imapPassword) missing.push('imapPassword')
  if (missing.length > 0) {
    throw new Error(`Missing IMAP config keys: ${missing.join(', ')}`)
  }
}

async function markSeen(client: ImapFlow, uid: number): Promise<void> {
  try {
    await client.messageFlagsAdd([uid], ['\\Seen'], { uid: true })
  } catch (error) {
    console.warn(`Unable to mark LDXP mail uid=${uid} as seen: ${errorMessage(error)}`)
  }
}

async function safeLogout(client: ImapFlow): Promise<void> {
  try {
    await client.logout()
  } catch {
    // Nothing useful to do during shutdown; do not log connection internals.
  }
}

function redactForLog(value: string): string {
  return value ? redactValue(value) : ''
}

function toDate(value: Date | string | undefined): Date | undefined {
  if (!value) {
    return undefined
  }
  return value instanceof Date ? value : new Date(value)
}

function unixSeconds(date: Date): number {
  return Math.floor(date.getTime() / 1000)
}

function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
