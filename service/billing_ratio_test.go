package service

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRealtimeBilling struct {
	deltas []int
}

func (b *recordingRealtimeBilling) Settle(int) error         { return nil }
func (b *recordingRealtimeBilling) Refund(*gin.Context)      {}
func (b *recordingRealtimeBilling) NeedsRefund() bool        { return false }
func (b *recordingRealtimeBilling) GetPreConsumedQuota() int { return 0 }
func (b *recordingRealtimeBilling) Reserve(int) error        { return nil }
func (b *recordingRealtimeBilling) ReserveRealtime(delta int) (int, error) {
	b.deltas = append(b.deltas, delta)
	return delta, nil
}

func preserveRealtimeRatioSettings(t *testing.T) {
	t.Helper()
	oldGroupRatios := ratio_setting.GroupRatio2JSONString()
	oldGroupGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	oldModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldGroupGroupRatios))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModelRatios))
	})
}

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

func TestResolveSubscriptionBillingRatio(t *testing.T) {
	tests := []struct {
		name       string
		info       *relaycommon.RelayInfo
		wantRatio  float64
		wantSource string
	}{
		{
			name:       "nil relay info is regular subscription",
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceRegular,
		},
		{
			name: "regular subscription ignores special ratio",
			info: &relaycommon.RelayInfo{PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.6, GroupSpecialRatio: 0.6, HasSpecialRatio: true,
			}}},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceRegular,
		},
		{
			name: "value package uses explicit special ratio",
			info: &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1, PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.6, GroupSpecialRatio: 0.6, HasSpecialRatio: true,
			}}},
			wantRatio:  0.6,
			wantSource: SubscriptionRatioSourceConfigured,
		},
		{
			name:       "value package missing pair defaults to one",
			info:       &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceDefault,
		},
		{
			name: "value package does not infer a pair from ordinary group ratio",
			info: &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1, PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.6, GroupSpecialRatio: 0.6, HasSpecialRatio: false,
			}}},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceDefault,
		},
		{
			name: "value package does not fall back to group ratio when special ratio is invalid",
			info: &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1, PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.6, GroupSpecialRatio: 0, HasSpecialRatio: true,
			}}},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceDefault,
		},
	}
	for _, invalid := range []struct {
		name  string
		ratio float64
	}{
		{name: "zero", ratio: 0},
		{name: "negative", ratio: -1},
		{name: "nan", ratio: math.NaN()},
		{name: "positive infinity", ratio: math.Inf(1)},
		{name: "negative infinity", ratio: math.Inf(-1)},
	} {
		tests = append(tests, struct {
			name       string
			info       *relaycommon.RelayInfo
			wantRatio  float64
			wantSource string
		}{
			name: "value package invalid special ratio " + invalid.name,
			info: &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1, PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: invalid.ratio, GroupSpecialRatio: invalid.ratio, HasSpecialRatio: true,
			}}},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceDefault,
		})
	}
	tests = append(tests,
		struct {
			name       string
			info       *relaycommon.RelayInfo
			wantRatio  float64
			wantSource string
		}{
			name: "applied ratio is frozen instead of reading original candidate",
			info: &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1, PriceData: types.PriceData{
				SubscriptionRatioApplied: true,
				SubscriptionRatioSource:  SubscriptionRatioSourceConfigured,
				GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: 0.7},
				OriginalGroupRatioInfo:   types.GroupRatioInfo{GroupRatio: 1.8, GroupSpecialRatio: 1.8, HasSpecialRatio: true},
			}},
			wantRatio:  0.7,
			wantSource: SubscriptionRatioSourceConfigured,
		},
		struct {
			name       string
			info       *relaycommon.RelayInfo
			wantRatio  float64
			wantSource string
		}{
			name: "corrupt applied value package ratio defaults safely",
			info: &relaycommon.RelayInfo{ValuePackageSubscriptionId: 1, PriceData: types.PriceData{
				SubscriptionRatioApplied: true,
				SubscriptionRatioSource:  SubscriptionRatioSourceConfigured,
				GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: math.NaN()},
			}},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceDefault,
		},
		struct {
			name       string
			info       *relaycommon.RelayInfo
			wantRatio  float64
			wantSource string
		}{
			name: "corrupt applied regular subscription ratio defaults safely",
			info: &relaycommon.RelayInfo{PriceData: types.PriceData{
				SubscriptionRatioApplied: true,
				SubscriptionRatioSource:  SubscriptionRatioSourceRegular,
				GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: math.Inf(1)},
			}},
			wantRatio:  1,
			wantSource: SubscriptionRatioSourceRegular,
		},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, source := resolveSubscriptionBillingRatio(tt.info)
			require.Equal(t, tt.wantRatio, ratio)
			require.Equal(t, tt.wantSource, source)
		})
	}
}

func TestNewBillingSessionRegularSubscriptionBillingRatioIsOneX(t *testing.T) {
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
	assert.Equal(t, SubscriptionRatioSourceRegular, relayInfo.PriceData.SubscriptionRatioSource)
	assert.True(t, relayInfo.PriceData.SubscriptionRatioApplied)
	assert.True(t, relayInfo.PriceData.HasOriginalGroupRatioInfo)
	assert.Equal(t, 0.3, relayInfo.PriceData.OriginalGroupRatioInfo.GroupRatio)
	assert.Equal(t, int64(1000), getSubscriptionUsed(t, subId))
}

func TestNewBillingSessionSubscriptionFallbackRestoresWalletBillingRatio(t *testing.T) {
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
	assert.Empty(t, relayInfo.PriceData.SubscriptionRatioSource)
	assert.Equal(t, 9700, getUserQuota(t, userId))
}

func TestNewBillingSessionWalletFirstKeepsWalletBillingRatioWhenWalletEnough(t *testing.T) {
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
	assert.Empty(t, relayInfo.PriceData.SubscriptionRatioSource)
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

func TestApplySubscriptionBillingRatioUsesConfiguredRatioOnceForTieredSnapshot(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		ValuePackageSubscriptionId: 1,
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			GroupRatio:                0.3,
			EstimatedQuotaBeforeGroup: 1000,
			EstimatedQuotaAfterGroup:  300,
		},
		PriceData: types.PriceData{
			QuotaBeforeGroup:  1000,
			QuotaToPreConsume: 300,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 0.3},
		},
	}

	applySubscriptionBillingRatio(relayInfo, 1500, 1.5, SubscriptionRatioSourceConfigured)
	applySubscriptionBillingRatio(relayInfo, 2000, 2, SubscriptionRatioSourceConfigured)

	require.Equal(t, 1.5, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, 1.5, relayInfo.TieredBillingSnapshot.GroupRatio)
	require.Equal(t, 1500, relayInfo.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
	require.Equal(t, 1500, relayInfo.PriceData.QuotaToPreConsume)
	require.Equal(t, SubscriptionRatioSourceConfigured, relayInfo.PriceData.SubscriptionRatioSource)
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

func TestEnsureSubscriptionBillingRatioKeepsFrozenValuePackageRatioAfterReprice(t *testing.T) {
	preserveRealtimeRatioSettings(t)
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"vip":0.6}}`))
	relayInfo := &relaycommon.RelayInfo{
		BillingSource:              BillingSourceSubscription,
		ValuePackageSubscriptionId: 42,
		ValuePackageBillingGroup:   "month-card",
		UsingGroup:                 "vip",
		PriceData: types.PriceData{
			QuotaBeforeGroup:  1000,
			QuotaToPreConsume: 1800,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1.8, GroupSpecialRatio: 1.8, HasSpecialRatio: true,
			},
		},
	}
	session := &BillingSession{
		relayInfo:               relayInfo,
		funding:                 &SubscriptionFunding{valuePackageSubscriptionId: 42},
		preConsumedQuota:        600,
		subscriptionRatio:       0.6,
		subscriptionRatioSource: SubscriptionRatioSourceConfigured,
	}
	relayInfo.Billing = session
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"vip":1.8}}`))

	EnsureSubscriptionBillingRatio(relayInfo)

	require.Equal(t, 0.6, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, 600, relayInfo.PriceData.QuotaToPreConsume)
	require.Equal(t, SubscriptionRatioSourceConfigured, relayInfo.PriceData.SubscriptionRatioSource)
	require.Equal(t, 1.8, relayInfo.PriceData.OriginalGroupRatioInfo.GroupRatio)
}

func TestRealtimePreWssConsumeQuotaKeepsFrozenSubscriptionRatioAcrossConfigChange(t *testing.T) {
	preserveRealtimeRatioSettings(t)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"billing-ratio-realtime":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":0.2,"vip":2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"default":0.5}}`))

	billing := &recordingRealtimeBilling{}
	ctx := newBillingRatioContext()
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName:            "billing-ratio-realtime",
		UsingGroup:                 "default",
		BillingUserGroup:           "month-card",
		ValuePackageSubscriptionId: 1,
		Billing:                    billing,
		PriceData: types.PriceData{
			SubscriptionRatioApplied: true,
			SubscriptionRatioSource:  SubscriptionRatioSourceConfigured,
			GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: 0.5},
		},
	}
	usage := &dto.RealtimeUsage{InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}}

	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"default":3,"vip":4}}`))
	ctx.Set("auto_group", "vip")
	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))

	require.Equal(t, []int{10, 10}, billing.deltas)
	require.Equal(t, "default", relayInfo.UsingGroup)
}

func TestRealtimePreWssConsumeQuotaUsesOneForValuePackageWithoutConfiguredPair(t *testing.T) {
	preserveRealtimeRatioSettings(t)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"billing-ratio-realtime":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))

	billing := &recordingRealtimeBilling{}
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName:            "billing-ratio-realtime",
		UsingGroup:                 "default",
		BillingUserGroup:           "month-card",
		ValuePackageSubscriptionId: 1,
		Billing:                    billing,
		PriceData: types.PriceData{
			SubscriptionRatioApplied: true,
			SubscriptionRatioSource:  SubscriptionRatioSourceDefault,
			GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: 1},
		},
	}

	require.NoError(t, PreWssConsumeQuota(newBillingRatioContext(), relayInfo, &dto.RealtimeUsage{
		InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20},
	}))

	require.Equal(t, []int{20}, billing.deltas)
}

func TestRealtimePreWssConsumeQuotaWalletStillReadsCurrentGroupRatio(t *testing.T) {
	preserveRealtimeRatioSettings(t)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"billing-ratio-realtime":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":0.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))

	billing := &recordingRealtimeBilling{}
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "billing-ratio-realtime",
		UsingGroup:      "default",
		UserGroup:       "wallet-user",
		Billing:         billing,
	}
	usage := &dto.RealtimeUsage{InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}}

	require.NoError(t, PreWssConsumeQuota(newBillingRatioContext(), relayInfo, usage))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2}`))
	require.NoError(t, PreWssConsumeQuota(newBillingRatioContext(), relayInfo, usage))

	require.Equal(t, []int{10, 40}, billing.deltas)
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
