import test from 'node:test'
import assert from 'node:assert/strict'
import { createReusableBrowserManager } from '../src/browser-manager.js'

test('BrowserManager reuses one Chromium browser across contexts', async () => {
  const fake = new FakeBrowser()
  let launches = 0
  const manager = createReusableBrowserManager({
    launch: async () => {
      launches += 1
      return fake.asBrowser()
    },
  })

  const first = await manager.getContext()
  const second = await manager.getContext()

  assert.equal(launches, 1)
  assert.equal(fake.contexts, 2)
  await first.close()
  await second.close()
  await manager.close()
  assert.equal(fake.closed, true)
})

test('BrowserManager restarts disconnected Chromium browser', async () => {
  const first = new FakeBrowser()
  const second = new FakeBrowser()
  const launched = [first, second]
  const manager = createReusableBrowserManager({
    launch: async () => {
      const next = launched.shift()
      assert.ok(next)
      return next.asBrowser()
    },
  })

  await manager.getContext()
  first.disconnect()
  await manager.getContext()

  assert.equal(first.contexts, 1)
  assert.equal(second.contexts, 1)
  await manager.close()
})

class FakeBrowser {
  contexts = 0
  closed = false
  private connected = true
  private disconnectedHandlers: Array<() => void> = []

  asBrowser() {
    return {
      newContext: async () => {
        this.contexts += 1
        return { close: async () => undefined }
      },
      close: async () => {
        this.closed = true
        this.connected = false
      },
      isConnected: () => this.connected,
      on: (event: string, handler: () => void) => {
        if (event === 'disconnected') {
          this.disconnectedHandlers.push(handler)
        }
        return this.asBrowser()
      },
    } as never
  }

  disconnect(): void {
    this.connected = false
    for (const handler of this.disconnectedHandlers) {
      handler()
    }
  }
}
