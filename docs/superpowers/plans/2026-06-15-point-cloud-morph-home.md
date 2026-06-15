# Point Cloud Morph Home Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a homepage-only Active Theory-like point-cloud morph hero with balance-aware face state, generated black-hole and Lorenz-attractor 30,000-point shapes, and pointer disturbance.

**Architecture:** Generate deterministic equal-count binary point-cloud assets at build-time and commit them under `web/default/public/pointclouds/`. Keep user balance state selection in a small tested helper, keep raw WebGL rendering inside a client-only homepage component, and rewrite only the public homepage hero while preserving existing auth/docs/CTA behavior.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, raw WebGL, Node scripts, Node test runner, transient `npx tsx --test` for TS helper tests where Bun is unavailable locally.

---

## Files and responsibilities

- Modify: `docs/superpowers/specs/2026-06-15-event-horizon-public-frontend-design.md`
  - Update accepted design to point-cloud morph hero.
- Create: `web/default/scripts/pointcloud-utils.mjs`
  - Parse OBJ vertices, normalize/resample points, generate deterministic black-hole and Lorenz-attractor point clouds, write binary Float32 assets.
- Create: `web/default/scripts/pointcloud-utils.test.mjs`
  - Node tests for deterministic asset generation and point counts.
- Create: `web/default/scripts/generate-pointcloud-assets.mjs`
  - Generate committed runtime assets and manifest.
- Create: `web/default/scripts/pointcloud-source/face-current.obj`
  - Source face OBJ copied from the supplied offline point-cloud viewer.
- Create: `web/default/scripts/pointcloud-source/face-previous.obj`
  - Secondary source face OBJ copied from the available backup OBJ.
- Create: `web/default/public/pointclouds/*.bin`
  - Generated equal-count Float32 point-cloud positions.
- Create: `web/default/public/pointclouds/manifest.json`
  - Runtime manifest with names, URLs, counts, and shape roles.
- Create: `web/default/src/features/home/point-cloud/point-cloud-state.ts`
  - Pure helper for balance to face state and runtime morph sequence.
- Create: `web/default/src/features/home/point-cloud/point-cloud-state.test.ts`
  - Tests for balance-aware face selection and three-state sequence.
- Create: `web/default/src/features/home/point-cloud/point-cloud-morph-canvas.tsx`
  - Client-side raw WebGL point renderer, asset loader, morph loop, pointer disturbance, cleanup.
- Create: `web/default/src/features/home/point-cloud/index.ts`
  - Public exports for the point-cloud module.
- Modify: `web/default/src/features/home/components/sections/hero.tsx`
  - Replace current generic hero with point-cloud morph artwork while preserving CTA, docs URL, auth state, and public route behavior.
- Modify: `web/default/src/features/home/index.tsx`
  - Pass current user quota into the hero.

## Task 1: Update design spec and branch state

- [ ] **Step 1: Confirm spec no longer bans homepage WebGL**

Run:

```bash
grep -n "WebGL\|Three.js" docs/superpowers/specs/2026-06-15-event-horizon-public-frontend-design.md
```

Expected: The spec allows raw WebGL only in the homepage hero and avoids Three.js as a dependency.

- [ ] **Step 2: Commit spec and plan together**

Run:

```bash
git add docs/superpowers/specs/2026-06-15-event-horizon-public-frontend-design.md docs/superpowers/plans/2026-06-15-point-cloud-morph-home.md
git commit -m "docs: revise public frontend plan for point cloud morph hero"
```

Expected: A docs-only commit is created before production code changes.

## Task 2: Test and implement point-cloud state helper

- [ ] **Step 1: Write the failing TS helper test**

Create `web/default/src/features/home/point-cloud/point-cloud-state.test.ts`:

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  getFaceStateForQuota,
  getRuntimePointCloudSequence,
} from './point-cloud-state'

test('uses closed face when quota is missing or zero', () => {
  assert.equal(getFaceStateForQuota(undefined), 'closed')
  assert.equal(getFaceStateForQuota(null), 'closed')
  assert.equal(getFaceStateForQuota(0), 'closed')
  assert.equal(getFaceStateForQuota(-1), 'closed')
})

test('uses open face when quota is positive', () => {
  assert.equal(getFaceStateForQuota(1), 'open')
  assert.equal(getFaceStateForQuota(2500), 'open')
})

test('returns selected face, black hole, and Lorenz attractor sequence', () => {
  assert.deepEqual(getRuntimePointCloudSequence('closed'), [
    'face-closed',
    'black-hole',
    'lorenz-attractor',
  ])
  assert.deepEqual(getRuntimePointCloudSequence('open'), [
    'face-open',
    'black-hole',
    'lorenz-attractor',
  ])
})
```

- [ ] **Step 2: Run test to verify red**

Run:

```bash
cd web/default
npx -y tsx --test src/features/home/point-cloud/point-cloud-state.test.ts
```

Expected: FAIL because `point-cloud-state.ts` does not exist or exports are missing.

- [ ] **Step 3: Implement the minimal helper**

Create `web/default/src/features/home/point-cloud/point-cloud-state.ts`:

```ts
export type PointCloudFaceState = 'closed' | 'open'
export type RuntimePointCloudId = 'face-closed' | 'face-open' | 'black-hole' | 'lorenz-attractor'

export function getFaceStateForQuota(quota: number | null | undefined): PointCloudFaceState {
  return typeof quota === 'number' && quota > 0 ? 'open' : 'closed'
}

export function getRuntimePointCloudSequence(faceState: PointCloudFaceState): RuntimePointCloudId[] {
  return [faceState === 'open' ? 'face-open' : 'face-closed', 'black-hole', 'lorenz-attractor']
}
```

- [ ] **Step 4: Run test to verify green**

Run:

```bash
cd web/default
npx -y tsx --test src/features/home/point-cloud/point-cloud-state.test.ts
```

Expected: PASS.

## Task 3: Test and implement deterministic point-cloud asset generator

- [ ] **Step 1: Write the failing generator test**

Create `web/default/scripts/pointcloud-utils.test.mjs`:

```js
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  generateLorenzAttractorPoints,
  generateBlackHolePoints,
  normalizePointCount,
} from './pointcloud-utils.mjs'

test('generated black hole has exactly requested float count and is deterministic', () => {
  const a = generateBlackHolePoints(30000)
  const b = generateBlackHolePoints(30000)
  assert.equal(a.length, 90000)
  assert.equal(b.length, 90000)
  assert.deepEqual(Array.from(a.slice(0, 24)), Array.from(b.slice(0, 24)))
})

test('generated Lorenz attractor has exactly requested float count and is deterministic', () => {
  const a = generateLorenzAttractorPoints(30000)
  const b = generateLorenzAttractorPoints(30000)
  assert.equal(a.length, 90000)
  assert.equal(b.length, 90000)
  assert.deepEqual(Array.from(a.slice(0, 24)), Array.from(b.slice(0, 24)))
})

test('normalizePointCount resamples smaller clouds to the requested point count', () => {
  const source = new Float32Array([0, 0, 0, 1, 0, 0, 0, 1, 0])
  const out = normalizePointCount(source, 5, 7)
  assert.equal(out.length, 15)
  assert.deepEqual(Array.from(out.slice(0, 9)), Array.from(source))
})
```

- [ ] **Step 2: Run generator test to verify red**

Run:

```bash
cd web/default
node --test scripts/pointcloud-utils.test.mjs
```

Expected: FAIL because `pointcloud-utils.mjs` does not exist or exports are missing.

- [ ] **Step 3: Implement generator utilities**

Create `web/default/scripts/pointcloud-utils.mjs` with deterministic RNG, OBJ parsing, normalization, resampling, black-hole generation, Lorenz-attractor generation, and binary writer.

The key exported API must include:

```js
export function parseOBJVertices(objText) {}
export function normalizePointCount(points, targetCount, seed = 1) {}
export function generateBlackHolePoints(count) {}
export function generateLorenzAttractorPoints(count) {}
export function writeFloat32Binary(filePath, floatArray) {}
```

- [ ] **Step 4: Run generator test to verify green**

Run:

```bash
cd web/default
node --test scripts/pointcloud-utils.test.mjs
```

Expected: PASS.

## Task 4: Generate committed point-cloud assets

- [ ] **Step 1: Copy supplied face OBJ sources**

Run:

```bash
mkdir -p web/default/scripts/pointcloud-source
cp /Users/ethan/Documents/Codex/2026-06-04/face_pointcloud_offline/model.obj web/default/scripts/pointcloud-source/face-current.obj
cp /Users/ethan/Documents/Codex/2026-06-04/face_pointcloud_offline_backups/model.previous-before-e1df6acbc2b19770e7e25aae564f79c8.obj web/default/scripts/pointcloud-source/face-previous.obj
```

Expected: two OBJ source files exist in `web/default/scripts/pointcloud-source/`.

- [ ] **Step 2: Create asset generation script**

Create `web/default/scripts/generate-pointcloud-assets.mjs` to read both OBJ sources, derive `face-closed` and `face-open`, generate `black-hole` and `lorenz-attractor`, normalize all to 30,000 points, and write `.bin` files plus `manifest.json`.

- [ ] **Step 3: Run asset generation**

Run:

```bash
cd web/default
node scripts/generate-pointcloud-assets.mjs
```

Expected: `public/pointclouds/manifest.json` reports `pointCount: 30000` and four `.bin` files are written.

- [ ] **Step 4: Verify binary sizes**

Run:

```bash
python3 - <<'PY'
from pathlib import Path
base = Path('web/default/public/pointclouds')
for path in sorted(base.glob('*.bin')):
    print(path.name, path.stat().st_size)
PY
```

Expected: each `.bin` is `360000` bytes.

## Task 5: Implement raw WebGL point-cloud renderer

- [ ] **Step 1: Create renderer component**

Create `web/default/src/features/home/point-cloud/point-cloud-morph-canvas.tsx` with:

- `PointCloudMorphCanvas` component,
- manifest fetch,
- `.bin` asset fetch,
- WebGL program setup,
- `aFrom` and `aTo` position attributes,
- `uMorph`, `uTime`, `uMouse`, `uMouseStrength`, `uAspect`, `uPixelRatio` uniforms,
- automatic cycle and click-to-advance,
- cleanup for animation frame, event listeners, buffers, and program.

- [ ] **Step 2: Add public exports**

Create `web/default/src/features/home/point-cloud/index.ts`:

```ts
export {
  getFaceStateForQuota,
  getRuntimePointCloudSequence,
  type PointCloudFaceState,
  type RuntimePointCloudId,
} from './point-cloud-state'
export { PointCloudMorphCanvas } from './point-cloud-morph-canvas'
```

Expected: homepage can import from `../point-cloud` or `../../point-cloud` depending relative path.

## Task 6: Replace homepage hero with the point-cloud artwork

- [ ] **Step 1: Modify `web/default/src/features/home/index.tsx`**

Pass user quota into Hero:

```tsx
<Hero isAuthenticated={isAuthenticated} userQuota={auth.user?.quota} />
```

- [ ] **Step 2: Modify Hero props and layout**

Update `web/default/src/features/home/components/sections/hero.tsx` so `HeroProps` includes:

```ts
userQuota?: number | null
```

Use:

```ts
const faceState = getFaceStateForQuota(props.userQuota)
```

Render `PointCloudMorphCanvas` as the full hero visual and preserve current CTA logic for dashboard/sign-up/pricing/docs.

- [ ] **Step 3: Keep docs URL and auth logic intact**

Do not modify `useStatus()`, docs URL fallback, `props.isAuthenticated`, or existing Link targets.

## Task 7: Verification

- [ ] **Step 1: Run pure tests**

Run:

```bash
cd web/default
node --test scripts/pointcloud-utils.test.mjs
npx -y tsx --test src/features/home/point-cloud/point-cloud-state.test.ts
```

Expected: both pass.

- [ ] **Step 2: Run typecheck/build in Docker because local Bun is unavailable**

Run:

```bash
cd /Users/ethan/Desktop/云贝/云贝网站/new-api
docker build --target builder -t new-api-web-pointcloud-check .
```

Expected: Rsbuild frontend build succeeds in the Docker builder stage.

- [ ] **Step 3: Build full image if frontend builder passes**

Run:

```bash
docker build -t new-api:pointcloud-morph .
```

Expected: full image builds.

- [ ] **Step 4: Browser verify homepage**

Run the app or update the existing compose image, then open `http://localhost:3000/` and verify:

- point-cloud hero appears,
- clicking point-cloud advances morph,
- mouse movement disturbs particles,
- public CTA buttons still navigate,
- homepage custom content branch remains untouched when configured.

