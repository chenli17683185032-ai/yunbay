package service

import (
	"math"
	"sort"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

const (
	SubscriptionRatioSourceRegular    = "regular_subscription_1x"
	SubscriptionRatioSourceDefault    = "default_1x"
	SubscriptionRatioSourceConfigured = "configured"
)

func validSubscriptionBillingRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio > 0
}

func defaultSubscriptionBillingRatio(info *relaycommon.RelayInfo) (float64, string) {
	if info != nil && info.ValuePackageSubscriptionId > 0 {
		return 1, SubscriptionRatioSourceDefault
	}
	return 1, SubscriptionRatioSourceRegular
}

func normalizeFrozenSubscriptionBillingRatio(info *relaycommon.RelayInfo, ratio float64, source string) (float64, string) {
	if !validSubscriptionBillingRatio(ratio) {
		return defaultSubscriptionBillingRatio(info)
	}
	isValuePackage := info != nil && info.ValuePackageSubscriptionId > 0
	switch source {
	case SubscriptionRatioSourceRegular:
		if !isValuePackage && ratio == 1 {
			return ratio, source
		}
	case SubscriptionRatioSourceDefault:
		if isValuePackage && ratio == 1 {
			return ratio, source
		}
	case SubscriptionRatioSourceConfigured:
		if isValuePackage {
			return ratio, source
		}
	}
	return defaultSubscriptionBillingRatio(info)
}

func resolveSubscriptionBillingRatio(info *relaycommon.RelayInfo) (float64, string) {
	if info == nil {
		return defaultSubscriptionBillingRatio(nil)
	}
	if info.PriceData.SubscriptionRatioApplied {
		return normalizeFrozenSubscriptionBillingRatio(
			info,
			info.PriceData.GroupRatioInfo.GroupRatio,
			info.PriceData.SubscriptionRatioSource,
		)
	}
	if info.ValuePackageSubscriptionId <= 0 {
		return 1, SubscriptionRatioSourceRegular
	}
	candidate := info.PriceData.GroupRatioInfo
	if candidate.HasSpecialRatio && validSubscriptionBillingRatio(candidate.GroupSpecialRatio) {
		return candidate.GroupSpecialRatio, SubscriptionRatioSourceConfigured
	}
	return 1, SubscriptionRatioSourceDefault
}

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

func subscriptionPreConsumeQuota(relayInfo *relaycommon.RelayInfo, fallback int, ratio float64, source string) int {
	if relayInfo == nil {
		return fallback
	}
	ratio, _ = normalizeFrozenSubscriptionBillingRatio(relayInfo, ratio, source)
	priceData := relayInfo.PriceData
	if snap := relayInfo.TieredBillingSnapshot; snap != nil && snap.BillingMode == "tiered_expr" {
		return billingexpr.QuotaRound(snap.EstimatedQuotaBeforeGroup * ratio)
	}
	quota := quotaWithGroupRatio(priceData, ratio)
	if quota <= 0 {
		if priceData.QuotaBeforeGroup > 0 {
			quota = applyOtherRatiosStepwise(int(decimal.NewFromFloat(priceData.QuotaBeforeGroup).Mul(decimal.NewFromFloat(ratio)).IntPart()), priceData.OtherRatios)
		} else {
			quota = fallback
		}
	}
	return quota
}

func applySubscriptionBillingRatio(relayInfo *relaycommon.RelayInfo, preConsumedQuota int, ratio float64, source string) {
	if relayInfo == nil {
		return
	}
	priceData := &relayInfo.PriceData
	wasApplied := priceData.SubscriptionRatioApplied
	if wasApplied {
		ratio, source = resolveSubscriptionBillingRatio(relayInfo)
	} else {
		ratio, source = normalizeFrozenSubscriptionBillingRatio(relayInfo, ratio, source)
	}
	shouldSyncPerCallQuota := priceData.Quota > 0 || priceData.FreeByGroupRatio
	if !priceData.HasOriginalGroupRatioInfo {
		priceData.OriginalGroupRatioInfo = priceData.GroupRatioInfo
		priceData.HasOriginalGroupRatioInfo = true
	}
	priceData.SubscriptionRatioApplied = true
	priceData.SubscriptionRatioSource = source
	priceData.GroupRatioInfo = types.GroupRatioInfo{
		GroupRatio:        ratio,
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
		if !wasApplied {
			snap.GroupRatio = ratio
			snap.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(snap.EstimatedQuotaBeforeGroup * ratio)
		}
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
	ratio, source := resolveSubscriptionBillingRatio(relayInfo)
	if !relayInfo.PriceData.SubscriptionRatioApplied {
		if session, ok := relayInfo.Billing.(*BillingSession); ok {
			if frozenRatio, frozenSource, frozen := session.subscriptionBillingRatioSnapshot(); frozen {
				ratio, source = frozenRatio, frozenSource
			}
		}
	}
	preConsumedQuota := relayInfo.FinalPreConsumedQuota
	if relayInfo.Billing != nil {
		preConsumedQuota = relayInfo.Billing.GetPreConsumedQuota()
	}
	if preConsumedQuota <= 0 {
		preConsumedQuota = subscriptionPreConsumeQuota(relayInfo, relayInfo.PriceData.QuotaToPreConsume, ratio, source)
	}
	if preConsumedQuota <= 0 {
		return
	}
	applySubscriptionBillingRatio(relayInfo, preConsumedQuota, ratio, source)
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
	priceData.SubscriptionRatioSource = ""
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
