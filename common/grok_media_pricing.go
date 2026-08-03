package common

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	GrokImageStandardBasePrice = 0.02
	GrokImageQualityBasePrice  = 0.05
	GrokVideoStandardBasePrice = 0.05
	GrokVideo15BasePrice       = 0.08

	GrokVideoDefaultDurationSeconds = 8
	GrokVideoMinDurationSeconds     = 1
	GrokVideoMaxDurationSeconds     = 15
)

type GrokImagePrice struct {
	BasePrice   float64
	TotalPrice  float64
	Multiplier  float64
	BillingSize string
	OutputPrice float64
	InputPrice  float64
	OutputCount int
	InputCount  int
}

type GrokVideoPrice struct {
	BasePrice          float64
	TotalPrice         float64
	Multiplier         float64
	EffectiveModel     string
	Resolution         string
	DurationSeconds    int
	OutputPricePerSec  float64
	InputImagePrice    float64
	InputVideoPriceSec float64
}

func IsGrokImageGenerationModel(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "grok-imagine", "grok-imagine-edit", "grok-imagine-image", "grok-imagine-image-quality":
		return true
	default:
		return false
	}
}

func IsGrokVideoGenerationModel(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "grok-imagine-video", "grok-imagine-video-1.5":
		return true
	default:
		return false
	}
}

func CountJSONMediaReferences(rawValues ...[]byte) int {
	count := 0
	for _, raw := range rawValues {
		if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
			continue
		}
		var value any
		if err := Unmarshal(raw, &value); err == nil {
			count += countJSONMediaReference(value)
		}
	}
	return count
}

func countJSONMediaReference(value any) int {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			return 1
		}
	case []any:
		count := 0
		for _, item := range typed {
			count += countJSONMediaReference(item)
		}
		return count
	case map[string]any:
		if len(typed) > 0 {
			return 1
		}
	}
	return 0
}

func CalculateGrokImagePrice(modelName, size string, outputCount, inputCount int) (GrokImagePrice, error) {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if outputCount <= 0 {
		outputCount = 1
	}
	if inputCount < 0 {
		return GrokImagePrice{}, fmt.Errorf("input image count must not be negative")
	}

	basePrice := GrokImageStandardBasePrice
	outputPrice := GrokImageStandardBasePrice
	inputPrice := 0.002
	switch modelName {
	case "grok-imagine-image", "grok-imagine-edit":
	case "grok-imagine", "grok-imagine-image-quality":
		basePrice = GrokImageQualityBasePrice
		inputPrice = 0.01
	default:
		return GrokImagePrice{}, fmt.Errorf("unsupported Grok image model %q", modelName)
	}

	billingSize := normalizeGrokImageBillingSize(size)
	if basePrice == GrokImageQualityBasePrice {
		if billingSize == "1K" {
			outputPrice = 0.05
		} else {
			outputPrice = 0.07
		}
	}

	totalPrice := float64(outputCount)*outputPrice + float64(inputCount)*inputPrice
	return GrokImagePrice{
		BasePrice:   basePrice,
		TotalPrice:  roundGrokMediaPrice(totalPrice),
		Multiplier:  roundGrokMediaPrice(totalPrice / basePrice),
		BillingSize: billingSize,
		OutputPrice: outputPrice,
		InputPrice:  inputPrice,
		OutputCount: outputCount,
		InputCount:  inputCount,
	}, nil
}

func CalculateGrokVideoPrice(modelName, resolution string, durationSeconds, inputImageCount, inputVideoSeconds int) (GrokVideoPrice, error) {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if !IsGrokVideoGenerationModel(modelName) {
		return GrokVideoPrice{}, fmt.Errorf("unsupported Grok video model %q", modelName)
	}
	if inputImageCount < 0 || inputVideoSeconds < 0 {
		return GrokVideoPrice{}, fmt.Errorf("input media count must not be negative")
	}
	if inputImageCount > 0 && inputVideoSeconds > 0 {
		return GrokVideoPrice{}, fmt.Errorf("image and video inputs cannot be combined")
	}
	if inputVideoSeconds > GrokVideoMaxDurationSeconds {
		return GrokVideoPrice{}, fmt.Errorf("input video duration must be between %d and %d seconds", GrokVideoMinDurationSeconds, GrokVideoMaxDurationSeconds)
	}
	if durationSeconds == 0 {
		durationSeconds = GrokVideoDefaultDurationSeconds
	}
	if durationSeconds < GrokVideoMinDurationSeconds || durationSeconds > GrokVideoMaxDurationSeconds {
		return GrokVideoPrice{}, fmt.Errorf("duration must be between %d and %d seconds", GrokVideoMinDurationSeconds, GrokVideoMaxDurationSeconds)
	}

	normalizedResolution, err := normalizeGrokVideoResolution(resolution)
	if err != nil {
		return GrokVideoPrice{}, err
	}
	basePrice := GrokVideoStandardBasePrice
	if modelName == "grok-imagine-video-1.5" {
		basePrice = GrokVideo15BasePrice
	}

	effectiveModel := modelName
	if modelName == "grok-imagine-video-1.5" && inputImageCount == 0 && inputVideoSeconds == 0 {
		effectiveModel = "grok-imagine-video"
	}
	if effectiveModel == "grok-imagine-video-1.5" && inputVideoSeconds > 0 {
		return GrokVideoPrice{}, fmt.Errorf("grok-imagine-video-1.5 does not support video input pricing")
	}

	outputPrice := 0.0
	inputImagePrice := 0.002
	inputVideoPrice := 0.01
	if effectiveModel == "grok-imagine-video-1.5" {
		inputImagePrice = 0.01
		inputVideoPrice = 0
		switch normalizedResolution {
		case "480p":
			outputPrice = 0.08
		case "720p":
			outputPrice = 0.14
		case "1080p":
			outputPrice = 0.25
		}
	} else {
		switch normalizedResolution {
		case "480p":
			outputPrice = 0.05
		case "720p":
			outputPrice = 0.07
		case "1080p":
			return GrokVideoPrice{}, fmt.Errorf("grok-imagine-video does not support 1080p")
		}
	}

	totalPrice := outputPrice*float64(durationSeconds) + inputImagePrice*float64(inputImageCount) + inputVideoPrice*float64(inputVideoSeconds)
	return GrokVideoPrice{
		BasePrice:          basePrice,
		TotalPrice:         roundGrokMediaPrice(totalPrice),
		Multiplier:         roundGrokMediaPrice(totalPrice / basePrice),
		EffectiveModel:     effectiveModel,
		Resolution:         normalizedResolution,
		DurationSeconds:    durationSeconds,
		OutputPricePerSec:  outputPrice,
		InputImagePrice:    inputImagePrice,
		InputVideoPriceSec: inputVideoPrice,
	}, nil
}

func normalizeGrokImageBillingSize(size string) string {
	normalized := strings.ToLower(strings.TrimSpace(size))
	switch normalized {
	case "1k", "1024x1024", "1024x768", "768x1024":
		return "1K"
	case "2k", "2048x2048", "2048x1152", "1152x2048":
		return "2K"
	}
	parts := strings.Split(normalized, "x")
	if len(parts) == 2 {
		width, widthErr := strconv.Atoi(parts[0])
		height, heightErr := strconv.Atoi(parts[1])
		if widthErr == nil && heightErr == nil && width > 0 && height > 0 {
			if width <= 1024 && height <= 1024 {
				return "1K"
			}
			return "2K"
		}
	}
	return "2K"
}

func normalizeGrokVideoResolution(resolution string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "", "480", "480p", "sd":
		return "480p", nil
	case "720", "720p", "hd":
		return "720p", nil
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		return "1080p", nil
	default:
		return "", fmt.Errorf("unsupported Grok video resolution %q", resolution)
	}
}

func roundGrokMediaPrice(value float64) float64 {
	return math.Round(value*1e9) / 1e9
}
