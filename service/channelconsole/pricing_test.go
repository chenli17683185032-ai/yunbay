package channelconsole

import (
	"math"
	"testing"
)

func floatPtr(v float64) *float64 { return &v }

func TestCompileTokenPriceToRatios(t *testing.T) {
	price := NormalizedModelPrice{
		ModelName:                  "example-model",
		InputUSDPer1MTokens:        floatPtr(2.0),
		OutputUSDPer1MTokens:       floatPtr(10.0),
		CachedInputUSDPer1MTokens:  floatPtr(0.5),
		CacheWrite5mUSDPer1MTokens: floatPtr(2.5),
		CacheWrite1hUSDPer1MTokens: floatPtr(6.0),
	}

	compiled := CompileTokenPrice(price, 1.2)

	if compiled.PriceStatus != PriceStatusSynced {
		t.Fatalf("status = %s", compiled.PriceStatus)
	}
	if !compiled.Enabled {
		t.Fatalf("compiled price should be enabled")
	}
	assertFloatPtr(t, "model ratio", compiled.ModelRatio, 1.2)
	assertFloatPtr(t, "completion ratio", compiled.CompletionRatio, 5.0)
	assertFloatPtr(t, "cache ratio", compiled.CacheRatio, 0.25)
	assertFloatPtr(t, "create cache ratio", compiled.CreateCacheRatio, 1.25)
	assertFloatPtr(t, "create cache 1h ratio", compiled.CreateCache1hRatio, 3.0)
}

func TestCompileTokenPriceDefaultsMarkupToOne(t *testing.T) {
	compiled := CompileTokenPrice(NormalizedModelPrice{
		ModelName:           "example-model",
		InputUSDPer1MTokens: floatPtr(2.0),
	}, 0)

	assertFloatPtr(t, "model ratio", compiled.ModelRatio, 1.0)
}

func TestCompileUnknownPrice(t *testing.T) {
	compiled := CompileTokenPrice(NormalizedModelPrice{ModelName: "unknown"}, 1.2)
	if compiled.PriceStatus != PriceStatusUnknown {
		t.Fatalf("status = %s", compiled.PriceStatus)
	}
	if compiled.Enabled {
		t.Fatalf("unknown price must not auto-enable")
	}
	if compiled.ModelRatio != nil || compiled.ModelPrice != nil {
		t.Fatalf("unknown price should not compile ratios or fixed price: %#v", compiled)
	}
}

func TestCompilePerCallPrice(t *testing.T) {
	compiled := CompileTokenPrice(NormalizedModelPrice{
		ModelName:         "image-model",
		RequestUSDPerCall: floatPtr(0.02),
	}, 1.2)

	if compiled.PriceStatus != PriceStatusSynced {
		t.Fatalf("status = %s", compiled.PriceStatus)
	}
	if !compiled.Enabled {
		t.Fatalf("per-call price should be enabled")
	}
	assertFloatPtr(t, "model price", compiled.ModelPrice, 0.024)
}

func TestCompileImageUnitPrice(t *testing.T) {
	compiled := CompileTokenPrice(NormalizedModelPrice{
		ModelName:       "image-model",
		ImageUSDPerUnit: floatPtr(0.03),
	}, 1.2)

	if compiled.PriceStatus != PriceStatusSynced {
		t.Fatalf("status = %s", compiled.PriceStatus)
	}
	if !compiled.Enabled {
		t.Fatalf("image price should be enabled")
	}
	assertFloatPtr(t, "model price", compiled.ModelPrice, 0.036)
	if compiled.ModelRatio != nil ||
		compiled.CompletionRatio != nil ||
		compiled.CacheRatio != nil ||
		compiled.CreateCacheRatio != nil ||
		compiled.CreateCache1hRatio != nil {
		t.Fatalf("image fixed price should not compile token ratios: %#v", compiled)
	}
}

func TestBuiltInOpenAIPriceTemplate(t *testing.T) {
	prices := BuiltInPrices(ProviderOpenAI)
	price, ok := prices["gpt-4o-mini"]
	if !ok {
		t.Fatalf("expected gpt-4o-mini price")
	}
	if price.Source != PriceSourceOpenAI {
		t.Fatalf("source = %s", price.Source)
	}
	assertFloatPtr(t, "input price", price.InputUSDPer1MTokens, 0.15)
	assertFloatPtr(t, "output price", price.OutputUSDPer1MTokens, 0.60)
}

func TestBuiltInPricesReturnsIndependentMaps(t *testing.T) {
	first := BuiltInPrices(ProviderOpenAI)
	delete(first, "gpt-4o-mini")

	second := BuiltInPrices(ProviderOpenAI)
	if _, ok := second["gpt-4o-mini"]; !ok {
		t.Fatalf("built-in prices should return independent maps")
	}
}

func assertFloatPtr(t *testing.T, name string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil {
		t.Fatalf("%s is nil", name)
	}
	if math.Abs(*actual-expected) > 1e-9 {
		t.Fatalf("%s = %.12f, want %.12f", name, *actual, expected)
	}
}
