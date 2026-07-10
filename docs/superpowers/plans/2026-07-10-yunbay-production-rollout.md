# Yunbay Production Rollout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate, push, back up, deploy, and prove all four approved Yunbay fixes on production with independent stop and rollback points.

**Architecture:** Execute the four subsystem plans as committed units, run one completion audit, merge the feature branch into local `main`, and push the exact merge SHA. Production rollout is phased: Caddy first, source/images second, quota migration, new-api, sub2api, then optional per-model channel exposure; every stateful phase has a SHA-addressed backup and rollback command.

**Tech Stack:** Git/GitHub, Go/Bun/Docker Compose, SSH over the configured local CONNECT proxy, PostgreSQL `pg_dump`, curl, Codex in-app browser for authenticated admin verification.

---

## Required execution order

1. `2026-07-10-runtime-stability-remediation.md` — Caddy artifact and LDXP fix first.
2. `2026-07-10-value-package-quota-repair.md` — API, renewal, migration, UI.
3. `2026-07-10-value-package-group-pricing.md` — resolver, atomic settings, frontend.
4. `2026-07-10-gpt-5-6-support.md` — new-api/sub2api support and full overlay verification.
5. This rollout plan — review, merge, push, backup, phased deployment, completion audit.

Do not run production mutation steps until all implementation plans are GREEN and committed.

## Task 1: Establish the final local verification baseline

**Files:** all changed implementation and plan files.

- [ ] **Step 1: Confirm branch and preserve the pre-existing untracked file**

```bash
cd /Users/ethan/Documents/yunbay
test "$(git branch --show-current)" = "codex/yunbay-production-remediation"
test -f docs/superpowers/specs/2026-07-08-sub2api-force-priority-server-design.md
git status --short --branch
```

Expected: implementation files are committed; the pre-existing 2026-07-08 spec may remain untracked and must not be staged.

- [ ] **Step 2: Run complete new-api backend verification**

```bash
go test ./... -count=1
```

Expected: all packages PASS; no package is silently skipped due compile errors.

- [ ] **Step 3: Run complete default frontend verification**

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
bun test
bun run typecheck
bun run build
```

Expected: all commands exit 0 and `dist/index.html` is produced.

- [ ] **Step 4: Run Worker verification**

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run check
```

Expected: all tests/type checks pass.

- [ ] **Step 5: Run real sub2api verification**

```bash
cd /Users/ethan/Documents/yunbay
infra/sub2api/scripts/verify-overlay.sh /tmp/yunbay-sub2api-verify
```

Expected: pinned backend unit suite, frontend tests, and frontend build all pass. Manifest absence in the main repository is not an acceptable skip.

- [ ] **Step 6: Run deployment artifact checks**

```bash
bash -n deploy/*.sh infra/sub2api/scripts/verify-overlay.sh
bash deploy/build-source-release.test.sh
release=/tmp/yunbay-remediation-$(git rev-parse --short HEAD).tar.gz
deploy/build-source-release.sh . HEAD "$release"
test -f "$release.sha256"
```

Expected: PASS and a release archive exists.

## Task 2: Review the implementation against the approved spec

**Files:**
- `docs/superpowers/specs/2026-07-10-yunbay-production-remediation-design.md`
- all changed files since `f7358ed8`.

- [ ] **Step 1: Run the requirement-to-evidence audit**

Create a local checklist from spec sections 14.1–14.4 and map each item to a test, file, or later production check. At minimum prove locally:

```text
Bug 1: period arrays, renewal idempotency, migration dry-run/apply tests, frontend labels
Bug 2: wallet/regular/value-package matrix, tiered path, atomic save, source logging
Bug 3: 404-only empty queue, Worker 500 rejection, compose init merge, clean archive
Bug 4: four exact models, four prices, alias parity, unknown suffix no 5.4 fallback
```

- [ ] **Step 2: Inspect the entire diff**

```bash
git diff --check main...HEAD
git diff --stat main...HEAD
git diff --name-status main...HEAD
git diff main...HEAD -- . ':(exclude)web/default/src/i18n/locales/*.json'
```

Expected: no accidental protected-brand edits, no secret values, no unrelated image-routing refactor, no production credential file.

- [ ] **Step 3: Run focused forbidden-pattern scans**

```bash
rg -n 'subscriptionBillingGroupRatio = 1\.0' service || true
rg -n 'gpt-5\.6.*gpt-5\.4|gpt-5\.6-pro' setting infra/sub2api --glob '!**/*_test.go' || true
git diff main...HEAD | rg 'BEGIN .*PRIVATE KEY|experimental_bearer_token|SESSION_SECRET=|POSTGRES_PASSWORD=' && exit 1 || true
```

Expected: no prohibited runtime fallback or committed secret.

- [ ] **Step 4: Perform code review using the project review skill**

Invoke `requesting-code-review`; address every correctness issue before proceeding. Re-run the smallest affected tests after each fix, then rerun Task 1's full verification.

- [ ] **Step 5: Ensure the branch is fully committed**

```bash
git status --short
git log --oneline --decorate main..HEAD
```

Expected: only the pre-existing untracked 2026-07-08 spec remains; all implementation/plan changes are committed in reviewable commits.

## Task 3: Merge the workspace and push GitHub

**Files:** Git history only.

- [ ] **Step 1: Record current Git identity and historical authors**

```bash
git config user.name
git config user.email
git log --format='%aN <%aE>' -n 200 | sort | uniq -c | sort -nr | head -n 15
```

Do not change Git configuration. This repository rule matters only if a PR is created; direct push does not need an ad hoc PR body.

- [ ] **Step 2: Push the feature branch as a safety reference**

```bash
git push -u origin codex/yunbay-production-remediation
```

Expected: remote branch points to local HEAD.

- [ ] **Step 3: Merge into current `main` without touching the untracked spec**

```bash
git switch main
git pull --ff-only origin main
git merge --no-ff codex/yunbay-production-remediation -m "merge: remediate yunbay production issues"
```

If `origin/main` advanced and conflicts exist, return to the feature branch, rebase/merge deliberately, rerun full verification, then repeat. Do not resolve by discarding either side wholesale.

- [ ] **Step 4: Verify the exact merge tree**

```bash
git status --short --branch
git diff --check HEAD^1..HEAD
go test ./... -count=1
cd web/default && bun run typecheck && bun run build
```

Expected: PASS; only the pre-existing untracked file remains.

- [ ] **Step 5: Push `main` and freeze the release SHA**

```bash
git push origin main
RELEASE_SHA=$(git rev-parse HEAD)
test "$(git rev-parse origin/main)" = "$RELEASE_SHA"
printf '%s\n' "$RELEASE_SHA" > /tmp/yunbay-release-sha
```

Expected: local main, origin/main, and `/tmp/yunbay-release-sha` agree.

- [ ] **Step 6: Build the final archive from the merge SHA**

```bash
RELEASE_SHA=$(cat /tmp/yunbay-release-sha)
RELEASE=/tmp/yunbay-${RELEASE_SHA}.tar.gz
deploy/build-source-release.sh . "$RELEASE_SHA" "$RELEASE"
```

Expected: archive and checksum files exist.

## Task 4: Create a production backup and immutable preflight record

**Files/State:** `/opt/new-api/backups`, Docker images, PostgreSQL, production compose/config.

- [ ] **Step 1: Define the approved SSH command locally**

```bash
SSH=(ssh -p 2222 \
  -o 'ProxyCommand=nc -X connect -x 127.0.0.1:7897 %h %p' \
  -o HostKeyAlias=13.140.180.223 \
  -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' \
  -o IdentitiesOnly=yes \
  -o StrictHostKeyChecking=yes \
  -o UserKnownHostsFile='/Users/ethan/Desktop/云贝/服务器相关/ssh/known_hosts' \
  deploy@13.140.180.223)
```

Never echo or copy the key content.

- [ ] **Step 2: Create a restricted backup directory**

```bash
RELEASE_SHA=$(cat /tmp/yunbay-release-sha)
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP="/opt/new-api/backups/yunbay-remediation-${STAMP}-${RELEASE_SHA}"
"${SSH[@]}" "install -d -m 700 '$BACKUP' && printf '%s' '$BACKUP' > /tmp/yunbay-remediation-backup-path"
```

- [ ] **Step 3: Back up configuration and exact image IDs without printing secrets**

```bash
"${SSH[@]}" 'set -e
BACKUP=$(cat /tmp/yunbay-remediation-backup-path)
install -m 600 /opt/new-api/secrets/prod.env "$BACKUP/prod.env"
install -m 600 /opt/new-api/app/docker-compose.prod.yml "$BACKUP/docker-compose.prod.yml"
install -m 600 /opt/new-api/app/Caddyfile "$BACKUP/Caddyfile"
cp -a /opt/new-api/app/.yunbay-deploy-sha "$BACKUP/deploy-sha.before" 2>/dev/null || true
for c in yunbay-new-api yunbay-sub2api yunbay-caddy yunbay-ldxp-browser-worker; do
  docker inspect --format "{{.Name}}|{{.Image}}|{{.Config.Image}}|{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}" "$c"
done >"$BACKUP/container-images.before"
chmod 600 "$BACKUP"/*'
```

- [ ] **Step 4: Back up PostgreSQL**

```bash
"${SSH[@]}" 'set -e
BACKUP=$(cat /tmp/yunbay-remediation-backup-path)
docker exec yunbay-postgres sh -c '\''pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc'\'' >"$BACKUP/newapi.dump"
test -s "$BACKUP/newapi.dump"
chmod 600 "$BACKUP/newapi.dump"'
```

This command reads database credentials only inside the database container and does not print them.

- [ ] **Step 5: Back up sub2api overlay targets and channel state**

```bash
"${SSH[@]}" 'set -e
BACKUP=$(cat /tmp/yunbay-remediation-backup-path)
tar -C /opt/new-api/sub2api -czf "$BACKUP/sub2api-source.before.tar.gz" source
docker exec yunbay-postgres sh -c '\''psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "COPY (SELECT id,name,models,base_url FROM channels WHERE id IN (35,36)) TO STDOUT WITH CSV HEADER"'\'' >"$BACKUP/channels-35-36.csv"
chmod 600 "$BACKUP/sub2api-source.before.tar.gz" "$BACKUP/channels-35-36.csv"'
```

- [ ] **Step 6: Check capacity and baseline endpoints**

```bash
"${SSH[@]}" 'df -h /opt/new-api; docker system df; curl -fsS https://yunbay.xyz/api/status >/dev/null; curl -fsS https://sub2api.yunbay.xyz/ >/dev/null'
```

Stop if free disk is below 15 GiB, PostgreSQL dump is empty, or a baseline endpoint is already failing for an unexplained reason.

## Task 5: Repair Caddy first and prove PID stability

**Files/State:** production Caddy container and tracked override.

- [ ] **Step 1: Upload only the override and verifier**

```bash
scp -P 2222 \
  -o 'ProxyCommand=nc -X connect -x 127.0.0.1:7897 %h %p' \
  -o HostKeyAlias=13.140.180.223 \
  -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' \
  deploy/docker-compose.caddy-init.yml deploy/verify-caddy-init.sh \
  deploy@13.140.180.223:/opt/new-api/app/deploy/
```

- [ ] **Step 2: Validate the production merge before recreation**

```bash
"${SSH[@]}" 'cd /opt/new-api/app && deploy/verify-caddy-init.sh docker-compose.prod.yml /opt/new-api/secrets/prod.env'
```

Expected: `caddy init override verified`.

- [ ] **Step 3: Recreate only Caddy**

```bash
"${SSH[@]}" 'cd /opt/new-api/app && docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml up -d --no-deps --force-recreate caddy'
```

- [ ] **Step 4: Observe at least two healthcheck intervals**

```bash
"${SSH[@]}" 'set -e
for i in 1 2 3; do
  sleep 35
  docker inspect --format "health={{.State.Health.Status}} init={{.HostConfig.Init}} image={{.Image}}" yunbay-caddy
  docker top yunbay-caddy -eo stat,ppid,pid,comm | awk '\''$1 ~ /^Z/ {n++} END {print "zombies=" n+0}'\''
  curl -fsS -H "Host: yunbay.xyz" http://127.0.0.1/api/status >/dev/null
done'
```

Expected each iteration: `health=healthy`, `init=true`, `zombies=0`.

Stop and restore `$BACKUP/docker-compose.prod.yml` if the public endpoint, certificate path, volume mount, or routing changes unexpectedly.

## Task 6: Install the exact source release and build rollback-tagged images

**Files/State:** `/opt/new-api/releases/$SHA`, `/opt/new-api/app`, Docker images.

- [ ] **Step 1: Upload and verify the release archive**

```bash
RELEASE_SHA=$(cat /tmp/yunbay-release-sha)
RELEASE=/tmp/yunbay-${RELEASE_SHA}.tar.gz
scp -P 2222 -o 'ProxyCommand=nc -X connect -x 127.0.0.1:7897 %h %p' -o HostKeyAlias=13.140.180.223 -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' "$RELEASE" "$RELEASE.sha256" deploy@13.140.180.223:/tmp/
"${SSH[@]}" "cd /tmp && sha256sum -c '$(basename "$RELEASE").sha256'"
```

- [ ] **Step 2: Extract into a SHA-addressed release directory**

```bash
"${SSH[@]}" "set -e; RELEASE_SHA='$RELEASE_SHA'; install -d -m 755 /opt/new-api/releases/\"\$RELEASE_SHA\"; tar -xzf '/tmp/$(basename "$RELEASE")' -C /opt/new-api/releases/\"\$RELEASE_SHA\"; grep -qx 'git_sha=$RELEASE_SHA' /opt/new-api/releases/\"\$RELEASE_SHA\"/source/.yunbay-source-manifest"
```

- [ ] **Step 3: Sync tracked source while preserving production-local files**

```bash
"${SSH[@]}" "set -e
BACKUP=\$(cat /tmp/yunbay-remediation-backup-path)
cp -a /opt/new-api/app/.git \"\$BACKUP/app-dot-git.before\" 2>/dev/null || true
rsync -a --delete \
  --exclude docker-compose.prod.yml \
  --exclude .yunbay-deploy-sha \
  /opt/new-api/releases/$RELEASE_SHA/source/ /opt/new-api/app/
rm -f /opt/new-api/app/.git
find /opt/new-api/app -name '._*' -type f -delete
test -z \"\$(find /opt/new-api/app -name '._*' -type f -print -quit)\""
```

- [ ] **Step 4: Tag current images for rollback**

```bash
"${SSH[@]}" "set -e
STAMP='$STAMP'
docker image tag \"\$(docker inspect --format '{{.Image}}' yunbay-new-api)\" yunbay-new-api:rollback-\$STAMP
docker image tag \"\$(docker inspect --format '{{.Image}}' yunbay-sub2api)\" yunbay-sub2api:rollback-\$STAMP
docker image tag \"\$(docker inspect --format '{{.Image}}' yunbay-ldxp-browser-worker)\" yunbay-ldxp-browser-worker:rollback-\$STAMP"
```

- [ ] **Step 5: Build new-api and Worker without recreating them yet**

```bash
"${SSH[@]}" 'cd /opt/new-api/app && docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml build new-api ldxp-browser-worker'
```

Expected: build succeeds and current running containers still reference old image IDs.

## Task 7: Dry-run and apply the approved B2 quota migration

**State:** `user_subscriptions.amount_total` for target active value packages.

- [ ] **Step 1: Run dry-run from the newly built image**

```bash
"${SSH[@]}" 'set -e
cd /opt/new-api/app
BACKUP=$(cat /tmp/yunbay-remediation-backup-path)
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml run --rm --no-deps --entrypoint /value-package-quota-migrate new-api >"$BACKUP/value-package-migration-dry-run.json"
python3 - <<'\''PY'\'' "$BACKUP/value-package-migration-dry-run.json"
import json,sys
d=json.load(open(sys.argv[1]))
print({"rows":len(d["rows"]),"manifest_hash":d["manifest_hash"],"skipped":d["skipped"],"total_grant":sum(r["grant"] for r in d["rows"])})
assert d["rows"]
assert all(r["old_total"] == 0 and r["new_total"] == r["amount_used"] + r["grant"] for r in d["rows"])
PY'
```

Expected: only active, unexpired day/week/month value packages with positive plan totals appear. Compare counts with the investigation baseline, but trust the execution-time list if subscriptions naturally expired or were newly purchased.

- [ ] **Step 2: Reconfirm no invalid-plan skips**

Stop if `skipped.invalid_plan`, `skipped.missing_plan`, or an unknown package type is nonzero. Investigate rather than guessing a quota.

- [ ] **Step 3: Apply using the exact manifest hash**

```bash
"${SSH[@]}" 'set -e
cd /opt/new-api/app
BACKUP=$(cat /tmp/yunbay-remediation-backup-path)
MANIFEST=$(python3 -c '\''import json; print(json.load(open("'"$BACKUP"'/value-package-migration-dry-run.json"))["manifest_hash"])'\'')
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml run --rm --no-deps --entrypoint /value-package-quota-migrate new-api --apply --manifest "$MANIFEST" >"$BACKUP/value-package-migration-applied.json"
chmod 600 "$BACKUP"/value-package-migration-*.json'
```

Normal concurrent usage does not invalidate the manifest: apply reads the latest `amount_used` under the transaction lock. If target membership, plan ID, package type, grant, or expiry changed, the command must fail without writes; rerun dry-run and review the new stable authorization summary.

- [ ] **Step 4: Verify B2 invariants in the apply report**

```bash
"${SSH[@]}" 'BACKUP=$(cat /tmp/yunbay-remediation-backup-path); python3 - <<'\''PY'\'' "$BACKUP/value-package-migration-applied.json"
import json,sys
d=json.load(open(sys.argv[1]))
assert d["updated"] == len(d["rows"])
assert all(r["new_total"] - r["amount_used"] == r["grant"] for r in d["rows"])
print({"updated":d["updated"],"manifest_hash":d["manifest_hash"]})
PY'
```

## Task 8: Deploy new-api and Worker; verify Bugs 1–3 and Bug 2 UI

**State:** `yunbay-new-api`, `yunbay-ldxp-browser-worker`, authenticated admin UI.

- [ ] **Step 1: Recreate new-api and Worker only**

```bash
"${SSH[@]}" 'cd /opt/new-api/app && docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml up -d --no-deps --force-recreate new-api ldxp-browser-worker'
```

- [ ] **Step 2: Wait for health and public API**

```bash
"${SSH[@]}" 'set -e
for i in $(seq 1 18); do
  health=$(docker inspect --format "{{.State.Health.Status}}" yunbay-new-api 2>/dev/null || true)
  [[ "$health" == healthy ]] && break
  sleep 5
done
test "$(docker inspect --format "{{.State.Health.Status}}" yunbay-new-api)" = healthy
curl -fsS https://yunbay.xyz/api/status >/dev/null
curl -fsS https://yunbay.xyz/api/notice >/dev/null'
```

- [ ] **Step 3: Verify package API data from the authenticated UI**

Use the in-app browser with the existing admin session:

1. Open the value-package management page.
2. Confirm week rows show `7-day total remaining`, a numeric remaining/limit, and no `Not applicable` for lifecycle quota.
3. Confirm month rows show 5-hour, current 7-day stage, and 30-day total.
4. Use a controlled day-card fixture if no active production day card exists; do not purchase a real card solely for testing. Backend/controller tests are the authoritative day-card proof.
5. Capture a screenshot and console/network evidence without exposing user emails, tokens, or API keys.

- [ ] **Step 4: Verify the group-ratio frontend save contract**

In the authenticated root settings page:

1. Open Group ratios.
2. Confirm enabled package groups appear with `Package billing group` and `Default 1x until an override is added`.
3. Add a temporary explicit `week-card -> gpt-plus = 1` override; this proves the configured path without changing cost.
4. Save once and verify exactly one success toast.
5. Refresh and verify the value persisted.
6. Observe a matching week-card request log if traffic occurs; it must show `value_package_ratio_source=configured` and `value_package_effective_ratio=1`.
7. Remove the temporary override and verify the restored snapshot. If no matching traffic occurs, do not generate requests using another user's credentials; rely on backend end-to-end tests plus the successful production save/readback.

- [ ] **Step 5: Verify the LDXP log storm stopped**

```bash
"${SSH[@]}" 'set -e
sleep 15
docker logs --since 15s yunbay-ldxp-browser-worker 2>&1 | grep -E "record not found|claim session request failed" && exit 1 || true
docker logs --since 15s yunbay-new-api 2>&1 | grep -E "panic|fatal|database unavailable" && exit 1 || true'
```

Expected: no empty-queue errors.

## Task 9: Overlay, build, and deploy sub2api

**State:** `/opt/new-api/sub2api/source`, `yunbay-sub2api`.

- [ ] **Step 1: Overlay only tracked sub2api source while preserving manifests**

```bash
"${SSH[@]}" 'set -e
rsync -a --exclude go.mod --exclude go.sum /opt/new-api/app/infra/sub2api/backend/ /opt/new-api/sub2api/source/backend/
rsync -a --exclude package.json --exclude bun.lock --exclude bun.lockb /opt/new-api/app/infra/sub2api/frontend/ /opt/new-api/sub2api/source/frontend/
test -f /opt/new-api/sub2api/source/backend/go.mod
test -f /opt/new-api/sub2api/source/frontend/package.json'
```

- [ ] **Step 2: Run production-source tests before building**

```bash
"${SSH[@]}" 'set -e
cd /opt/new-api/sub2api/source/backend && go test -tags=unit ./...
cd /opt/new-api/sub2api/source/frontend && bun install --frozen-lockfile && bun test && bun run build'
```

Expected: PASS. Do not proceed on a skipped or partial suite.

- [ ] **Step 3: Build and recreate sub2api**

```bash
"${SSH[@]}" 'cd /opt/new-api/app && docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml build sub2api && docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml -f deploy/docker-compose.caddy-init.yml up -d --no-deps --force-recreate sub2api'
```

- [ ] **Step 4: Wait for health**

```bash
"${SSH[@]}" 'set -e
for i in $(seq 1 18); do
  [[ "$(docker inspect --format "{{.State.Health.Status}}" yunbay-sub2api 2>/dev/null || true)" == healthy ]] && break
  sleep 5
done
test "$(docker inspect --format "{{.State.Health.Status}}" yunbay-sub2api)" = healthy
curl -fsS https://sub2api.yunbay.xyz/ >/dev/null'
```

## Task 10: Smoke GPT-5.6 and expose only working models

**State:** new-api channels 35/36 and public `/v1/responses`.

- [ ] **Step 1: Load the smoke API key locally without printing it**

Read `[model_providers.codex_local_access].experimental_bearer_token` from `/Users/ethan/.codex/config.toml` into `SMOKE_API_KEY`. Never echo it, include it in shell tracing, logs, screenshots, or Git artifacts.

- [ ] **Step 2: Smoke already-exposed alias and sol**

For each model, send the smallest non-streaming Responses request:

```bash
for model in gpt-5.6 gpt-5.6-sol; do
  curl -fsS https://yunbay.xyz/v1/responses \
    -H "Authorization: Bearer $SMOKE_API_KEY" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"$model\",\"input\":\"Reply only OK\",\"max_output_tokens\":16,\"stream\":false}" \
    | python3 -c 'import json,sys; d=json.load(sys.stdin); print({"id":d.get("id"),"model":d.get("model"),"status":d.get("status")})'
done
```

Expected: alias resolves to sol; sol remains sol. Save only request ID/model/status in the protected deployment evidence.

- [ ] **Step 3: Verify unknown 5.6 is not GPT-5.4**

```bash
status=$(curl -sS -o /tmp/gpt56-unknown.json -w '%{http_code}' https://yunbay.xyz/v1/responses \
  -H "Authorization: Bearer $SMOKE_API_KEY" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5.6-pro","input":"x","max_output_tokens":1,"stream":false}')
test "$status" -ge 400
! rg -q 'gpt-5\.4' /tmp/gpt56-unknown.json
rm -f /tmp/gpt56-unknown.json
```

- [ ] **Step 4: Probe terra and luna one at a time**

Before each probe, add only that exact model to one verified sub2api-backed channel allowlist using the authenticated admin channel editor, wait for the channel cache refresh, and run the same minimal request. Confirm response/log model and price family. If it fails, restore the channel's backed-up models immediately and leave that model unexposed.

Do not add both at once; change one variable at a time.

- [ ] **Step 5: Verify production price values**

Through the admin price API/UI or a protected DB query, verify:

```text
alias/sol: input 5, cached .5, write 6.25, output 30
terra: input 2.5, cached .25, write 3.125, output 15
luna: input 1, cached .1, write 1.25, output 6
```

If an automatic upstream sync overwrites terra/luna with sol values, disable exposure for the affected model and restore the correct four entries before continuing.

## Task 11: Final observation, deploy marker, and completion audit

**State:** all deployed services and evidence bundle.

- [ ] **Step 1: Observe production for at least ten minutes**

```bash
"${SSH[@]}" 'set -e
sleep 600
docker ps --format "{{.Names}}|{{.Status}}" | grep "^yunbay-" | sort
docker logs --since 10m yunbay-new-api 2>&1 | grep -Ei "panic|fatal|out of memory" && exit 1 || true
docker logs --since 10m yunbay-ldxp-browser-worker 2>&1 | grep -E "record not found|claim session request failed" && exit 1 || true
docker top yunbay-caddy -eo stat,ppid,pid,comm | awk '\''$1 ~ /^Z/ {n++} END {print n+0}'\'' | grep -qx 0
```

- [ ] **Step 2: Verify all public endpoints**

```bash
for url in \
  https://yunbay.xyz/ \
  https://yunbay.xyz/api/status \
  https://yunbay.xyz/api/notice \
  https://sub2api.yunbay.xyz/; do
  curl -fsS "$url" >/dev/null
done
```

- [ ] **Step 3: Atomically write the deployed SHA only after all checks pass**

```bash
"${SSH[@]}" "printf '%s\n' '$RELEASE_SHA' >/opt/new-api/app/.yunbay-deploy-sha.tmp && mv /opt/new-api/app/.yunbay-deploy-sha.tmp /opt/new-api/app/.yunbay-deploy-sha && test \"\$(cat /opt/new-api/app/.yunbay-deploy-sha)\" = '$RELEASE_SHA'"
```

- [ ] **Step 4: Save a compact evidence summary in the protected backup directory**

Include:

```text
release SHA and origin/main SHA
new/old image IDs
migration manifest hash and updated row count
Caddy init/health/zombie observations
LDXP error count after deployment
package UI/API checks
group-ratio save/readback check
GPT-5.6 per-model smoke result and final channel exposure
rollback image tags and backup path
```

Do not include access tokens, request bodies, user emails, or secret environment values.

- [ ] **Step 5: Audit every explicit user requirement**

Before claiming completion, prove:

1. server investigation occurred and live evidence was used;
2. spec exists and was approved;
3. plans exist and were executed;
4. week/day/month balances match the requested semantics;
5. group pricing is globally effective for its intended funding path;
6. selected historical new-api issues are fixed and verified;
7. GPT-5.6 support is correct or an unsupported production model remains intentionally hidden;
8. workspace was merged and GitHub contains the release SHA;
9. backups exist;
10. production runs the release SHA and passes regression checks.

Only then mark the persistent goal complete.

## Rollback commands

Run the narrowest rollback for the failed phase.

### Application images

```bash
docker image tag yunbay-new-api:rollback-$STAMP yunbay-new-api:prod
docker image tag yunbay-sub2api:rollback-$STAMP yunbay-sub2api:source
docker image tag yunbay-ldxp-browser-worker:rollback-$STAMP yunbay-ldxp-browser-worker:prod
docker compose --env-file /opt/new-api/secrets/prod.env -f /opt/new-api/app/docker-compose.prod.yml -f /opt/new-api/app/deploy/docker-compose.caddy-init.yml up -d --no-deps --force-recreate new-api sub2api ldxp-browser-worker
```

### Package data

Restore `user_subscriptions.amount_total` from `$BACKUP/newapi.dump` into a temporary database/table or use the protected pre-migration row export; restore only the backed-up target IDs and only the old `amount_total`. Never overwrite current `amount_used`.

### Ratio options and channels

Restore the backed-up `GroupRatio`, `GroupGroupRatio`, and channel `models`, then restart/sync new-api and read the values back. Do not subtract a guessed multiplier or plan amount.

### Caddy

If Caddy itself regresses, restore `$BACKUP/docker-compose.prod.yml` and `$BACKUP/Caddyfile`, recreate only Caddy, and recheck public routing. If `init:true` is healthy, retain it even when rolling back unrelated services.
