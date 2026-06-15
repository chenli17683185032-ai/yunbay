package channelconsole

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

const (
	defaultMultiKeyMode            = "polling"
	modelDiscoveryOpenAICompatible = "openai-compatible"
	modelDiscoveryProviderAPI      = "provider_api"
	noKeyWarning                   = "未识别到 API Key，请确认粘贴内容"
)

var (
	urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)
	keyPattern = regexp.MustCompile(`(?:sk-or-[A-Za-z0-9._-]+|sk-[A-Za-z0-9._-]+|AIza[A-Za-z0-9._-]+)`)
)

type providerDefaults struct {
	provider         string
	label            string
	channelType      int
	baseURL          string
	priceSource      string
	modelDiscovery   string
	defaultTestModel string
	requiresConfirm  bool
}

// PreviewImport parses pasted channel credentials into a safe, non-persistent import preview.
func PreviewImport(raw string) ImportPreview {
	importKind := detectImportKind(raw)
	baseURL := extractBaseURL(raw)
	keys := extractKeys(raw)
	defaults := detectProvider(raw, baseURL, keys)

	if baseURL == "" {
		baseURL = defaults.baseURL
	}

	keyPreviews := make([]string, 0, len(keys))
	for _, key := range keys {
		keyPreviews = append(keyPreviews, MaskCredential(key))
	}

	preview := ImportPreview{
		Provider:             defaults.provider,
		ProviderLabel:        defaults.label,
		ChannelType:          defaults.channelType,
		BaseURL:              baseURL,
		Keys:                 keys,
		KeyPreviews:          keyPreviews,
		IsMultiKey:           len(keys) > 1,
		MultiKeyMode:         defaultMultiKeyMode,
		ImportKind:           importKind,
		PriceSource:          defaults.priceSource,
		ModelDiscovery:       defaults.modelDiscovery,
		DefaultTestModel:     defaults.defaultTestModel,
		SuggestedName:        defaults.label + " API 池",
		RequiresConfirmation: defaults.requiresConfirm,
	}

	if len(keys) == 0 {
		preview.RequiresConfirmation = true
		preview.Warnings = append(preview.Warnings, noKeyWarning)
	}

	return preview
}

// MaskCredential returns a display-safe representation of a credential.
func MaskCredential(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	runes := []rune(key)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	if len(runes) <= 10 {
		return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
	}

	prefixLen := 6
	if strings.HasPrefix(key, "sk-or-") {
		prefixLen = 7
	}
	if len(runes) <= prefixLen+4 {
		prefixLen = 3
	}
	return string(runes[:prefixLen]) + strings.Repeat("*", 6) + string(runes[len(runes)-4:])
}

func detectImportKind(raw string) string {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasPrefix(lower, "curl "):
		return ImportKindCurl
	case strings.HasPrefix(trimmed, "{"):
		return ImportKindJSON
	case hasStructuredImportHint(lower):
		return ImportKindStructured
	default:
		return ImportKindKeyOnly
	}
}

func extractBaseURL(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		lowerLine := strings.ToLower(line)
		if !hasBaseURLHint(lowerLine) {
			continue
		}
		if match := urlPattern.FindString(line); match != "" {
			return normalizeBaseURL(match)
		}
	}

	match := urlPattern.FindString(raw)
	if match == "" {
		return ""
	}
	return normalizeBaseURL(match)
}

func normalizeBaseURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	trimmed = strings.TrimRight(trimmed, "\\.,);]}'\"")

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}

	schemeHost := parsed.Scheme + "://" + parsed.Host
	host := strings.ToLower(parsed.Hostname())
	path := strings.TrimRight(parsed.EscapedPath(), "/")

	switch {
	case host == "api.openai.com":
		return schemeHost
	case host == "openrouter.ai":
		return "https://openrouter.ai/api/v1"
	case strings.Contains(host, "anthropic.com"):
		return schemeHost
	case host == "generativelanguage.googleapis.com":
		return schemeHost
	}

	for _, endpoint := range []string{
		"/chat/completions",
		"/completions",
		"/embeddings",
		"/responses",
		"/messages",
	} {
		if strings.HasSuffix(path, endpoint) {
			basePath := strings.TrimRight(strings.TrimSuffix(path, endpoint), "/")
			if basePath == "" {
				return schemeHost
			}
			return schemeHost + basePath
		}
	}

	if path == "" {
		return schemeHost
	}
	return schemeHost + path
}

func extractKeys(raw string) []string {
	matches := keyPattern.FindAllString(raw, -1)
	keys := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		key := strings.TrimSpace(match)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func detectProvider(raw string, baseURL string, keys []string) providerDefaults {
	text := strings.ToLower(raw + " " + baseURL)

	switch {
	case strings.Contains(text, "openrouter.ai") || hasKeyPrefix(keys, "sk-or-"):
		return providerDefaults{
			provider:         ProviderOpenRouter,
			label:            "OpenRouter",
			channelType:      constant.ChannelTypeOpenRouter,
			baseURL:          "https://openrouter.ai/api/v1",
			priceSource:      PriceSourceOpenRouter,
			modelDiscovery:   modelDiscoveryProviderAPI,
			defaultTestModel: "openai/gpt-4o-mini",
		}
	case strings.Contains(text, "anthropic.com") || hasKeyPrefix(keys, "sk-ant-") || strings.Contains(text, "anthropic-version"):
		return providerDefaults{
			provider:         ProviderAnthropic,
			label:            "Anthropic",
			channelType:      constant.ChannelTypeAnthropic,
			baseURL:          "https://api.anthropic.com",
			priceSource:      PriceSourceAnthropic,
			modelDiscovery:   modelDiscoveryProviderAPI,
			defaultTestModel: "claude-3-5-haiku-20241022",
		}
	case strings.Contains(text, "generativelanguage.googleapis.com") || hasKeyPrefix(keys, "AIza"):
		return providerDefaults{
			provider:         ProviderGemini,
			label:            "Gemini",
			channelType:      constant.ChannelTypeGemini,
			baseURL:          "https://generativelanguage.googleapis.com",
			priceSource:      PriceSourceGemini,
			modelDiscovery:   modelDiscoveryProviderAPI,
			defaultTestModel: "gemini-1.5-flash",
		}
	case strings.Contains(text, "api.openai.com") || (baseURL == "" && hasGenericOpenAIKey(keys)):
		return providerDefaults{
			provider:         ProviderOpenAI,
			label:            "OpenAI",
			channelType:      constant.ChannelTypeOpenAI,
			baseURL:          "https://api.openai.com",
			priceSource:      PriceSourceOpenAI,
			modelDiscovery:   modelDiscoveryOpenAICompatible,
			defaultTestModel: "gpt-4o-mini",
		}
	default:
		return providerDefaults{
			provider:         ProviderCustomOpenAICompatible,
			label:            "OpenAI 兼容",
			channelType:      constant.ChannelTypeCustom,
			baseURL:          "https://api.openai.com/v1",
			priceSource:      PriceSourceManual,
			modelDiscovery:   modelDiscoveryOpenAICompatible,
			defaultTestModel: "gpt-4o-mini",
			requiresConfirm:  true,
		}
	}
}

func hasKeyPrefix(keys []string, prefix string) bool {
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func hasStructuredImportHint(lower string) bool {
	for _, hint := range []string{
		"base url",
		"baseurl",
		"base_url",
		"api base",
		"endpoint",
		"key:",
		"key =",
		"api_key",
		"api key",
		"api-key",
	} {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func hasBaseURLHint(lowerLine string) bool {
	for _, hint := range []string{
		"base url",
		"baseurl",
		"base_url",
		"api base",
		"endpoint",
	} {
		if strings.Contains(lowerLine, hint) {
			return true
		}
	}
	return false
}

func hasGenericOpenAIKey(keys []string) bool {
	for _, key := range keys {
		if strings.HasPrefix(key, "sk-") && !strings.HasPrefix(key, "sk-or-") && !strings.HasPrefix(key, "sk-ant-") {
			return true
		}
	}
	return false
}
