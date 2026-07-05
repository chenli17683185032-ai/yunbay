package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedBillingRatioUserToken(t *testing.T, userId int, walletQuota int, tokenQuota int) {
	t.Helper()
	user := &model.User{Id: userId, Username: "billing_ratio_user", Quota: walletQuota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		Id:          userId,
		UserId:      userId,
		Key:         "billing-ratio-token",
		Name:        "billing_ratio_token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: tokenQuota,
		UsedQuota:   0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedBillingRatioSubscription(t *testing.T, userId int, planTotal int64, used int64) int {
	t.Helper()
	plan := &model.SubscriptionPlan{
		Title:         "billing ratio plan",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   planTotal,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	sub := &model.UserSubscription{
		UserId:      userId,
		PlanId:      plan.Id,
		AmountTotal: planTotal,
		AmountUsed:  used,
		Status:      "active",
		StartTime:   time.Now().Add(-time.Hour).Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
	return sub.Id
}

func newBillingRatioContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Set("token_quota", 100000)
	return c
}

func newBillingRatioRelayInfo(userId int, preference string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		TokenId:         userId,
		TokenKey:        "billing-ratio-token",
		UserId:          userId,
		UsingGroup:      "vip",
		UserGroup:       "vip",
		OriginModelName: "gpt-test",
		RequestId:       "billing-ratio-request",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPreference: preference,
		},
		PriceData: types.PriceData{
			QuotaBeforeGroup:  1000,
			QuotaToPreConsume: 300,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio:        0.3,
				GroupSpecialRatio: -1,
			},
		},
	}
}

func TestNewBillingSessionSubscriptionAppliesOneXGroupRatio(t *testing.T) {
	truncate(t)
	const userId = 101
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	subId := seedBillingRatioSubscription(t, userId, 5000, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "subscription_first")
	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	assert.Equal(t, 1000, session.GetPreConsumedQuota())
	assert.Equal(t, 1000, relayInfo.PriceData.QuotaToPreConsume)
	assert.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.True(t, relayInfo.PriceData.SubscriptionRatioApplied)
	assert.True(t, relayInfo.PriceData.HasOriginalGroupRatioInfo)
	assert.Equal(t, 0.3, relayInfo.PriceData.OriginalGroupRatioInfo.GroupRatio)
	assert.Equal(t, int64(1000), getSubscriptionUsed(t, subId))
}

func TestNewBillingSessionSubscriptionFallbackRestoresWalletRatio(t *testing.T) {
	truncate(t)
	const userId = 102
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	seedBillingRatioSubscription(t, userId, 500, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "subscription_first")
	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceWallet, relayInfo.BillingSource)
	assert.Equal(t, 300, session.GetPreConsumedQuota())
	assert.Equal(t, 300, relayInfo.PriceData.QuotaToPreConsume)
	assert.Equal(t, 0.3, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.False(t, relayInfo.PriceData.SubscriptionRatioApplied)
	assert.Equal(t, 9700, getUserQuota(t, userId))
}

func TestNewBillingSessionWalletFirstKeepsWalletRatioWhenWalletEnough(t *testing.T) {
	truncate(t)
	const userId = 103
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	subId := seedBillingRatioSubscription(t, userId, 5000, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "wallet_first")
	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceWallet, relayInfo.BillingSource)
	assert.Equal(t, 300, session.GetPreConsumedQuota())
	assert.Equal(t, 300, relayInfo.PriceData.QuotaToPreConsume)
	assert.Equal(t, 0.3, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.False(t, relayInfo.PriceData.SubscriptionRatioApplied)
	assert.Equal(t, int64(0), getSubscriptionUsed(t, subId))
}

func TestNewBillingSessionSubscriptionAppliesOneXQuotaForFreeByGroupRatioPerCall(t *testing.T) {
	truncate(t)
	const userId = 105
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	seedBillingRatioSubscription(t, userId, 5000, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "subscription_first")
	relayInfo.PriceData = types.PriceData{
		FreeModel:         true,
		FreeByGroupRatio:  true,
		QuotaBeforeGroup:  1000,
		Quota:             0,
		QuotaToPreConsume: 0,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:        0,
			GroupSpecialRatio: -1,
		},
	}

	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.Quota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	assert.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 1000, relayInfo.PriceData.Quota)
	assert.Equal(t, 1000, relayInfo.PriceData.QuotaToPreConsume)
	assert.Equal(t, 1000, session.GetPreConsumedQuota())
}

func TestGenerateTextOtherInfoIncludesSubscriptionRatioAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	now := time.Now()
	relayInfo := &relaycommon.RelayInfo{
		ChannelMeta:       &relaycommon.ChannelMeta{},
		BillingSource:     BillingSourceSubscription,
		StartTime:         now,
		FirstResponseTime: now.Add(10 * time.Millisecond),
		PriceData: types.PriceData{
			SubscriptionRatioApplied:  true,
			HasOriginalGroupRatioInfo: true,
			OriginalGroupRatioInfo: types.GroupRatioInfo{
				GroupRatio:        0.3,
				GroupSpecialRatio: -1,
				HasSpecialRatio:   false,
			},
		},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 2, 1, 1, 0, 0, 0, -1)
	assert.Equal(t, BillingSourceSubscription, other["billing_source"])
	assert.Equal(t, true, other["subscription_ratio_applied"])
	assert.Equal(t, 0.3, other["original_group_ratio"])
	assert.Equal(t, -1.0, other["original_user_group_ratio"])
}

func TestNewBillingSessionSubscriptionAppliesOneXToTieredSnapshot(t *testing.T) {
	truncate(t)
	const userId = 104
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	subId := seedBillingRatioSubscription(t, userId, 5000, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "subscription_first")
	relayInfo.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		BillingMode:               "tiered_expr",
		GroupRatio:                0.3,
		EstimatedQuotaBeforeGroup: 1000,
		EstimatedQuotaAfterGroup:  300,
	}

	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	assert.Equal(t, 1000, session.GetPreConsumedQuota())
	assert.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 1.0, relayInfo.TieredBillingSnapshot.GroupRatio)
	assert.Equal(t, 1000, relayInfo.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
	assert.Equal(t, int64(1000), getSubscriptionUsed(t, subId))
}

func TestRetryPreservesSubscriptionRatioAfterPriceDataRecalculation(t *testing.T) {
	truncate(t)
	const userId = 106
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	seedBillingRatioSubscription(t, userId, 5000, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "subscription_first")
	relayInfo.PriceData.Quota = 300
	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.Quota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session
	require.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	require.Equal(t, 1000, relayInfo.PriceData.Quota)
	require.True(t, relayInfo.PriceData.SubscriptionRatioApplied)

	// Simulate a task retry: ModelPriceHelperPerCall recalculates from the original
	// wallet/group snapshot and overwrites PriceData, while the existing billing
	// session remains subscription-backed.
	relayInfo.PriceData = types.PriceData{
		QuotaBeforeGroup:  1000,
		Quota:             300,
		QuotaToPreConsume: 300,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:        0.3,
			GroupSpecialRatio: -1,
		},
	}

	EnsureSubscriptionBillingRatio(relayInfo)

	assert.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 1000, relayInfo.PriceData.Quota)
	assert.Equal(t, 1000, relayInfo.PriceData.QuotaToPreConsume)
	assert.True(t, relayInfo.PriceData.SubscriptionRatioApplied)
	assert.True(t, relayInfo.PriceData.HasOriginalGroupRatioInfo)
	assert.Equal(t, 0.3, relayInfo.PriceData.OriginalGroupRatioInfo.GroupRatio)
}

func TestCompactTextRepriceCanRestoreSubscriptionRatioSnapshot(t *testing.T) {
	truncate(t)
	const userId = 107
	seedBillingRatioUserToken(t, userId, 10000, 10000)
	seedBillingRatioSubscription(t, userId, 5000, 0)

	relayInfo := newBillingRatioRelayInfo(userId, "subscription_first")
	session, apiErr := NewBillingSession(newBillingRatioContext(), relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session
	require.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	require.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, 1000, relayInfo.PriceData.QuotaToPreConsume)

	// Simulate /v1/responses/compact recalculating text pricing after the outer
	// request already selected subscription funding. Text pricing has Quota=0,
	// but settlement must still use the subscription 1x ratio snapshot.
	relayInfo.PriceData = types.PriceData{
		QuotaBeforeGroup:  1000,
		Quota:             0,
		QuotaToPreConsume: 300,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:        0.3,
			GroupSpecialRatio: -1,
		},
	}

	EnsureSubscriptionBillingRatio(relayInfo)

	assert.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 1000, relayInfo.PriceData.QuotaToPreConsume)
	assert.Equal(t, 0, relayInfo.PriceData.Quota)
	assert.True(t, relayInfo.PriceData.SubscriptionRatioApplied)
	assert.True(t, relayInfo.PriceData.HasOriginalGroupRatioInfo)
	assert.Equal(t, 0.3, relayInfo.PriceData.OriginalGroupRatioInfo.GroupRatio)
}

func TestQuotaWithGroupRatioMatchesTaskPipelineStepwiseOtherRatios(t *testing.T) {
	priceData := types.PriceData{
		QuotaBeforeGroup: 333,
		OtherRatios: map[string]float64{
			"a": 1.5,
			"b": 1.5,
		},
	}

	assert.Equal(t, 748, quotaWithGroupRatio(priceData, 1))
}

func TestGenerateMjOtherInfoIncludesValuePackageBillingAudit(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		ValuePackageSubscriptionId: 123,
		ValuePackagePlanId:         456,
		ValuePackageModelGroup:     "month-card",
		ValuePackagePackageType:    "month",
	}
	priceData := types.PriceData{
		ModelPrice: 0.02,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:        1,
			GroupSpecialRatio: -1,
			HasSpecialRatio:   false,
		},
		SubscriptionRatioApplied:  true,
		HasOriginalGroupRatioInfo: true,
		OriginalGroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:        0.3,
			GroupSpecialRatio: -1,
			HasSpecialRatio:   false,
		},
	}

	other := GenerateMjOtherInfo(relayInfo, priceData)

	assert.Equal(t, true, other["subscription_ratio_applied"])
	assert.Equal(t, 0.3, other["original_group_ratio"])
	assert.Equal(t, -1.0, other["original_user_group_ratio"])
	assert.Equal(t, 123, other["value_package_subscription_id"])
	assert.Equal(t, 456, other["value_package_plan_id"])
	assert.Equal(t, "month-card", other["value_package_model_group"])
	assert.Equal(t, "month", other["value_package_package_type"])
	assert.Equal(t, 1.0, other["value_package_effective_ratio"])
}
