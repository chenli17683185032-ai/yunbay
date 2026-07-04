package service

import (
	"sort"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

const subscriptionBillingGroupRatio = 1.0

func otherRatioProduct(priceData types.PriceData) decimal.Decimal {
	product := decimal.NewFromInt(1)
	for _, ratio := range priceData.OtherRatios {
		if ratio > 0 && ratio != 1.0 {
			product = product.Mul(decimal.NewFromFloat(ratio))
		}
	}
	return product
}

func applyOtherRatiosStepwise(quota int, ratios map[string]float64) int {
	if len(ratios) == 0 {
		return quota
	}
	keys := make([]string, 0, len(ratios))
	for key := range ratios {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ratio := ratios[key]
		if ratio > 0 && ratio != 1.0 {
			quota = int(float64(quota) * ratio)
		}
	}
	return quota
}

func quotaWithGroupRatio(priceData types.PriceData, groupRatio float64) int {
	base := decimal.NewFromFloat(priceData.QuotaBeforeGroup)
	if base.IsZero() && priceData.GroupRatioInfo.GroupRatio != 0 {
		base = decimal.NewFromInt(int64(priceData.QuotaToPreConsume)).Div(decimal.NewFromFloat(priceData.GroupRatioInfo.GroupRatio))
	}
	quota := int(base.Mul(decimal.NewFromFloat(groupRatio)).IntPart())
	return applyOtherRatiosStepwise(quota, priceData.OtherRatios)
}

func subscriptionPreConsumeQuota(relayInfo *relaycommon.RelayInfo, fallback int) int {
	if relayInfo == nil {
		return fallback
	}
	priceData := relayInfo.PriceData
	if snap := relayInfo.TieredBillingSnapshot; snap != nil && snap.BillingMode == "tiered_expr" {
		return billingexpr.QuotaRound(snap.EstimatedQuotaBeforeGroup * subscriptionBillingGroupRatio)
	}
	quota := quotaWithGroupRatio(priceData, subscriptionBillingGroupRatio)
	if quota <= 0 {
		if priceData.QuotaBeforeGroup > 0 {
			quota = applyOtherRatiosStepwise(int(decimal.NewFromFloat(priceData.QuotaBeforeGroup).IntPart()), priceData.OtherRatios)
		} else {
			quota = fallback
		}
	}
	return quota
}

func applySubscriptionBillingRatio(relayInfo *relaycommon.RelayInfo, preConsumedQuota int) {
	if relayInfo == nil {
		return
	}
	priceData := &relayInfo.PriceData
	shouldSyncPerCallQuota := priceData.Quota > 0 || priceData.FreeByGroupRatio
	if !priceData.HasOriginalGroupRatioInfo {
		priceData.OriginalGroupRatioInfo = priceData.GroupRatioInfo
		priceData.HasOriginalGroupRatioInfo = true
	}
	priceData.SubscriptionRatioApplied = true
	priceData.GroupRatioInfo = types.GroupRatioInfo{
		GroupRatio:        subscriptionBillingGroupRatio,
		GroupSpecialRatio: -1,
		HasSpecialRatio:   false,
	}
	priceData.FreeByGroupRatio = false
	priceData.FreeModel = false
	priceData.QuotaToPreConsume = preConsumedQuota
	if shouldSyncPerCallQuota {
		priceData.Quota = preConsumedQuota
	}
	if snap := relayInfo.TieredBillingSnapshot; snap != nil && snap.BillingMode == "tiered_expr" {
		snap.GroupRatio = subscriptionBillingGroupRatio
		snap.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(snap.EstimatedQuotaBeforeGroup * subscriptionBillingGroupRatio)
		priceData.QuotaToPreConsume = snap.EstimatedQuotaAfterGroup
		if shouldSyncPerCallQuota {
			priceData.Quota = snap.EstimatedQuotaAfterGroup
		}
	}
}

func EnsureSubscriptionBillingRatio(relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil || relayInfo.BillingSource != BillingSourceSubscription {
		return
	}
	preConsumedQuota := relayInfo.FinalPreConsumedQuota
	if relayInfo.Billing != nil {
		preConsumedQuota = relayInfo.Billing.GetPreConsumedQuota()
	}
	if preConsumedQuota <= 0 {
		preConsumedQuota = subscriptionPreConsumeQuota(relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	}
	if preConsumedQuota <= 0 {
		return
	}
	applySubscriptionBillingRatio(relayInfo, preConsumedQuota)
}

func restoreOriginalBillingRatio(relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil {
		return
	}
	priceData := &relayInfo.PriceData
	if !priceData.HasOriginalGroupRatioInfo {
		return
	}
	priceData.GroupRatioInfo = priceData.OriginalGroupRatioInfo
	priceData.SubscriptionRatioApplied = false
	restoredQuota := quotaWithGroupRatio(*priceData, priceData.GroupRatioInfo.GroupRatio)
	if restoredQuota > 0 || priceData.FreeByGroupRatio {
		priceData.QuotaToPreConsume = restoredQuota
		if priceData.Quota > 0 || priceData.FreeByGroupRatio {
			priceData.Quota = restoredQuota
		}
	}
	if snap := relayInfo.TieredBillingSnapshot; snap != nil && snap.BillingMode == "tiered_expr" {
		snap.GroupRatio = priceData.GroupRatioInfo.GroupRatio
		snap.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(snap.EstimatedQuotaBeforeGroup * priceData.GroupRatioInfo.GroupRatio)
		priceData.QuotaToPreConsume = snap.EstimatedQuotaAfterGroup
		if priceData.Quota > 0 || priceData.FreeByGroupRatio {
			priceData.Quota = snap.EstimatedQuotaAfterGroup
		}
	}
}

func ApplyTaskOtherRatios(quota int, ratios map[string]float64) int {
	return applyOtherRatiosStepwise(quota, ratios)
}
