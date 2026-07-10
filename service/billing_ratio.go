package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
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

const invalidValuePackageBillingRatioWarningWindow = 5 * time.Minute

type billingRatioWarningLimiter struct {
	mu     sync.Mutex
	window time.Duration
	last   map[string]time.Time
}

func newBillingRatioWarningLimiter(window time.Duration) *billingRatioWarningLimiter {
	return &billingRatioWarningLimiter{
		window: window,
		last:   make(map[string]time.Time),
	}
}

func (l *billingRatioWarningLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if previous, ok := l.last[key]; ok && now.Sub(previous) < l.window {
		return false
	}
	l.last[key] = now
	return true
}

var invalidValuePackageBillingRatioWarnings = newBillingRatioWarningLimiter(invalidValuePackageBillingRatioWarningWindow)
var invalidValuePackageBillingRatioWarningNow = time.Now

func validSubscriptionBillingRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio > 0
}

func invalidSubscriptionBillingRatioKind(ratio float64) string {
	switch {
	case math.IsNaN(ratio):
		return "nan"
	case math.IsInf(ratio, 1):
		return "positive_infinity"
	case math.IsInf(ratio, -1):
		return "negative_infinity"
	case ratio == 0:
		return "zero"
	default:
		return "negative"
	}
}

func warnInvalidValuePackageBillingRatio(info *relaycommon.RelayInfo, ratio float64) {
	reason := invalidSubscriptionBillingRatioKind(ratio)
	packageGroup := strings.TrimSpace(info.ValuePackageBillingGroup)
	usingGroup := info.BillingGroup()
	key := packageGroup + "\x00" + usingGroup + "\x00" + reason
	if !invalidValuePackageBillingRatioWarnings.Allow(key, invalidValuePackageBillingRatioWarningNow()) {
		return
	}
	logger.LogWarn(context.Background(), fmt.Sprintf(
		"invalid value package billing ratio (package_group=%q, using_group=%q, reason=%s); falling back to default 1x",
		packageGroup, usingGroup, reason,
	))
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
		if info.ValuePackageSubscriptionId > 0 &&
			info.PriceData.SubscriptionRatioSource == SubscriptionRatioSourceConfigured &&
			!validSubscriptionBillingRatio(info.PriceData.GroupRatioInfo.GroupRatio) {
			warnInvalidValuePackageBillingRatio(info, info.PriceData.GroupRatioInfo.GroupRatio)
		}
		return normalizeFrozenSubscriptionBillingRatio(
			info,
			info.PriceData.GroupRatioInfo.GroupRatio,
			info.PriceData.SubscriptionRatioSource,
		)
	}
	if info.ValuePackageSubscriptionId <= 0 {
		return 1, SubscriptionRatioSourceRegular
	}
	packageGroup := strings.TrimSpace(info.ValuePackageBillingGroup)
	if packageGroup == "" {
		return 1, SubscriptionRatioSourceDefault
	}
	billingUserGroup := strings.TrimSpace(info.BillingUserGroup)
	if billingUserGroup != "" && billingUserGroup != packageGroup {
		return 1, SubscriptionRatioSourceDefault
	}
	candidate := info.PriceData.GroupRatioInfo
	if candidate.HasSpecialRatio && validSubscriptionBillingRatio(candidate.GroupSpecialRatio) {
		return candidate.GroupSpecialRatio, SubscriptionRatioSourceConfigured
	}
	if candidate.HasSpecialRatio {
		warnInvalidValuePackageBillingRatio(info, candidate.GroupSpecialRatio)
	}
	return 1, SubscriptionRatioSourceDefault
}

func resolveAuthoritativeSubscriptionBillingRatio(info *relaycommon.RelayInfo) (float64, string, bool) {
	if info != nil {
		if session, ok := info.Billing.(*BillingSession); ok {
			if ratio, source, frozen := session.subscriptionBillingRatioSnapshot(); frozen {
				return ratio, source, true
			}
		}
	}
	ratio, source := resolveSubscriptionBillingRatio(info)
	return ratio, source, false
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
	if frozenRatio, frozenSource, frozen := resolveAuthoritativeSubscriptionBillingRatio(relayInfo); frozen {
		ratio, source = frozenRatio, frozenSource
	} else if wasApplied {
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
		snap.GroupRatio = ratio
		snap.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(snap.EstimatedQuotaBeforeGroup * ratio)
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
	ratio, source, _ := resolveAuthoritativeSubscriptionBillingRatio(relayInfo)
	if session, ok := relayInfo.Billing.(*BillingSession); ok {
		if billingGroup := session.billingUsingGroupSnapshot(); billingGroup != "" {
			relayInfo.BillingUsingGroup = billingGroup
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
