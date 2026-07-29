# Bedrock Claude Model ID Repair Plan

## 1. Status And Scope

- Status: in progress
- Started: 2026-07-29 (Asia/Shanghai)
- Production host: `vmi3369877`
- Service: `yunbay-sub2api`
- Production baseline: Sub2API `0.1.166`
- Affected account: `admin@yunbay.xyz` (`bedrock`)
- Reported request model: `us.anthropic.claude-opus-5-v1`
- Reported result: AWS HTTP 400, `The provided model identifier is invalid.`
- Allowed interruption: less than 60 seconds; target less than 30 seconds
- Change boundary: audit and repair every Claude entry in the Bedrock default model map and its UI presets without resetting, importing, restoring, or otherwise changing existing account or global settings.

This is the single execution plan for the repair. It will be updated at each control point instead of creating another plan file.

## 2. Goal And Performance Metrics

The controlled outcome is that Sub2API sends the AWS-recognized model identifiers for the affected Claude model family while preserving all existing configuration and service state.

Initially reported mappings:

| Requested alias | Bedrock model identifier |
| --- | --- |
| `claude-opus-4-7` | `us.anthropic.claude-opus-4-7` |
| `claude-opus-4-8` | `us.anthropic.claude-opus-4-8` |
| `claude-opus-5` | `us.anthropic.claude-opus-5` |
| `claude-fable-5` | `us.anthropic.claude-fable-5` |

The table above is not the implementation boundary. Every Claude mapping in `DefaultBedrockModelMapping` must be checked against the AWS model catalog. Preliminary catalog evidence also identifies `claude-sonnet-5 -> us.anthropic.claude-sonnet-5` as requiring correction. Version suffixes must be retained where AWS still defines them, including Opus 4.6 `-v1` and dated model IDs ending in `-v1:0`.

Acceptance metrics:

| Metric | Required result |
| --- | --- |
| Resolver behavior | Every mapped Claude alias resolves to its audited AWS identifier in a US region |
| Regional behavior | Existing `eu.`, `jp.`, `apac.`, `au.`, `us-gov.`, and `global.` prefix adjustment remains intact |
| Forbidden suffix | Opus 4.7, 4.8, and 5 requests do not contain `-v1` |
| Account preservation | Affected account credentials/settings hash is unchanged by deployment and testing |
| Global preservation | Existing settings/config/environment/mount fingerprints are unchanged |
| AWS model recognition | Opus 5 test no longer returns `400 invalid model identifier` |
| Permission boundary | A resulting AWS 403 is recorded as account entitlement, not treated as a mapping regression |
| Availability | Public and local health recover within 60 seconds |
| Isolation | Only `yunbay-sub2api` may be recreated; sibling services remain untouched |

Fable 5 account entitlement and `provider_data_share` are outside this repair. They must not be enabled automatically.

## 3. Control-System Model

| Element | Concrete object |
| --- | --- |
| Controlled object | Sub2API Bedrock model resolution, request URL construction, and production process |
| Controller | Exact mapping tests, image build, bounded deployment watchdog, and post-deploy account probe |
| Measurement | Resolver unit tests, final model ID event/log, AWS status/body, health probes, and configuration fingerprints |
| Actuator | Surgical source patch and recreation of only the Sub2API container |
| Environment | AWS Bedrock region/profile availability, account entitlements, network egress, PostgreSQL, Redis, and Caddy |
| Disturbances | AWS permission rollout, region-specific inference-profile availability, transient network errors, image build delay, and SSH loss |
| Delay/saturation | Container startup, image pull, AWS entitlement propagation, and the 60-second interruption limit |

The stable closed loop is proved first with deterministic resolver tests, then with an isolated/local image check, and finally with one bounded production cutover. A 403 after the corrected identifier is a valid measurement of the separate entitlement layer.

## 4. GitHub And Upstream Evidence

Reviewed before implementation:

- Official repository: `Wei-Shaw/sub2api`
- Latest stable release: `v0.1.166`, published 2026-07-27
- Official `v0.1.166` source maps `claude-opus-5` to `us.anthropic.claude-opus-5-v1`.
- Official issue `#1714` reports the same AWS `invalid model identifier` failure for Opus 4.7 and states that AWS recognizes the identifier without `-v1`.
- Official issue `#4853` and commit `6c9b84cc7aad` show the recent Opus 5 integration path, including Bedrock defaults and resolver tests.
- The local pinned overlay lacks the Opus 5 default mapping even though the official `v0.1.166` source contains it. The build must therefore be checked carefully so the repair is applied on top of `0.1.166` rather than replacing it with an older source snapshot.
- User-provided AWS Playground evidence distinguishes the two response layers:
  - malformed model ID: HTTP 400 `invalid model identifier`;
  - recognized model without account access: HTTP 403 `not available for this account`.
- The user has already observed a 403 for the correct Opus 4.8 identifier, so deployment success does not require an AWS 200 for models that the account has not yet been granted.

## 5. Invariants: No Settings Rollback

The following are immutable inputs:

- `/opt/new-api/sub2api/data/config.yaml`
- Production Compose environment and secrets
- PostgreSQL and Redis targets and data
- All settings rows and values
- Account credentials, auth mode, region, `aws_force_global`, model mappings, schedulability, priority, concurrency, proxy, quota, and status
- Groups, keys, billing rules, pricing configuration, routes, monitoring, and panel settings
- Caddy and every non-Sub2API service

No settings import, database restore, configuration regeneration, credential rewrite, account update, or volume replacement is authorized. Before any production mutation, back up the affected account row and record a redacted fingerprint. After deployment and tests, prove the same fingerprint remains.

## 6. Implementation Boundary

Expected source changes:

1. Audit every Claude value in `DefaultBedrockModelMapping` against AWS catalog evidence.
2. Correct every mismatched value, including but not limited to the initially reported four aliases.
3. Keep frontend Bedrock presets exactly aligned with backend defaults.
4. Add table-driven tests for the complete Claude mapping set, exact US identifiers, regional-prefix adjustment, and deprecated invalid IDs.
5. Include the missing Opus 5 alias only where required for the corrected Bedrock mapping path.
6. Do not change account data to work around a code default.
7. Do not enable Fable 5 `provider_data_share` or request AWS entitlements.

Any broader source sync is a stop condition unless it is proven necessary to build against the pinned `0.1.166` source and its diff remains auditable.

## 7. Execution Stages

### Stage A: Discovery And Read-Only Production Baseline

Status: completed

1. Confirm official tag/source and related GitHub reports.
2. Inspect the local overlay/build mechanism and compare it with upstream `v0.1.166`.
3. Read the affected production account row without printing secrets.
4. Record account ID, Bedrock region, auth mode, force-global flag, model-mapping keys/values, and a full-row fingerprint.
5. Inspect recent Sub2API logs for the final model ID/request URL if available.
6. Confirm current container image, health, restart count, config fingerprint, and sibling-service baseline.

Stop conditions:

- Production is already unhealthy.
- The affected account cannot be uniquely identified.
- Existing account mapping intentionally overrides the four aliases in a way that conflicts with the requested repair.
- The build mechanism would replace `0.1.166` with an older source tree.

### Stage B: Surgical Implementation And Deterministic Tests

Status: completed

1. Patch backend defaults and frontend presets.
2. Add or update resolver/default-mapping tests.
3. Run focused Go tests for Bedrock resolution and request URL construction.
4. Run relevant frontend tests/typecheck for the preset list.
5. Run formatting and `git diff --check`.
6. Compare the final patch against upstream and ensure protected project metadata remains untouched.

### Stage C: Candidate Image And Preflight

Status: completed

1. Build a versioned candidate from pinned upstream `0.1.166` plus only the audited repair.
2. Record candidate image ID/digest and labels.
3. Start an isolated candidate or run the binary's health path without production data.
4. Confirm resolver behavior in the actual built artifact.
5. Prepare a bounded watchdog that restores only the previous image reference if health fails; it must never restore settings or database data.

### Stage D: Production Cutover And Closed-Loop Test

Status: completed

1. Acquire the dedicated Sub2API deployment lock.
2. Create a protected backup of the Compose file, Sub2API data/config, affected account row, and relevant fingerprints.
3. Change only the Sub2API image reference.
4. Recreate only `yunbay-sub2api` with a maximum 60-second health window.
5. Automatically restore the previous image reference if the candidate fails health.
6. Re-run the account test with prompt `hi` using the corrected Opus 5 alias/identifier.
7. Require that the emitted/final model ID is `us.anthropic.claude-opus-5` for a US account region and that AWS no longer returns invalid-identifier 400.
8. Treat a 403 as the expected account-entitlement boundary and preserve the account unchanged.

### Stage E: Validation, Documentation, GitHub, And Cleanup

Status: completed

1. Verify public/local health, image identity, restart/OOM state, logs, and sibling-service isolation.
2. Prove account/config/settings/environment/mount fingerprints are unchanged.
3. Append the production result and rollback command to `docs/yunbay-maintenance.md`.
4. Update `/Users/ethan/Desktop/云贝/服务器相关/yunbay-new-api-vps-连接信息.md` in place.
5. Update this plan with actual evidence and final status.
6. Commit only task-owned files to `main` and push `origin/main`.
7. Remove only task-created temporary files/images; preserve all unrelated untracked `* 2.*` files and `outputs/`.
8. Restore the local Clash `Proxy` selection to its pre-task value unless a newer manual user selection is detected; never overwrite that newer choice.

## 8. Rollback Contract

Rollback is binary-only:

1. Restore the previous Sub2API image reference in the production Compose file.
2. Recreate only `yunbay-sub2api`.
3. Verify local/public health.
4. Do not restore PostgreSQL, Redis, `config.yaml`, account credentials, or settings.
5. Keep the protected pre-change backup for forensic recovery.

The mapping patch does not require a database migration. A database restore is therefore forbidden for routine rollback.

## 9. Execution Log

| Time (Asia/Shanghai) | Stage | Observation / action | Result |
| --- | --- | --- | --- |
| 2026-07-29 | Discovery | Confirmed official release `v0.1.166` and inspected Bedrock resolver/default mappings | Upstream Opus identifiers contain erroneous `-v1`; local overlay also lacks Opus 5 default |
| 2026-07-29 | GitHub audit | Reviewed upstream issue `#1714`, issue `#4853`, and Opus 5 integration commit | Existing report confirms no-`-v1` workaround for the same AWS 400 class |
| 2026-07-29 | User evidence | Recorded AWS Playground distinction between malformed-ID 400 and entitlement 403 | Acceptance split into identifier recognition and account authorization |
| 2026-07-29 | Connectivity | Switched local Clash selector from `JP 22 GMO x1.0` to whitelisted `US 33 AI加速 x1.0`; first SSH attempt timed out | Retry and network diagnosis required; selector must be restored at closure |
| 2026-07-29 | Plan | Created this single full implementation plan before source or production mutation | Complete |
| 2026-07-29 | Scope correction | User clarified that all Bedrock Claude mappings require audit, not only the four reported examples | Plan expanded to a complete map audit; suffix removal will be evidence-based rather than mechanical |
| 2026-07-29 | AWS catalog audit | Queried Bedrock foundation-model and inference-profile catalogs with the affected account | Confirmed exactly five default corrections; retained valid Opus 4.6 `-v1` and dated `-v1:0` identifiers |
| 2026-07-29 | Production baseline | Identified account `540`, recorded redacted row fingerprint `f185fa4765d1961c344247ae3ab75f37`, and verified Sub2API `0.1.166` healthy | No custom model mapping; production remains unchanged |
| 2026-07-29 | Surgical implementation | Patched seven source/test files on the official `v0.1.166` baseline and excluded accidental package-manager files | Focused backend tests passed locally; remote reproducible build is next |
| 2026-07-29 | Deterministic tests | Ran `internal/domain`, `internal/service`, and the Bedrock frontend preset test | Go packages passed; frontend test passed 16/16; `git diff --check` passed |
| 2026-07-29 | Candidate image | Built `yunbay/sub2api:0.1.166-bedrock-model-id-20260729` from the clean official checkout | Image ID `sha256:b0ccb3a6057f...`; full frontend and Go production build passed |
| 2026-07-29 | Protected backup | Saved Compose, config, account 540, settings, container, mount, and environment baselines | Backup `/home/deploy/backups/sub2api-bedrock-model-id-20260729.JohfDb`; account fingerprint unchanged at `f185fa4765d1961c344247ae3ab75f37` |
| 2026-07-29 | Production cutover | Independent watchdog changed only the Sub2API image line and recreated only `yunbay-sub2api` | Healthy in `20.375s`; container `eb02eb4c08ba...`, restart `0`, OOM `false`; rollback not triggered |
| 2026-07-29 | Closed-loop account test | Logged in through the normal admin API and tested account `540` with alias `claude-opus-5` and message `hi` | Emitted `us.anthropic.claude-opus-5`; AWS returned entitlement `403`, with no invalid-identifier `400` |
| 2026-07-29 | Final preservation audit | Rechecked account, settings, config, sorted environment, mounts, health, logs, lock, and sibling container IDs | All fingerprints unchanged; local/public probes 5/5; fatal logs 0; deployment lock free |
| 2026-07-29 | Reproducibility and records | Added the encoded seven-file patch, pinned build script, Compose override, repository maintenance entry, and updated the single desktop operations manual | Patch re-applied cleanly to official commit; no credentials or retired overlay files included |
| 2026-07-29 | Cleanup and local state | Archived the remote source worktree/build log under the protected backup and moved local task temporaries to Trash | Release/rollback images and forensic backup retained; unrelated untracked files preserved |
| 2026-07-29 | Clash selector audit | Local API reported `Proxy` as a manual Selector currently set to `X56 美国 IPLC专线 x1.8`, not the task's temporary `US 33` value | Treated as a newer user selection and left unchanged instead of restoring stale `JP 22` state |

## 10. Current Decision

Repair complete. Production resolves Opus 5 to `us.anthropic.claude-opus-5`; the remaining AWS 403 is account entitlement, not a Sub2API model-ID defect. Keep the protected backup and release/rollback image tags, and do not enable Fable 5 `provider_data_share` until the account has model access.
