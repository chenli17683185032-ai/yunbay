package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMidjourneyBillingContextRoundTripAndLegacyCompatibility(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Midjourney{}))

	task := &Midjourney{
		UserId: 1,
		MjId:   "mj-billing-context",
		BillingContext: MidjourneyBillingContext{
			BillingSource:              "subscription",
			SubscriptionId:             12,
			RequestId:                  "mj-request",
			TokenId:                    34,
			ValuePackageSubscriptionId: 56,
			ValuePackagePlanId:         78,
			ValuePackageBillingGroup:   "month-card",
			BillingUsingGroup:          "group-a",
			EffectiveGroupRatio:        0.45,
			SubscriptionRatioSource:    "configured",
		},
	}
	require.NoError(t, task.Insert())

	var stored Midjourney
	require.NoError(t, DB.First(&stored, task.Id).Error)
	require.Equal(t, task.BillingContext, stored.BillingContext)

	legacy := &Midjourney{UserId: 1, MjId: "mj-legacy"}
	require.NoError(t, legacy.Insert())
	var storedLegacy Midjourney
	require.NoError(t, DB.First(&storedLegacy, legacy.Id).Error)
	require.Equal(t, MidjourneyBillingContext{}, storedLegacy.BillingContext)

	value, err := task.BillingContext.Value()
	require.NoError(t, err)
	encoded, ok := value.(string)
	require.True(t, ok)
	var fromString MidjourneyBillingContext
	require.NoError(t, fromString.Scan(encoded))
	require.Equal(t, task.BillingContext, fromString)
	var fromBytes MidjourneyBillingContext
	require.NoError(t, fromBytes.Scan([]byte(encoded)))
	require.Equal(t, task.BillingContext, fromBytes)
}
