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
