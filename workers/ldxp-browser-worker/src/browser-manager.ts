import { chromium, type Browser, type BrowserContext, type BrowserContextOptions } from 'playwright'
import { buildBrowserContextOptions, buildBrowserLaunchOptions } from './browser-flow.js'

export interface BrowserManager {
  getContext(): Promise<BrowserContext>
  close(): Promise<void>
  restart(): Promise<void>
}

type BrowserLauncher = (options: ReturnType<typeof buildBrowserLaunchOptions>) => Promise<Browser>

interface ReusableBrowserManagerOptions {
  launch?: BrowserLauncher
  contextOptions?: BrowserContextOptions
}

export function createReusableBrowserManager(options: ReusableBrowserManagerOptions = {}): BrowserManager {
  return new ReusableBrowserManager(options.launch ?? ((launchOptions) => chromium.launch(launchOptions)), options.contextOptions)
}

class ReusableBrowserManager implements BrowserManager {
  private browserPromise: Promise<Browser> | undefined

  constructor(
    private readonly launch: BrowserLauncher,
    private readonly contextOptions: BrowserContextOptions | undefined,
  ) {}

  async getContext(): Promise<BrowserContext> {
    const browser = await this.getBrowser()
    return browser.newContext(this.contextOptions ?? buildBrowserContextOptions())
  }

  async close(): Promise<void> {
    const browserPromise = this.browserPromise
    this.browserPromise = undefined
    if (!browserPromise) {
      return
    }
    const browser = await browserPromise.catch(() => undefined)
    await browser?.close().catch(() => undefined)
  }

  async restart(): Promise<void> {
    await this.close()
    await this.getBrowser()
  }

  private async getBrowser(): Promise<Browser> {
    if (this.browserPromise) {
      const existing = await this.browserPromise.catch(() => undefined)
      if (existing?.isConnected()) {
        return existing
      }
      this.browserPromise = undefined
    }

    const browserPromise = this.launch(buildBrowserLaunchOptions())
    this.browserPromise = browserPromise
    const browser = await browserPromise
    browser.on('disconnected', () => {
      if (this.browserPromise === browserPromise) {
        this.browserPromise = undefined
      }
    })
    return browser
  }
}
