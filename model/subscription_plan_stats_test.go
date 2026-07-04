package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSubscriptionPlanStatsMapAggregatesActiveOnly(t *testing.T) {
	truncateTables(t)
	now := int64(1783200000)
	planA := SubscriptionPlan{Title: "日卡", DurationUnit: SubscriptionDurationDay, DurationValue: 1, Enabled: true, TotalAmount: 1000}
	planB := SubscriptionPlan{Title: "月卡", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 0}
	require.NoError(t, DB.Create(&planA).Error)
	require.NoError(t, DB.Create(&planB).Error)

	subs := []UserSubscription{
		{UserId: 1, PlanId: planA.Id, AmountTotal: 1000, AmountUsed: 200, StartTime: now - 10, EndTime: now + 100, Status: "active"},
		{UserId: 1, PlanId: planA.Id, AmountTotal: 1000, AmountUsed: 400, StartTime: now - 10, EndTime: now + 200, Status: "active"},
		{UserId: 2, PlanId: planA.Id, AmountTotal: 1000, AmountUsed: 1200, StartTime: now - 10, EndTime: now + 300, Status: "active"},
		{UserId: 3, PlanId: planA.Id, AmountTotal: 1000, AmountUsed: 100, StartTime: now - 500, EndTime: now - 1, Status: "active"},
		{UserId: 4, PlanId: planA.Id, AmountTotal: 1000, AmountUsed: 100, StartTime: now - 10, EndTime: now + 400, Status: "cancelled"},
		{UserId: 5, PlanId: planB.Id, AmountTotal: 0, AmountUsed: 0, StartTime: now - 10, EndTime: now + 500, Status: "active"},
	}
	for i := range subs {
		require.NoError(t, DB.Create(&subs[i]).Error)
	}

	stats, err := GetSubscriptionPlanStatsMap(now)
	require.NoError(t, err)

	assert.Equal(t, int64(2), stats[planA.Id].ActiveUserCount)
	assert.Equal(t, int64(3), stats[planA.Id].ActiveSubscriptionCount)
	assert.Equal(t, int64(1400), stats[planA.Id].RemainingAmount)
	assert.Equal(t, int64(0), stats[planA.Id].UnlimitedCount)

	assert.Equal(t, int64(1), stats[planB.Id].ActiveUserCount)
	assert.Equal(t, int64(1), stats[planB.Id].ActiveSubscriptionCount)
	assert.Equal(t, int64(0), stats[planB.Id].RemainingAmount)
	assert.Equal(t, int64(1), stats[planB.Id].UnlimitedCount)
}
