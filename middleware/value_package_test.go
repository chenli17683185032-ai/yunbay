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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.UserValuePackagePreference{}, &model.ValuePackageUsageRecord{}))

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
	t.Helper()
	user := model.User{Username: "vp-mw-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, Quota: 1000000}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{
		Title:            "day card",
		PriceAmount:      3.9,
		Currency:         "USD",
		DurationUnit:     model.SubscriptionDurationDay,
		DurationValue:    1,
		Enabled:          false,
		PlanKind:         model.SubscriptionPlanKindValuePackage,
		PackageType:      model.ValuePackageTypeDay,
		PackageLevel:     model.ValuePackageLevelDay,
		ModelGroup:       "day-card",
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

func TestValuePackageReadOnlyRequestsOnlyApplyPackageGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "exhausted", Quota: 100, CreatedAt: now}))

	readOnly := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodGet, "/v1/models", false)
	require.Equal(t, http.StatusOK, readOnly.Code, readOnly.Body.String())
	require.Contains(t, readOnly.Body.String(), `"using_group":"day-card"`)

	fetchPost := runValuePackageMiddlewareRequestWithMethod(t, user.Id, "gpt-plus", http.MethodPost, "/suno/fetch", false)
	require.Equal(t, http.StatusOK, fetchPost.Code, fetchPost.Body.String())
	require.Contains(t, fetchPost.Body.String(), `"using_group":"day-card"`)

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
		c.JSON(http.StatusOK, gin.H{"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup)})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"day-card"`)
}

func TestValuePackageMiddlewareForcesPackageGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, sub := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"day-card"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"day-card"`)
	require.Contains(t, recorder.Body.String(), fmt.Sprintf(`"value_package_subscription_id":%d`, sub.Id))
	require.Contains(t, recorder.Body.String(), `"value_package_model_group":"day-card"`)
}

func TestValuePackageMiddlewareRejectsOverRollingWindows(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 100, 500, 1)
	now := common.GetTimestamp()
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "hit-5h", Quota: 100, CreatedAt: now}))

	recorder := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())

	require.NoError(t, model.DB.Where("1 = 1").Delete(&model.ValuePackageUsageRecord{}).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]any{"limit_5h_amount": int64(0), "limit_7d_amount": int64(100)}).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "hit-7d", Quota: 100, CreatedAt: now - int64(6*time.Hour/time.Second)}))

	recorder = runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
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

func TestValuePackagePlaygroundDistributeKeepsPackageGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, _ := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	priority := int64(0)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "sk-test", Status: common.ChannelStatusEnabled, Name: "day-card-channel", Models: "gpt-4o", Group: plan.ModelGroup, Priority: &priority}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: plan.ModelGroup, Model: "gpt-4o", ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)

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
	req := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-4o","group":"gpt-pro","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"day-card"`)
}
