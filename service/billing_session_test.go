package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	_ "unsafe"
)

//go:linkname initModelColumns github.com/QuantumNous/new-api/model.initCol
func initModelColumns()

func setupValuePackageBillingSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initModelColumns()
	ratio_setting.InitRatioSettings()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}, &model.UserValuePackagePreference{}, &model.ValuePackageUsageRecord{}, &model.ValuePackageQuotaReset{}, &model.ValuePackageResetCountLedger{}, &model.Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initModelColumns()
	})
	return db
}

func TestBillingSessionQuotaSnapshotTracksActualLegs(t *testing.T) {
	for _, tt := range []struct {
		name        string
		force       bool
		playground  bool
		wantFunding int
		wantToken   int
	}{
		{name: "ordinary force preconsume", force: true, wantFunding: 100, wantToken: 100},
		{name: "playground", force: true, playground: true, wantFunding: 100, wantToken: 0},
		{name: "trusted", force: false, wantFunding: 0, wantToken: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupValuePackageBillingSessionTestDB(t)
			quota := common.GetTrustQuota() + 1000
			user := model.User{Username: "billing-snapshot", Status: common.UserStatusEnabled, Quota: quota}
			require.NoError(t, model.DB.Create(&user).Error)
			token := model.Token{
				UserId: user.Id, Key: "billing-snapshot-token", Name: "billing-snapshot-token",
				Status: common.TokenStatusEnabled, RemainQuota: quota,
			}
			require.NoError(t, model.DB.Create(&token).Error)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Set("token_quota", quota)
			info := &relaycommon.RelayInfo{
				UserId: user.Id, UserQuota: quota, TokenId: token.Id, TokenKey: token.Key,
				OriginModelName: "gpt-test", IsPlayground: tt.playground, ForcePreConsume: tt.force,
				UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
				PriceData:   types.PriceData{QuotaToPreConsume: 100},
			}

			session, apiErr := NewBillingSession(ctx, info, 100)

			require.Nil(t, apiErr)
			require.Equal(t, relaycommon.BillingQuotaSnapshot{
				FundingQuota: tt.wantFunding,
				TokenQuota:   tt.wantToken,
			}, session.GetQuotaSnapshot())
		})
	}
}

func TestValuePackageBillingIgnoresWalletOnlyPreference(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-billing-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, Quota: 1000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "day card", PriceAmount: 3.9, Currency: "USD", DurationUnit: model.SubscriptionDurationDay, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeDay, PackageLevel: model.ValuePackageLevelDay, ModelGroup: "day-card", ConcurrencyLimit: 1, TotalAmount: 10000}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: now - 10, EndTime: now + int64(time.Hour/time.Second), Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     user.Id,
		RequestId:                  "vp-billing-request",
		OriginModelName:            "gpt-test",
		IsPlayground:               true,
		UserSetting:                dto.UserSetting{BillingPreference: "wallet_only"},
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	require.Equal(t, sub.Id, relayInfo.SubscriptionId)
	require.EqualValues(t, 100, relayInfo.SubscriptionPreConsumed)

	var reloadedUser model.User
	require.NoError(t, model.DB.First(&reloadedUser, user.Id).Error)
	require.Equal(t, 1000, reloadedUser.Quota)
	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 100, reloadedSub.AmountUsed)

	require.NoError(t, session.Settle(150))
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 150, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 150, used5h)
	require.EqualValues(t, 0, used7d)
}

func TestValuePackageBillingFallsBackToWalletWithUserGroupRatio(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	preserveRealtimeRatioSettings(t)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-plus":0.3}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"gpt-plus":0.5},"vip":{"gpt-plus":2}}`))

	user := model.User{Username: "vp-wallet-fallback", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 1000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "fallback month card", Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, ConcurrencyLimit: 1, TotalAmount: 100}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 100, StartTime: now - 10, EndTime: now + 86400, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     user.Id,
		RequestId:                  "vp-wallet-fallback-request",
		OriginModelName:            "gpt-test",
		IsPlayground:               true,
		UsingGroup:                 "gpt-plus",
		UserGroup:                  model.UserGroupVIP,
		RealUserGroup:              model.UserGroupVIP,
		BillingUserGroup:           plan.ModelGroup,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageBillingGroup:   plan.ModelGroup,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		ValuePackageWalletFallback: true,
		PriceData: types.PriceData{
			QuotaBeforeGroup:  0,
			QuotaToPreConsume: 50,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio:        0.5,
				GroupSpecialRatio: 0.5,
				HasSpecialRatio:   true,
			},
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, 50)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceWallet, relayInfo.BillingSource)
	require.Equal(t, model.UserGroupVIP, relayInfo.BillingUserGroup)
	require.Equal(t, 2.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, 200, session.GetPreConsumedQuota())
	require.Zero(t, relayInfo.ValuePackageSubscriptionId)
	require.Empty(t, relayInfo.ValuePackageBillingGroup)
	require.True(t, relayInfo.ValuePackageUseWallet)

	var reloadedUser model.User
	require.NoError(t, model.DB.First(&reloadedUser, user.Id).Error)
	require.Equal(t, 800, reloadedUser.Quota)
	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 100, reloadedSub.AmountUsed)
}

func TestValuePackageBillingDoesNotFallbackWhenDisabled(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-wallet-fallback-disabled", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 1000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "strict month card", Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, ConcurrencyLimit: 1, TotalAmount: 100}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 100, StartTime: now - 10, EndTime: now + 86400, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     user.Id,
		RequestId:                  "vp-wallet-fallback-disabled-request",
		OriginModelName:            "gpt-test",
		IsPlayground:               true,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageBillingGroup:   plan.ModelGroup,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, 50)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	var reloadedUser model.User
	require.NoError(t, model.DB.First(&reloadedUser, user.Id).Error)
	require.Equal(t, 1000, reloadedUser.Quota)
}

func TestValuePackageBillingDefaultsToOneXWithoutConfiguredPair(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-billing-ratio-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 1000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "month card", PriceAmount: 19.9, Currency: "CNY", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "plus", ConcurrencyLimit: 1, TotalAmount: 10000}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: now - 10, EndTime: now + int64(time.Hour/time.Second), Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     user.Id,
		RequestId:                  "vp-billing-ratio-request",
		OriginModelName:            "gpt-plus",
		IsPlayground:               true,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			QuotaBeforeGroup:  1000,
			QuotaToPreConsume: 300,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio:        0.3,
				GroupSpecialRatio: -1,
			},
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, relayInfo.PriceData.QuotaToPreConsume)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	require.Equal(t, 1000, session.GetPreConsumedQuota())
	require.Equal(t, 1000, relayInfo.PriceData.QuotaToPreConsume)
	require.Equal(t, 1.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, SubscriptionRatioSourceDefault, relayInfo.PriceData.SubscriptionRatioSource)
	require.True(t, relayInfo.PriceData.SubscriptionRatioApplied)
	require.True(t, relayInfo.PriceData.HasOriginalGroupRatioInfo)
	require.Equal(t, 0.3, relayInfo.PriceData.OriginalGroupRatioInfo.GroupRatio)

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 1000, reloadedSub.AmountUsed)
}

func TestValuePackageBillingAppliesConfiguredSpecialRatio(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-billing-configured-ratio-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 1000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "configured month card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", ConcurrencyLimit: 1, TotalAmount: 10000}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: now - 10, EndTime: now + int64(time.Hour/time.Second), Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     user.Id,
		RequestId:                  "vp-billing-configured-ratio-request",
		OriginModelName:            "gpt-plus",
		IsPlayground:               true,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageBillingGroup:   plan.ModelGroup,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		BillingUserGroup:           plan.ModelGroup,
		PriceData: types.PriceData{
			QuotaBeforeGroup:  1000,
			QuotaToPreConsume: 600,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.6, GroupSpecialRatio: 0.6, HasSpecialRatio: true,
			},
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, relayInfo.PriceData.QuotaToPreConsume)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	require.Equal(t, 600, session.GetPreConsumedQuota())
	require.Equal(t, 600, relayInfo.PriceData.QuotaToPreConsume)
	require.Equal(t, 0.6, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, SubscriptionRatioSourceConfigured, relayInfo.PriceData.SubscriptionRatioSource)
	require.True(t, relayInfo.PriceData.SubscriptionRatioApplied)

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 600, reloadedSub.AmountUsed)
}

func TestValuePackageBillingSettleZeroActualQuotaClearsUsageReservation(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-billing-zero-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, Quota: 1000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "zero day card", PriceAmount: 3.9, Currency: "USD", DurationUnit: model.SubscriptionDurationDay, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeDay, PackageLevel: model.ValuePackageLevelDay, ModelGroup: "day-card", ConcurrencyLimit: 1, TotalAmount: 10000}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: now - 10, EndTime: now + int64(time.Hour/time.Second), Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     user.Id,
		RequestId:                  "vp-billing-zero-request",
		OriginModelName:            "gpt-test",
		IsPlayground:               true,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 100, used5h)
	require.EqualValues(t, 0, used7d)

	require.NoError(t, session.Settle(0))
	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 0, reloadedSub.AmountUsed)
	used5h, used7d, err = model.GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, used5h)
	require.EqualValues(t, 0, used7d)
}

func TestValuePackageBillingSettleAfterResetDoesNotChangeNewEpochUsage(t *testing.T) {
	for _, actualQuota := range []int{40, 160} {
		t.Run(fmt.Sprintf("actual_%d", actualQuota), func(t *testing.T) {
			setupValuePackageBillingSessionTestDB(t)
			user := model.User{Username: fmt.Sprintf("vp-reset-settle-%d", actualQuota), Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 1000}
			require.NoError(t, model.DB.Create(&user).Error)
			plan := model.SubscriptionPlan{Title: "reset settle week", Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeWeek, PackageLevel: model.ValuePackageLevelWeek, ModelGroup: "week-card", DurationUnit: model.SubscriptionDurationDay, DurationValue: 7, ConcurrencyLimit: 1, TotalAmount: 1000}
			require.NoError(t, model.DB.Create(&plan).Error)
			now := common.GetTimestamp()
			sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: now - 10, EndTime: now + 86400, Status: model.UserSubscriptionStatusActive}
			require.NoError(t, model.DB.Create(&sub).Error)
			require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			relayInfo := &relaycommon.RelayInfo{UserId: user.Id, RequestId: fmt.Sprintf("vp-reset-settle-%d", actualQuota), OriginModelName: "gpt-test", IsPlayground: true, ValuePackageSubscriptionId: sub.Id, ValuePackagePlanId: plan.Id, ValuePackageModelGroup: plan.ModelGroup, ValuePackagePackageType: plan.PackageType}
			session, apiErr := NewBillingSession(ctx, relayInfo, 100)
			require.Nil(t, apiErr)
			require.NotNil(t, session)
			_, err := model.ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "settle epoch test")
			require.NoError(t, err)

			require.NoError(t, session.Settle(actualQuota))
			var reloaded model.UserSubscription
			require.NoError(t, model.DB.First(&reloaded, sub.Id).Error)
			require.Zero(t, reloaded.AmountUsed)
			require.EqualValues(t, 1, reloaded.QuotaEpoch)
			var usage model.ValuePackageUsageRecord
			require.NoError(t, model.DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, relayInfo.RequestId).First(&usage).Error)
			require.EqualValues(t, actualQuota, usage.Quota)
			require.Zero(t, usage.QuotaEpoch)
		})
	}
}

func TestValuePackageBillingSettleReturnsUsageRecordError(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{
			UserId:                     1,
			ValuePackageSubscriptionId: 1,
			ValuePackagePlanId:         1,
			ValuePackageModelGroup:     "day-card",
			ValuePackagePackageType:    model.ValuePackageTypeDay,
		},
		preConsumedQuota: 10,
	}

	err := session.Settle(10)

	require.Error(t, err)
	require.Contains(t, err.Error(), "requestId")
}

func TestRealtimeValuePackageReserveDoesNotDoubleCountOnFinalSettle(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-token", Status: common.TokenStatusEnabled, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 100000, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		IsPlayground:               false,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaBeforeGroup:  0,
			QuotaToPreConsume: 1,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	usage := &dto.RealtimeUsage{TotalTokens: 20, InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}}
	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	require.NoError(t, SettleBilling(ctx, relayInfo, 20))

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 20, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 20, used5h)
	require.EqualValues(t, 20, used7d)
}

func TestRealtimePostWssConsumeQuotaSettlesAndLogsFrozenSubscriptionBillingRatio(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	preserveRealtimeRatioSettings(t)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"billing-ratio-realtime":1}`))
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldDataExportEnabled := common.DataExportEnabled
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	common.DataExportEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		common.DataExportEnabled = oldDataExportEnabled
	})

	user := model.User{Username: "vp-realtime-post-ratio-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	channel := model.Channel{Name: "vp-realtime-post-ratio-channel", Key: "test", Status: common.ChannelStatusEnabled, Group: "default"}
	require.NoError(t, model.DB.Create(&channel).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Frozen Ratio", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 100000, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-post-ratio-request",
		UserId:                     user.Id,
		ChannelMeta:                &relaycommon.ChannelMeta{ChannelId: channel.Id},
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		BillingUserGroup:           plan.ModelGroup,
		ValuePackageBillingGroup:   plan.ModelGroup,
		OriginModelName:            "billing-ratio-realtime",
		StartTime:                  time.Now(),
		IsPlayground:               true,
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaBeforeGroup:  10,
			QuotaToPreConsume: 6,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.6, GroupSpecialRatio: 0.6, HasSpecialRatio: true,
			},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, relayInfo.PriceData.QuotaToPreConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session
	require.Equal(t, 0.6, relayInfo.PriceData.GroupRatioInfo.GroupRatio)

	// Simulate stale applied state after a reprice while the live session retains 0.6.
	relayInfo.PriceData.GroupRatioInfo.GroupRatio = 1.8
	relayInfo.PriceData.SubscriptionRatioApplied = true
	relayInfo.PriceData.SubscriptionRatioSource = SubscriptionRatioSourceConfigured
	usage := &dto.RealtimeUsage{
		TotalTokens: 20,
		InputTokens: 20,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens: 20,
		},
	}

	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	PostWssConsumeQuota(ctx, relayInfo, relayInfo.OriginModelName, usage, "")

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 12, reloadedSub.AmountUsed)
	require.Equal(t, 0.6, relayInfo.PriceData.GroupRatioInfo.GroupRatio)

	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, 12, log.Quota)
	require.Contains(t, log.Content, "分组倍率 0.60")
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	require.Equal(t, 0.6, other["group_ratio"])
	require.Equal(t, 0.6, other["value_package_effective_ratio"])
}

func TestRealtimeValuePackageReserveRollingWindowOverflowIsAtomic(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-limit-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-limit-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Limited Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 10, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-limit-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaToPreConsume: 1,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	err := PreWssConsumeQuota(ctx, relayInfo, &dto.RealtimeUsage{TotalTokens: 20, InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}})
	require.Error(t, err)
	require.Contains(t, err.Error(), model.ValuePackageQuotaExhaustedUserMessage)

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 1, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 1, used5h)
	require.EqualValues(t, 1, used7d)
	require.EqualValues(t, 0, relayInfo.RealtimeReservedQuota)
}

func TestRealtimeValuePackageReserveIgnoresWalletQuotaPrecheck(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-zero-wallet-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 0}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-zero-wallet-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Zero Wallet Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 100000, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-zero-wallet-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaToPreConsume: 1,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, &dto.RealtimeUsage{TotalTokens: 20, InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}}))

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 20, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 20, used5h)
	require.EqualValues(t, 20, used7d)
}

func TestRealtimeValuePackageReserveRealtimeConcurrentDeltas(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-concurrent-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-concurrent-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Concurrent Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 100000, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-concurrent-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := session.ReserveRealtime(10)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 20, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 20, used5h)
	require.EqualValues(t, 20, used7d)
	require.EqualValues(t, 20, relayInfo.RealtimeReservedQuota)
}

func TestWalletRealtimeReserveDoesNotOverdrawUserQuota(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "wallet-realtime-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 15}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "wallet-realtime-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:        "wallet-realtime-request",
		UserId:           user.Id,
		TokenId:          token.Id,
		TokenKey:         token.Key,
		TokenUnlimited:   true,
		UserQuota:        user.Quota,
		UsingGroup:       "default",
		UserGroup:        model.UserGroupVIP,
		BillingUserGroup: model.UserGroupVIP,
		OriginModelName:  "gpt-4.1",
		UserSetting:      dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaToPreConsume: 10,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	err := PreWssConsumeQuota(ctx, relayInfo, &dto.RealtimeUsage{TotalTokens: 20, InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}})
	require.Error(t, err)

	var reloaded model.User
	require.NoError(t, model.DB.First(&reloaded, user.Id).Error)
	require.EqualValues(t, 5, reloaded.Quota)
	require.EqualValues(t, 0, relayInfo.RealtimeActualQuota)
	require.EqualValues(t, 0, relayInfo.RealtimeReservedQuota)
}

func TestRealtimeValuePackageSettlePositiveDeltaEnforcesRollingLimitAtomically(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-settle-limit-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-settle-limit-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Settle Limited Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 10, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-settle-limit-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaToPreConsume: 1,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	err := SettleBilling(ctx, relayInfo, 20)
	require.Error(t, err)
	require.Contains(t, err.Error(), model.ValuePackageQuotaExhaustedUserMessage)

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 1, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 1, used5h)
	require.EqualValues(t, 1, used7d)
}

func TestRealtimeValuePackageReserveFailureDoesNotAdvanceActualQuota(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-failure-state-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-failure-state-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime Failure State Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 10, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-failure-state-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaToPreConsume: 1,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	err := PreWssConsumeQuota(ctx, relayInfo, &dto.RealtimeUsage{TotalTokens: 20, InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}})
	require.Error(t, err)
	require.EqualValues(t, 0, relayInfo.RealtimeActualQuota)
	require.EqualValues(t, 0, relayInfo.RealtimeReservedQuota)
}

func TestRealtimeValuePackagePreWssAccumulatesDeltas(t *testing.T) {
	setupValuePackageBillingSessionTestDB(t)
	user := model.User{Username: "vp-realtime-prewss-concurrent-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "vp-realtime-prewss-concurrent-token", Status: common.TokenStatusEnabled, UnlimitedQuota: true, RemainQuota: 100000}
	require.NoError(t, model.DB.Create(&token).Error)
	plan := model.SubscriptionPlan{Title: "Realtime PreWss Concurrent Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 100000, Limit7dAmount: 100000, ConcurrencyLimit: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
	require.NoError(t, model.DB.Create(&sub).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:                  "vp-realtime-prewss-concurrent-request",
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		UserQuota:                  user.Quota,
		UsingGroup:                 "default",
		UserGroup:                  model.UserGroupVIP,
		BillingUserGroup:           "month-card",
		ValuePackageBillingGroup:   "month-card",
		OriginModelName:            "gpt-4.1",
		ValuePackageSubscriptionId: sub.Id,
		ValuePackagePlanId:         plan.Id,
		ValuePackageModelGroup:     plan.ModelGroup,
		ValuePackagePackageType:    plan.PackageType,
		PriceData: types.PriceData{
			ModelRatio:        1,
			QuotaToPreConsume: 1,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
		},
	}
	session, apiErr := NewBillingSession(ctx, relayInfo, 1)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	relayInfo.Billing = session

	for i := 0; i < 2; i++ {
		require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, &dto.RealtimeUsage{TotalTokens: 10, InputTokens: 10, InputTokenDetails: dto.InputTokenDetails{TextTokens: 10}}))
	}

	var reloadedSub model.UserSubscription
	require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 20, reloadedSub.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 20, used5h)
	require.EqualValues(t, 20, used7d)
}
