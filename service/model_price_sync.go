package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	openRouterModelsPath         = "/v1/models"
	openRouterPublicModelsURL    = "https://openrouter.ai/api" + openRouterModelsPath
	modelPriceSyncMaxBodyBytes   = 10 << 20
	modelPriceSyncRequestTimeout = 20 * time.Second
)

var directModelsDevProviders = map[string]bool{
	"anthropic":  true,
	"cohere":     true,
	"deepseek":   true,
	"google":     true,
	"mistral":    true,
	"openai":     true,
	"perplexity": true,
	"xai":        true,
}

// These aliases mirror the xAI model mapping used by the imported Grok
// upstream. Keys use canonicalModelID format; values are source catalog IDs.
var modelPriceAliasTargets = map[string]string{
	"grok":                    "grok-4.5",
	"grok-latest":             "grok-4.5",
	"grok-4-5-latest":         "grok-4.5",
	"grok-build":              "grok-build-0.1",
	"grok-build-latest":       "grok-4.5",
	"grok-composer":           "grok-composer-2.5-fast",
	"composer-2-5":            "grok-composer-2.5-fast",
	"grok-4-20-reasoning":     "grok-4.20-0309-reasoning",
	"grok-4-20-non-reasoning": "grok-4.20-0309-non-reasoning",
}

var curatedModelPrices = map[string]CanonicalModelPrice{
	"grok-composer-2.5-fast": {
		Input:     ptrFloat(2),
		Output:    ptrFloat(4),
		CacheRead: ptrFloat(0.4),
	},
}

var modelPriceSyncProtectedModels = map[string]string{
	"grok-build":                 "manual_price_protected",
	"grok-imagine":               "media_unit_pricing",
	"grok-imagine-edit":          "media_unit_pricing",
	"grok-imagine-image":         "media_unit_pricing",
	"grok-imagine-image-quality": "media_unit_pricing",
	"grok-imagine-video":         "media_unit_pricing",
	"grok-imagine-video-1-5":     "media_unit_pricing",
}

// CanonicalModelPrice stores real USD / 1M token prices. OpenRouter's token
// prices are normalized to this unit before any comparison or expression build.
type CanonicalModelPrice struct {
	Input        *float64 `json:"input,omitempty"`
	Output       *float64 `json:"output,omitempty"`
	CacheRead    *float64 `json:"cache_read,omitempty"`
	CacheWrite   *float64 `json:"cache_write,omitempty"`
	CacheWrite1h *float64 `json:"cache_write_1h,omitempty"`
	ImageInput   *float64 `json:"image_input,omitempty"`
	AudioInput   *float64 `json:"audio_input,omitempty"`
	AudioOutput  *float64 `json:"audio_output,omitempty"`
	Reasoning    *float64 `json:"reasoning,omitempty"`
	WebSearch    *float64 `json:"web_search,omitempty"`
}

type ModelPriceSourceChoice struct {
	Dimension string   `json:"dimension"`
	Source    string   `json:"source"`
	Value     *float64 `json:"value,omitempty"`
}

type ModelPriceMatch struct {
	ModelName    string              `json:"model_name"`
	OpenRouterID string              `json:"openrouter_id,omitempty"`
	Price        CanonicalModelPrice `json:"price"`
	Status       string              `json:"status"`
	Reason       string              `json:"reason,omitempty"`
}

type ModelPriceSyncRequest struct {
	OpenRouterChannelID int                            `json:"openrouter_channel_id"`
	Models              []string                       `json:"models"`
	Overrides           map[string]CanonicalModelPrice `json:"overrides,omitempty"`
}

type ModelPriceSyncItem struct {
	ModelName     string                   `json:"model_name"`
	OpenRouterID  string                   `json:"openrouter_id,omitempty"`
	Current       CanonicalModelPrice      `json:"current"`
	Official      CanonicalModelPrice      `json:"official"`
	OpenRouter    CanonicalModelPrice      `json:"openrouter"`
	Final         CanonicalModelPrice      `json:"final"`
	BillingExpr   string                   `json:"billing_expr,omitempty"`
	Status        string                   `json:"status"`
	Reason        string                   `json:"reason,omitempty"`
	SourceChoices []ModelPriceSourceChoice `json:"source_choices,omitempty"`
	WouldApply    bool                     `json:"would_apply"`
	Applied       bool                     `json:"applied,omitempty"`
	Changed       bool                     `json:"changed"`
}

type ModelPriceSyncResult struct {
	Items        []ModelPriceSyncItem `json:"items"`
	Requested    int                  `json:"requested"`
	Syncable     int                  `json:"syncable"`
	AppliedCount int                  `json:"applied_count,omitempty"`
	SkippedCount int                  `json:"skipped_count"`
}

type openRouterModelPriceFetcher interface {
	FetchOpenRouterModelPrices(ctx context.Context, channelID int) (map[string]CanonicalModelPrice, error)
}

type officialModelPriceProvider interface {
	OfficialModelPrices(ctx context.Context) map[string]CanonicalModelPrice
}

type defaultOpenRouterModelPriceFetcher struct{}
type defaultOfficialModelPriceProvider struct{}

func ptrFloat(v float64) *float64 { return &v }

func ParseOpenRouterModelPrices(reader io.Reader) (map[string]CanonicalModelPrice, error) {
	var orResp struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt            string `json:"prompt"`
				Completion        string `json:"completion"`
				InputCacheRead    string `json:"input_cache_read"`
				InputCacheWrite   string `json:"input_cache_write"`
				InputCacheWrite1h string `json:"input_cache_write_1h"`
				Image             string `json:"image"`
				Audio             string `json:"audio"`
				InternalReasoning string `json:"internal_reasoning"`
				WebSearch         string `json:"web_search"`
			} `json:"pricing"`
		} `json:"data"`
	}

	if err := common.DecodeJson(reader, &orResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenRouter response: %w", err)
	}

	prices := make(map[string]CanonicalModelPrice, len(orResp.Data))
	for _, item := range orResp.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		price := CanonicalModelPrice{
			Input:        parseOpenRouterTokenPrice(item.Pricing.Prompt),
			Output:       parseOpenRouterTokenPrice(item.Pricing.Completion),
			CacheRead:    parseOpenRouterTokenPrice(item.Pricing.InputCacheRead),
			CacheWrite:   parseOpenRouterTokenPrice(item.Pricing.InputCacheWrite),
			CacheWrite1h: parseOpenRouterTokenPrice(item.Pricing.InputCacheWrite1h),
			ImageInput:   parseOpenRouterTokenPrice(item.Pricing.Image),
			AudioInput:   parseOpenRouterTokenPrice(item.Pricing.Audio),
			Reasoning:    parseOpenRouterTokenPrice(item.Pricing.InternalReasoning),
			WebSearch:    parseOpenRouterRawPrice(item.Pricing.WebSearch),
		}
		if !price.hasAnyBillablePrice() && price.WebSearch == nil {
			continue
		}
		prices[id] = price
	}
	return prices, nil
}

func parseOpenRouterTokenPrice(raw string) *float64 {
	price := parseOpenRouterRawPrice(raw)
	if price == nil {
		return nil
	}
	value := *price * 1_000_000
	return &value
}

func parseOpenRouterRawPrice(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !isFiniteNonNegative(value) {
		return nil
	}
	return &value
}

func (price CanonicalModelPrice) hasAnyBillablePrice() bool {
	for _, value := range price.dimensionPtrs() {
		if value != nil {
			return true
		}
	}
	return false
}

func (price CanonicalModelPrice) dimensionPtrs() []*float64 {
	return []*float64{
		price.Input,
		price.Output,
		price.CacheRead,
		price.CacheWrite,
		price.CacheWrite1h,
		price.ImageInput,
		price.AudioInput,
		price.AudioOutput,
		price.Reasoning,
	}
}

func isFiniteNonNegative(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}

func MatchRequestedModelPrices(requested []string, catalog []model.Pricing, openRouterPrices map[string]CanonicalModelPrice) map[string]ModelPriceMatch {
	catalogSet := make(map[string]struct{}, len(catalog))
	for _, item := range catalog {
		name := strings.TrimSpace(item.ModelName)
		if name != "" {
			catalogSet[name] = struct{}{}
		}
	}

	openRouterIDs := make([]string, 0, len(openRouterPrices))
	for id := range openRouterPrices {
		openRouterIDs = append(openRouterIDs, id)
	}
	sort.Strings(openRouterIDs)

	matches := make(map[string]ModelPriceMatch)
	seen := make(map[string]struct{}, len(requested))
	for _, rawModel := range requested {
		modelName := strings.TrimSpace(rawModel)
		if modelName == "" {
			continue
		}
		if _, duplicate := seen[modelName]; duplicate {
			continue
		}
		seen[modelName] = struct{}{}
		if _, ok := catalogSet[modelName]; !ok {
			continue
		}

		if price, ok := openRouterPrices[modelName]; ok {
			matches[modelName] = ModelPriceMatch{ModelName: modelName, OpenRouterID: modelName, Price: price, Status: "matched"}
			continue
		}

		canonicalLocal := canonicalModelID(modelName)
		candidateID, candidatePrice, candidateCount := findCanonicalPriceCandidates(canonicalLocal, openRouterIDs, openRouterPrices)
		if candidateCount == 0 {
			if aliasTarget, ok := modelPriceAliasTargets[canonicalLocal]; ok {
				candidateID, candidatePrice, candidateCount = findPriceAliasTarget(aliasTarget, openRouterIDs, openRouterPrices)
			}
		}
		if candidateCount == 1 {
			matches[modelName] = ModelPriceMatch{ModelName: modelName, OpenRouterID: candidateID, Price: candidatePrice, Status: "matched"}
		} else if candidateCount > 1 {
			matches[modelName] = ModelPriceMatch{ModelName: modelName, Status: "skipped", Reason: "multiple_openrouter_matches"}
		} else {
			matches[modelName] = ModelPriceMatch{ModelName: modelName, Status: "skipped", Reason: "no_openrouter_match"}
		}
	}
	return matches
}

func canonicalModelID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if slash := strings.LastIndex(id, "/"); slash >= 0 && slash < len(id)-1 {
		id = id[slash+1:]
	}
	id = strings.ReplaceAll(id, ".", "-")
	id = strings.ReplaceAll(id, "_", "-")
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	return strings.Trim(id, "-")
}

func findCanonicalPriceCandidates(canonical string, ids []string, prices map[string]CanonicalModelPrice) (string, CanonicalModelPrice, int) {
	var matchedID string
	var matchedPrice CanonicalModelPrice
	count := 0
	for _, id := range ids {
		if canonicalModelID(id) != canonical {
			continue
		}
		matchedID = id
		matchedPrice = prices[id]
		count++
	}
	return matchedID, matchedPrice, count
}

func findPriceAliasTarget(target string, ids []string, prices map[string]CanonicalModelPrice) (string, CanonicalModelPrice, int) {
	if price, ok := prices[target]; ok {
		return target, price, 1
	}
	return findCanonicalPriceCandidates(canonicalModelID(target), ids, prices)
}

func FindCanonicalPriceForModel(modelName string, prices map[string]CanonicalModelPrice) CanonicalModelPrice {
	if price, ok := prices[modelName]; ok {
		return price
	}
	canonical := canonicalModelID(modelName)
	ids := make([]string, 0, len(prices))
	for id := range prices {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	_, matched, count := findCanonicalPriceCandidates(canonical, ids, prices)
	if count == 1 {
		return matched
	}
	if count == 0 {
		if aliasTarget, ok := modelPriceAliasTargets[canonical]; ok {
			_, matched, count = findPriceAliasTarget(aliasTarget, ids, prices)
			if count == 1 {
				return matched
			}
		}
	}
	return CanonicalModelPrice{}
}

func MergeHigherPrices(official CanonicalModelPrice, openRouter CanonicalModelPrice) CanonicalModelPrice {
	return CanonicalModelPrice{
		Input:        maxPricePtr(official.Input, openRouter.Input),
		Output:       maxPricePtr(official.Output, openRouter.Output),
		CacheRead:    maxPricePtr(official.CacheRead, openRouter.CacheRead),
		CacheWrite:   maxPricePtr(official.CacheWrite, openRouter.CacheWrite),
		CacheWrite1h: maxPricePtr(official.CacheWrite1h, openRouter.CacheWrite1h),
		ImageInput:   maxPricePtr(official.ImageInput, openRouter.ImageInput),
		AudioInput:   maxPricePtr(official.AudioInput, openRouter.AudioInput),
		AudioOutput:  maxPricePtr(official.AudioOutput, openRouter.AudioOutput),
		Reasoning:    maxPricePtr(official.Reasoning, openRouter.Reasoning),
		WebSearch:    maxPricePtr(official.WebSearch, openRouter.WebSearch),
	}
}

func maxPricePtr(a, b *float64) *float64 {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return ptrFloat(*b)
	}
	if b == nil {
		return ptrFloat(*a)
	}
	if *b > *a {
		return ptrFloat(*b)
	}
	return ptrFloat(*a)
}

func BuildBillingExprFromPrice(price CanonicalModelPrice) (string, error) {
	if price.Input == nil {
		return "", fmt.Errorf("missing input price")
	}

	parts := make([]string, 0, 8)
	addTerm := func(variable string, value *float64) {
		if value == nil {
			return
		}
		parts = append(parts, fmt.Sprintf("%s * %s", variable, formatPriceCoefficient(*value)))
	}
	addTerm("p", price.Input)
	addTerm("c", price.Output)
	addTerm("cr", price.CacheRead)
	addTerm("cc", price.CacheWrite)
	addTerm("cc1h", price.CacheWrite1h)
	addTerm("img", price.ImageInput)
	addTerm("ai", price.AudioInput)
	addTerm("ao", price.AudioOutput)

	if len(parts) == 0 {
		return "", fmt.Errorf("no supported billable dimensions")
	}

	expr := "tier(\"base\", " + strings.Join(parts, " + ") + ")"
	if _, err := billingexpr.CompileFromCache(expr); err != nil {
		return "", err
	}
	if err := billing_setting.SmokeTestExpr(expr); err != nil {
		return "", err
	}
	return expr, nil
}

func formatPriceCoefficient(v float64) string {
	return strconv.FormatFloat(roundPrice(v), 'f', -1, 64)
}

func roundPrice(v float64) float64 {
	return math.Round(v*1e9) / 1e9
}

func CurrentCanonicalPriceFromPricing(pricing model.Pricing) CanonicalModelPrice {
	var input *float64
	var output *float64
	var cacheRead *float64
	var cacheWrite *float64
	var imageInput *float64
	var audioInput *float64
	var audioOutput *float64

	if pricing.QuotaType == 1 {
		// Per-request models do not map cleanly to token prices.
		return CanonicalModelPrice{}
	}

	if pricing.ModelRatio > 0 {
		inputPrice := pricing.ModelRatio * 1_000_000 / common.QuotaPerUnit
		input = ptrFloat(roundPrice(inputPrice))
		output = ptrFloat(roundPrice(inputPrice * pricing.CompletionRatio))
		if pricing.CacheRatio != nil {
			cacheRead = ptrFloat(roundPrice(inputPrice * *pricing.CacheRatio))
		}
		if pricing.CreateCacheRatio != nil {
			cacheWrite = ptrFloat(roundPrice(inputPrice * *pricing.CreateCacheRatio))
		}
		if pricing.ImageRatio != nil {
			imageInput = ptrFloat(roundPrice(inputPrice * *pricing.ImageRatio))
		}
		if pricing.AudioRatio != nil {
			audioInput = ptrFloat(roundPrice(inputPrice * *pricing.AudioRatio))
		}
		if pricing.AudioCompletionRatio != nil {
			audioOutput = ptrFloat(roundPrice(inputPrice * *pricing.AudioCompletionRatio))
		}
	}

	return CanonicalModelPrice{
		Input:       input,
		Output:      output,
		CacheRead:   cacheRead,
		CacheWrite:  cacheWrite,
		ImageInput:  imageInput,
		AudioInput:  audioInput,
		AudioOutput: audioOutput,
	}
}

func OfficialModelPricesFromPricingData(pricings []model.Pricing) map[string]CanonicalModelPrice {
	prices := make(map[string]CanonicalModelPrice, len(pricings))
	for _, item := range pricings {
		prices[item.ModelName] = CurrentCanonicalPriceFromPricing(item)
	}
	return prices
}

type modelsDevCanonicalCost struct {
	Input           *float64                `json:"input"`
	Output          *float64                `json:"output"`
	CacheRead       *float64                `json:"cache_read"`
	CacheWrite      *float64                `json:"cache_write"`
	InputAudio      *float64                `json:"input_audio"`
	OutputAudio     *float64                `json:"output_audio"`
	Reasoning       *float64                `json:"reasoning"`
	ContextOver200k *modelsDevCanonicalCost `json:"context_over_200k"`
}

func canonicalPriceFromModelsDevCost(cost modelsDevCanonicalCost) CanonicalModelPrice {
	price := CanonicalModelPrice{
		Input:       cloneValidPrice(cost.Input),
		Output:      cloneValidPrice(cost.Output),
		CacheRead:   cloneValidPrice(cost.CacheRead),
		CacheWrite:  cloneValidPrice(cost.CacheWrite),
		AudioInput:  cloneValidPrice(cost.InputAudio),
		AudioOutput: cloneValidPrice(cost.OutputAudio),
		Reasoning:   cloneValidPrice(cost.Reasoning),
	}
	if cost.ContextOver200k != nil {
		price = MergeHigherPrices(price, canonicalPriceFromModelsDevCost(*cost.ContextOver200k))
	}
	return price
}

func ParseModelsDevCanonicalPrices(reader io.Reader) (map[string]CanonicalModelPrice, error) {
	var upstreamData map[string]struct {
		Models map[string]struct {
			Cost modelsDevCanonicalCost `json:"cost"`
		} `json:"models"`
	}
	if err := common.DecodeJson(reader, &upstreamData); err != nil {
		return nil, fmt.Errorf("failed to decode models.dev response: %w", err)
	}
	if len(upstreamData) == 0 {
		return nil, fmt.Errorf("empty models.dev response")
	}

	providers := make([]string, 0, len(upstreamData))
	for provider := range upstreamData {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	prices := make(map[string]CanonicalModelPrice)
	for _, provider := range providers {
		if !directModelsDevProviders[provider] {
			continue
		}
		providerData := upstreamData[provider]
		modelNames := make([]string, 0, len(providerData.Models))
		for modelName := range providerData.Models {
			modelNames = append(modelNames, modelName)
		}
		sort.Strings(modelNames)
		for _, modelName := range modelNames {
			cost := providerData.Models[modelName].Cost
			price := canonicalPriceFromModelsDevCost(cost)
			if !price.hasAnyBillablePrice() {
				continue
			}
			keys := []string{modelName}
			if !strings.Contains(modelName, "/") {
				keys = append(keys, provider+"/"+modelName)
			}
			for _, key := range keys {
				current, exists := prices[key]
				if !exists {
					prices[key] = price
				} else {
					prices[key] = MergeHigherPrices(current, price)
				}
			}
		}
	}
	return prices, nil
}

func cloneValidPrice(value *float64) *float64 {
	if value == nil || !isFiniteNonNegative(*value) {
		return nil
	}
	return ptrFloat(*value)
}

func (defaultOfficialModelPriceProvider) OfficialModelPrices(ctx context.Context) map[string]CanonicalModelPrice {
	requestCtx, cancel := context.WithTimeout(ctx, modelPriceSyncRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "https://models.dev/api.json", nil)
	if err == nil {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "new-api/price-sync")
		client := &http.Client{Timeout: modelPriceSyncRequestTimeout}
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if prices, err := ParseModelsDevCanonicalPrices(io.LimitReader(resp.Body, modelPriceSyncMaxBodyBytes)); err == nil && len(prices) > 0 {
					return prices
				}
			}
		}
	}
	return OfficialModelPricesFromPricingData(model.GetPricing())
}

func (defaultOpenRouterModelPriceFetcher) FetchOpenRouterModelPrices(ctx context.Context, channelID int) (map[string]CanonicalModelPrice, error) {
	if channelID <= 0 {
		return fetchOpenRouterModelPrices(ctx, openRouterPublicModelsURL, "")
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenRouter channel: %w", err)
	}
	if channel.Type != constant.ChannelTypeOpenRouter {
		return nil, fmt.Errorf("selected channel is not OpenRouter")
	}
	key, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return nil, fmt.Errorf("failed to get enabled OpenRouter key: %w", apiErr)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("OpenRouter channel has no API key")
	}

	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("OpenRouter channel has no base URL")
	}

	return fetchOpenRouterModelPrices(ctx, baseURL+openRouterModelsPath, key)
}

func fetchOpenRouterModelPrices(ctx context.Context, url string, key string) (map[string]CanonicalModelPrice, error) {
	requestCtx, cancel := context.WithTimeout(ctx, modelPriceSyncRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api/price-sync")

	client := &http.Client{Timeout: modelPriceSyncRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter returned %s", resp.Status)
	}
	return ParseOpenRouterModelPrices(io.LimitReader(resp.Body, modelPriceSyncMaxBodyBytes))
}

func PreviewSelectedModelPriceSync(ctx context.Context, req ModelPriceSyncRequest) (ModelPriceSyncResult, error) {
	return previewSelectedModelPriceSync(ctx, req, defaultOpenRouterModelPriceFetcher{}, defaultOfficialModelPriceProvider{})
}

func previewSelectedModelPriceSync(ctx context.Context, req ModelPriceSyncRequest, fetcher openRouterModelPriceFetcher, officialProvider officialModelPriceProvider) (ModelPriceSyncResult, error) {
	catalog := model.GetPricing()
	openRouterPrices, err := fetcher.FetchOpenRouterModelPrices(ctx, req.OpenRouterChannelID)
	if err != nil {
		return ModelPriceSyncResult{}, err
	}
	officialPrices := officialProvider.OfficialModelPrices(ctx)
	return buildModelPriceSyncPreview(req.Models, catalog, openRouterPrices, officialPrices), nil
}

func buildModelPriceSyncPreview(requested []string, catalog []model.Pricing, openRouterPrices map[string]CanonicalModelPrice, officialPrices map[string]CanonicalModelPrice) ModelPriceSyncResult {
	catalogByName := make(map[string]model.Pricing, len(catalog))
	for _, item := range catalog {
		if item.ModelName != "" {
			catalogByName[item.ModelName] = item
		}
	}
	matches := MatchRequestedModelPrices(requested, catalog, openRouterPrices)
	items := make([]ModelPriceSyncItem, 0, len(matches))
	seenRequested := make(map[string]struct{}, len(requested))

	for _, rawModel := range requested {
		modelName := strings.TrimSpace(rawModel)
		if modelName == "" {
			continue
		}
		if _, seen := seenRequested[modelName]; seen {
			continue
		}
		seenRequested[modelName] = struct{}{}
		pricing, inCatalog := catalogByName[modelName]
		if !inCatalog {
			items = append(items, ModelPriceSyncItem{ModelName: modelName, Status: "skipped", Reason: "not_in_model_square"})
			continue
		}
		if reason, protected := modelPriceSyncProtectionReason(modelName); protected {
			items = append(items, ModelPriceSyncItem{
				ModelName: modelName,
				Current:   CurrentCanonicalPriceFromPricing(pricing),
				Status:    "skipped",
				Reason:    reason,
			})
			continue
		}

		match, hasMatch := matches[modelName]
		official := MergeHigherPrices(
			FindCanonicalPriceForModel(modelName, officialPrices),
			FindCanonicalPriceForModel(modelName, curatedModelPrices),
		)
		if !hasMatch || match.Status != "matched" {
			if official.hasAnyBillablePrice() {
				appendModelPriceSyncReadyItem(&items, modelName, "", pricing, official, CanonicalModelPrice{}, official)
				continue
			}
			reason := "no_openrouter_match"
			if hasMatch && match.Reason != "" {
				reason = match.Reason
			}
			items = append(items, ModelPriceSyncItem{
				ModelName: modelName,
				Current:   CurrentCanonicalPriceFromPricing(pricing),
				Official:  official,
				Status:    "skipped",
				Reason:    reason,
			})
			continue
		}

		final := MergeHigherPrices(official, match.Price)
		appendModelPriceSyncReadyItem(&items, modelName, match.OpenRouterID, pricing, official, match.Price, final)
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].ModelName < items[j].ModelName })
	result := ModelPriceSyncResult{Items: items, Requested: len(requested)}
	for _, item := range items {
		if item.WouldApply {
			result.Syncable++
		} else {
			result.SkippedCount++
		}
	}
	return result
}

func appendModelPriceSyncReadyItem(items *[]ModelPriceSyncItem, modelName string, openRouterID string, pricing model.Pricing, official CanonicalModelPrice, openRouter CanonicalModelPrice, final CanonicalModelPrice) {
	expr, err := BuildBillingExprFromPrice(final)
	if err != nil {
		*items = append(*items, ModelPriceSyncItem{
			ModelName:    modelName,
			OpenRouterID: openRouterID,
			Current:      CurrentCanonicalPriceFromPricing(pricing),
			Official:     official,
			OpenRouter:   openRouter,
			Final:        final,
			Status:       "skipped",
			Reason:       "invalid_billing_expr: " + err.Error(),
		})
		return
	}

	current := CurrentCanonicalPriceFromPricing(pricing)
	*items = append(*items, ModelPriceSyncItem{
		ModelName:     modelName,
		OpenRouterID:  openRouterID,
		Current:       current,
		Official:      official,
		OpenRouter:    openRouter,
		Final:         final,
		BillingExpr:   expr,
		Status:        "ready",
		SourceChoices: BuildModelPriceSourceChoices(official, openRouter, final),
		WouldApply:    true,
		Changed:       !CanonicalPricesEqual(current, final),
	})
}

func BuildModelPriceSourceChoices(official CanonicalModelPrice, openRouter CanonicalModelPrice, final CanonicalModelPrice) []ModelPriceSourceChoice {
	choices := make([]ModelPriceSourceChoice, 0, 9)
	appendChoice := func(dimension string, officialValue, openRouterValue, finalValue *float64) {
		if finalValue == nil {
			return
		}
		source := "official"
		if officialValue == nil {
			source = "openrouter"
		} else if openRouterValue != nil && *openRouterValue > *officialValue {
			source = "openrouter"
		} else if openRouterValue != nil && *openRouterValue == *officialValue {
			source = "same"
		}
		choices = append(choices, ModelPriceSourceChoice{Dimension: dimension, Source: source, Value: ptrFloat(*finalValue)})
	}
	appendChoice("input", official.Input, openRouter.Input, final.Input)
	appendChoice("output", official.Output, openRouter.Output, final.Output)
	appendChoice("cache_read", official.CacheRead, openRouter.CacheRead, final.CacheRead)
	appendChoice("cache_write", official.CacheWrite, openRouter.CacheWrite, final.CacheWrite)
	appendChoice("cache_write_1h", official.CacheWrite1h, openRouter.CacheWrite1h, final.CacheWrite1h)
	appendChoice("image_input", official.ImageInput, openRouter.ImageInput, final.ImageInput)
	appendChoice("audio_input", official.AudioInput, openRouter.AudioInput, final.AudioInput)
	appendChoice("audio_output", official.AudioOutput, openRouter.AudioOutput, final.AudioOutput)
	appendChoice("reasoning", official.Reasoning, openRouter.Reasoning, final.Reasoning)
	return choices
}

func CanonicalPricesEqual(a, b CanonicalModelPrice) bool {
	return pricePtrEqual(a.Input, b.Input) &&
		pricePtrEqual(a.Output, b.Output) &&
		pricePtrEqual(a.CacheRead, b.CacheRead) &&
		pricePtrEqual(a.CacheWrite, b.CacheWrite) &&
		pricePtrEqual(a.CacheWrite1h, b.CacheWrite1h) &&
		pricePtrEqual(a.ImageInput, b.ImageInput) &&
		pricePtrEqual(a.AudioInput, b.AudioInput) &&
		pricePtrEqual(a.AudioOutput, b.AudioOutput) &&
		pricePtrEqual(a.Reasoning, b.Reasoning)
}

func pricePtrEqual(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) < 1e-9
}

func ApplySelectedModelPriceSync(ctx context.Context, req ModelPriceSyncRequest) (ModelPriceSyncResult, error) {
	preview, err := PreviewSelectedModelPriceSync(ctx, req)
	if err != nil {
		return ModelPriceSyncResult{}, err
	}
	if err := applyModelPriceSyncOverrides(&preview, req.Overrides); err != nil {
		return ModelPriceSyncResult{}, err
	}
	if err := ApplyModelPriceSyncPreview(preview); err != nil {
		return ModelPriceSyncResult{}, err
	}
	for i := range preview.Items {
		if preview.Items[i].WouldApply {
			preview.Items[i].Applied = true
			preview.AppliedCount++
		}
	}
	return preview, nil
}

func applyModelPriceSyncOverrides(preview *ModelPriceSyncResult, overrides map[string]CanonicalModelPrice) error {
	if len(overrides) == 0 {
		return nil
	}
	preview.Syncable, preview.SkippedCount = 0, 0
	for i := range preview.Items {
		item := &preview.Items[i]
		if _, protected := modelPriceSyncProtectionReason(item.ModelName); protected {
			item.WouldApply = false
			preview.SkippedCount++
			continue
		}
		if price, ok := overrides[item.ModelName]; ok {
			expr, err := BuildBillingExprFromPrice(price)
			if err != nil {
				return fmt.Errorf("%s: %w", item.ModelName, err)
			}
			item.Final, item.BillingExpr = price, expr
			item.Status, item.Reason, item.WouldApply = "ready", "", true
			item.Changed = !CanonicalPricesEqual(item.Current, price)
		}
		if item.WouldApply {
			preview.Syncable++
		} else {
			preview.SkippedCount++
		}
	}
	return nil
}

func ApplyModelPriceSyncPreview(preview ModelPriceSyncResult) error {
	updates, err := BuildModelPriceSyncOptionUpdates(preview)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	if err := model.UpdateOptionsBulk(updates); err != nil {
		return err
	}
	model.InvalidatePricingCache()
	ratio_setting.InvalidateExposedDataCache()
	return nil
}

func BuildModelPriceSyncOptionUpdates(preview ModelPriceSyncResult) (map[string]string, error) {
	modelPriceMap := ratio_setting.GetModelPriceCopy()
	modelRatioMap := ratio_setting.GetModelRatioCopy()
	completionRatioMap := ratio_setting.GetCompletionRatioCopy()
	cacheRatioMap := ratio_setting.GetCacheRatioCopy()
	createCacheRatioMap := ratio_setting.GetCreateCacheRatioCopy()
	imageRatioMap := ratio_setting.GetImageRatioCopy()
	audioRatioMap := ratio_setting.GetAudioRatioCopy()
	audioCompletionRatioMap := ratio_setting.GetAudioCompletionRatioCopy()
	billingModeMap := billing_setting.GetBillingModeCopy()
	billingExprMap := billing_setting.GetBillingExprCopy()

	changed := false
	for _, item := range preview.Items {
		if !item.WouldApply || strings.TrimSpace(item.BillingExpr) == "" {
			continue
		}
		modelName := item.ModelName
		if _, protected := modelPriceSyncProtectionReason(modelName); protected {
			continue
		}
		delete(modelPriceMap, modelName)
		delete(modelRatioMap, modelName)
		delete(completionRatioMap, modelName)
		delete(cacheRatioMap, modelName)
		delete(createCacheRatioMap, modelName)
		delete(imageRatioMap, modelName)
		delete(audioRatioMap, modelName)
		delete(audioCompletionRatioMap, modelName)
		price := item.Final
		if price.Input != nil {
			modelRatioMap[modelName] = roundRatioValueForSync(*price.Input * common.QuotaPerUnit / 1_000_000)
			if price.Output != nil && *price.Input > 0 {
				completionRatioMap[modelName] = roundRatioValueForSync(*price.Output / *price.Input)
			}
			setRatioFromPrice(cacheRatioMap, modelName, price.CacheRead, *price.Input)
			setRatioFromPrice(createCacheRatioMap, modelName, price.CacheWrite, *price.Input)
			setRatioFromPrice(imageRatioMap, modelName, price.ImageInput, *price.Input)
			setRatioFromPrice(audioRatioMap, modelName, price.AudioInput, *price.Input)
			setRatioFromPrice(audioCompletionRatioMap, modelName, price.AudioOutput, *price.Input)
		}
		billingModeMap[modelName] = billing_setting.BillingModeTieredExpr
		billingExprMap[modelName] = item.BillingExpr
		changed = true
	}
	if !changed {
		return nil, nil
	}

	updates := map[string]string{}
	putJSON := func(key string, value any) error {
		bytes, err := common.Marshal(value)
		if err != nil {
			return err
		}
		updates[key] = string(bytes)
		return nil
	}
	if err := putJSON("ModelPrice", modelPriceMap); err != nil {
		return nil, err
	}
	if err := putJSON("ModelRatio", modelRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("CompletionRatio", completionRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("CacheRatio", cacheRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("CreateCacheRatio", createCacheRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("ImageRatio", imageRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("AudioRatio", audioRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("AudioCompletionRatio", audioCompletionRatioMap); err != nil {
		return nil, err
	}
	if err := putJSON("billing_setting."+billing_setting.BillingModeField, billingModeMap); err != nil {
		return nil, err
	}
	if err := putJSON("billing_setting."+billing_setting.BillingExprField, billingExprMap); err != nil {
		return nil, err
	}
	return updates, nil
}

func modelPriceSyncProtectionReason(modelName string) (string, bool) {
	reason, ok := modelPriceSyncProtectedModels[canonicalModelID(modelName)]
	return reason, ok
}

func setRatioFromPrice(target map[string]float64, modelName string, price *float64, input float64) {
	if price == nil || input <= 0 {
		return
	}
	target[modelName] = roundRatioValueForSync(*price / input)
}

func roundRatioValueForSync(value float64) float64 {
	return math.Round(value*1e9) / 1e9
}
