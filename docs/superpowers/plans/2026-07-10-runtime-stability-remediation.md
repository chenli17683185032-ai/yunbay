# Runtime Stability Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop Caddy PID exhaustion, eliminate LDXP empty-queue error storms, and make new-api source releases attributable and free of workstation metadata.

**Architecture:** Apply the Caddy fix as an isolated compose override so production-only compose data remains local. Map only `gorm.ErrRecordNotFound` to the Worker's already-supported 404 empty-queue contract. Build deployments from `git archive` with a SHA manifest instead of copying a development worktree.

**Tech Stack:** Docker Compose, tini/Docker init, Bash, Git archive, Go/Gin/GORM, Bun/Node test runner.

---

## File map

**Create**

- `deploy/docker-compose.caddy-init.yml` — secret-free production override.
- `deploy/verify-caddy-init.sh` — compose-merge and runtime PID assertions.
- `deploy/build-source-release.sh` — tracked-file-only release archive and SHA manifest.
- `deploy/build-source-release.test.sh` — release hygiene regression.

**Modify**

- `controller/ldxp_topup.go` — return 404 only for an empty claim queue.
- `controller/ldxp_topup_test.go` — empty queue and real DB error distinction.
- `workers/ldxp-browser-worker/tests/backend.test.ts` — retain explicit 404-as-null regression.

**No change required**

- `workers/ldxp-browser-worker/src/backend.ts` already returns `null` before parsing a permitted 404.
- Production `/opt/new-api/app/docker-compose.prod.yml` remains environment-local and is backed up before applying the tracked override.

## Task 1: Correct the LDXP empty-queue HTTP contract

**Files:**
- Modify: `controller/ldxp_topup_test.go`
- Modify: `controller/ldxp_topup.go`
- Modify: `workers/ldxp-browser-worker/tests/backend.test.ts`

- [ ] **Step 1: Add a failing controller test for an empty queue**

```go
func TestWorkerClaimLdxpTopupSessionReturns404WhenQueueIsEmpty(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	recorder := performLdxpControllerRequest(
		WorkerClaimLdxpTopupSession,
		http.MethodPost,
		"/ldxp/worker/sessions/claim",
		gin.H{"worker_id": "worker-a"},
		0,
		map[string]string{"x-ldxp-worker-token": ldxpControllerTestWorkerToken},
	)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}
```

Add a second test that drops `ldxp_topup_sessions`, calls the same handler, and asserts the response is not 404 and contains `success:false`.

- [ ] **Step 2: Confirm RED**

```bash
go test ./controller -run '^TestWorkerClaimLdxpTopupSession(Returns404WhenQueueIsEmpty|DoesNotHideDatabaseErrors)$' -count=1
```

Expected: the empty queue currently returns HTTP 200 with `success:false`.

- [ ] **Step 3: Implement the narrow error mapping**

Add `errors` and `gorm.io/gorm` imports, then change only the top-up claim handler:

```go
session, err := service.ClaimLdxpTopupSession(req.WorkerID, cfg)
if errors.Is(err, gorm.ErrRecordNotFound) {
	c.Status(http.StatusNotFound)
	return
}
if err != nil {
	common.ApiError(c, err)
	return
}
common.ApiSuccess(c, buildLdxpWorkerClaimResponse(session))
```

Do not catch all errors and do not change the paid-watch claim handler unless a separate failing test proves the same production noise.

- [ ] **Step 4: Keep the Worker contract explicit**

Extend the existing Worker test to count that no JSON parse is required for a plain-text 404 and that HTTP 500 still rejects:

```ts
test('claimSession rejects a real backend error', async () => {
  const server = await withServer((_req, res) => {
    res.statusCode = 500
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ success: false, message: 'database unavailable' }))
  })
  try {
    await assert.rejects(() => claimSession(config(server.baseUrl)), /database unavailable/)
  } finally {
    await server.close()
  }
})
```

- [ ] **Step 5: Run backend and Worker tests**

```bash
go test ./controller -run 'WorkerClaimLdxpTopupSession' -count=1
cd workers/ldxp-browser-worker
bun test tests/backend.test.ts
bun run check
```

Expected: controller tests pass and the full Worker check remains 75+ tests passing.

- [ ] **Step 6: Commit the LDXP fix**

```bash
git add controller/ldxp_topup.go controller/ldxp_topup_test.go workers/ldxp-browser-worker/tests/backend.test.ts
git commit -m "fix: treat empty ldxp queue as idle"
```

## Task 2: Add the Caddy init override and static verifier

**Files:**
- Create: `deploy/docker-compose.caddy-init.yml`
- Create: `deploy/verify-caddy-init.sh`

- [ ] **Step 1: Write the verifier first**

`deploy/verify-caddy-init.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

base=${1:?usage: verify-caddy-init.sh BASE_COMPOSE [ENV_FILE]}
env_file=${2:-}
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
override="$root/deploy/docker-compose.caddy-init.yml"

args=(-f "$base" -f "$override")
if [[ -n "$env_file" ]]; then
  args=(--env-file "$env_file" "${args[@]}")
fi

rendered=$(docker compose "${args[@]}" config --format json)
python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["services"]["caddy"].get("init") is True, d["services"]["caddy"]' <<<"$rendered"
printf 'caddy init override verified\n'
```

Make it executable.

- [ ] **Step 2: Run it before the override exists and confirm RED**

```bash
deploy/verify-caddy-init.sh docker-compose.yml
```

Expected: failure because the override file is absent.

- [ ] **Step 3: Add the minimal override**

`deploy/docker-compose.caddy-init.yml`:

```yaml
services:
  caddy:
    init: true
```

Do not duplicate the production Caddy image, volumes, ports, environment, or healthcheck in the override.

- [ ] **Step 4: Verify the merged configuration**

The public root compose has no Caddy service, so create a temporary fixture:

```bash
cat >/tmp/caddy-compose.yml <<'YAML'
services:
  caddy:
    image: caddy:2-alpine
YAML
deploy/verify-caddy-init.sh /tmp/caddy-compose.yml
bash -n deploy/verify-caddy-init.sh
```

Expected: `caddy init override verified`.

- [ ] **Step 5: Commit the override**

```bash
git add deploy/docker-compose.caddy-init.yml deploy/verify-caddy-init.sh
git commit -m "fix: add caddy init process reaper"
```

## Task 3: Build reproducible source releases without `.git` or AppleDouble

**Files:**
- Create: `deploy/build-source-release.sh`
- Create: `deploy/build-source-release.test.sh`

- [ ] **Step 1: Write the release hygiene test**

The test initializes a temporary Git repository with one tracked file and untracked `.git`, `._junk`, and `.DS_Store` fixtures, invokes the builder, then asserts only committed content plus the manifest is present.

```bash
#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/repo"
git -C "$tmp/repo" init -q
git -C "$tmp/repo" config user.name test
git -C "$tmp/repo" config user.email test@example.test
printf 'tracked\n' >"$tmp/repo/tracked.txt"
git -C "$tmp/repo" add tracked.txt
git -C "$tmp/repo" commit -qm init
printf 'junk\n' >"$tmp/repo/._junk"
printf 'junk\n' >"$tmp/repo/.DS_Store"

"$repo_root/deploy/build-source-release.sh" "$tmp/repo" HEAD "$tmp/release.tar.gz"
mkdir "$tmp/out"
tar -xzf "$tmp/release.tar.gz" -C "$tmp/out"
test -f "$tmp/out/source/tracked.txt"
test -f "$tmp/out/source/.yunbay-source-manifest"
test ! -e "$tmp/out/source/.git"
test ! -e "$tmp/out/source/._junk"
test ! -e "$tmp/out/source/.DS_Store"
```

- [ ] **Step 2: Confirm RED**

```bash
bash deploy/build-source-release.test.sh
```

Expected: command-not-found because the builder does not exist.

- [ ] **Step 3: Implement the builder using `git archive`**

```bash
#!/usr/bin/env bash
set -euo pipefail

repo=${1:?usage: build-source-release.sh REPO REV OUTPUT}
rev=${2:?usage: build-source-release.sh REPO REV OUTPUT}
output=${3:?usage: build-source-release.sh REPO REV OUTPUT}
sha=$(git -C "$repo" rev-parse --verify "$rev^{commit}")
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/source"
git -C "$repo" archive "$sha" | tar -xf - -C "$tmp/source"

if find "$tmp/source" -name '._*' -o -name '.DS_Store' | grep -q .; then
  echo 'release contains workstation metadata' >&2
  exit 1
fi
printf 'git_sha=%s\nbuilt_at_utc=%s\n' "$sha" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$tmp/source/.yunbay-source-manifest"
tar -C "$tmp" -czf "$output" source
sha256sum "$output" >"$output.sha256"
```

Use `shasum -a 256` when `sha256sum` is unavailable on macOS:

```bash
if command -v sha256sum >/dev/null; then sha256sum "$output" >"$output.sha256"; else shasum -a 256 "$output" >"$output.sha256"; fi
```

- [ ] **Step 4: Run the test and build a real archive**

```bash
chmod +x deploy/build-source-release.sh deploy/build-source-release.test.sh
bash deploy/build-source-release.test.sh
deploy/build-source-release.sh . HEAD /tmp/yunbay-source-$(git rev-parse --short HEAD).tar.gz
tar -tzf /tmp/yunbay-source-$(git rev-parse --short HEAD).tar.gz | rg '/(\.git|\._|\.DS_Store)' && exit 1 || true
```

Expected: test passes and forbidden-path search has no output.

- [ ] **Step 5: Commit release tooling**

```bash
git add deploy/build-source-release.sh deploy/build-source-release.test.sh
git commit -m "build: package tracked deployment sources"
```

## Task 4: Run the runtime-unit verification gate

**Files:** all files in this plan.

- [ ] **Step 1: Run Go and Worker checks**

```bash
go test ./controller -run 'Ldxp' -count=1
cd workers/ldxp-browser-worker
bun run check
```

Expected: PASS.

- [ ] **Step 2: Run shell/config checks**

```bash
cd /Users/ethan/Documents/yunbay
bash -n deploy/verify-caddy-init.sh deploy/build-source-release.sh deploy/build-source-release.test.sh
bash deploy/build-source-release.test.sh
```

Expected: PASS.

- [ ] **Step 3: Build and inspect the candidate release**

```bash
release=/tmp/yunbay-source-$(git rev-parse --short HEAD).tar.gz
deploy/build-source-release.sh . HEAD "$release"
tar -tzf "$release" | sort >"$release.files"
rg '/(\.git|\._[^/]*|\.DS_Store)$' "$release.files" && exit 1 || true
```

Expected: no forbidden entries.

- [ ] **Step 4: Record production verification commands for rollout**

The deployment plan must run, without printing secrets:

```bash
docker compose --env-file /opt/new-api/secrets/prod.env \
  -f /opt/new-api/app/docker-compose.prod.yml \
  -f /opt/new-api/app/deploy/docker-compose.caddy-init.yml \
  config --format json | python3 -c 'import json,sys; assert json.load(sys.stdin)["services"]["caddy"]["init"] is True'
```

and after recreation:

```bash
docker inspect --format '{{.State.Health.Status}} init={{.HostConfig.Init}} pids={{.HostConfig.PidsLimit}}' yunbay-caddy
docker top yunbay-caddy -eo stat,ppid,pid,comm | awk '$1 ~ /^Z/ {count++} END {print count+0}'
```

Expected during rollout: `healthy`, `init=true`, and zombie count `0` across at least two 30-second healthcheck intervals.
