package channelconsole

const (
	ProviderOpenRouter             = "openrouter"
	ProviderOpenAI                 = "openai"
	ProviderAnthropic              = "anthropic"
	ProviderGemini                 = "gemini"
	ProviderCustomOpenAICompatible = "custom_openai_compatible"

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
	Provider             string   `json:"provider"`
	ProviderLabel        string   `json:"provider_label"`
	ChannelType          int      `json:"channel_type"`
	BaseURL              string   `json:"base_url"`
	Keys                 []string `json:"-"`
	KeyPreviews          []string `json:"key_previews"`
	IsMultiKey           bool     `json:"is_multi_key"`
	MultiKeyMode         string   `json:"multi_key_mode"`
	ImportKind           string   `json:"import_kind"`
	PriceSource          string   `json:"price_source"`
	ModelDiscovery       string   `json:"model_discovery"`
	DefaultTestModel     string   `json:"default_test_model"`
	SuggestedName        string   `json:"suggested_name"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Warnings             []string `json:"warnings"`
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
