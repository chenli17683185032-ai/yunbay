package common

import (
	"math"
	"testing"
)

func TestCalculateGrokImagePrice(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		size      string
		n         int
		inputs    int
		wantBase  float64
		wantTotal float64
		wantRatio float64
		wantTier  string
	}{
		{name: "standard 1K", model: "grok-imagine-image", size: "1K", n: 1, wantBase: 0.02, wantTotal: 0.02, wantRatio: 1, wantTier: "1K"},
		{name: "standard 2K with inputs", model: "grok-imagine-image", size: "2048x1152", n: 3, inputs: 2, wantBase: 0.02, wantTotal: 0.064, wantRatio: 3.2, wantTier: "2K"},
		{name: "edit inputs", model: "grok-imagine-edit", size: "auto", n: 2, inputs: 2, wantBase: 0.02, wantTotal: 0.044, wantRatio: 2.2, wantTier: "2K"},
		{name: "quality 1K", model: "grok-imagine-image-quality", size: "1024x1024", n: 2, inputs: 1, wantBase: 0.05, wantTotal: 0.11, wantRatio: 2.2, wantTier: "1K"},
		{name: "quality 2K", model: "grok-imagine", size: "2k", n: 2, inputs: 3, wantBase: 0.05, wantTotal: 0.17, wantRatio: 3.4, wantTier: "2K"},
		{name: "unknown size is conservative", model: "grok-imagine", size: "future-size", n: 1, wantBase: 0.05, wantTotal: 0.07, wantRatio: 1.4, wantTier: "2K"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateGrokImagePrice(tt.model, tt.size, tt.n, tt.inputs)
			if err != nil {
				t.Fatalf("CalculateGrokImagePrice returned error: %v", err)
			}
			assertClose(t, got.BasePrice, tt.wantBase)
			assertClose(t, got.TotalPrice, tt.wantTotal)
			assertClose(t, got.Multiplier, tt.wantRatio)
			if got.BillingSize != tt.wantTier {
				t.Fatalf("BillingSize = %q, want %q", got.BillingSize, tt.wantTier)
			}
		})
	}
}

func TestCalculateGrokVideoPrice(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		resolution     string
		duration       int
		inputImages    int
		inputVideoSecs int
		wantEffective  string
		wantResolution string
		wantDuration   int
		wantBase       float64
		wantTotal      float64
		wantMultiplier float64
	}{
		{name: "standard 720p", model: "grok-imagine-video", resolution: "hd", duration: 10, inputImages: 2, wantEffective: "grok-imagine-video", wantResolution: "720p", wantDuration: 10, wantBase: 0.05, wantTotal: 0.704, wantMultiplier: 14.08},
		{name: "video 1.5 image 1080p", model: "grok-imagine-video-1.5", resolution: "1080p", duration: 4, inputImages: 1, wantEffective: "grok-imagine-video-1.5", wantResolution: "1080p", wantDuration: 4, wantBase: 0.08, wantTotal: 1.01, wantMultiplier: 12.625},
		{name: "video 1.5 text falls back", model: "grok-imagine-video-1.5", resolution: "720p", duration: 0, wantEffective: "grok-imagine-video", wantResolution: "720p", wantDuration: 8, wantBase: 0.08, wantTotal: 0.56, wantMultiplier: 7},
		{name: "standard video input", model: "grok-imagine-video", resolution: "480p", duration: 2, inputVideoSecs: 6, wantEffective: "grok-imagine-video", wantResolution: "480p", wantDuration: 2, wantBase: 0.05, wantTotal: 0.16, wantMultiplier: 3.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateGrokVideoPrice(tt.model, tt.resolution, tt.duration, tt.inputImages, tt.inputVideoSecs)
			if err != nil {
				t.Fatalf("CalculateGrokVideoPrice returned error: %v", err)
			}
			if got.EffectiveModel != tt.wantEffective || got.Resolution != tt.wantResolution || got.DurationSeconds != tt.wantDuration {
				t.Fatalf("normalized result = %#v", got)
			}
			assertClose(t, got.BasePrice, tt.wantBase)
			assertClose(t, got.TotalPrice, tt.wantTotal)
			assertClose(t, got.Multiplier, tt.wantMultiplier)
		})
	}
}

func TestCalculateGrokVideoPriceRejectsUnsupportedValues(t *testing.T) {
	tests := []struct {
		model      string
		resolution string
		duration   int
		images     int
		videoSecs  int
	}{
		{model: "grok-imagine-video", resolution: "1080p", duration: 8},
		{model: "grok-imagine-video", resolution: "cinema", duration: 8},
		{model: "grok-imagine-video", resolution: "480p", duration: 16},
		{model: "grok-imagine-video-1.5", resolution: "720p", duration: 8, images: 1, videoSecs: 1},
		{model: "unknown-video", resolution: "480p", duration: 8},
	}
	for _, tt := range tests {
		if _, err := CalculateGrokVideoPrice(tt.model, tt.resolution, tt.duration, tt.images, tt.videoSecs); err == nil {
			t.Fatalf("CalculateGrokVideoPrice(%q, %q, %d, %d, %d) returned nil error", tt.model, tt.resolution, tt.duration, tt.images, tt.videoSecs)
		}
	}
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
