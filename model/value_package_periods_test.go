package model

import (
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestBuildValuePackagePeriodLimits(t *testing.T) {
	const now = int64(1783700000)
	usageDetails := &ValuePackageWindowUsageDetails{
		Used5h:    100,
		ResetAt5h: now + 60,
		Used7d:    700,
		ResetAt7d: now + 120,
	}

	tests := []struct {
		name        string
		packageType string
		amountTotal int64
		want        []ValuePackagePeriodLimit
	}{
		{
			name:        "day card exposes refreshing five hour and lifecycle periods",
			packageType: ValuePackageTypeDay,
			amountTotal: 2400,
			want: []ValuePackagePeriodLimit{
				{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true, ResetAt: now + 60, Limited: false},
				{Kind: "lifecycle", LabelUnit: "day", LabelValue: 1, Limit: 2400, Used: 600, Remaining: 1800, Percent: 25, Refreshes: false, ResetAt: 0, Limited: false},
			},
		},
		{
			name:        "week card exposes five hour and seven day lifecycle periods",
			packageType: ValuePackageTypeWeek,
			amountTotal: 4500,
			want: []ValuePackagePeriodLimit{
				{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true, ResetAt: now + 60, Limited: false},
				{Kind: "lifecycle", LabelUnit: "day", LabelValue: 7, Limit: 4500, Used: 600, Remaining: 3900, Percent: 600.0 / 4500 * 100, Refreshes: false, ResetAt: 0, Limited: false},
			},
		},
		{
			name:        "month card exposes five hour seven day stage and lifecycle periods",
			packageType: ValuePackageTypeMonth,
			amountTotal: 22000,
			want: []ValuePackagePeriodLimit{
				{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true, ResetAt: now + 60, Limited: false},
				{Kind: "seven_day_stage", LabelUnit: "day", LabelValue: 7, Limit: 5500, Used: 700, Remaining: 4800, Percent: 700.0 / 5500 * 100, Refreshes: true, ResetAt: now + 120, Limited: false},
				{Kind: "lifecycle", LabelUnit: "day", LabelValue: 30, Limit: 22000, Used: 600, Remaining: 21400, Percent: 600.0 / 22000 * 100, Refreshes: false, ResetAt: 0, Limited: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &UserSubscription{AmountTotal: tt.amountTotal, AmountUsed: 600}
			plan := &SubscriptionPlan{
				PlanKind:      SubscriptionPlanKindValuePackage,
				PackageType:   tt.packageType,
				Limit5hAmount: 900,
				Limit7dAmount: 5500,
			}

			got := buildValuePackagePeriodLimits(sub, plan, usageDetails)

			requireValuePackagePeriodLimitsEqual(t, tt.want, got)
		})
	}

	t.Run("used at or above limit clamps remaining and percent", func(t *testing.T) {
		sub := &UserSubscription{AmountTotal: 500, AmountUsed: 600}
		plan := &SubscriptionPlan{
			PlanKind:      SubscriptionPlanKindValuePackage,
			PackageType:   ValuePackageTypeDay,
			Limit5hAmount: 100,
		}
		details := &ValuePackageWindowUsageDetails{Used5h: 150, ResetAt5h: now + 60}
		want := []ValuePackagePeriodLimit{
			{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 100, Used: 150, Remaining: 0, Percent: 100, Refreshes: true, ResetAt: now + 60, Limited: true},
			{Kind: "lifecycle", LabelUnit: "day", LabelValue: 1, Limit: 500, Used: 600, Remaining: 0, Percent: 100, Refreshes: false, ResetAt: 0, Limited: true},
		}

		requireValuePackagePeriodLimitsEqual(t, want, buildValuePackagePeriodLimits(sub, plan, details))
	})

	t.Run("invalid inputs do not expose periods", func(t *testing.T) {
		validSub := &UserSubscription{AmountTotal: 2400, AmountUsed: 600}
		validPlan := &SubscriptionPlan{
			PlanKind:      SubscriptionPlanKindValuePackage,
			PackageType:   ValuePackageTypeDay,
			Limit5hAmount: 900,
		}
		validDetails := &ValuePackageWindowUsageDetails{}
		invalidInputs := []struct {
			name    string
			sub     *UserSubscription
			plan    *SubscriptionPlan
			details *ValuePackageWindowUsageDetails
		}{
			{name: "nil subscription", sub: nil, plan: validPlan, details: validDetails},
			{name: "nil plan", sub: validSub, plan: nil, details: validDetails},
			{name: "nil usage details", sub: validSub, plan: validPlan, details: nil},
			{name: "non value package", sub: validSub, plan: &SubscriptionPlan{PlanKind: SubscriptionPlanKindSubscription, PackageType: ValuePackageTypeDay}, details: validDetails},
		}

		for _, tt := range invalidInputs {
			t.Run(tt.name, func(t *testing.T) {
				require.Nil(t, buildValuePackagePeriodLimits(tt.sub, tt.plan, tt.details))
			})
		}
	})

	t.Run("unknown package type retains independent five hour period", func(t *testing.T) {
		unknownPlan := &SubscriptionPlan{
			PlanKind:      SubscriptionPlanKindValuePackage,
			PackageType:   "unknown",
			Limit5hAmount: 900,
			Limit7dAmount: 5500,
		}
		want := []ValuePackagePeriodLimit{
			{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true, ResetAt: now + 60, Limited: false},
		}

		requireValuePackagePeriodLimitsEqual(t, want, buildValuePackagePeriodLimits(&UserSubscription{AmountTotal: 22000, AmountUsed: 600}, unknownPlan, usageDetails))

		unknownPlan.Limit5hAmount = 0
		require.Empty(t, buildValuePackagePeriodLimits(&UserSubscription{AmountTotal: 22000, AmountUsed: 600}, unknownPlan, usageDetails))
	})

	t.Run("known package types retain zero quota capabilities", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			packageType   string
			lifecycleDays int
		}{
			{name: "day", packageType: ValuePackageTypeDay, lifecycleDays: 1},
			{name: "week", packageType: ValuePackageTypeWeek, lifecycleDays: 7},
			{name: "month without seven day stage", packageType: ValuePackageTypeMonth, lifecycleDays: 30},
		} {
			t.Run(tt.name, func(t *testing.T) {
				plan := &SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: tt.packageType}
				sub := &UserSubscription{AmountTotal: 0, AmountUsed: 600}
				want := []ValuePackagePeriodLimit{
					{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 0, Used: 100, Remaining: 0, Percent: 0, Refreshes: true, ResetAt: now + 60, Limited: false},
					{Kind: "lifecycle", LabelUnit: "day", LabelValue: tt.lifecycleDays, Limit: 0, Used: 600, Remaining: 0, Percent: 0, Refreshes: false, ResetAt: 0, Limited: false},
				}

				requireValuePackagePeriodLimitsEqual(t, want, buildValuePackagePeriodLimits(sub, plan, usageDetails))
			})
		}
	})

	t.Run("negative limits are clamped without removing known capabilities", func(t *testing.T) {
		plan := &SubscriptionPlan{
			PlanKind:      SubscriptionPlanKindValuePackage,
			PackageType:   ValuePackageTypeMonth,
			Limit5hAmount: -1,
			Limit7dAmount: -1,
		}
		sub := &UserSubscription{AmountTotal: -1, AmountUsed: 600}
		want := []ValuePackagePeriodLimit{
			{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 0, Used: 100, Remaining: 0, Percent: 0, Refreshes: true, ResetAt: now + 60, Limited: false},
			{Kind: "lifecycle", LabelUnit: "day", LabelValue: 30, Limit: 0, Used: 600, Remaining: 0, Percent: 0, Refreshes: false, ResetAt: 0, Limited: false},
		}

		requireValuePackagePeriodLimitsEqual(t, want, buildValuePackagePeriodLimits(sub, plan, usageDetails))
	})
}

func TestBuildValuePackageUsageSummaryIncludesPeriodLimits(t *testing.T) {
	const now = int64(1783700000)
	sub := &UserSubscription{AmountTotal: 22000, AmountUsed: 600}
	plan := &SubscriptionPlan{
		PlanKind:      SubscriptionPlanKindValuePackage,
		PackageType:   ValuePackageTypeMonth,
		Limit5hAmount: 900,
		Limit7dAmount: 5500,
	}
	details := &ValuePackageWindowUsageDetails{
		Used5h:         100,
		ResetAt5h:      now + 60,
		ResetSeconds5h: 60,
		Used7d:         700,
		ResetAt7d:      now + 120,
		ResetSeconds7d: 120,
	}
	wantPeriods := []ValuePackagePeriodLimit{
		{Kind: "five_hour", LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true, ResetAt: now + 60, Limited: false},
		{Kind: "seven_day_stage", LabelUnit: "day", LabelValue: 7, Limit: 5500, Used: 700, Remaining: 4800, Percent: 700.0 / 5500 * 100, Refreshes: true, ResetAt: now + 120, Limited: false},
		{Kind: "lifecycle", LabelUnit: "day", LabelValue: 30, Limit: 22000, Used: 600, Remaining: 21400, Percent: 600.0 / 22000 * 100, Refreshes: false, ResetAt: 0, Limited: false},
	}

	summary := buildValuePackageUsageSummaryFromDetails(sub, plan, details, now)

	require.NotNil(t, summary)
	requireValuePackagePeriodLimitsEqual(t, wantPeriods, summary.PeriodLimits)
	require.EqualValues(t, 600, summary.TotalUsed)
	require.EqualValues(t, 22000, summary.TotalLimit)
	require.EqualValues(t, 21400, summary.TotalRemaining)
	require.InDelta(t, 600.0/22000*100, summary.TotalPercent, 0.000001)
	require.EqualValues(t, 100, summary.Used5h)
	require.EqualValues(t, 900, summary.Limit5h)
	require.InDelta(t, 100.0/900*100, summary.Percent5h, 0.000001)
	require.EqualValues(t, now+60, summary.ResetAt5h)
	require.EqualValues(t, 60, summary.ResetSeconds5h)
	require.False(t, summary.Limited5h)
	require.EqualValues(t, 700, summary.Used7d)
	require.EqualValues(t, 5500, summary.Limit7d)
	require.InDelta(t, 700.0/5500*100, summary.Percent7d, 0.000001)
	require.EqualValues(t, now+120, summary.ResetAt7d)
	require.EqualValues(t, 120, summary.ResetSeconds7d)
	require.False(t, summary.Limited7d)
	require.False(t, summary.Exhausted)
	require.Empty(t, summary.ExhaustedReason)
	require.Empty(t, summary.ExhaustedMessage)

	payload, err := common.Marshal(summary)
	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, common.Unmarshal(payload, &decoded))

	rawPeriodLimits, ok := decoded["period_limits"]
	require.True(t, ok)
	require.NotNil(t, rawPeriodLimits)
	periodLimits, ok := rawPeriodLimits.([]interface{})
	require.True(t, ok)
	require.Len(t, periodLimits, 3)
	expectedPeriodKeys := []string{"kind", "label_unit", "label_value", "limit", "limited", "percent", "refreshes", "remaining", "reset_at", "used"}
	for i, rawPeriod := range periodLimits {
		period, ok := rawPeriod.(map[string]interface{})
		require.True(t, ok, "period %d must be a JSON object", i)
		require.Equal(t, expectedPeriodKeys, sortedValuePackageJSONKeys(period), "period %d JSON keys", i)
	}
	require.Equal(t, "five_hour", periodLimits[0].(map[string]interface{})["kind"])
	require.Equal(t, "seven_day_stage", periodLimits[1].(map[string]interface{})["kind"])
	require.Equal(t, "lifecycle", periodLimits[2].(map[string]interface{})["kind"])

	legacyNumbers := map[string]float64{
		"total_used":       600,
		"total_limit":      22000,
		"total_remaining":  21400,
		"total_percent":    600.0 / 22000 * 100,
		"used_5h":          100,
		"limit_5h":         900,
		"percent_5h":       100.0 / 900 * 100,
		"reset_at_5h":      float64(now + 60),
		"reset_seconds_5h": 60,
		"used_7d":          700,
		"limit_7d":         5500,
		"percent_7d":       700.0 / 5500 * 100,
		"reset_at_7d":      float64(now + 120),
		"reset_seconds_7d": 120,
	}
	for key, want := range legacyNumbers {
		got, ok := decoded[key].(float64)
		require.True(t, ok, "legacy JSON key %s must exist as a number", key)
		require.InDelta(t, want, got, 0.000001, "legacy JSON key %s", key)
	}
	for _, key := range []string{"limited_5h", "limited_7d", "exhausted"} {
		got, ok := decoded[key].(bool)
		require.True(t, ok, "legacy JSON key %s must exist as a boolean", key)
		require.False(t, got, "legacy JSON key %s", key)
	}
	for _, key := range []string{"exhausted_reason", "exhausted_message"} {
		got, ok := decoded[key].(string)
		require.True(t, ok, "legacy JSON key %s must exist as a string", key)
		require.Empty(t, got, "legacy JSON key %s", key)
	}
}

func requireValuePackagePeriodLimitsEqual(t *testing.T, want []ValuePackagePeriodLimit, got []ValuePackagePeriodLimit) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		require.Equal(t, want[i].Kind, got[i].Kind, "period %d kind", i)
		require.Equal(t, want[i].LabelUnit, got[i].LabelUnit, "period %d label unit", i)
		require.Equal(t, want[i].LabelValue, got[i].LabelValue, "period %d label value", i)
		require.Equal(t, want[i].Limit, got[i].Limit, "period %d limit", i)
		require.Equal(t, want[i].Used, got[i].Used, "period %d used", i)
		require.Equal(t, want[i].Remaining, got[i].Remaining, "period %d remaining", i)
		require.InDelta(t, want[i].Percent, got[i].Percent, 0.000001, "period %d percent", i)
		require.Equal(t, want[i].Refreshes, got[i].Refreshes, "period %d refreshes", i)
		require.Equal(t, want[i].ResetAt, got[i].ResetAt, "period %d reset at", i)
		require.Equal(t, want[i].Limited, got[i].Limited, "period %d limited", i)
	}
}

func sortedValuePackageJSONKeys(value map[string]interface{}) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
