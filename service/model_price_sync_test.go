package service

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestModelPriceSync_OpenRouterParserCoversCacheWriteDimensions(t *testing.T) {
	body := `{
		"data": [
			{
				"id": "anthropic/claude-sonnet-4.5",
				"pricing": {
					"prompt": "0.000003",
					"completion": "0.000015",
					"input_cache_read": "0.0000003",
					"input_cache_write": "0.00000375",
					"input_cache_write_1h": "0.000006",
					"image": "0.000004",
					"audio": "0.000005",
					"internal_reasoning": "0.000002",
					"web_search": "0.01"
				}
			}
		]
	}`

	prices, err := ParseOpenRouterModelPrices(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseOpenRouterModelPrices returned error: %v", err)
	}

	price := prices["anthropic/claude-sonnet-4.5"]
	assertPrice(t, price.Input, 3)
	assertPrice(t, price.Output, 15)
	assertPrice(t, price.CacheRead, 0.3)
	assertPrice(t, price.CacheWrite, 3.75)
	assertPrice(t, price.CacheWrite1h, 6)
	assertPrice(t, price.ImageInput, 4)
	assertPrice(t, price.AudioInput, 5)
	assertPrice(t, price.Reasoning, 2)
	if price.WebSearch == nil || *price.WebSearch != 0.01 {
		t.Fatalf("WebSearch = %#v, want 0.01", price.WebSearch)
	}
}

func TestModelPriceSync_DefaultOpenRouterFetcherUsesPublicCatalogWithoutChannel(t *testing.T) {
	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://openrouter.ai/api/v1/models" {
			t.Fatalf("requested URL = %q, want OpenRouter public catalog", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q, want empty for public catalog", got)
		}
		body := `{"data":[{"id":"openai/gpt-4.1","pricing":{"prompt":"0.000002","completion":"0.000008"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	defer func() {
		http.DefaultTransport = originalTransport
	}()

	prices, err := defaultOpenRouterModelPriceFetcher{}.FetchOpenRouterModelPrices(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchOpenRouterModelPrices without channel returned error: %v", err)
	}

	price := prices["openai/gpt-4.1"]
	assertPrice(t, price.Input, 2)
	assertPrice(t, price.Output, 8)
}

func TestModelPriceSync_MatchOpenRouterModelIDConservatively(t *testing.T) {
	catalog := []model.Pricing{
		{ModelName: "gpt-4.1"},
		{ModelName: "claude-sonnet-4-5"},
		{ModelName: "gemini-2.5-pro"},
	}
	openRouter := map[string]CanonicalModelPrice{
		"openai/gpt-4.1":                  {Input: ptrFloat(2)},
		"anthropic/claude-sonnet-4.5":     {Input: ptrFloat(3)},
		"google/gemini-2.5-pro":           {Input: ptrFloat(4)},
		"anthropic/claude-opus-4.1-extra": {Input: ptrFloat(5)},
	}

	matched := MatchRequestedModelPrices([]string{
		"gpt-4.1",
		"claude-sonnet-4-5",
		"gemini-2.5-pro",
		"not-in-catalog",
	}, catalog, openRouter)

	if got := matched["gpt-4.1"].OpenRouterID; got != "openai/gpt-4.1" {
		t.Fatalf("gpt-4.1 matched %q", got)
	}
	if got := matched["claude-sonnet-4-5"].OpenRouterID; got != "anthropic/claude-sonnet-4.5" {
		t.Fatalf("claude-sonnet-4-5 matched %q", got)
	}
	if got := matched["gemini-2.5-pro"].OpenRouterID; got != "google/gemini-2.5-pro" {
		t.Fatalf("gemini-2.5-pro matched %q", got)
	}
	if _, ok := matched["not-in-catalog"]; ok {
		t.Fatalf("not-in-catalog should be skipped")
	}
}

func TestModelPriceSync_MergeUsesHigherPricePerDimensionAndBuildsExpr(t *testing.T) {
	official := CanonicalModelPrice{
		Input:      ptrFloat(3),
		Output:     ptrFloat(12),
		CacheRead:  ptrFloat(0.3),
		CacheWrite: ptrFloat(3.75),
	}
	openRouter := CanonicalModelPrice{
		Input:        ptrFloat(2.5),
		Output:       ptrFloat(15),
		CacheRead:    ptrFloat(0.2),
		CacheWrite1h: ptrFloat(6),
	}

	merged := MergeHigherPrices(official, openRouter)
	assertPrice(t, merged.Input, 3)
	assertPrice(t, merged.Output, 15)
	assertPrice(t, merged.CacheRead, 0.3)
	assertPrice(t, merged.CacheWrite, 3.75)
	assertPrice(t, merged.CacheWrite1h, 6)

	expr, err := BuildBillingExprFromPrice(merged)
	if err != nil {
		t.Fatalf("BuildBillingExprFromPrice returned error: %v", err)
	}
	wantParts := []string{"p * 3", "c * 15", "cr * 0.3", "cc * 3.75", "cc1h * 6"}
	for _, part := range wantParts {
		if !strings.Contains(expr, part) {
			t.Fatalf("expr %q does not contain %q", expr, part)
		}
	}
}

func TestModelPriceSync_BuildPreviewUsesOfficialPriceWhenOpenRouterMissing(t *testing.T) {
	result := buildModelPriceSyncPreview(
		[]string{"gpt-image-2"},
		[]model.Pricing{{ModelName: "gpt-image-2"}},
		map[string]CanonicalModelPrice{},
		map[string]CanonicalModelPrice{
			"gpt-image-2": {
				Input:     ptrFloat(5),
				Output:    ptrFloat(30),
				CacheRead: ptrFloat(1.25),
			},
		},
	)

	if result.Syncable != 1 {
		t.Fatalf("Syncable = %d, want 1; result = %#v", result.Syncable, result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Items length = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != "ready" {
		t.Fatalf("Status = %q, want ready; reason = %q", item.Status, item.Reason)
	}
	if item.OpenRouterID != "" {
		t.Fatalf("OpenRouterID = %q, want empty for official-only match", item.OpenRouterID)
	}
	assertPrice(t, item.Final.Input, 5)
	assertPrice(t, item.Final.Output, 30)
	assertPrice(t, item.Final.CacheRead, 1.25)
	if !strings.Contains(item.BillingExpr, "p * 5") || !strings.Contains(item.BillingExpr, "c * 30") {
		t.Fatalf("BillingExpr = %q, want official prices", item.BillingExpr)
	}
}

func TestModelPriceSync_ModelsDevParserMergesDuplicateOfficialPricesByHigherDimension(t *testing.T) {
	body := `{
		"anthropic": {
			"models": {
				"shared-model": {
					"cost": {
						"input": 2,
						"output": 8,
						"cache_read": 0.2
					}
				}
			}
		},
		"openai": {
			"models": {
				"shared-model": {
					"cost": {
						"input": 3,
						"output": 6,
						"cache_read": 0.3,
						"cache_write": 3.75
					}
				}
			}
		}
	}`

	prices, err := ParseModelsDevCanonicalPrices(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseModelsDevCanonicalPrices returned error: %v", err)
	}

	price := prices["shared-model"]
	assertPrice(t, price.Input, 3)
	assertPrice(t, price.Output, 8)
	assertPrice(t, price.CacheRead, 0.3)
	assertPrice(t, price.CacheWrite, 3.75)
}

func TestModelPriceSync_BuildBillingExprRequiresInputPrice(t *testing.T) {
	_, err := BuildBillingExprFromPrice(CanonicalModelPrice{Output: ptrFloat(15)})
	if err == nil {
		t.Fatal("BuildBillingExprFromPrice returned nil error for output-only price")
	}
	if !strings.Contains(err.Error(), "missing input price") {
		t.Fatalf("error = %q, want missing input price", err.Error())
	}
}

func TestModelPriceSync_OverridesCanMakeSkippedModelSyncable(t *testing.T) {
	input, output := 3.0, 15.0
	preview := ModelPriceSyncResult{Items: []ModelPriceSyncItem{{
		ModelName: "grok-3", Status: "skipped", Reason: "no_openrouter_match",
	}}, SkippedCount: 1}
	err := applyModelPriceSyncOverrides(&preview, map[string]CanonicalModelPrice{
		"grok-3": {Input: &input, Output: &output},
	})
	if err != nil {
		t.Fatalf("apply overrides: %v", err)
	}
	item := preview.Items[0]
	if !item.WouldApply || item.Status != "ready" || item.BillingExpr == "" {
		t.Fatalf("override was not made applyable: %+v", item)
	}
	if preview.Syncable != 1 || preview.SkippedCount != 0 {
		t.Fatalf("unexpected counts: %+v", preview)
	}
}

func assertPrice(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("price is nil, want %v", want)
	}
	if *got != want {
		t.Fatalf("price = %v, want %v", *got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type ratioJSONUpdates struct {
	modelPrice           string
	modelRatio           string
	completionRatio      string
	cacheRatio           string
	createCacheRatio     string
	imageRatio           string
	audioRatio           string
	audioCompletionRatio string
}

func snapshotRatioSettings(t *testing.T) func() {
	t.Helper()
	snapshot := ratioJSONUpdates{
		modelPrice:           ratio_setting.ModelPrice2JSONString(),
		modelRatio:           ratio_setting.ModelRatio2JSONString(),
		completionRatio:      ratio_setting.CompletionRatio2JSONString(),
		cacheRatio:           ratio_setting.CacheRatio2JSONString(),
		createCacheRatio:     ratio_setting.CreateCacheRatio2JSONString(),
		imageRatio:           ratio_setting.ImageRatio2JSONString(),
		audioRatio:           ratio_setting.AudioRatio2JSONString(),
		audioCompletionRatio: ratio_setting.AudioCompletionRatio2JSONString(),
	}
	return func() {
		mustUpdateRatioJSON(t, snapshot)
	}
}

func mustUpdateRatioJSON(t *testing.T, updates ratioJSONUpdates) {
	t.Helper()
	updateFns := []struct {
		name  string
		value string
		fn    func(string) error
	}{
		{"ModelPrice", updates.modelPrice, ratio_setting.UpdateModelPriceByJSONString},
		{"ModelRatio", updates.modelRatio, ratio_setting.UpdateModelRatioByJSONString},
		{"CompletionRatio", updates.completionRatio, ratio_setting.UpdateCompletionRatioByJSONString},
		{"CacheRatio", updates.cacheRatio, ratio_setting.UpdateCacheRatioByJSONString},
		{"CreateCacheRatio", updates.createCacheRatio, ratio_setting.UpdateCreateCacheRatioByJSONString},
		{"ImageRatio", updates.imageRatio, ratio_setting.UpdateImageRatioByJSONString},
		{"AudioRatio", updates.audioRatio, ratio_setting.UpdateAudioRatioByJSONString},
		{"AudioCompletionRatio", updates.audioCompletionRatio, ratio_setting.UpdateAudioCompletionRatioByJSONString},
	}
	for _, update := range updateFns {
		if update.value == "" {
			continue
		}
		if err := update.fn(update.value); err != nil {
			t.Fatalf("failed to update %s test map: %v", update.name, err)
		}
	}
}

func assertJSONMapValue(t *testing.T, raw string, key string, want float64) {
	t.Helper()
	values := decodeFloatMap(t, raw)
	got, ok := values[key]
	if !ok {
		t.Fatalf("map %s is missing key %q", raw, key)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("map[%q] = %v, want %v in %s", key, got, want, raw)
	}
}

func assertJSONMapMissing(t *testing.T, raw string, key string) {
	t.Helper()
	values := decodeFloatMap(t, raw)
	if got, ok := values[key]; ok {
		t.Fatalf("map[%q] = %v, want missing in %s", key, got, raw)
	}
}

func decodeFloatMap(t *testing.T, raw string) map[string]float64 {
	t.Helper()
	values := map[string]float64{}
	if err := common.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("failed to decode float map %q: %v", raw, err)
	}
	return values
}

func assertJSONStringMapValue(t *testing.T, raw string, key string, want string) string {
	t.Helper()
	values := map[string]string{}
	if err := common.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("failed to decode string map %q: %v", raw, err)
	}
	got, ok := values[key]
	if !ok {
		t.Fatalf("map %s is missing key %q", raw, key)
	}
	if got != want {
		t.Fatalf("map[%q] = %q, want %q in %s", key, got, want, raw)
	}
	return got
}

func TestModelPriceSync_OptionUpdatesClearStaleLegacyRatiosAndKeepCacheWrite1h(t *testing.T) {
	modelName := "sync-test-cache-legacy-cleanup"
	restoreRatioSettings := snapshotRatioSettings(t)
	defer restoreRatioSettings()

	mustUpdateRatioJSON(t, ratioJSONUpdates{
		modelPrice:           `{"` + modelName + `":9}`,
		modelRatio:           `{"` + modelName + `":1}`,
		completionRatio:      `{"` + modelName + `":2}`,
		cacheRatio:           `{"` + modelName + `":0.5}`,
		createCacheRatio:     `{"` + modelName + `":1.25}`,
		imageRatio:           `{"` + modelName + `":3}`,
		audioRatio:           `{"` + modelName + `":4}`,
		audioCompletionRatio: `{"` + modelName + `":5}`,
	})

	preview := ModelPriceSyncResult{Items: []ModelPriceSyncItem{{
		ModelName:   modelName,
		Final:       CanonicalModelPrice{Input: ptrFloat(3), Output: ptrFloat(15), CacheWrite1h: ptrFloat(6)},
		BillingExpr: `tier("base", p * 3 + c * 15 + cc1h * 6)`,
		WouldApply:  true,
	}}}

	updates, err := BuildModelPriceSyncOptionUpdates(preview)
	if err != nil {
		t.Fatalf("BuildModelPriceSyncOptionUpdates returned error: %v", err)
	}

	assertJSONMapMissing(t, updates["ModelPrice"], modelName)
	assertJSONMapValue(t, updates["ModelRatio"], modelName, 1.5)
	assertJSONMapValue(t, updates["CompletionRatio"], modelName, 5)
	assertJSONMapMissing(t, updates["CacheRatio"], modelName)
	assertJSONMapMissing(t, updates["CreateCacheRatio"], modelName)
	assertJSONMapMissing(t, updates["ImageRatio"], modelName)
	assertJSONMapMissing(t, updates["AudioRatio"], modelName)
	assertJSONMapMissing(t, updates["AudioCompletionRatio"], modelName)

	assertJSONStringMapValue(t, updates["billing_setting."+billing_setting.BillingModeField], modelName, billing_setting.BillingModeTieredExpr)
	expr := assertJSONStringMapValue(t, updates["billing_setting."+billing_setting.BillingExprField], modelName, preview.Items[0].BillingExpr)
	if !strings.Contains(expr, "cc1h * 6") {
		t.Fatalf("billing expr %q does not include cache write 1h price", expr)
	}
}
