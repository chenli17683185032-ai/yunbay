package channelconsole

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestPreviewOpenRouterCurl(t *testing.T) {
	input := `curl https://openrouter.ai/api/v1/chat/completions -H "Authorization: Bearer sk-or-redacted"`
	preview := PreviewImport(input)

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderOpenRouter,
		label:            "OpenRouter",
		channelType:      constant.ChannelTypeOpenRouter,
		baseURL:          "https://openrouter.ai/api/v1",
		priceSource:      PriceSourceOpenRouter,
		modelDiscovery:   modelDiscoveryProviderAPI,
		defaultTestModel: "openai/gpt-4o-mini",
		requiresConfirm:  false,
	})
	if preview.ImportKind != ImportKindCurl {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
	if len(preview.Keys) != 1 || preview.Keys[0] != "sk-or-redacted" {
		t.Fatalf("keys = %#v", preview.Keys)
	}
	if preview.SuggestedName != "OpenRouter API 池" {
		t.Fatalf("suggested name = %s", preview.SuggestedName)
	}
	if preview.MultiKeyMode != "polling" {
		t.Fatalf("multi key mode = %s", preview.MultiKeyMode)
	}
}

func TestPreviewOpenAICurl(t *testing.T) {
	input := `curl https://api.openai.com/v1/chat/completions -H "Authorization: Bearer sk-redacted"`
	preview := PreviewImport(input)

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderOpenAI,
		label:            "OpenAI",
		channelType:      constant.ChannelTypeOpenAI,
		baseURL:          "https://api.openai.com",
		priceSource:      PriceSourceOpenAI,
		modelDiscovery:   modelDiscoveryOpenAICompatible,
		defaultTestModel: "gpt-4o-mini",
		requiresConfirm:  false,
	})
	if preview.ImportKind != ImportKindCurl {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
}

func TestPreviewOpenAIKeyOnlyDefaults(t *testing.T) {
	preview := PreviewImport("sk-redacted-example")

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderOpenAI,
		label:            "OpenAI",
		channelType:      constant.ChannelTypeOpenAI,
		baseURL:          "https://api.openai.com",
		priceSource:      PriceSourceOpenAI,
		modelDiscovery:   modelDiscoveryOpenAICompatible,
		defaultTestModel: "gpt-4o-mini",
		requiresConfirm:  false,
	})
	if preview.ImportKind != ImportKindKeyOnly {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
}

func TestPreviewBaseURLAndMultipleKeys(t *testing.T) {
	input := "Base URL: https://gateway.example.com/v1\nKey: sk-one\nsk-two"
	preview := PreviewImport(input)

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderCustomOpenAICompatible,
		label:            "OpenAI 兼容",
		channelType:      constant.ChannelTypeCustom,
		baseURL:          "https://gateway.example.com/v1",
		priceSource:      PriceSourceManual,
		modelDiscovery:   modelDiscoveryOpenAICompatible,
		defaultTestModel: "gpt-4o-mini",
		requiresConfirm:  true,
	})
	if preview.ImportKind != ImportKindStructured {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
	if len(preview.Keys) != 2 || preview.Keys[0] != "sk-one" || preview.Keys[1] != "sk-two" {
		t.Fatalf("keys = %#v", preview.Keys)
	}
	if !preview.IsMultiKey {
		t.Fatalf("expected multi-key")
	}
	if preview.MultiKeyMode != "polling" {
		t.Fatalf("multi key mode = %s", preview.MultiKeyMode)
	}
}

func TestPreviewStructuredTextAliases(t *testing.T) {
	cases := []string{
		"BaseURL: https://gateway.example.com/v1\napi_key = sk-one",
		"base_url: https://gateway.example.com/v1\napi key: sk-one",
		"api base: https://gateway.example.com/v1\nkey = sk-one",
		"endpoint: https://gateway.example.com/v1/chat/completions\nkey: sk-one",
	}

	for _, input := range cases {
		preview := PreviewImport(input)
		if preview.ImportKind != ImportKindStructured {
			t.Fatalf("%q import kind = %s", input, preview.ImportKind)
		}
		if preview.BaseURL != "https://gateway.example.com/v1" {
			t.Fatalf("%q base url = %s", input, preview.BaseURL)
		}
	}
}

func TestPreviewJSONImportKind(t *testing.T) {
	preview := PreviewImport(`{"base_url":"https://api.openai.com/v1/chat/completions","api_key":"sk-redacted"}`)
	if preview.ImportKind != ImportKindJSON {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
	if preview.Provider != ProviderOpenAI {
		t.Fatalf("provider = %s", preview.Provider)
	}
}

func TestPreviewGeminiKey(t *testing.T) {
	input := "AIzaSyRedactedExample"
	preview := PreviewImport(input)

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderGemini,
		label:            "Gemini",
		channelType:      constant.ChannelTypeGemini,
		baseURL:          "https://generativelanguage.googleapis.com",
		priceSource:      PriceSourceGemini,
		modelDiscovery:   modelDiscoveryProviderAPI,
		defaultTestModel: "gemini-1.5-flash",
		requiresConfirm:  false,
	})
	if preview.ImportKind != ImportKindKeyOnly {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
}

func TestPreviewAnthropicCurlAndMaskedKeys(t *testing.T) {
	input := `curl https://api.anthropic.com/v1/messages -H "x-api-key: sk-ant-redacted"`
	preview := PreviewImport(input)

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderAnthropic,
		label:            "Anthropic",
		channelType:      constant.ChannelTypeAnthropic,
		baseURL:          "https://api.anthropic.com",
		priceSource:      PriceSourceAnthropic,
		modelDiscovery:   modelDiscoveryProviderAPI,
		defaultTestModel: "claude-3-5-haiku-20241022",
		requiresConfirm:  false,
	})
	if len(preview.KeyPreviews) != 1 {
		t.Fatalf("key previews = %#v", preview.KeyPreviews)
	}
	if preview.KeyPreviews[0] == "sk-ant-redacted" {
		t.Fatalf("raw key leaked in preview")
	}
}

func TestPreviewAnthropicKeyOnlyDefaults(t *testing.T) {
	preview := PreviewImport("sk-ant-redacted")

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderAnthropic,
		label:            "Anthropic",
		channelType:      constant.ChannelTypeAnthropic,
		baseURL:          "https://api.anthropic.com",
		priceSource:      PriceSourceAnthropic,
		modelDiscovery:   modelDiscoveryProviderAPI,
		defaultTestModel: "claude-3-5-haiku-20241022",
		requiresConfirm:  false,
	})
	if preview.ImportKind != ImportKindKeyOnly {
		t.Fatalf("import kind = %s", preview.ImportKind)
	}
}

func TestPreviewAnthropicHeaderDefaults(t *testing.T) {
	input := "x-api-key: sk-redacted\nanthropic-version: 2023-06-01"
	preview := PreviewImport(input)

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderAnthropic,
		label:            "Anthropic",
		channelType:      constant.ChannelTypeAnthropic,
		baseURL:          "https://api.anthropic.com",
		priceSource:      PriceSourceAnthropic,
		modelDiscovery:   modelDiscoveryProviderAPI,
		defaultTestModel: "claude-3-5-haiku-20241022",
		requiresConfirm:  false,
	})
}

func TestPreviewCustomDefaultBaseRequiresConfirmation(t *testing.T) {
	preview := PreviewImport("OpenAI compatible provider")

	assertProviderDefaults(t, preview, providerExpectation{
		provider:         ProviderCustomOpenAICompatible,
		label:            "OpenAI 兼容",
		channelType:      constant.ChannelTypeCustom,
		baseURL:          "https://api.openai.com/v1",
		priceSource:      PriceSourceManual,
		modelDiscovery:   modelDiscoveryOpenAICompatible,
		defaultTestModel: "gpt-4o-mini",
		requiresConfirm:  true,
	})
}

func TestPreviewNoKeyWarning(t *testing.T) {
	preview := PreviewImport("Base URL: https://gateway.example.com/v1")
	if !preview.RequiresConfirmation {
		t.Fatalf("expected confirmation")
	}
	if len(preview.Warnings) != 1 || preview.Warnings[0] != "未识别到 API Key，请确认粘贴内容" {
		t.Fatalf("warnings = %#v", preview.Warnings)
	}
}

func TestPreviewKeyPreviewsAreMaskedAndJSONOmitsRawKeys(t *testing.T) {
	preview := PreviewImport("sk-redacted-example")
	if len(preview.KeyPreviews) != 1 {
		t.Fatalf("key previews = %#v", preview.KeyPreviews)
	}
	if preview.KeyPreviews[0] == preview.Keys[0] {
		t.Fatalf("raw key leaked in key preview")
	}

	body, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	if strings.Contains(string(body), preview.Keys[0]) {
		t.Fatalf("raw key leaked in json: %s", body)
	}
}

type providerExpectation struct {
	provider         string
	label            string
	channelType      int
	baseURL          string
	priceSource      string
	modelDiscovery   string
	defaultTestModel string
	requiresConfirm  bool
}

func assertProviderDefaults(t *testing.T, preview ImportPreview, expected providerExpectation) {
	t.Helper()
	if preview.Provider != expected.provider {
		t.Fatalf("provider = %s", preview.Provider)
	}
	if preview.ProviderLabel != expected.label {
		t.Fatalf("provider label = %s", preview.ProviderLabel)
	}
	if preview.ChannelType != expected.channelType {
		t.Fatalf("channel type = %d", preview.ChannelType)
	}
	if preview.BaseURL != expected.baseURL {
		t.Fatalf("base url = %s", preview.BaseURL)
	}
	if preview.PriceSource != expected.priceSource {
		t.Fatalf("price source = %s", preview.PriceSource)
	}
	if preview.ModelDiscovery != expected.modelDiscovery {
		t.Fatalf("model discovery = %s", preview.ModelDiscovery)
	}
	if preview.DefaultTestModel != expected.defaultTestModel {
		t.Fatalf("default test model = %s", preview.DefaultTestModel)
	}
	if preview.RequiresConfirmation != expected.requiresConfirm {
		t.Fatalf("requires confirmation = %t", preview.RequiresConfirmation)
	}
}
