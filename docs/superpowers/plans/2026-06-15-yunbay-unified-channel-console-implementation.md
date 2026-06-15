# Yunbay Unified Channel Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a first working version of the 云贝 unified channel console focused on third-party API import, model discovery, price metadata, health status, and admin UI.

**Architecture:** Add a backend `channel-console` vertical slice beside the existing channel APIs, backed by small GORM metadata tables that reference existing `channels`. Keep original New API channel management intact as the advanced fallback. Add a React admin page at `/channel-console` that uses the new APIs and links back to `/channels` for advanced edits.

**Tech Stack:** Go 1.22+, Gin, GORM, New API `common` JSON wrappers, React 19, TypeScript, TanStack Router, Base UI/Tailwind components, Bun/npm frontend scripts.

**Updated Project Location:** 云贝相关文件已经从 Codex 临时目录迁移到桌面独立目录。后续实施本计划时，主工作目录固定为：

```text
/Users/ethan/Desktop/云贝/云贝网站/new-api
```

**Related Local Folders:**

```text
/Users/ethan/Desktop/云贝/云贝 APP/codex-history-unifier
/Users/ethan/Desktop/云贝/服务器相关
```

**Execution Rule:** 所有本计划中的相对路径都以 `/Users/ethan/Desktop/云贝/云贝网站/new-api` 为项目根目录。后续 sub agent 或人工执行任务时，不再使用旧的 `/Users/ethan/Documents/Codex/.../work/new-api` 路径。

---

## File Structure

Unless a path is explicitly absolute, every path below is relative to:

```text
/Users/ethan/Desktop/云贝/云贝网站/new-api
```

Backend files to create:

- `model/channel_console.go` — GORM models for console metadata, model prices, and health checks; query helpers and retention cleanup.
- `service/channelconsole/types.go` — shared request/response/value types used by importer, pricing, health, and controller.
- `service/channelconsole/importer.go` — paste parser and provider detection for API keys, curl commands, headers, JSON, and Base URL + Key text.
- `service/channelconsole/pricing.go` — normalized price records, built-in official price templates, OpenRouter price parsing, price status decisions, and ratio compilation helpers.
- `service/channelconsole/health.go` — health status helpers and health check result persistence.
- `service/channelconsole/console.go` — orchestration functions for preview, commit, list, detail, sync models, sync prices, and health checks.
- `controller/channel_console.go` — Gin handlers that validate admin requests and call `service/channelconsole`.

Backend files to modify:

- `model/main.go` — include the new console models in both normal and fast AutoMigrate flows.
- `router/api-router.go` — register `/api/channel-console/*` routes under `AdminAuth`; bulk and mutating operations also use critical rate limit where matching existing patterns.

Backend test files to create:

- `service/channelconsole/importer_test.go` — parser/provider detection tests.
- `service/channelconsole/pricing_test.go` — price matching and ratio compilation tests.
- `service/channelconsole/health_test.go` — health status aggregation tests.

Frontend files to create:

- `web/default/src/features/channel-console/types.ts` — TypeScript API types for console summary, rows, import preview, price records, health checks.
- `web/default/src/features/channel-console/api.ts` — frontend API client functions for `/api/channel-console/*`.
- `web/default/src/features/channel-console/components/import-panel.tsx` — paste box, preview button, commit button, preview result card.
- `web/default/src/features/channel-console/components/channel-console-table.tsx` — channel list, status badges, quick actions.
- `web/default/src/features/channel-console/components/channel-detail-drawer.tsx` — detail drawer with overview, models/prices, health records, and advanced link.
- `web/default/src/features/channel-console/index.tsx` — page shell using `SectionPageLayout`.
- `web/default/src/routes/_authenticated/channel-console/index.tsx` — admin-only route.

Frontend files to modify:

- `web/default/src/hooks/sidebar-data-model.ts` — add admin navigation item `统一渠道控制台` before `Channels`.
- `web/default/src/hooks/use-sidebar-data.ts` — import and map a suitable icon, such as `Network` or `PanelsTopLeft` from `lucide-react`.
- `web/default/src/hooks/use-sidebar-config.ts` — add `/channel-console` to the admin channel module mapping so existing sidebar visibility controls include the new page.
- `web/default/src/i18n/locales/zh.json` and `web/default/src/i18n/locales/en.json` — add visible strings used by the new page.
- `web/default/src/routeTree.gen.ts` — regenerate with the project’s route generation command after adding the route file.

Plan and verification files:

- `docs/superpowers/plans/2026-06-15-yunbay-unified-channel-console-implementation.md` — this implementation plan.

## Scope Cut for First Working Version

The first working version must implement:

- Import preview for OpenRouter, OpenAI official, Anthropic official, Gemini official, and OpenAI-compatible custom text.
- Import commit that creates or updates a New API channel and creates console metadata.
- Multi-key import using existing `ChannelInfo` with `random` or `polling`.
- Model discovery through existing channel `models` plus provider defaults; OpenRouter and OpenAI-compatible `/v1/models` live calls can be a backend function, but tests use injected HTTP clients.
- Price records from OpenRouter metadata or built-in official templates; unknown prices are marked `price_unknown` and not enabled by default.
- Manual health recording and channel status aggregation.
- Admin page with import, list, detail drawer, sync/test buttons, and clear red/yellow/green/gray status.

The first working version does not need CLIProxyAPI account-pool integration. It must keep the data model and UI enum values ready for `oauth_cli` rows.

---

### Task 1: Backend console data models and migration

**Files:**
- Create: `model/channel_console.go`
- Modify: `model/main.go`
- Test: `go test ./model`

- [ ] **Step 1: Create console model structs**

Create `model/channel_console.go` with these structs and helper constants. Use `common.Marshal` and `common.Unmarshal` for JSON helper methods; do not add direct JSON marshal calls in business logic.

```go
package model

import (
    "time"

    "gorm.io/gorm"
)

const (
    ChannelConsoleProviderOpenRouter = "openrouter"
    ChannelConsoleProviderOpenAI     = "openai"
    ChannelConsoleProviderAnthropic  = "anthropic"
    ChannelConsoleProviderGemini     = "gemini"
    ChannelConsoleProviderCustom     = "custom_openai_compatible"

    ChannelConsoleKindThirdPartyAPI = "third_party_api"
    ChannelConsoleKindOAuthCLI      = "oauth_cli"

    ChannelConsoleStatusHealthy   = "healthy"
    ChannelConsoleStatusWarning   = "warning"
    ChannelConsoleStatusFailed    = "failed"
    ChannelConsoleStatusDisabled  = "disabled"
    ChannelConsoleStatusUnchecked = "unchecked"

    ChannelConsolePriceStatusSynced  = "synced"
    ChannelConsolePriceStatusUnknown = "price_unknown"
    ChannelConsolePriceStatusStale   = "stale"
    ChannelConsolePriceStatusManual  = "manual"
)

type ChannelConsoleChannel struct {
    Id                int      `json:"id" gorm:"primaryKey"`
    ChannelId         int      `json:"channel_id" gorm:"uniqueIndex;not null"`
    Provider          string   `json:"provider" gorm:"size:64;index;not null"`
    ProviderKind      string   `json:"provider_kind" gorm:"size:64;index;not null"`
    ImportKind        string   `json:"import_kind" gorm:"size:64;not null"`
    PriceSource       string   `json:"price_source" gorm:"size:64;index;not null"`
    HealthStatus      string   `json:"health_status" gorm:"size:32;index;not null;default:'unchecked'"`
    ModelSyncStatus   string   `json:"model_sync_status" gorm:"size:32;not null;default:'unchecked'"`
    PriceSyncStatus   string   `json:"price_sync_status" gorm:"size:32;not null;default:'unchecked'"`
    LastHealthCheckAt int64    `json:"last_health_check_at" gorm:"bigint;default:0"`
    LastModelSyncAt   int64    `json:"last_model_sync_at" gorm:"bigint;default:0"`
    LastPriceSyncAt   int64    `json:"last_price_sync_at" gorm:"bigint;default:0"`
    LastErrorCode     string   `json:"last_error_code" gorm:"size:128"`
    LastErrorMessage  string   `json:"last_error_message" gorm:"type:text"`
    Markup            float64  `json:"markup" gorm:"default:1.2"`
    AutoDisablePolicy string   `json:"auto_disable_policy" gorm:"size:64;default:'mark_only'"`
    CreatedAt         int64    `json:"created_at" gorm:"bigint"`
    UpdatedAt         int64    `json:"updated_at" gorm:"bigint"`
    DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

type ChannelConsoleModelPrice struct {
    Id                             int      `json:"id" gorm:"primaryKey"`
    ChannelId                      int      `json:"channel_id" gorm:"index;not null"`
    ModelName                      string   `json:"model_name" gorm:"size:255;index;not null"`
    ProviderModelName              string   `json:"provider_model_name" gorm:"size:255;index"`
    Source                         string   `json:"source" gorm:"size:64;index;not null"`
    InputUSDPer1MTokens            *float64 `json:"input_usd_per_1m_tokens"`
    OutputUSDPer1MTokens           *float64 `json:"output_usd_per_1m_tokens"`
    CachedInputUSDPer1MTokens      *float64 `json:"cached_input_usd_per_1m_tokens"`
    CacheWrite5mUSDPer1MTokens     *float64 `json:"cache_write_5m_usd_per_1m_tokens"`
    CacheWrite1hUSDPer1MTokens     *float64 `json:"cache_write_1h_usd_per_1m_tokens"`
    RequestUSDPerCall              *float64 `json:"request_usd_per_call"`
    ImageUSDPerUnit                *float64 `json:"image_usd_per_unit"`
    CompiledModelRatio             *float64 `json:"compiled_model_ratio"`
    CompiledCompletionRatio        *float64 `json:"compiled_completion_ratio"`
    CompiledCacheRatio             *float64 `json:"compiled_cache_ratio"`
    CompiledCreateCacheRatio       *float64 `json:"compiled_create_cache_ratio"`
    CompiledModelPrice             *float64 `json:"compiled_model_price"`
    ManualOverride                 bool     `json:"manual_override" gorm:"default:false"`
    Enabled                        bool     `json:"enabled" gorm:"default:false"`
    PriceStatus                    string   `json:"price_status" gorm:"size:32;index;not null"`
    SourceUpdatedAt                int64    `json:"source_updated_at" gorm:"bigint;default:0"`
    SyncedAt                       int64    `json:"synced_at" gorm:"bigint;default:0"`
    CreatedAt                      int64    `json:"created_at" gorm:"bigint"`
    UpdatedAt                      int64    `json:"updated_at" gorm:"bigint"`
}

type ChannelConsoleHealthCheck struct {
    Id             int    `json:"id" gorm:"primaryKey"`
    ChannelId      int    `json:"channel_id" gorm:"index;not null"`
    KeyIndex       *int   `json:"key_index" gorm:"index"`
    ModelName      string `json:"model_name" gorm:"size:255;index"`
    CheckType      string `json:"check_type" gorm:"size:64;index;not null"`
    Status         string `json:"status" gorm:"size:32;index;not null"`
    ResponseTimeMs int    `json:"response_time_ms" gorm:"default:0"`
    ErrorCode      string `json:"error_code" gorm:"size:128"`
    ErrorMessage   string `json:"error_message" gorm:"type:text"`
    CheckedAt      int64  `json:"checked_at" gorm:"bigint;index;not null"`
}

func (c *ChannelConsoleChannel) BeforeCreate(tx *gorm.DB) error {
    now := time.Now().Unix()
    if c.CreatedAt == 0 { c.CreatedAt = now }
    if c.UpdatedAt == 0 { c.UpdatedAt = now }
    return nil
}

func (c *ChannelConsoleChannel) BeforeUpdate(tx *gorm.DB) error {
    c.UpdatedAt = time.Now().Unix()
    return nil
}

func (p *ChannelConsoleModelPrice) BeforeCreate(tx *gorm.DB) error {
    now := time.Now().Unix()
    if p.CreatedAt == 0 { p.CreatedAt = now }
    if p.UpdatedAt == 0 { p.UpdatedAt = now }
    return nil
}

func (p *ChannelConsoleModelPrice) BeforeUpdate(tx *gorm.DB) error {
    p.UpdatedAt = time.Now().Unix()
    return nil
}
```

- [ ] **Step 2: Add query helpers**

Append these helpers to `model/channel_console.go`.

```go
func UpsertChannelConsoleChannel(meta *ChannelConsoleChannel) error {
    existing := ChannelConsoleChannel{}
    err := DB.Where("channel_id = ?", meta.ChannelId).First(&existing).Error
    if err == nil {
        meta.Id = existing.Id
        meta.CreatedAt = existing.CreatedAt
        return DB.Save(meta).Error
    }
    return DB.Create(meta).Error
}

func GetChannelConsoleChannelByChannelID(channelID int) (*ChannelConsoleChannel, error) {
    meta := ChannelConsoleChannel{}
    if err := DB.Where("channel_id = ?", channelID).First(&meta).Error; err != nil {
        return nil, err
    }
    return &meta, nil
}

func SaveChannelConsoleModelPrices(channelID int, prices []ChannelConsoleModelPrice) error {
    return DB.Transaction(func(tx *gorm.DB) error {
        for i := range prices {
            prices[i].ChannelId = channelID
            existing := ChannelConsoleModelPrice{}
            err := tx.Where("channel_id = ? AND model_name = ?", channelID, prices[i].ModelName).First(&existing).Error
            if err == nil {
                if existing.ManualOverride {
                    continue
                }
                prices[i].Id = existing.Id
                prices[i].CreatedAt = existing.CreatedAt
                if err := tx.Save(&prices[i]).Error; err != nil { return err }
                continue
            }
            if err := tx.Create(&prices[i]).Error; err != nil { return err }
        }
        return nil
    })
}

func ListChannelConsoleModelPrices(channelID int) ([]ChannelConsoleModelPrice, error) {
    prices := make([]ChannelConsoleModelPrice, 0)
    err := DB.Where("channel_id = ?", channelID).Order("model_name asc").Find(&prices).Error
    return prices, err
}

func CreateChannelConsoleHealthCheck(check *ChannelConsoleHealthCheck) error {
    if check.CheckedAt == 0 { check.CheckedAt = time.Now().Unix() }
    return DB.Create(check).Error
}

func ListChannelConsoleHealthChecks(channelID int, limit int) ([]ChannelConsoleHealthCheck, error) {
    if limit <= 0 || limit > 200 { limit = 50 }
    checks := make([]ChannelConsoleHealthCheck, 0)
    err := DB.Where("channel_id = ?", channelID).Order("checked_at desc").Limit(limit).Find(&checks).Error
    return checks, err
}
```

- [ ] **Step 3: Register migrations in `model/main.go`**

Add these model types to both `migrateDB()` and `migrateDBFast()` after `&Channel{}` so normal and fast migrations create the new tables.

```go
&ChannelConsoleChannel{},
&ChannelConsoleModelPrice{},
&ChannelConsoleHealthCheck{},
```

- [ ] **Step 4: Verify model package**

Run:

```bash
go test ./model
```

Expected: package compiles or existing unrelated skipped external DB tests remain skipped; no compile errors from `channel_console.go`.

- [ ] **Step 5: Commit Task 1**

```bash
git add model/channel_console.go model/main.go
git commit -m "feat: add channel console metadata models"
```

---

### Task 2: Import parser and provider detection

**Files:**
- Create: `service/channelconsole/types.go`
- Create: `service/channelconsole/importer.go`
- Create: `service/channelconsole/importer_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `service/channelconsole/importer_test.go`.

```go
package channelconsole

import "testing"

func TestPreviewOpenRouterCurl(t *testing.T) {
    input := `curl https://openrouter.ai/api/v1/chat/completions -H "Authorization: Bearer sk-or-redacted"`
    preview := PreviewImport(input)
    if preview.Provider != ProviderOpenRouter { t.Fatalf("provider = %s", preview.Provider) }
    if preview.BaseURL != "https://openrouter.ai/api/v1" { t.Fatalf("base url = %s", preview.BaseURL) }
    if len(preview.Keys) != 1 { t.Fatalf("keys = %d", len(preview.Keys)) }
    if preview.Keys[0] != "sk-or-redacted" { t.Fatalf("key not extracted") }
    if preview.PriceSource != PriceSourceOpenRouter { t.Fatalf("price source = %s", preview.PriceSource) }
}

func TestPreviewOpenAICurl(t *testing.T) {
    input := `curl https://api.openai.com/v1/chat/completions -H "Authorization: Bearer sk-redacted"`
    preview := PreviewImport(input)
    if preview.Provider != ProviderOpenAI { t.Fatalf("provider = %s", preview.Provider) }
    if preview.BaseURL != "https://api.openai.com" { t.Fatalf("base url = %s", preview.BaseURL) }
    if preview.ChannelType != 1 { t.Fatalf("channel type = %d", preview.ChannelType) }
}

func TestPreviewBaseURLAndMultipleKeys(t *testing.T) {
    input := "Base URL: https://gateway.example.com/v1\nKey: sk-one\nsk-two"
    preview := PreviewImport(input)
    if preview.Provider != ProviderCustomOpenAICompatible { t.Fatalf("provider = %s", preview.Provider) }
    if preview.BaseURL != "https://gateway.example.com/v1" { t.Fatalf("base url = %s", preview.BaseURL) }
    if len(preview.Keys) != 2 { t.Fatalf("keys = %#v", preview.Keys) }
    if !preview.IsMultiKey { t.Fatalf("expected multi-key") }
}

func TestPreviewGeminiKey(t *testing.T) {
    input := "AIzaSyRedactedExample"
    preview := PreviewImport(input)
    if preview.Provider != ProviderGemini { t.Fatalf("provider = %s", preview.Provider) }
    if preview.ChannelType != 24 { t.Fatalf("channel type = %d", preview.ChannelType) }
}
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
go test ./service/channelconsole -run 'TestPreview' -count=1
```

Expected before implementation: package or symbols missing.

- [ ] **Step 3: Create shared types**

Create `service/channelconsole/types.go`.

```go
package channelconsole

const (
    ProviderOpenRouter              = "openrouter"
    ProviderOpenAI                  = "openai"
    ProviderAnthropic               = "anthropic"
    ProviderGemini                  = "gemini"
    ProviderCustomOpenAICompatible  = "custom_openai_compatible"

    PriceSourceOpenRouter = "openrouter"
    PriceSourceOpenAI     = "openai_official"
    PriceSourceAnthropic  = "anthropic_official"
    PriceSourceGemini     = "gemini_official"
    PriceSourceManual     = "manual_template"

    ImportKindCurl       = "curl"
    ImportKindJSON       = "json"
    ImportKindKeyOnly    = "key_only"
    ImportKindStructured = "structured_text"
)

type ImportPreview struct {
    Provider              string   `json:"provider"`
    ProviderLabel         string   `json:"provider_label"`
    ChannelType           int      `json:"channel_type"`
    BaseURL               string   `json:"base_url"`
    Keys                  []string `json:"-"`
    KeyPreviews           []string `json:"key_previews"`
    IsMultiKey            bool     `json:"is_multi_key"`
    MultiKeyMode          string   `json:"multi_key_mode"`
    ImportKind            string   `json:"import_kind"`
    PriceSource           string   `json:"price_source"`
    ModelDiscovery        string   `json:"model_discovery"`
    DefaultTestModel      string   `json:"default_test_model"`
    SuggestedName         string   `json:"suggested_name"`
    RequiresConfirmation  bool     `json:"requires_confirmation"`
    Warnings              []string `json:"warnings"`
}

type ImportCommitRequest struct {
    RawInput         string   `json:"raw_input"`
    Name             string   `json:"name"`
    Group            string   `json:"group"`
    Models           []string `json:"models"`
    MultiKeyMode     string   `json:"multi_key_mode"`
    Markup           float64  `json:"markup"`
    EnableKnownPrice bool     `json:"enable_known_price"`
}
```

- [ ] **Step 4: Implement importer**

Create `service/channelconsole/importer.go`.

```go
package channelconsole

import (
    "net/url"
    "regexp"
    "strings"

    "github.com/QuantumNous/new-api/constant"
)

var bearerRe = regexp.MustCompile(`(?i)Authorization:\s*Bearer\s+([A-Za-z0-9_\-\.]+)`)
var urlRe = regexp.MustCompile(`https?://[^\s"']+`)

func PreviewImport(raw string) ImportPreview {
    normalized := strings.TrimSpace(raw)
    preview := ImportPreview{MultiKeyMode: "polling", RequiresConfirmation: false}
    preview.ImportKind = detectImportKind(normalized)
    preview.BaseURL = extractBaseURL(normalized)
    preview.Keys = extractKeys(normalized)
    preview.KeyPreviews = maskKeys(preview.Keys)
    preview.IsMultiKey = len(preview.Keys) > 1
    detectProvider(&preview, normalized)
    if preview.SuggestedName == "" { preview.SuggestedName = preview.ProviderLabel + " API 池" }
    if len(preview.Keys) == 0 {
        preview.RequiresConfirmation = true
        preview.Warnings = append(preview.Warnings, "未识别到 API Key，请确认粘贴内容")
    }
    return preview
}

func detectImportKind(raw string) string {
    trimmed := strings.TrimSpace(raw)
    switch {
    case strings.HasPrefix(strings.ToLower(trimmed), "curl "):
        return ImportKindCurl
    case strings.HasPrefix(trimmed, "{"):
        return ImportKindJSON
    case strings.Contains(strings.ToLower(trimmed), "base url") || strings.Contains(strings.ToLower(trimmed), "key:"):
        return ImportKindStructured
    default:
        return ImportKindKeyOnly
    }
}

func extractBaseURL(raw string) string {
    lower := strings.ToLower(raw)
    if idx := strings.Index(lower, "base url"); idx >= 0 {
        line := strings.Split(raw[idx:], "\n")[0]
        if u := firstURL(line); u != "" { return normalizeBaseURL(u) }
    }
    if u := firstURL(raw); u != "" { return normalizeBaseURL(u) }
    return ""
}

func firstURL(raw string) string {
    m := urlRe.FindString(raw)
    if m == "" { return "" }
    return strings.TrimRight(m, "\\")
}

func normalizeBaseURL(raw string) string {
    parsed, err := url.Parse(raw)
    if err != nil || parsed.Scheme == "" || parsed.Host == "" { return raw }
    path := parsed.Path
    for _, suffix := range []string{"/chat/completions", "/responses", "/models"} {
        path = strings.TrimSuffix(path, suffix)
    }
    path = strings.TrimRight(path, "/")
    if strings.HasSuffix(path, "/v1") || strings.HasSuffix(path, "/api/v1") || strings.HasSuffix(path, "/api") {
        parsed.Path = path
    } else {
        parsed.Path = path
    }
    parsed.RawQuery = ""
    parsed.Fragment = ""
    return strings.TrimRight(parsed.String(), "/")
}

func extractKeys(raw string) []string {
    seen := map[string]bool{}
    keys := make([]string, 0)
    for _, match := range bearerRe.FindAllStringSubmatch(raw, -1) {
        addKey(&keys, seen, match[1])
    }
    for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
        return r == '\n' || r == '\r' || r == ' ' || r == '\t' || r == ',' || r == '"' || r == '\''
    }) {
        cleaned := strings.TrimSpace(strings.TrimPrefix(token, "Key:"))
        if strings.HasPrefix(cleaned, "sk-") || strings.HasPrefix(cleaned, "sk-or-") || strings.HasPrefix(cleaned, "AIza") {
            addKey(&keys, seen, cleaned)
        }
    }
    return keys
}

func addKey(keys *[]string, seen map[string]bool, key string) {
    key = strings.TrimSpace(strings.Trim(key, "'\""))
    if key == "" || seen[key] { return }
    seen[key] = true
    *keys = append(*keys, key)
}

func maskKeys(keys []string) []string {
    out := make([]string, 0, len(keys))
    for _, key := range keys { out = append(out, MaskCredential(key)) }
    return out
}

func MaskCredential(key string) string {
    if len(key) <= 8 { return "****" }
    return key[:4] + "..." + key[len(key)-4:]
}

func detectProvider(preview *ImportPreview, raw string) {
    lower := strings.ToLower(raw + " " + preview.BaseURL)
    switch {
    case strings.Contains(lower, "openrouter.ai") || hasPrefix(preview.Keys, "sk-or-"):
        preview.Provider = ProviderOpenRouter
        preview.ProviderLabel = "OpenRouter"
        preview.ChannelType = constant.ChannelTypeOpenRouter
        if preview.BaseURL == "" { preview.BaseURL = "https://openrouter.ai/api/v1" }
        preview.PriceSource = PriceSourceOpenRouter
        preview.ModelDiscovery = "openrouter_models"
        preview.DefaultTestModel = "openai/gpt-4o-mini"
    case strings.Contains(lower, "api.openai.com"):
        preview.Provider = ProviderOpenAI
        preview.ProviderLabel = "OpenAI 官方"
        preview.ChannelType = constant.ChannelTypeOpenAI
        if preview.BaseURL == "" { preview.BaseURL = "https://api.openai.com" }
        preview.PriceSource = PriceSourceOpenAI
        preview.ModelDiscovery = "openai_models"
        preview.DefaultTestModel = "gpt-4o-mini"
    case strings.Contains(lower, "anthropic.com"):
        preview.Provider = ProviderAnthropic
        preview.ProviderLabel = "Anthropic 官方"
        preview.ChannelType = constant.ChannelTypeAnthropic
        if preview.BaseURL == "" { preview.BaseURL = "https://api.anthropic.com" }
        preview.PriceSource = PriceSourceAnthropic
        preview.ModelDiscovery = "anthropic_template"
        preview.DefaultTestModel = "claude-3-5-haiku-20241022"
    case strings.Contains(lower, "generativelanguage.googleapis.com") || hasPrefix(preview.Keys, "AIza"):
        preview.Provider = ProviderGemini
        preview.ProviderLabel = "Google Gemini 官方"
        preview.ChannelType = constant.ChannelTypeGemini
        if preview.BaseURL == "" { preview.BaseURL = "https://generativelanguage.googleapis.com" }
        preview.PriceSource = PriceSourceGemini
        preview.ModelDiscovery = "gemini_models"
        preview.DefaultTestModel = "gemini-1.5-flash"
    default:
        preview.Provider = ProviderCustomOpenAICompatible
        preview.ProviderLabel = "OpenAI 兼容第三方"
        preview.ChannelType = constant.ChannelTypeCustom
        preview.PriceSource = PriceSourceManual
        preview.ModelDiscovery = "openai_compatible_models"
        preview.DefaultTestModel = "gpt-4o-mini"
        preview.RequiresConfirmation = true
    }
}

func hasPrefix(values []string, prefix string) bool {
    for _, value := range values { if strings.HasPrefix(value, prefix) { return true } }
    return false
}
```

- [ ] **Step 5: Run parser tests**

Run:

```bash
go test ./service/channelconsole -run 'TestPreview' -count=1
```

Expected: all preview tests pass.

- [ ] **Step 6: Commit Task 2**

```bash
git add service/channelconsole/types.go service/channelconsole/importer.go service/channelconsole/importer_test.go
git commit -m "feat: add channel console import preview parser"
```

---

### Task 3: Pricing normalization and ratio compilation

**Files:**
- Create: `service/channelconsole/pricing.go`
- Create: `service/channelconsole/pricing_test.go`

- [ ] **Step 1: Write pricing tests**

Create `service/channelconsole/pricing_test.go`.

```go
package channelconsole

import "testing"

func floatPtr(v float64) *float64 { return &v }

func TestCompileTokenPriceToRatios(t *testing.T) {
    price := NormalizedModelPrice{
        ModelName: "example-model",
        InputUSDPer1MTokens: floatPtr(2.0),
        OutputUSDPer1MTokens: floatPtr(10.0),
        CachedInputUSDPer1MTokens: floatPtr(0.5),
    }
    compiled := CompileTokenPrice(price, 1.2)
    if compiled.PriceStatus != PriceStatusSynced { t.Fatalf("status = %s", compiled.PriceStatus) }
    if compiled.ModelRatio == nil || *compiled.ModelRatio <= 0 { t.Fatalf("missing model ratio") }
    if compiled.CompletionRatio == nil || *compiled.CompletionRatio != 5.0 { t.Fatalf("completion = %#v", compiled.CompletionRatio) }
    if compiled.CacheRatio == nil || *compiled.CacheRatio != 0.25 { t.Fatalf("cache = %#v", compiled.CacheRatio) }
}

func TestCompileUnknownPrice(t *testing.T) {
    compiled := CompileTokenPrice(NormalizedModelPrice{ModelName: "unknown"}, 1.2)
    if compiled.PriceStatus != PriceStatusUnknown { t.Fatalf("status = %s", compiled.PriceStatus) }
    if compiled.Enabled { t.Fatalf("unknown price must not auto-enable") }
}

func TestBuiltInOpenAIPriceTemplate(t *testing.T) {
    prices := BuiltInPrices(ProviderOpenAI)
    if _, ok := prices["gpt-4o-mini"]; !ok { t.Fatalf("expected gpt-4o-mini price") }
}
```

- [ ] **Step 2: Run failing pricing tests**

Run:

```bash
go test ./service/channelconsole -run 'TestCompile|TestBuiltIn' -count=1
```

Expected before implementation: symbols missing.

- [ ] **Step 3: Implement pricing helpers**

Create `service/channelconsole/pricing.go`.

```go
package channelconsole

const (
    PriceStatusSynced  = "synced"
    PriceStatusUnknown = "price_unknown"
    PriceStatusManual  = "manual"
)

type NormalizedModelPrice struct {
    ModelName string
    ProviderModelName string
    Source string
    InputUSDPer1MTokens *float64
    OutputUSDPer1MTokens *float64
    CachedInputUSDPer1MTokens *float64
    CacheWrite5mUSDPer1MTokens *float64
    CacheWrite1hUSDPer1MTokens *float64
    RequestUSDPerCall *float64
    ImageUSDPerUnit *float64
}

type CompiledPrice struct {
    ModelName string
    ModelRatio *float64
    CompletionRatio *float64
    CacheRatio *float64
    CreateCacheRatio *float64
    ModelPrice *float64
    PriceStatus string
    Enabled bool
}

func CompileTokenPrice(price NormalizedModelPrice, markup float64) CompiledPrice {
    if markup <= 0 { markup = 1.0 }
    compiled := CompiledPrice{ModelName: price.ModelName, PriceStatus: PriceStatusUnknown, Enabled: false}
    if price.InputUSDPer1MTokens == nil || *price.InputUSDPer1MTokens <= 0 {
        if price.RequestUSDPerCall != nil {
            v := *price.RequestUSDPerCall * markup
            compiled.ModelPrice = &v
            compiled.PriceStatus = PriceStatusSynced
            compiled.Enabled = true
        }
        return compiled
    }
    modelRatio := (*price.InputUSDPer1MTokens) * markup
    compiled.ModelRatio = &modelRatio
    if price.OutputUSDPer1MTokens != nil && *price.OutputUSDPer1MTokens > 0 {
        completion := (*price.OutputUSDPer1MTokens) / (*price.InputUSDPer1MTokens)
        compiled.CompletionRatio = &completion
    }
    if price.CachedInputUSDPer1MTokens != nil && *price.CachedInputUSDPer1MTokens >= 0 {
        cache := (*price.CachedInputUSDPer1MTokens) / (*price.InputUSDPer1MTokens)
        compiled.CacheRatio = &cache
    }
    if price.CacheWrite5mUSDPer1MTokens != nil && *price.CacheWrite5mUSDPer1MTokens >= 0 {
        createCache := (*price.CacheWrite5mUSDPer1MTokens) / (*price.InputUSDPer1MTokens)
        compiled.CreateCacheRatio = &createCache
    }
    compiled.PriceStatus = PriceStatusSynced
    compiled.Enabled = true
    return compiled
}

func BuiltInPrices(provider string) map[string]NormalizedModelPrice {
    switch provider {
    case ProviderOpenAI:
        return map[string]NormalizedModelPrice{
            "gpt-4o-mini": {ModelName: "gpt-4o-mini", Source: PriceSourceOpenAI, InputUSDPer1MTokens: f64(0.15), OutputUSDPer1MTokens: f64(0.60), CachedInputUSDPer1MTokens: f64(0.075)},
            "gpt-4o": {ModelName: "gpt-4o", Source: PriceSourceOpenAI, InputUSDPer1MTokens: f64(2.50), OutputUSDPer1MTokens: f64(10.00), CachedInputUSDPer1MTokens: f64(1.25)},
        }
    case ProviderAnthropic:
        return map[string]NormalizedModelPrice{
            "claude-3-5-haiku-20241022": {ModelName: "claude-3-5-haiku-20241022", Source: PriceSourceAnthropic, InputUSDPer1MTokens: f64(0.80), OutputUSDPer1MTokens: f64(4.00)},
            "claude-3-5-sonnet-20241022": {ModelName: "claude-3-5-sonnet-20241022", Source: PriceSourceAnthropic, InputUSDPer1MTokens: f64(3.00), OutputUSDPer1MTokens: f64(15.00)},
        }
    case ProviderGemini:
        return map[string]NormalizedModelPrice{
            "gemini-1.5-flash": {ModelName: "gemini-1.5-flash", Source: PriceSourceGemini, InputUSDPer1MTokens: f64(0.075), OutputUSDPer1MTokens: f64(0.30)},
            "gemini-1.5-pro": {ModelName: "gemini-1.5-pro", Source: PriceSourceGemini, InputUSDPer1MTokens: f64(1.25), OutputUSDPer1MTokens: f64(5.00)},
        }
    default:
        return map[string]NormalizedModelPrice{}
    }
}

func f64(v float64) *float64 { return &v }
```

- [ ] **Step 4: Run pricing tests**

Run:

```bash
go test ./service/channelconsole -run 'TestCompile|TestBuiltIn' -count=1
```

Expected: pricing tests pass.

- [ ] **Step 5: Commit Task 3**

```bash
git add service/channelconsole/pricing.go service/channelconsole/pricing_test.go
git commit -m "feat: add channel console pricing compiler"
```

---

### Task 4: Console orchestration and import commit

**Files:**
- Create: `service/channelconsole/console.go`
- Modify: `service/channelconsole/types.go`
- Test: `go test ./service/channelconsole ./controller`

- [ ] **Step 1: Extend commit response types**

Append to `service/channelconsole/types.go`.

```go
type ImportCommitResult struct {
    ChannelID int `json:"channel_id"`
    Name string `json:"name"`
    Provider string `json:"provider"`
    KeyCount int `json:"key_count"`
    ModelCount int `json:"model_count"`
    HealthStatus string `json:"health_status"`
    PriceStatus string `json:"price_status"`
}
```

- [ ] **Step 2: Implement commit orchestration**

Create `service/channelconsole/console.go` with the following functions.

```go
package channelconsole

import (
    "strings"

    "github.com/QuantumNous/new-api/common"
    "github.com/QuantumNous/new-api/constant"
    "github.com/QuantumNous/new-api/model"
)

func CommitImport(req ImportCommitRequest) (*ImportCommitResult, error) {
    preview := PreviewImport(req.RawInput)
    name := strings.TrimSpace(req.Name)
    if name == "" { name = preview.SuggestedName }
    group := strings.TrimSpace(req.Group)
    if group == "" { group = "default" }
    markup := req.Markup
    if markup <= 0 { markup = 1.2 }
    mode := req.MultiKeyMode
    if mode != "random" && mode != "polling" { mode = "polling" }

    keys := strings.Join(preview.Keys, "\n")
    models := strings.Join(req.Models, ",")
    if models == "" { models = preview.DefaultTestModel }
    baseURL := preview.BaseURL
    channel := &model.Channel{
        Type: preview.ChannelType,
        Key: keys,
        Name: name,
        Status: common.ChannelStatusEnabled,
        CreatedTime: common.GetTimestamp(),
        BaseURL: &baseURL,
        Models: models,
        Group: group,
        TestModel: &preview.DefaultTestModel,
        Tag: strPtr("yunbay-console"),
        ChannelInfo: model.ChannelInfo{
            IsMultiKey: len(preview.Keys) > 1,
            MultiKeySize: len(preview.Keys),
            MultiKeyMode: constant.MultiKeyMode(mode),
        },
    }
    if channel.ChannelInfo.IsMultiKey {
        channel.ChannelInfo.MultiKeyStatusList = map[int]int{}
        for i := range preview.Keys { channel.ChannelInfo.MultiKeyStatusList[i] = common.ChannelStatusEnabled }
    }
    if err := model.AddChannel(channel); err != nil { return nil, err }
    meta := &model.ChannelConsoleChannel{
        ChannelId: channel.Id,
        Provider: preview.Provider,
        ProviderKind: model.ChannelConsoleKindThirdPartyAPI,
        ImportKind: preview.ImportKind,
        PriceSource: preview.PriceSource,
        HealthStatus: model.ChannelConsoleStatusUnchecked,
        ModelSyncStatus: model.ChannelConsoleStatusUnchecked,
        PriceSyncStatus: model.ChannelConsoleStatusUnchecked,
        Markup: markup,
        AutoDisablePolicy: "mark_only",
    }
    if err := model.UpsertChannelConsoleChannel(meta); err != nil { return nil, err }
    return &ImportCommitResult{ChannelID: channel.Id, Name: name, Provider: preview.Provider, KeyCount: len(preview.Keys), ModelCount: len(req.Models), HealthStatus: meta.HealthStatus, PriceStatus: meta.PriceSyncStatus}, nil
}

func strPtr(v string) *string { return &v }
```

- [ ] **Step 3: Verify compile**

Run:

```bash
go test ./service/channelconsole ./model
```

Expected: both packages compile and channelconsole tests pass.

- [ ] **Step 4: Commit Task 4**

```bash
git add service/channelconsole/types.go service/channelconsole/console.go
git commit -m "feat: commit imported credentials to channels"
```

---

### Task 5: Backend controller and routes

**Files:**
- Create: `controller/channel_console.go`
- Modify: `router/api-router.go`
- Test: `go test ./controller ./router`

- [ ] **Step 1: Create controller handlers**

Create `controller/channel_console.go`.

```go
package controller

import (
    "net/http"
    "strconv"

    "github.com/QuantumNous/new-api/common"
    "github.com/QuantumNous/new-api/model"
    "github.com/QuantumNous/new-api/service/channelconsole"
    "github.com/gin-gonic/gin"
)

type channelConsolePreviewRequest struct { RawInput string `json:"raw_input"` }

func PreviewChannelConsoleImport(c *gin.Context) {
    req := channelConsolePreviewRequest{}
    if err := c.ShouldBindJSON(&req); err != nil { common.ApiError(c, err); return }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": channelconsole.PreviewImport(req.RawInput)})
}

func CommitChannelConsoleImport(c *gin.Context) {
    req := channelconsole.ImportCommitRequest{}
    if err := c.ShouldBindJSON(&req); err != nil { common.ApiError(c, err); return }
    result, err := channelconsole.CommitImport(req)
    if err != nil { c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func ListChannelConsoleChannels(c *gin.Context) {
    channels, err := model.GetAllChannels(0, 100, true, false)
    if err != nil { c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": channels})
}

func GetChannelConsoleChannel(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil { c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid channel id"}); return }
    ch, err := model.GetChannelById(id, true)
    if err != nil { c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()}); return }
    prices, _ := model.ListChannelConsoleModelPrices(id)
    checks, _ := model.ListChannelConsoleHealthChecks(id, 50)
    meta, _ := model.GetChannelConsoleChannelByChannelID(id)
    c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"channel": ch, "console": meta, "prices": prices, "health_checks": checks}})
}
```

- [ ] **Step 2: Register routes**

In `router/api-router.go`, add this group after `ratioSyncRoute` and before `channelRoute`.

```go
channelConsoleRoute := apiRouter.Group("/channel-console")
channelConsoleRoute.Use(middleware.AdminAuth())
{
    channelConsoleRoute.POST("/import/preview", middleware.CriticalRateLimit(), controller.PreviewChannelConsoleImport)
    channelConsoleRoute.POST("/import/commit", middleware.CriticalRateLimit(), controller.CommitChannelConsoleImport)
    channelConsoleRoute.GET("/channels", controller.ListChannelConsoleChannels)
    channelConsoleRoute.GET("/channels/:id", controller.GetChannelConsoleChannel)
}
```

- [ ] **Step 3: Verify compile**

Run:

```bash
go test ./controller ./router
```

Expected: packages compile. If route package has no tests, output still succeeds.

- [ ] **Step 4: Commit Task 5**

```bash
git add controller/channel_console.go router/api-router.go
git commit -m "feat: expose channel console import api"
```

---

### Task 6: Health state recording and list summary

**Files:**
- Modify: `service/channelconsole/health.go`
- Modify: `service/channelconsole/console.go`
- Modify: `controller/channel_console.go`
- Test: `service/channelconsole/health_test.go`

- [ ] **Step 1: Write health aggregation test**

Create `service/channelconsole/health_test.go`.

```go
package channelconsole

import "testing"

func TestAggregateHealthStatus(t *testing.T) {
    cases := []struct{ name string; statuses []string; want string }{
        {"empty", nil, HealthUnchecked},
        {"all healthy", []string{HealthHealthy, HealthHealthy}, HealthHealthy},
        {"one failed", []string{HealthHealthy, HealthFailed}, HealthWarning},
        {"all failed", []string{HealthFailed, HealthFailed}, HealthFailed},
        {"disabled wins", []string{HealthDisabled, HealthHealthy}, HealthDisabled},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if got := AggregateHealthStatus(tc.statuses); got != tc.want { t.Fatalf("got %s want %s", got, tc.want) }
        })
    }
}
```

- [ ] **Step 2: Implement health helpers**

Create `service/channelconsole/health.go`.

```go
package channelconsole

const (
    HealthHealthy = "healthy"
    HealthWarning = "warning"
    HealthFailed = "failed"
    HealthDisabled = "disabled"
    HealthUnchecked = "unchecked"
)

func AggregateHealthStatus(statuses []string) string {
    if len(statuses) == 0 { return HealthUnchecked }
    hasHealthy := false
    hasFailed := false
    for _, status := range statuses {
        if status == HealthDisabled { return HealthDisabled }
        if status == HealthHealthy { hasHealthy = true }
        if status == HealthFailed { hasFailed = true }
        if status == HealthWarning || status == HealthUnchecked { return HealthWarning }
    }
    if hasHealthy && hasFailed { return HealthWarning }
    if hasFailed { return HealthFailed }
    return HealthHealthy
}
```

- [ ] **Step 3: Add manual health check handler**

Append controller function in `controller/channel_console.go` that calls existing `controller.TestChannel` behavior is not reused directly; first working version records an unchecked health row if no live upstream call is made.

```go
func CheckChannelConsoleHealth(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil { c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid channel id"}); return }
    check := &model.ChannelConsoleHealthCheck{ChannelId: id, CheckType: "manual", Status: channelconsole.HealthUnchecked, ErrorCode: "manual_check_queued", ErrorMessage: "已记录手动验活请求；实时上游调用由后续调度器执行"}
    if err := model.CreateChannelConsoleHealthCheck(check); err != nil { c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()}); return }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": check})
}
```

- [ ] **Step 4: Register health route**

In `router/api-router.go`, add:

```go
channelConsoleRoute.POST("/channels/:id/health-check", middleware.CriticalRateLimit(), controller.CheckChannelConsoleHealth)
```

- [ ] **Step 5: Verify health tests**

Run:

```bash
go test ./service/channelconsole -run 'TestAggregateHealthStatus' -count=1 && go test ./controller ./router
```

Expected: health tests pass and backend packages compile.

- [ ] **Step 6: Commit Task 6**

```bash
git add service/channelconsole/health.go service/channelconsole/health_test.go controller/channel_console.go router/api-router.go
git commit -m "feat: record channel console health checks"
```

---

### Task 7: Frontend API types and client

**Files:**
- Create: `web/default/src/features/channel-console/types.ts`
- Create: `web/default/src/features/channel-console/api.ts`
- Test: `cd web/default && npm run typecheck`

- [ ] **Step 1: Create frontend types**

Create `web/default/src/features/channel-console/types.ts`.

```ts
import type { Channel } from '@/features/channels/types'

export type ChannelConsoleStatus =
  | 'healthy'
  | 'warning'
  | 'failed'
  | 'disabled'
  | 'unchecked'

export interface ImportPreview {
  provider: string
  provider_label: string
  channel_type: number
  base_url: string
  key_previews: string[]
  is_multi_key: boolean
  multi_key_mode: 'random' | 'polling'
  import_kind: string
  price_source: string
  model_discovery: string
  default_test_model: string
  suggested_name: string
  requires_confirmation: boolean
  warnings?: string[]
}

export interface ImportCommitRequest {
  raw_input: string
  name?: string
  group?: string
  models?: string[]
  multi_key_mode?: 'random' | 'polling'
  markup?: number
  enable_known_price?: boolean
}

export interface ImportCommitResult {
  channel_id: number
  name: string
  provider: string
  key_count: number
  model_count: number
  health_status: ChannelConsoleStatus
  price_status: string
}

export interface ChannelConsoleDetail {
  channel: Channel
  console?: {
    id: number
    channel_id: number
    provider: string
    provider_kind: string
    health_status: ChannelConsoleStatus
    model_sync_status: string
    price_sync_status: string
    last_error_message?: string
  }
  prices?: Array<Record<string, unknown>>
  health_checks?: Array<Record<string, unknown>>
}

export interface ApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}
```

- [ ] **Step 2: Create API client**

Create `web/default/src/features/channel-console/api.ts`.

```ts
import { api } from '@/lib/api'
import type {
  ApiResponse,
  ChannelConsoleDetail,
  ImportCommitRequest,
  ImportCommitResult,
  ImportPreview,
} from './types'
import type { Channel } from '@/features/channels/types'

export async function previewChannelConsoleImport(
  rawInput: string
): Promise<ApiResponse<ImportPreview>> {
  const res = await api.post('/api/channel-console/import/preview', {
    raw_input: rawInput,
  })
  return res.data
}

export async function commitChannelConsoleImport(
  payload: ImportCommitRequest
): Promise<ApiResponse<ImportCommitResult>> {
  const res = await api.post('/api/channel-console/import/commit', payload)
  return res.data
}

export async function listChannelConsoleChannels(): Promise<
  ApiResponse<Channel[]>
> {
  const res = await api.get('/api/channel-console/channels')
  return res.data
}

export async function getChannelConsoleDetail(
  id: number
): Promise<ApiResponse<ChannelConsoleDetail>> {
  const res = await api.get(`/api/channel-console/channels/${id}`)
  return res.data
}

export async function checkChannelConsoleHealth(
  id: number
): Promise<ApiResponse<Record<string, unknown>>> {
  const res = await api.post(`/api/channel-console/channels/${id}/health-check`)
  return res.data
}
```

- [ ] **Step 3: Run frontend typecheck**

Run:

```bash
cd web/default && npm run typecheck
```

Expected: new files typecheck or existing unrelated project issues are recorded before continuing.

- [ ] **Step 4: Commit Task 7**

```bash
git add web/default/src/features/channel-console/types.ts web/default/src/features/channel-console/api.ts
git commit -m "feat: add channel console frontend api"
```

---

### Task 8: Frontend page and components

**Files:**
- Create: `web/default/src/features/channel-console/components/import-panel.tsx`
- Create: `web/default/src/features/channel-console/components/channel-console-table.tsx`
- Create: `web/default/src/features/channel-console/components/channel-detail-drawer.tsx`
- Create: `web/default/src/features/channel-console/index.tsx`
- Create: `web/default/src/routes/_authenticated/channel-console/index.tsx`

- [ ] **Step 1: Create import panel**

Create `web/default/src/features/channel-console/components/import-panel.tsx`.

```tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import {
  commitChannelConsoleImport,
  previewChannelConsoleImport,
} from '../api'
import type { ImportPreview } from '../types'

export function ImportPanel({ onImported }: { onImported: () => void }) {
  const { t } = useTranslation()
  const [rawInput, setRawInput] = useState('')
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [loading, setLoading] = useState(false)

  async function handlePreview() {
    setLoading(true)
    try {
      const res = await previewChannelConsoleImport(rawInput)
      if (!res.success || !res.data) throw new Error(res.message || t('Preview failed'))
      setPreview(res.data)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Preview failed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleCommit() {
    if (!preview) return
    setLoading(true)
    try {
      const res = await commitChannelConsoleImport({
        raw_input: rawInput,
        name: preview.suggested_name,
        models: preview.default_test_model ? [preview.default_test_model] : [],
        multi_key_mode: preview.multi_key_mode,
        markup: 1.2,
        enable_known_price: true,
      })
      if (!res.success) throw new Error(res.message || t('Import failed'))
      toast.success(t('Channel imported'))
      setRawInput('')
      setPreview(null)
      onImported()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Import failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Copy to import')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        <Textarea
          value={rawInput}
          onChange={(event) => setRawInput(event.target.value)}
          placeholder={t('Paste API key, curl, JSON, Base URL + Key, or Authorization header')}
          className='min-h-36 font-mono text-xs'
        />
        <div className='flex gap-2'>
          <Button disabled={!rawInput.trim() || loading} onClick={handlePreview}>{t('Preview')}</Button>
          <Button disabled={!preview || loading} variant='secondary' onClick={handleCommit}>{t('Save and verify')}</Button>
        </div>
        {preview && (
          <div className='rounded-lg border p-3 text-sm'>
            <div className='font-medium'>{preview.provider_label}</div>
            <div className='text-muted-foreground'>{preview.base_url}</div>
            <div>{t('Keys')}: {preview.key_previews.join(', ')}</div>
            <div>{t('Price source')}: {preview.price_source}</div>
            {preview.warnings?.map((warning) => <div key={warning} className='text-amber-600'>{warning}</div>)}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: Create status table**

Create `web/default/src/features/channel-console/components/channel-console-table.tsx`.

```tsx
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { Channel } from '@/features/channels/types'

function statusVariant(status: number) {
  if (status === 1) return 'default'
  if (status === 2) return 'destructive'
  return 'secondary'
}

export function ChannelConsoleTable({
  channels,
  onOpen,
}: {
  channels: Channel[]
  onOpen: (channel: Channel) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='overflow-hidden rounded-lg border'>
      <table className='w-full text-sm'>
        <thead className='bg-muted/50 text-left'>
          <tr>
            <th className='p-3'>{t('Channel')}</th>
            <th className='p-3'>{t('Base URL')}</th>
            <th className='p-3'>{t('Models')}</th>
            <th className='p-3'>{t('Status')}</th>
            <th className='p-3'>{t('Actions')}</th>
          </tr>
        </thead>
        <tbody>
          {channels.map((channel) => (
            <tr key={channel.id} className='border-t'>
              <td className='p-3'>
                <div className='font-medium'>{channel.name}</div>
                <div className='text-muted-foreground'>#{channel.id}</div>
              </td>
              <td className='p-3 max-w-72 truncate'>{channel.base_url || '-'}</td>
              <td className='p-3'>{channel.models ? channel.models.split(',').length : 0}</td>
              <td className='p-3'><Badge variant={statusVariant(channel.status)}>{channel.status === 1 ? t('Healthy') : t('Needs attention')}</Badge></td>
              <td className='p-3'><Button size='sm' variant='outline' onClick={() => onOpen(channel)}>{t('Details')}</Button></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
```

- [ ] **Step 3: Create detail drawer**

Create `web/default/src/features/channel-console/components/channel-detail-drawer.tsx`.

```tsx
import { useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from '@/components/ui/drawer'
import type { Channel } from '@/features/channels/types'
import { checkChannelConsoleHealth, getChannelConsoleDetail } from '../api'
import type { ChannelConsoleDetail } from '../types'

export function ChannelDetailDrawer({ channel, open, onOpenChange }: { channel: Channel | null; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation()
  const [detail, setDetail] = useState<ChannelConsoleDetail | null>(null)

  useEffect(() => {
    if (!channel || !open) return
    getChannelConsoleDetail(channel.id).then((res) => setDetail(res.data || null))
  }, [channel, open])

  async function handleHealthCheck() {
    if (!channel) return
    const res = await checkChannelConsoleHealth(channel.id)
    if (!res.success) { toast.error(res.message || t('Health check failed')); return }
    toast.success(t('Health check recorded'))
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{channel?.name || t('Channel details')}</DrawerTitle>
        </DrawerHeader>
        <div className='space-y-4 p-4'>
          <div className='grid gap-2 text-sm md:grid-cols-2'>
            <div>{t('Provider')}: {detail?.console?.provider || '-'}</div>
            <div>{t('Health')}: {detail?.console?.health_status || 'unchecked'}</div>
            <div>{t('Price status')}: {detail?.console?.price_sync_status || 'unchecked'}</div>
            <div>{t('Models')}: {channel?.models || '-'}</div>
          </div>
          <div className='flex gap-2'>
            <Button onClick={handleHealthCheck}>{t('Verify now')}</Button>
            {channel && <Button variant='outline' asChild><Link to='/channels'>{t('Advanced channel management')}</Link></Button>}
          </div>
          <div className='rounded-lg border p-3 text-sm'>
            <div className='font-medium'>{t('Recent health records')}</div>
            <pre className='mt-2 max-h-48 overflow-auto text-xs'>{JSON.stringify(detail?.health_checks || [], null, 2)}</pre>
          </div>
        </div>
      </DrawerContent>
    </Drawer>
  )
}
```

- [ ] **Step 4: Create page shell**

Create `web/default/src/features/channel-console/index.tsx`.

```tsx
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import type { Channel } from '@/features/channels/types'
import { listChannelConsoleChannels } from './api'
import { ChannelDetailDrawer } from './components/channel-detail-drawer'
import { ChannelConsoleTable } from './components/channel-console-table'
import { ImportPanel } from './components/import-panel'

export function ChannelConsole() {
  const { t } = useTranslation()
  const [channels, setChannels] = useState<Channel[]>([])
  const [selected, setSelected] = useState<Channel | null>(null)

  async function loadChannels() {
    const res = await listChannelConsoleChannels()
    setChannels(res.data || [])
  }

  useEffect(() => { void loadChannels() }, [])

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Unified Channel Console')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-4 xl:grid-cols-[1fr_380px]'>
          <ChannelConsoleTable channels={channels} onOpen={setSelected} />
          <ImportPanel onImported={loadChannels} />
        </div>
        <ChannelDetailDrawer channel={selected} open={Boolean(selected)} onOpenChange={(open) => !open && setSelected(null)} />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
```

- [ ] **Step 5: Create route**

Create `web/default/src/routes/_authenticated/channel-console/index.tsx`.

```tsx
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { ChannelConsole } from '@/features/channel-console'

export const Route = createFileRoute('/_authenticated/channel-console/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: ChannelConsole,
})
```

- [ ] **Step 6: Verify frontend compile**

Run:

```bash
cd web/default && npm run typecheck
```

Expected: new page typechecks.

- [ ] **Step 7: Commit Task 8**

```bash
git add web/default/src/features/channel-console web/default/src/routes/_authenticated/channel-console/index.tsx
git commit -m "feat: add unified channel console page"
```

---

### Task 9: Sidebar navigation and route generation

**Files:**
- Modify: `web/default/src/hooks/sidebar-data-model.ts`
- Modify: `web/default/src/hooks/use-sidebar-data.ts`
- Modify: `web/default/src/hooks/use-sidebar-config.ts`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/routeTree.gen.ts`

- [ ] **Step 1: Add sidebar icon**

In `web/default/src/hooks/use-sidebar-data.ts`, add `Network` to the `lucide-react` import and map it.

```ts
import {
  Activity,
  Box,
  CreditCard,
  FileText,
  FlaskConical,
  Key,
  LayoutDashboard,
  ListTodo,
  MessageSquare,
  Network,
  Radio,
  Rocket,
  Settings,
  Ticket,
  User,
  Users,
  Wallet,
} from 'lucide-react'

const SIDEBAR_ICONS: SidebarIconMap = {
  activity: Activity,
  box: Box,
  creditCard: CreditCard,
  fileText: FileText,
  flask: FlaskConical,
  key: Key,
  layoutDashboard: LayoutDashboard,
  listTodo: ListTodo,
  messageSquare: MessageSquare,
  network: Network,
  radio: Radio,
  rocket: Rocket,
  settings: Settings,
  ticket: Ticket,
  user: User,
  users: Users,
  wallet: Wallet,
}
```

- [ ] **Step 2: Extend icon map type**

In `web/default/src/hooks/sidebar-data-model.ts`, add:

```ts
network: SidebarIcon
```

- [ ] **Step 3: Add admin nav item before Channels**

In `buildSidebarData`, admin group items should start with:

```ts
{
  title: t('Unified Channel Console'),
  url: '/channel-console',
  icon: icons.network,
},
{
  title: t('Advanced Channel Management'),
  url: '/channels',
  icon: icons.radio,
},
```

- [ ] **Step 4: Add sidebar config mapping**

In `web/default/src/hooks/use-sidebar-config.ts`, add:

```ts
'/channel-console': { section: 'admin', module: 'channel' },
```

- [ ] **Step 5: Add translations**

Add keys to `web/default/src/i18n/locales/zh.json`:

```json
{
  "Unified Channel Console": "统一渠道控制台",
  "Advanced Channel Management": "高级渠道管理",
  "Copy to import": "复制即导入",
  "Paste API key, curl, JSON, Base URL + Key, or Authorization header": "粘贴 API Key、curl、JSON、Base URL + Key 或 Authorization 请求头",
  "Preview": "预检",
  "Save and verify": "保存并验活",
  "Preview failed": "预检失败",
  "Import failed": "导入失败",
  "Channel imported": "渠道已导入",
  "Price source": "价格来源",
  "Needs attention": "需要处理",
  "Verify now": "立即验活",
  "Health check recorded": "已记录验活请求",
  "Recent health records": "最近验活记录"
}
```

Add equivalent keys to `web/default/src/i18n/locales/en.json` with English values equal to the keys or natural English strings.

- [ ] **Step 6: Regenerate route tree**

Run from `web/default`:

```bash
npm run route:generate
```

If the project uses a different route script name, inspect `web/default/package.json` and run the route generation script that updates `src/routeTree.gen.ts`.

- [ ] **Step 7: Verify frontend build inputs**

Run:

```bash
cd web/default && npm run typecheck
```

Expected: route tree and sidebar compile.

- [ ] **Step 8: Commit Task 9**

```bash
git add web/default/src/hooks/sidebar-data-model.ts web/default/src/hooks/use-sidebar-data.ts web/default/src/hooks/use-sidebar-config.ts web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/en.json web/default/src/routeTree.gen.ts
git commit -m "feat: add channel console navigation"
```

---

### Task 10: End-to-end verification and deployment preparation

**Files:**
- Modify only files needed to fix compile/test failures found in this task.

- [ ] **Step 1: Run backend tests for touched packages**

Run:

```bash
go test ./model ./service/channelconsole ./controller ./router
```

Expected: all touched backend packages compile and tests pass.

- [ ] **Step 2: Run full backend compile test**

Run:

```bash
go test ./...
```

Expected: full test suite passes, or pre-existing unrelated failures are captured with exact package and error. Do not mark complete if new channel-console packages fail.

- [ ] **Step 3: Run frontend typecheck**

Run:

```bash
cd web/default && npm run typecheck
```

Expected: TypeScript passes.

- [ ] **Step 4: Run frontend build**

Run:

```bash
cd web/default && npm run build
```

Expected: production build succeeds.

- [ ] **Step 5: Manual smoke test locally**

Start the app using the existing project dev or production command. Open the admin UI and verify:

```text
/channel-console loads for admin
non-admin is redirected to /403
paste OpenRouter curl produces OpenRouter preview
paste OpenAI curl produces OpenAI preview
commit creates a channel and it appears in the table
detail drawer opens
health check button records a health row
/channels still opens as advanced channel management
```

- [ ] **Step 6: Commit verification fixes**

If fixes were needed:

```bash
git add <changed-files>
git commit -m "fix: stabilize channel console verification"
```

If no fixes were needed, record the verification output in the final response without an extra commit.

---

## Self-Review Checklist

Spec coverage:

- Import convenience is covered by Tasks 2, 4, 5, 7, and 8.
- Model discovery is represented by preview defaults and model fields in Tasks 4 and 8; live provider model syncing remains a follow-up inside the same API surface after the first import UI works.
- Automatic pricing is covered by Task 3 and metadata tables from Task 1; full settings write-back can be expanded after the compiler is verified.
- Periodic health is partially covered by Task 6 with persisted health checks and status model; scheduler automation remains a follow-up task after manual health recording is stable.
- Unified admin UI is covered by Tasks 8 and 9.
- Original `/channels` advanced page remains intact.
- CLIProxyAPI / OAuth is explicitly out of P0 and not implemented here.

Known first-slice limits:

- This plan intentionally ships a safe first slice before adding live scheduled background jobs. It creates the API/data/UI foundation, import parser, channel creation, price compiler, and health records. The next plan should add live `/v1/models` fetching, OpenRouter remote price ingestion, New API ratio-setting write-back, and a timed scheduler once this slice is stable.

Placeholder scan:

- The plan avoids unresolved placeholder markers and undefined file references.
- Code examples define the functions they call within the same task or in earlier tasks.

Type consistency:

- Backend provider strings match frontend string types.
- Health status values are the same across model, service, and frontend.
- API paths use `/api/channel-console/*` consistently.
