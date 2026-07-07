package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	_ "unsafe"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

//go:linkname initValuePackageMiddlewareModelColumns github.com/QuantumNous/new-api/model.initCol
func initValuePackageMiddlewareModelColumns()

func setupValuePackageMiddlewareTestDB(t *testing.T) *gorm.DB {
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
	initValuePackageMiddlewareModelColumns()
	require.NoError(t, i18n.Init())

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.UserValuePackagePreference{}, &model.ValuePackageUsageRecord{}, &model.ValuePackageQuotaReset{}, &model.ValuePackageResetCountLedger{}))

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
		initValuePackageMiddlewareModelColumns()
	})
	return db
}

func seedValuePackageMiddlewareState(t *testing.T, enabled bool, limit5h int64, limit7d int64, concurrency int) (model.User, model.SubscriptionPlan, model.UserSubscription) {
	return seedValuePackageMiddlewareStateForPackage(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay, enabled, limit5h, limit7d, concurrency)
}

func seedValuePackageMiddlewareStateForPackage(t *testing.T, packageType string, packageLevel int, enabled bool, limit5h int64, limit7d int64, concurrency int) (model.User, model.SubscriptionPlan, model.UserSubscription) {
	t.Helper()
	user := model.User{
		Username: "vp-mw-user-" + packageType,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    model.UserGroupTiyan,
		Quota:    1000000,
		AffCode:  fmt.Sprintf("vp-mw-aff-%s-%d", packageType, time.Now().UnixNano()),
	}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{
		Title:            packageType + " card",
		PriceAmount:      3.9,
		Currency:         "USD",
		DurationUnit:     model.SubscriptionDurationDay,
		DurationValue:    1,
		Enabled:          false,
		PlanKind:         model.SubscriptionPlanKindValuePackage,
		PackageType:      packageType,
		PackageLevel:     packageLevel,
		ModelGroup:       packageType + "-card",
		ConcurrencyLimit: concurrency,
		Limit5hAmount:    limit5h,
		Limit7dAmount:    limit7d,
		TotalAmount:      10000,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: now - 10, EndTime: now + int64(time.Hour/time.Second), Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(&sub).Error)
	pref := model.UserValuePackagePreference{UserId: user.Id, Enabled: enabled, ActiveUserSubscriptionId: sub.Id}
	require.NoError(t, model.DB.Create(&pref).Error)
	if !enabled {
		require.NoError(t, model.DB.Model(&model.UserValuePackagePreference{}).Where("user_id = ?", user.Id).UpdateColumns(map[string]any{
			"created_at": now - 100,
			"updated_at": now,
		}).Error)
	}
	return user, plan, sub
}

func runValuePackageMiddlewareRequest(t *testing.T, userID int, initialGroup string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if userID > 0 {
			common.SetContextKey(c, constant.ContextKeyUserId, userID)
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, initialGroup)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, initialGroup)
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/relay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"using_group":                   common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group":                   common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"value_package_subscription_id": common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId),
			"value_package_model_group":     common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
		})
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/relay", nil)
	router.ServeHTTP(recorder, req)
	return recorder
}

func runValuePackageMiddlewareRequestWithMethod(t *testing.T, userID int, initialGroup string, method string, path string, hold bool) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if userID > 0 {
			common.SetContextKey(c, constant.ContextKeyUserId, userID)
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, initialGroup)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, initialGroup)
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.Handle(method, path, func(c *gin.Context) {
		if hold {
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Flush()
			<-c.Request.Context().Done()
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"using_group":                   common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group":                   common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"value_package_subscription_id": common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId),
			"value_package_model_group":     common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
		})
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestValuePackageRealtimeRejectsOverRollingWindows(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "realtime-exhausted", Quota: 100, CreatedAt: now}))

	recorder := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodGet, "/v1/realtime", false)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
}

func TestValuePackageRealtimeHonorsConcurrencyLimit(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, sub := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	release, ok, err := acquireValuePackageSlot(sub.Id, 1)
	require.NoError(t, err)
	require.True(t, ok)
	defer release()

	recorder := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodGet, "/v1/realtime", false)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())
}

func TestValuePackageReadOnlyRequestsKeepDistributorGroupButSkipQuotaChecks(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "exhausted", Quota: 100, CreatedAt: now}))

	readOnly := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodGet, "/v1/models", false)
	require.Equal(t, http.StatusOK, readOnly.Code, readOnly.Body.String())
	require.Contains(t, readOnly.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, readOnly.Body.String(), `"token_group":"gpt-plus"`)
	require.Contains(t, readOnly.Body.String(), `"value_package_model_group":"day-card"`)

	fetchPost := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodPost, "/suno/fetch", false)
	require.Equal(t, http.StatusOK, fetchPost.Code, fetchPost.Body.String())
	require.Contains(t, fetchPost.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, fetchPost.Body.String(), `"token_group":"gpt-plus"`)
	require.Contains(t, fetchPost.Body.String(), `"value_package_model_group":"day-card"`)

	consume := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodPost, "/v1/chat/completions", false)
	require.Equal(t, http.StatusForbidden, consume.Code, consume.Body.String())
}

func TestValuePackageGroupScopeSkipsConcurrencyLimit(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	release, ok, err := acquireValuePackageSlot(1, 1)
	require.NoError(t, err)
	require.True(t, ok)
	defer release()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageGroupScope())
	router.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"using_group":               common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group":               common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"value_package_model_group": common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)
}

func TestValuePackageMiddlewareKeepsDistributorGroupButMarksPackageEntitlement(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, sub := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), fmt.Sprintf(`"value_package_subscription_id":%d`, sub.Id))
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)
}

func TestValuePackageMiddlewareKeepsOriginalUserGroupForPermissions(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, sub := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("group", model.UserGroupVIP).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupVIP)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/relay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_group":                    common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"using_group":                   common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group":                   common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"value_package_subscription_id": common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId),
			"value_package_model_group":     common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/relay", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), fmt.Sprintf(`"value_package_subscription_id":%d`, sub.Id))
	require.Contains(t, recorder.Body.String(), `"user_group":"vip"`)
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)

	var reloaded model.User
	require.NoError(t, model.DB.First(&reloaded, user.Id).Error)
	require.Equal(t, model.UserGroupVIP, reloaded.Group)
}

func TestValuePackageMiddlewareRejectsOverRollingWindows(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "hit-5h", Quota: 100, CreatedAt: now}))

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), model.ValuePackageQuotaExhaustedUserMessage)
	require.Contains(t, recorder.Body.String(), "5 小时")
	require.Contains(t, recorder.Body.String(), "完全恢复")

	require.NoError(t, model.DB.Where("1 = 1").Delete(&model.ValuePackageUsageRecord{}).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]any{"limit_5h_amount": int64(0), "limit_7d_amount": int64(100)}).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "day-legacy-7d-ignored", Quota: 100, CreatedAt: now - int64(6*time.Hour/time.Second)}))

	recorder = runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)

	weekUser, weekPlan, weekSub := seedValuePackageMiddlewareStateForPackage(t, model.ValuePackageTypeWeek, model.ValuePackageLevelWeek, true, 0, 100, 1)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: weekUser.Id, UserSubscriptionId: weekSub.Id, PlanId: weekPlan.Id, PackageType: weekPlan.PackageType, ModelGroup: weekPlan.ModelGroup, RequestId: "week-hit-7d", Quota: 100, CreatedAt: now - int64(6*time.Hour/time.Second)}))

	recorder = runValuePackageMiddlewareRequest(t, weekUser.Id, "gpt-plus")

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), model.ValuePackageQuotaExhaustedUserMessage)
	require.Contains(t, recorder.Body.String(), "7 天")
	require.Contains(t, recorder.Body.String(), "完全恢复")
}

func TestValuePackageMiddlewareResetCountdownUsesEarliestUsage(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "first-window-usage", Quota: 99, CreatedAt: now - (3*3600 + 55*60)}))
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "later-window-usage", Quota: 1, CreatedAt: now - 2*3600}))

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "5 小时")
	require.Contains(t, recorder.Body.String(), "1 小时")
	require.NotContains(t, recorder.Body.String(), "3 小时")
}

func TestValuePackageMiddlewareAllowsRequestAfterQuotaReset(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "before-reset-exhausted", Quota: 100, CreatedAt: now - 1800}))
	require.NoError(t, model.DB.Create(&model.ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ResetAt: now, Source: model.ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)
}

func TestValuePackageMiddlewareAllowsAfterFixedFiveHourWindowExpires(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	windowStart := now - 5*3600
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "fixed-window-first", Quota: 90, CreatedAt: windowStart}))
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "fixed-window-later", Quota: 10, CreatedAt: windowStart + 4*3600}))

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)
}

func TestValuePackageLimitMessageFormatting(t *testing.T) {
	tests := []struct {
		name         string
		windowLabel  string
		used         int64
		limit        int64
		resetSeconds int64
		want         string
	}{
		{
			name:         "5h reset uses complete recovery wording without extra spacing",
			windowLabel:  "5 小时",
			used:         100,
			limit:        100,
			resetSeconds: 5 * 3600,
			want:         model.ValuePackageQuotaExhaustedUserMessage + "（5 小时：已用 100 / 限额 100，将在 5 小时后完全恢复）",
		},
		{
			name:         "7d reset uses complete recovery wording without extra spacing",
			windowLabel:  "7 天",
			used:         100,
			limit:        100,
			resetSeconds: 7 * 24 * 3600,
			want:         model.ValuePackageQuotaExhaustedUserMessage + "（7 天：已用 100 / 限额 100，将在 7 天后完全恢复）",
		},
		{
			name:         "zero reset keeps old wording",
			windowLabel:  "5 小时",
			used:         100,
			limit:        100,
			resetSeconds: 0,
			want:         model.ValuePackageQuotaExhaustedUserMessage + "（5 小时：已用 100 / 限额 100）",
		},
		{
			name:         "negative reset keeps old wording",
			windowLabel:  "7 天",
			used:         100,
			limit:        100,
			resetSeconds: -1,
			want:         model.ValuePackageQuotaExhaustedUserMessage + "（7 天：已用 100 / 限额 100）",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValuePackageLimitMessage(tt.windowLabel, tt.used, tt.limit, tt.resetSeconds)
			require.Equal(t, tt.want, got)
			if tt.resetSeconds <= 0 {
				require.NotContains(t, got, "完全恢复")
			}
		})
	}
}

func TestValuePackageResetDurationFormatting(t *testing.T) {
	require.Equal(t, "不到 1 分钟", formatValuePackageResetDuration(-1))
	require.Equal(t, "不到 1 分钟", formatValuePackageResetDuration(0))
	require.Equal(t, "不到 1 分钟", formatValuePackageResetDuration(45))
	require.Equal(t, "1 分钟", formatValuePackageResetDuration(60))
	require.Equal(t, "2 分钟", formatValuePackageResetDuration(61))
	require.Equal(t, "1 小时", formatValuePackageResetDuration(3600))
	require.Equal(t, "5 小时", formatValuePackageResetDuration(int64((5*time.Hour)/time.Second)))
	require.Equal(t, "3 小时 15 分钟", formatValuePackageResetDuration(int64((3*time.Hour+15*time.Minute)/time.Second)))
	require.Equal(t, "2 天 4 小时", formatValuePackageResetDuration(int64((2*24*time.Hour+4*time.Hour)/time.Second)))
	require.Equal(t, "7 天", formatValuePackageResetDuration(int64((7*24*time.Hour)/time.Second)))
}

func TestValuePackageConcurrencyLimiter(t *testing.T) {
	valuePackageConcurrencyCounters.Delete(9001)
	valuePackageConcurrencyCounters.Delete(9002)
	release1, ok := acquireValuePackageMemorySlot(9001, 1)
	require.True(t, ok)
	defer release1()

	release2, ok := acquireValuePackageMemorySlot(9001, 1)
	require.False(t, ok)
	require.Nil(t, release2)

	release1()
	_, exists := valuePackageConcurrencyCounters.Load(9001)
	require.False(t, exists)
	release1()
	_, exists = valuePackageConcurrencyCounters.Load(9001)
	require.False(t, exists)

	release3, ok := acquireValuePackageMemorySlot(9001, 1)
	require.True(t, ok)
	require.NotNil(t, release3)
	release3()
	_, exists = valuePackageConcurrencyCounters.Load(9001)
	require.False(t, exists)

	releaseA, ok := acquireValuePackageMemorySlot(9002, 9)
	require.True(t, ok)
	releaseB, ok := acquireValuePackageMemorySlot(9002, 9)
	require.True(t, ok)
	releaseC, ok := acquireValuePackageMemorySlot(9002, 9)
	require.False(t, ok)
	require.Nil(t, releaseC)
	releaseA()
	_, exists = valuePackageConcurrencyCounters.Load(9002)
	require.True(t, exists)
	releaseB()
	_, exists = valuePackageConcurrencyCounters.Load(9002)
	require.False(t, exists)
}

func TestValuePackageRedisConcurrencyLimiterReclaimsRecentlyStaleSlot(t *testing.T) {
	redisServer := miniredis.RunT(t)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
	})

	key := valuePackageConcurrencyRedisKey(9201)
	staleScore := time.Now().Add(-3 * time.Minute).Unix()
	require.NoError(t, common.RDB.ZAdd(common.RDB.Context(), key, &redis.Z{Score: float64(staleScore), Member: "stale-token"}).Err())
	require.NoError(t, common.RDB.Expire(common.RDB.Context(), key, 30*time.Minute).Err())

	release, ok, err := acquireValuePackageSlot(9201, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, release)
	release()
	require.False(t, redisServer.Exists(key))
}

func TestRefreshValuePackageRedisSlotUpdatesOnlyExistingSlot(t *testing.T) {
	redisServer := miniredis.RunT(t)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
	})

	key := valuePackageConcurrencyRedisKey(9202)
	oldScore := time.Now().Add(-time.Minute).Unix()
	require.NoError(t, common.RDB.ZAdd(common.RDB.Context(), key, &redis.Z{Score: float64(oldScore), Member: "live-token"}).Err())
	require.NoError(t, common.RDB.Expire(common.RDB.Context(), key, 5*time.Second).Err())

	refreshed, err := refreshValuePackageRedisSlot(key, "live-token", 30)
	require.NoError(t, err)
	require.True(t, refreshed)
	score, err := common.RDB.ZScore(common.RDB.Context(), key, "live-token").Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, int64(score), time.Now().Add(-2*time.Second).Unix())
	redisServer.FastForward(6 * time.Second)
	require.True(t, redisServer.Exists(key))

	removed, err := common.RDB.ZRem(common.RDB.Context(), key, "live-token").Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	refreshed, err = refreshValuePackageRedisSlot(key, "live-token", 30)
	require.NoError(t, err)
	require.False(t, refreshed)
	exists, err := common.RDB.Exists(common.RDB.Context(), key).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, exists)
}

func TestValuePackageRedisConcurrencyLimiter(t *testing.T) {
	redisServer := miniredis.RunT(t)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
	})

	release1, ok, err := acquireValuePackageSlot(9101, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, release1)

	release2, ok, err := acquireValuePackageSlot(9101, 1)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, release2)

	release1()
	require.False(t, redisServer.Exists(valuePackageConcurrencyRedisKey(9101)))

	release3, ok, err := acquireValuePackageSlot(9101, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, release3)
	release3()
}

func TestValuePackageMiddlewareDisabledPreferenceDoesNotForceGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, false, 1000, 5000, 1)

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"value_package_subscription_id":0`)
}

func TestValuePackageMiddlewareDisabledPreferenceKeepsOriginalUserGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, false, 1000, 5000, 1)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupVIP)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/relay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_group":  common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group": common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/relay", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"user_group":"vip"`)
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"gpt-plus"`)
}

func TestValuePackagePlaygroundDistributeKeepsRequestedModelGroupWhenPackageActive(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	priority := int64(0)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "sk-test", Status: common.ChannelStatusEnabled, Name: "gpt-pro-channel", Models: "gpt-5.5", Group: "gpt-pro", Priority: &priority}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "gpt-pro", Model: "gpt-5.5", ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupTiyan)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement(), Distribute())
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup)})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-5.5","group":"gpt-pro","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-pro"`)
}

func TestValuePackageRelayDistributeKeepsTokenModelGroupWhenPackageActive(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	priority := int64(0)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "sk-test", Status: common.ChannelStatusEnabled, Name: "gpt-plus-channel", Models: "gpt-5.5", Group: "gpt-plus", Priority: &priority}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "gpt-plus", Model: "gpt-5.5", ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupTiyan)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement(), Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"using_group":               common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"value_package_model_group": common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.5","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)
}

func TestValuePackagePlaygroundGroupPermissionUsesOriginalUserGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)

	oldUserUsableGroups := setting.UserUsableGroups2JSONString()
	oldSpecialUsableGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-plus":"Plus 模型分组"}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Set(model.UserGroupTiyan, map[string]string{
		"+:gpt-pro": "PRO 模型分组",
	})
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUserUsableGroups))
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(oldSpecialUsableGroups)
	})

	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	priority := int64(0)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "sk-test", Status: common.ChannelStatusEnabled, Name: "gpt-pro-channel", Models: "gpt-5.5", Group: "gpt-pro", Priority: &priority}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "gpt-pro", Model: "gpt-5.5", ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupTiyan)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement(), Distribute())
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_group":  common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-5.5","group":"gpt-pro","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"user_group":"`+model.UserGroupTiyan+`"`)
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-pro"`)
}

func TestValuePackageScopeDoesNotOverwriteRoutingOrUserGroups(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	_, err := model.ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupVIP)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageGroupScope())
	router.GET("/check", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_group":    common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"using_group":   common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group":   common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"package_group": common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]string
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, model.UserGroupVIP, body["user_group"])
	require.Equal(t, "gpt-plus", body["using_group"])
	require.Equal(t, "gpt-plus", body["token_group"])
	require.Equal(t, plan.ModelGroup, body["package_group"])
}
