import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

interface PackageJson {
  scripts?: Record<string, string>
}

test('package start script points to the compiled worker entry', async () => {
  const packageJson = JSON.parse(await readFile(new URL('../../package.json', import.meta.url), 'utf8')) as PackageJson

  assert.ok(packageJson.scripts?.start, 'scripts.start must exist')
  assert.equal(packageJson.scripts.start, 'node dist/src/index.js')
})


test('Dockerfile pins the Playwright base image to the package-lock version', async () => {
  const packageLock = JSON.parse(await readFile(new URL('../../package-lock.json', import.meta.url), 'utf8')) as {
    packages?: Record<string, { version?: string }>
  }
  const dockerfile = await readFile(new URL('../../Dockerfile', import.meta.url), 'utf8')
  const playwrightVersion = packageLock.packages?.['node_modules/playwright']?.version

  assert.ok(playwrightVersion, 'package-lock must include node_modules/playwright')
  assert.match(
    dockerfile,
    new RegExp(`^FROM mcr\.microsoft\.com/playwright:v${playwrightVersion}-jammy$`, 'm'),
  )
})
