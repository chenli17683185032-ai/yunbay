package types

import "fmt"

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

type PriceData struct {
	FreeModel            bool
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	OtherRatios          map[string]float64
	UsePrice             bool
	Quota                int     // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume    int     // 按量计费的预消耗额度
	QuotaBeforeGroup     float64 // 应用分组倍率前的额度快照
	FreeByGroupRatio     bool    // 因分组倍率为 0 触发免费，而非模型本身免费
	GroupRatioInfo       GroupRatioInfo

	HasOriginalGroupRatioInfo bool
	OriginalGroupRatioInfo    GroupRatioInfo
	SubscriptionRatioApplied  bool
	SubscriptionRatioSource   string
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if p.OtherRatios == nil {
		p.OtherRatios = make(map[string]float64)
	}
	if ratio <= 0 {
		return
	}
	p.OtherRatios[key] = ratio
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, QuotaBeforeGroup: %f, SubscriptionRatioApplied: %t, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.QuotaBeforeGroup, p.SubscriptionRatioApplied, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
