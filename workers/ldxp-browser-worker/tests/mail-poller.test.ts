import test from 'node:test'
import assert from 'node:assert/strict'
import type { FetchMessageObject } from 'imapflow'
import { postFetchedMessages } from '../src/mail-poller.js'
import type { WorkerConfig } from '../src/config.js'

function config(): WorkerConfig {
  return {
    backendBaseUrl: 'https://backend.example',
    workerToken: 'worker-token-secret',
    workerId: 'worker-a',
    pollIntervalMs: 1000,
    claimIntervalMs: 1000,
    maxConcurrentSessions: 3,
    productLoadTimeoutMs: 30000,
    qrTimeoutMs: 60000,
    paymentTimeoutMs: 900000,
    resultTimeoutMs: 120000,
    debugSnapshotDir: '/app/snapshots',
  }
}

test('marks accepted mail seen only after fetch iteration completes', async () => {
  const events: string[] = []
  let fetchActive = false
  const message = { uid: 42, seq: 1, source: Buffer.from('mail') } as FetchMessageObject

  async function* fetchMessages(): AsyncIterable<FetchMessageObject> {
    fetchActive = true
    events.push('yield-message')
    yield message
    events.push('fetch-complete')
    fetchActive = false
  }

  const client = {
    async messageFlagsAdd(uids: number[], flags: string[], options: { uid?: boolean }): Promise<boolean> {
      if (fetchActive) {
        throw new Error('messageFlagsAdd called while fetch is active')
      }
      events.push('mark-seen')
      assert.deepEqual(uids, [42])
      assert.deepEqual(flags, ['\\Seen'])
      assert.deepEqual(options, { uid: true })
      return true
    },
  }

  const postedCount = await postFetchedMessages(config(), fetchMessages(), client, {
    parseAndPostMessage: async (_config, fetchedMessage) => {
      events.push('post')
      assert.equal(fetchedMessage.uid, 42)
      return true
    },
  })

  assert.equal(postedCount, 1)
  assert.deepEqual(events, ['yield-message', 'post', 'fetch-complete', 'mark-seen'])
})
