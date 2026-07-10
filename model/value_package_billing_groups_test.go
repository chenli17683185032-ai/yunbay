package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListEnabledValuePackageBillingGroupsReturnsDistinctSortedGroups(t *testing.T) {
	setupValuePackageTestDB(t)

	plans := []SubscriptionPlan{
		{Title: "Day", Enabled: true, PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, ModelGroup: " day-card "},
		{Title: "Week", Enabled: true, PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeWeek, ModelGroup: "week-card"},
		{Title: "Month", Enabled: true, PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeMonth, ModelGroup: "month-card"},
		{Title: "Duplicate", Enabled: true, PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, ModelGroup: "day-card"},
		{Title: "Blank", Enabled: true, PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, ModelGroup: "   "},
		{Title: "Disabled", Enabled: false, PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, ModelGroup: "disabled-card"},
		{Title: "Regular", Enabled: true, PlanKind: SubscriptionPlanKindSubscription, ModelGroup: "regular-group"},
	}
	for i := range plans {
		desiredEnabled := plans[i].Enabled
		require.NoError(t, DB.Create(&plans[i]).Error)
		if !desiredEnabled {
			require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plans[i].Id).Update("enabled", false).Error)
		}
	}

	groups, err := ListEnabledValuePackageBillingGroups()

	require.NoError(t, err)
	require.Equal(t, []string{"day-card", "month-card", "week-card"}, groups)
}
