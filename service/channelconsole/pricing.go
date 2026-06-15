package channelconsole

const (
	PriceStatusSynced  = "synced"
	PriceStatusUnknown = "price_unknown"
	PriceStatusManual  = "manual"

	newAPIUSDPer1MTokenRatioUnit = 2.0
)

type NormalizedModelPrice struct {
	ModelName                  string
	ProviderModelName          string
	Source                     string
	InputUSDPer1MTokens        *float64
	OutputUSDPer1MTokens       *float64
	CachedInputUSDPer1MTokens  *float64
	CacheWrite5mUSDPer1MTokens *float64
	CacheWrite1hUSDPer1MTokens *float64
	RequestUSDPerCall          *float64
	ImageUSDPerUnit            *float64
}

type CompiledPrice struct {
	ModelName          string
	ModelRatio         *float64
	CompletionRatio    *float64
	CacheRatio         *float64
	CreateCacheRatio   *float64
	CreateCache1hRatio *float64
	ModelPrice         *float64
	PriceStatus        string
	Enabled            bool
}

func CompileTokenPrice(price NormalizedModelPrice, markup float64) CompiledPrice {
	if markup <= 0 {
		markup = 1
	}

	compiled := CompiledPrice{
		ModelName:   price.ModelName,
		PriceStatus: PriceStatusUnknown,
		Enabled:     false,
	}

	if price.InputUSDPer1MTokens == nil || *price.InputUSDPer1MTokens <= 0 {
		if fixedPrice := firstPositivePrice(price.RequestUSDPerCall, price.ImageUSDPerUnit); fixedPrice != nil {
			value := *fixedPrice * markup
			compiled.ModelPrice = &value
			compiled.PriceStatus = PriceStatusSynced
			compiled.Enabled = true
		}
		return compiled
	}

	inputPrice := *price.InputUSDPer1MTokens
	modelRatio := inputPrice / newAPIUSDPer1MTokenRatioUnit * markup
	compiled.ModelRatio = &modelRatio
	compiled.CompletionRatio = ratioPtr(price.OutputUSDPer1MTokens, inputPrice)
	compiled.CacheRatio = ratioPtr(price.CachedInputUSDPer1MTokens, inputPrice)
	compiled.CreateCacheRatio = ratioPtr(price.CacheWrite5mUSDPer1MTokens, inputPrice)
	compiled.CreateCache1hRatio = ratioPtr(price.CacheWrite1hUSDPer1MTokens, inputPrice)
	compiled.PriceStatus = PriceStatusSynced
	compiled.Enabled = true
	return compiled
}

func BuiltInPrices(provider string) map[string]NormalizedModelPrice {
	switch provider {
	case ProviderOpenAI:
		return clonePrices(map[string]NormalizedModelPrice{
			"gpt-4o-mini": {
				ModelName:                 "gpt-4o-mini",
				Source:                    PriceSourceOpenAI,
				InputUSDPer1MTokens:       float64Pointer(0.15),
				OutputUSDPer1MTokens:      float64Pointer(0.60),
				CachedInputUSDPer1MTokens: float64Pointer(0.075),
			},
			"gpt-4o": {
				ModelName:                 "gpt-4o",
				Source:                    PriceSourceOpenAI,
				InputUSDPer1MTokens:       float64Pointer(2.50),
				OutputUSDPer1MTokens:      float64Pointer(10.00),
				CachedInputUSDPer1MTokens: float64Pointer(1.25),
			},
		})
	case ProviderAnthropic:
		return clonePrices(map[string]NormalizedModelPrice{
			"claude-3-5-haiku-20241022": {
				ModelName:                  "claude-3-5-haiku-20241022",
				Source:                     PriceSourceAnthropic,
				InputUSDPer1MTokens:        float64Pointer(0.80),
				OutputUSDPer1MTokens:       float64Pointer(4.00),
				CachedInputUSDPer1MTokens:  float64Pointer(0.08),
				CacheWrite5mUSDPer1MTokens: float64Pointer(1.00),
				CacheWrite1hUSDPer1MTokens: float64Pointer(1.60),
			},
			"claude-3-5-sonnet-20241022": {
				ModelName:                  "claude-3-5-sonnet-20241022",
				Source:                     PriceSourceAnthropic,
				InputUSDPer1MTokens:        float64Pointer(3.00),
				OutputUSDPer1MTokens:       float64Pointer(15.00),
				CachedInputUSDPer1MTokens:  float64Pointer(0.30),
				CacheWrite5mUSDPer1MTokens: float64Pointer(3.75),
				CacheWrite1hUSDPer1MTokens: float64Pointer(6.00),
			},
		})
	case ProviderGemini:
		return clonePrices(map[string]NormalizedModelPrice{
			"gemini-1.5-flash": {
				ModelName:            "gemini-1.5-flash",
				Source:               PriceSourceGemini,
				InputUSDPer1MTokens:  float64Pointer(0.075),
				OutputUSDPer1MTokens: float64Pointer(0.30),
			},
			"gemini-1.5-pro": {
				ModelName:            "gemini-1.5-pro",
				Source:               PriceSourceGemini,
				InputUSDPer1MTokens:  float64Pointer(1.25),
				OutputUSDPer1MTokens: float64Pointer(5.00),
			},
		})
	default:
		return map[string]NormalizedModelPrice{}
	}
}

func ratioPtr(value *float64, base float64) *float64 {
	if value == nil || *value < 0 || base <= 0 {
		return nil
	}
	ratio := *value / base
	return &ratio
}

func firstPositivePrice(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil && *value > 0 {
			return value
		}
	}
	return nil
}

func clonePrices(input map[string]NormalizedModelPrice) map[string]NormalizedModelPrice {
	output := make(map[string]NormalizedModelPrice, len(input))
	for key, price := range input {
		output[key] = clonePrice(price)
	}
	return output
}

func clonePrice(price NormalizedModelPrice) NormalizedModelPrice {
	price.InputUSDPer1MTokens = cloneFloat64(price.InputUSDPer1MTokens)
	price.OutputUSDPer1MTokens = cloneFloat64(price.OutputUSDPer1MTokens)
	price.CachedInputUSDPer1MTokens = cloneFloat64(price.CachedInputUSDPer1MTokens)
	price.CacheWrite5mUSDPer1MTokens = cloneFloat64(price.CacheWrite5mUSDPer1MTokens)
	price.CacheWrite1hUSDPer1MTokens = cloneFloat64(price.CacheWrite1hUSDPer1MTokens)
	price.RequestUSDPerCall = cloneFloat64(price.RequestUSDPerCall)
	price.ImageUSDPerUnit = cloneFloat64(price.ImageUSDPerUnit)
	return price
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func float64Pointer(value float64) *float64 {
	return &value
}
