package model

const (
	ValuePackagePeriodKindFiveHour      = "five_hour"
	ValuePackagePeriodKindSevenDayStage = "seven_day_stage"
	ValuePackagePeriodKindLifecycle     = "lifecycle"
)

type ValuePackagePeriodLimit struct {
	Kind       string  `json:"kind"`
	LabelUnit  string  `json:"label_unit"`
	LabelValue int     `json:"label_value"`
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percent    float64 `json:"percent"`
	Refreshes  bool    `json:"refreshes"`
	ResetAt    int64   `json:"reset_at"`
	Limited    bool    `json:"limited"`
}

func valuePackageRemaining(limit int64, used int64) int64 {
	if limit <= 0 {
		return 0
	}
	remaining := limit - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

func valuePackageClampNonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func buildValuePackagePeriodLimits(sub *UserSubscription, plan *SubscriptionPlan, details *ValuePackageWindowUsageDetails) []ValuePackagePeriodLimit {
	if sub == nil || plan == nil || details == nil || !plan.IsValuePackage() {
		return nil
	}

	lifecycleDays := 0
	switch plan.PackageType {
	case ValuePackageTypeDay:
		lifecycleDays = 1
	case ValuePackageTypeWeek:
		lifecycleDays = 7
	case ValuePackageTypeMonth:
		lifecycleDays = 30
	}

	periods := make([]ValuePackagePeriodLimit, 0, 3)
	fiveHourLimit := valuePackageClampNonNegative(plan.Limit5hAmount)
	if lifecycleDays > 0 || fiveHourLimit > 0 {
		periods = append(periods, ValuePackagePeriodLimit{
			Kind:       ValuePackagePeriodKindFiveHour,
			LabelUnit:  "hour",
			LabelValue: 5,
			Limit:      fiveHourLimit,
			Used:       details.Used5h,
			Remaining:  valuePackageRemaining(fiveHourLimit, details.Used5h),
			Percent:    valuePackagePercent(details.Used5h, fiveHourLimit),
			Refreshes:  true,
			ResetAt:    details.ResetAt5h,
			Limited:    fiveHourLimit > 0 && details.Used5h >= fiveHourLimit,
		})
	}

	if plan.PackageType == ValuePackageTypeMonth && plan.Limit7dAmount > 0 {
		sevenDayLimit := plan.Limit7dAmount
		periods = append(periods, ValuePackagePeriodLimit{
			Kind:       ValuePackagePeriodKindSevenDayStage,
			LabelUnit:  "day",
			LabelValue: 7,
			Limit:      sevenDayLimit,
			Used:       details.Used7d,
			Remaining:  valuePackageRemaining(sevenDayLimit, details.Used7d),
			Percent:    valuePackagePercent(details.Used7d, sevenDayLimit),
			Refreshes:  true,
			ResetAt:    details.ResetAt7d,
			Limited:    sevenDayLimit > 0 && details.Used7d >= sevenDayLimit,
		})
	}

	if lifecycleDays > 0 {
		lifecycleLimit := valuePackageClampNonNegative(sub.AmountTotal)
		periods = append(periods, ValuePackagePeriodLimit{
			Kind:       ValuePackagePeriodKindLifecycle,
			LabelUnit:  "day",
			LabelValue: lifecycleDays,
			Limit:      lifecycleLimit,
			Used:       sub.AmountUsed,
			Remaining:  valuePackageRemaining(lifecycleLimit, sub.AmountUsed),
			Percent:    valuePackagePercent(sub.AmountUsed, lifecycleLimit),
			Refreshes:  false,
			ResetAt:    0,
			Limited:    lifecycleLimit > 0 && sub.AmountUsed >= lifecycleLimit,
		})
	}

	return periods
}
