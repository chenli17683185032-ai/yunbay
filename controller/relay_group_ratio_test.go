package controller

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelayRetryBillingRatioTest(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldGroupRatio := ratio_setting.GroupRatio2JSONString()
	oldGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	oldSelector := cacheGetRandomSatisfiedChannel

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{},
		&model.ValuePackageUsageRecord{},
		&model.ValuePackageQuotaReset{},
	))
	model.DB = db
	model.LOG_DB = db
	cacheGetRandomSatisfiedChannel = func(*service.RetryParam) (*model.Channel, string, error) {
		return nil, "gpt-plus", errors.New("stop after billing ratio refresh")
	}

	t.Cleanup(func() {
		cacheGetRandomSatisfiedChannel = oldSelector
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldGroupGroupRatio))
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newRelayRetryBillingContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Set("token_quota", 100000)
	return c
}

func seedRelayRetryValuePackage(t *testing.T, db *gorm.DB) (int, int) {
	t.Helper()
	plan := &model.SubscriptionPlan{
		Title:         "relay retry package",
		PlanKind:      model.SubscriptionPlanKindValuePackage,
		PackageType:   model.ValuePackageTypeMonth,
		ModelGroup:    "month-card",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   100000,
	}
	require.NoError(t, db.Create(plan).Error)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	subscription := &model.UserSubscription{
		UserId:      801,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		Status:      model.UserSubscriptionStatusActive,
		StartTime:   time.Now().Add(-time.Hour).Unix(),
		EndTime:     time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, db.Create(subscription).Error)
	return plan.Id, subscription.Id
}

func newFrozenRelayRetryInfo(planID, subscriptionID int, tiered bool) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		ChannelMeta:                &relaycommon.ChannelMeta{},
		UserId:                     801,
		UsingGroup:                 "gpt-plus",
		BillingUserGroup:           "month-card",
		OriginModelName:            "gpt-test",
		RequestId:                  fmt.Sprintf("retry-frozen-%t", tiered),
		IsPlayground:               true,
		ForcePreConsume:            true,
		ValuePackageSubscriptionId: subscriptionID,
		ValuePackagePlanId:         planID,
		ValuePackageBillingGroup:   "month-card",
		PriceData: types.PriceData{
			Quota:             600,
			QuotaBeforeGroup:  1000,
			QuotaToPreConsume: 600,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio:        0.6,
				GroupSpecialRatio: 0.6,
				HasSpecialRatio:   true,
			},
		},
	}
	if tiered {
		info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			GroupRatio:                0.6,
			EstimatedQuotaBeforeGroup: 1000,
			EstimatedQuotaAfterGroup:  600,
		}
	}
	return info
}

func TestGetChannelRestoresFrozenSubscriptionRatioAfterLiveRefresh(t *testing.T) {
	for _, tiered := range []bool{false, true} {
		t.Run(fmt.Sprintf("tiered_%t", tiered), func(t *testing.T) {
			db := setupRelayRetryBillingRatioTest(t)
			planID, subscriptionID := seedRelayRetryValuePackage(t, db)
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-plus":1}`))
			require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"gpt-plus":0.6}}`))
			ctx := newRelayRetryBillingContext()
			info := newFrozenRelayRetryInfo(planID, subscriptionID, tiered)
			require.Nil(t, service.PreConsumeBilling(ctx, info.PriceData.QuotaToPreConsume, info))

			require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"gpt-plus":1.8}}`))
			_, apiErr := getChannel(ctx, info, &service.RetryParam{Ctx: ctx, TokenGroup: "gpt-plus", ModelName: info.OriginModelName})

			require.NotNil(t, apiErr)
			require.Equal(t, 0.6, info.PriceData.GroupRatioInfo.GroupRatio)
			require.Equal(t, service.SubscriptionRatioSourceConfigured, info.PriceData.SubscriptionRatioSource)
			if tiered {
				require.Equal(t, 0.6, info.TieredBillingSnapshot.GroupRatio)
				require.Equal(t, 600, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
			}
		})
	}
}

func TestGetChannelWithoutBillingSessionUsesLiveRatio(t *testing.T) {
	setupRelayRetryBillingRatioTest(t)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-plus":1}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"gpt-plus":1.8}}`))
	ctx := newRelayRetryBillingContext()
	info := &relaycommon.RelayInfo{
		ChannelMeta:      &relaycommon.ChannelMeta{},
		UsingGroup:       "gpt-plus",
		BillingUserGroup: "month-card",
		OriginModelName:  "gpt-test",
	}

	_, apiErr := getChannel(ctx, info, &service.RetryParam{Ctx: ctx, TokenGroup: "gpt-plus", ModelName: info.OriginModelName})

	require.NotNil(t, apiErr)
	require.Equal(t, 1.8, info.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, 1.8, info.PriceData.GroupRatioInfo.GroupSpecialRatio)
	require.True(t, info.PriceData.GroupRatioInfo.HasSpecialRatio)
}
