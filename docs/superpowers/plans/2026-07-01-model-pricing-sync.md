# Model Pricing Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin workflow that syncs selected existing model-square pricing entries from OpenRouter and official sources, takes the higher price per dimension, and pushes validated billing expressions into model billing settings.

**Architecture:** Add a focused backend service/controller under existing ratio sync APIs to normalize prices, conservatively match only requested model-square entries, preview diffs, and apply settings. Extend the existing default frontend model pricing table with row selection, a sync dialog, preview/apply calls, and localized UI strings.

**Tech Stack:** Go 1.22+, Gin, GORM-compatible option writes, existing billingexpr package, React 19/TypeScript, Bun, Base UI/shadcn-style components, i18next.

---

### Task 1: Backend price normalization and merge tests

**Files:**
- Create: `service/model_price_sync.go`
- Create: `service/model_price_sync_test.go`

- [ ] Write failing tests for OpenRouter pricing conversion, conservative model id matching, and per-dimension max merge.
- [ ] Run `go test ./service -run TestModelPriceSync -count=1` and confirm the tests fail because the functions do not exist.
- [ ] Implement minimal service types and functions: canonical price struct, OpenRouter parser, model matcher, max merge, expression generation.
- [ ] Re-run the same Go test command and confirm it passes.

### Task 2: Backend preview/apply controller and routes

**Files:**
- Create: `controller/model_price_sync.go`
- Modify: `router/api-router.go`
- Modify: `controller/ratio_sync.go` only if helper reuse requires exported types or functions
- Test: `controller/model_price_sync_test.go` if route/controller helpers can be tested without full server setup

- [ ] Write failing tests for request validation and selected-model filtering at service/controller helper level.
- [ ] Add `POST /api/ratio_sync/model_price/preview` and `POST /api/ratio_sync/model_price/apply` under root-only ratio sync routes.
- [ ] Implement controller handlers that decode JSON through `common.DecodeJson`, validate admin/root context through existing route middleware, call service preview/apply, and return standard success/error JSON.
- [ ] Implement apply persistence through `model.UpdateOptionsBulk()` and existing option keys.
- [ ] Re-run focused Go tests for `service` and `controller`.

### Task 3: Fix cache write semantic risk in tiered settlement

**Files:**
- Modify: `service/tiered_settle.go`
- Modify: `service/tiered_settle_test.go`

- [ ] Write a failing regression test where Claude usage semantic is passed via `isClaudeUsageSemantic=true` but `usage.UsageSemantic` is not exactly `anthropic`, and `cc`/`cc1h` still split correctly.
- [ ] Run `go test ./service -run TestBuildTieredTokenParams -count=1` and confirm the new test fails.
- [ ] Replace the narrow `usage.UsageSemantic == "anthropic"` cache creation branch with the existing `isClaudeUsageSemantic` decision.
- [ ] Re-run the focused test and confirm it passes.

### Task 4: Frontend API types and sync dialog

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/api.ts`
- Create: `web/default/src/features/system-settings/models/model-price-sync-dialog.tsx`

- [ ] Add TypeScript request/response types matching backend preview/apply JSON.
- [ ] Add API client functions for preview/apply endpoints.
- [ ] Build a dialog with OpenRouter channel selection, preview table, status badges, and apply button.
- [ ] Use existing UI primitives and `t()` for every human-facing string.

### Task 5: Frontend table row selection and batch action

**Files:**
- Modify: `web/default/src/features/system-settings/models/model-ratio-visual-editor.tsx`
- Modify: `web/default/src/features/system-settings/models/model-ratio-table-columns.tsx`

- [ ] Add row selection state keyed by `model_name`.
- [ ] Add a checkbox column and header select-all for visible rows.
- [ ] Add “Sync selected model prices” batch button near existing model pricing actions.
- [ ] Open the sync dialog with selected model names and refresh pricing/settings after successful apply.

### Task 6: i18n and verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] Add translations for all new UI strings.
- [ ] Run `cd web/default && bun run i18n:sync`.
- [ ] Run focused TypeScript/build checks available in `web/default/package.json`.
- [ ] Run available Go tests or record if Go is unavailable in PATH.
- [ ] Review `git diff` to ensure no unrelated files changed.
