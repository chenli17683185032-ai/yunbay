package channelconsole

import "testing"

func TestPreviewOpenRouterCurl(t *testing.T) {
	input := `curl https://openrouter.ai/api/v1/chat/completions -H "Authorization: Bearer sk-or-redacted"`
	preview := PreviewImport(input)
	if preview.Provider != ProviderOpenRouter {
		t.Fatalf("provider = %s", preview.Provider)
	}
	if preview.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base url = %s", preview.BaseURL)
	}
	if len(preview.Keys) != 1 {
		t.Fatalf("keys = %d", len(preview.Keys))
	}
	if preview.Keys[0] != "sk-or-redacted" {
		t.Fatalf("key not extracted")
	}
	if preview.PriceSource != PriceSourceOpenRouter {
		t.Fatalf("price source = %s", preview.PriceSource)
	}
}

func TestPreviewOpenAICurl(t *testing.T) {
	input := `curl https://api.openai.com/v1/chat/completions -H "Authorization: Bearer sk-redacted"`
	preview := PreviewImport(input)
	if preview.Provider != ProviderOpenAI {
		t.Fatalf("provider = %s", preview.Provider)
	}
	if preview.BaseURL != "https://api.openai.com" {
		t.Fatalf("base url = %s", preview.BaseURL)
	}
	if preview.ChannelType != 1 {
		t.Fatalf("channel type = %d", preview.ChannelType)
	}
}

func TestPreviewBaseURLAndMultipleKeys(t *testing.T) {
	input := "Base URL: https://gateway.example.com/v1\nKey: sk-one\nsk-two"
	preview := PreviewImport(input)
	if preview.Provider != ProviderCustomOpenAICompatible {
		t.Fatalf("provider = %s", preview.Provider)
	}
	if preview.BaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("base url = %s", preview.BaseURL)
	}
	if len(preview.Keys) != 2 {
		t.Fatalf("keys = %#v", preview.Keys)
	}
	if !preview.IsMultiKey {
		t.Fatalf("expected multi-key")
	}
}

func TestPreviewGeminiKey(t *testing.T) {
	input := "AIzaSyRedactedExample"
	preview := PreviewImport(input)
	if preview.Provider != ProviderGemini {
		t.Fatalf("provider = %s", preview.Provider)
	}
	if preview.ChannelType != 24 {
		t.Fatalf("channel type = %d", preview.ChannelType)
	}
}

func TestPreviewAnthropicCurlAndMaskedKeys(t *testing.T) {
	input := `curl https://api.anthropic.com/v1/messages -H "x-api-key: sk-ant-redacted"`
	preview := PreviewImport(input)
	if preview.Provider != ProviderAnthropic {
		t.Fatalf("provider = %s", preview.Provider)
	}
	if preview.BaseURL != "https://api.anthropic.com" {
		t.Fatalf("base url = %s", preview.BaseURL)
	}
	if len(preview.KeyPreviews) != 1 {
		t.Fatalf("key previews = %#v", preview.KeyPreviews)
	}
	if preview.KeyPreviews[0] == "sk-ant-redacted" {
		t.Fatalf("raw key leaked in preview")
	}
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
