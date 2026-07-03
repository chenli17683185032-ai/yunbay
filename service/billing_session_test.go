package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}, &model.ValuePackageUsageRecord{}))

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
	})
	return db
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
	require.EqualValues(t, 150, used7d)
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
