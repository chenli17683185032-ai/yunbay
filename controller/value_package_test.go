package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failingTaskSettleBilling struct {
	settleCalls int
}

type successfulTaskSettleBilling struct {
	settleCalls int
}

func (b *failingTaskSettleBilling) Settle(actualQuota int) error {
	b.settleCalls++
	return errors.New("usage reservation write failed")
}

func (b *failingTaskSettleBilling) Refund(c *gin.Context)         {}
func (b *failingTaskSettleBilling) NeedsRefund() bool             { return false }
func (b *failingTaskSettleBilling) GetPreConsumedQuota() int      { return 100 }
func (b *failingTaskSettleBilling) Reserve(targetQuota int) error { return nil }

func (b *successfulTaskSettleBilling) Settle(actualQuota int) error {
	b.settleCalls++
	return nil
}

func (b *successfulTaskSettleBilling) Refund(c *gin.Context)         {}
func (b *successfulTaskSettleBilling) NeedsRefund() bool             { return false }
func (b *successfulTaskSettleBilling) GetPreConsumedQuota() int      { return 100 }
func (b *successfulTaskSettleBilling) Reserve(targetQuota int) error { return nil }

func TestFinalizeSuccessfulRelayTaskPersistsBillingContextOnSettleError(t *testing.T) {
	setupValuePackageControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Task{}, &model.Log{}))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	billing := &failingTaskSettleBilling{}
	relayInfo := &relaycommon.RelayInfo{
		UserId:                   42,
		UsingGroup:               "day-card",
		OriginModelName:          "video-test",
		ChannelMeta:              &relaycommon.ChannelMeta{ChannelId: 3},
		TaskRelayInfo:            &relaycommon.TaskRelayInfo{Action: "submit"},
		Billing:                  billing,
		BillingSource:            "subscription",
		SubscriptionId:           7,
		TokenId:                  11,
		ValuePackageBillingGroup: "month-card",
		PriceData: types.PriceData{
			GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: 0.45},
			SubscriptionRatioApplied: true,
			SubscriptionRatioSource:  "configured",
		},
	}
	result := &relay.TaskSubmitResult{Platform: "test-platform", UpstreamTaskID: "upstream-task", Quota: 100, TaskData: []byte(`{"id":"upstream-task"}`)}

	taskErr := finalizeSuccessfulRelayTask(ctx, relayInfo, result)

	require.Nil(t, taskErr)
	require.Equal(t, 1, billing.settleCalls)
	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&taskCount).Error)
	require.EqualValues(t, 1, taskCount)
	var task model.Task
	require.NoError(t, model.DB.First(&task).Error)
	require.Equal(t, result.UpstreamTaskID, task.PrivateData.UpstreamTaskID)
	require.Equal(t, relayInfo.BillingSource, task.PrivateData.BillingSource)
	require.Equal(t, relayInfo.SubscriptionId, task.PrivateData.SubscriptionId)
	require.Equal(t, relayInfo.TokenId, task.PrivateData.TokenId)
	require.True(t, task.PrivateData.BillingSettleFailed)
	require.Equal(t, "usage reservation write failed", task.PrivateData.BillingSettleError)
	privateValue, err := task.PrivateData.Value()
	require.NoError(t, err)
	privateJSON, ok := privateValue.([]byte)
	require.True(t, ok)
	var privateData map[string]interface{}
	require.NoError(t, common.Unmarshal(privateJSON, &privateData))
	billingContext, ok := privateData["billing_context"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 0.45, billingContext["group_ratio"])
	require.Equal(t, "month-card", billingContext["value_package_billing_group"])
	require.Equal(t, "configured", billingContext["subscription_ratio_source"])
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Count(&logCount).Error)
	require.EqualValues(t, 0, logCount)
}

func TestRelayTaskSuccessInsertErrorReturnsTaskError(t *testing.T) {
	setupValuePackageControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Task{}, &model.Log{}))

	const callbackName = "test_fail_task_insert"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Task" {
			tx.AddError(errors.New("task insert failed for test"))
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Create().Remove(callbackName)
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	billing := &successfulTaskSettleBilling{}
	relayInfo := &relaycommon.RelayInfo{
		UserId:          42,
		UsingGroup:      "day-card",
		OriginModelName: "video-test",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 3},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "submit"},
		Billing:         billing,
	}
	result := &relay.TaskSubmitResult{Platform: "test-platform", UpstreamTaskID: "upstream-task", Quota: 100, TaskData: []byte(`{"id":"upstream-task"}`)}

	taskErr := finalizeSuccessfulRelayTask(ctx, relayInfo, result)

	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusInternalServerError, taskErr.StatusCode)
	require.Equal(t, "insert_task_failed", taskErr.Code)
	require.Equal(t, 0, billing.settleCalls)
	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&taskCount).Error)
	require.EqualValues(t, 0, taskCount)
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Count(&logCount).Error)
	require.EqualValues(t, 0, logCount)
}

func TestTaskSubmitResponseBufferDiscardsBufferedSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	originalWriter := ctx.Writer
	buffer := newTaskSubmitResponseBuffer(originalWriter)
	ctx.Writer = buffer

	ctx.Header("X-Task", "buffered")
	ctx.JSON(http.StatusAccepted, gin.H{"task_id": "upstream-task"})

	require.Empty(t, recorder.Body.String())
	require.Empty(t, recorder.Header().Get("X-Task"))

	ctx.Writer = originalWriter
	respondTaskError(ctx, &dto.TaskError{
		Code:       "insert_task_failed",
		Message:    "persist failed",
		StatusCode: http.StatusInternalServerError,
	})

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "upstream-task")
	require.Contains(t, recorder.Body.String(), "insert_task_failed")
}

func TestTaskSubmitResponseBufferFlushesBufferedSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	originalWriter := ctx.Writer
	buffer := newTaskSubmitResponseBuffer(originalWriter)
	ctx.Writer = buffer

	ctx.Header("X-Task", "buffered")
	ctx.JSON(http.StatusAccepted, gin.H{"task_id": "upstream-task"})

	require.NoError(t, buffer.FlushToOriginal())

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, "buffered", recorder.Header().Get("X-Task"))
	require.Contains(t, recorder.Body.String(), "upstream-task")
}

func TestTaskSubmitResponseBufferFlushOverridesExistingHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Task", "old-value")
	ctx, _ := gin.CreateTestContext(recorder)
	originalWriter := ctx.Writer
	buffer := newTaskSubmitResponseBuffer(originalWriter)
	ctx.Writer = buffer

	ctx.Header("X-Task", "buffered")
	ctx.JSON(http.StatusAccepted, gin.H{"task_id": "upstream-task"})

	require.NoError(t, buffer.FlushToOriginal())

	require.Equal(t, []string{"buffered"}, recorder.Result().Header.Values("X-Task"))
}

func setupValuePackageControllerTest(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	paymentSetting := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentSetting.ComplianceConfirmed
	oldComplianceVersion := paymentSetting.ComplianceTermsVersion
	oldComplianceAt := paymentSetting.ComplianceConfirmedAt
	oldComplianceBy := paymentSetting.ComplianceConfirmedBy
	oldComplianceIP := paymentSetting.ComplianceConfirmedIP

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	paymentSetting.ComplianceConfirmedAt = common.GetTimestamp()
	paymentSetting.ComplianceConfirmedBy = 1
	paymentSetting.ComplianceConfirmedIP = "127.0.0.1"

	t.Setenv("LDXP_AUTO_TOPUP_ENABLED", "true")
	t.Setenv("LDXP_WORKER_TOKEN", ldxpControllerTestWorkerToken)
	t.Setenv("LDXP_WORKER_TOKEN_FILE", "")
	t.Setenv("LDXP_CONTACT_EMAIL", "ldxp-value-package@example.test")
	t.Setenv("LDXP_TOPUP_PRODUCTS_JSON", `[
		{"amount":10,"money":0.10,"product_url":"https://ldxp.example.test/10","product_name":"LDXP 10 Test"},
		{"amount":20,"money":0.20,"product_url":"https://ldxp.example.test/20","product_name":"LDXP 20 Test"},
		{"amount":30,"money":0.30,"product_url":"https://ldxp.example.test/30","product_name":"LDXP 30 Test"},
		{"amount":50,"money":0.50,"product_url":"https://ldxp.example.test/50","product_name":"LDXP 50 Test"},
		{"amount":100,"money":1.00,"product_url":"https://ldxp.example.test/100","product_name":"LDXP 100 Test"},
		{"amount":500,"money":5.00,"product_url":"https://ldxp.example.test/500","product_name":"LDXP 500 Test"}
	]`)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.UserValuePackagePreference{},
		&model.ValuePackageUsageRecord{},
		&model.ValuePackageQuotaReset{},
		&model.ValuePackageResetCountLedger{},
		&model.LdxpTopupSession{},
		&model.LdxpMailEvent{},
		&model.TopUp{},
	))

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
		paymentSetting.ComplianceConfirmed = oldComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = oldComplianceVersion
		paymentSetting.ComplianceConfirmedAt = oldComplianceAt
		paymentSetting.ComplianceConfirmedBy = oldComplianceBy
		paymentSetting.ComplianceConfirmedIP = oldComplianceIP
		_ = os.Unsetenv("LDXP_WORKER_TOKEN_FILE")
	})

	return db
}

func valuePackageControllerRequest(handler gin.HandlerFunc, method string, path string, body any, userID int) *httptest.ResponseRecorder {
	router := gin.New()
	routePath := valuePackageControllerRoutePattern(path)
	router.Handle(method, routePath, func(c *gin.Context) {
		if userID > 0 {
			c.Set("id", userID)
		}
		handler(c)
	})

	var reqBody bytes.Buffer
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			panic(err)
		}
		reqBody.Write(payload)
	}
	req := httptest.NewRequest(method, path, &reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func valuePackageControllerRawRequest(handler gin.HandlerFunc, method string, path string, body string, userID int) *httptest.ResponseRecorder {
	router := gin.New()
	routePath := valuePackageControllerRoutePattern(path)
	router.Handle(method, routePath, func(c *gin.Context) {
		if userID > 0 {
			c.Set("id", userID)
		}
		handler(c)
	})

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func valuePackageControllerRoutePattern(path string) string {
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if strings.HasPrefix(path, "/value-packages/plans/") {
		if strings.HasSuffix(path, "/purchase-intent") {
			return "/value-packages/plans/:plan_id/purchase-intent"
		}
		if strings.HasSuffix(path, "/ldxp/session") {
			return "/value-packages/plans/:plan_id/ldxp/session"
		}
	}
	if strings.HasPrefix(path, "/subscription/admin/plans/") {
		return "/subscription/admin/plans/:id"
	}
	if strings.HasPrefix(path, "/subscription/admin/users/") && strings.HasSuffix(path, "/subscriptions") {
		return "/subscription/admin/users/:id/subscriptions"
	}
	return path
}

func seedValuePackageControllerPlan(t *testing.T, packageType string, packageLevel int) model.SubscriptionPlan {
	t.Helper()
	limit7dAmount := int64(5000)
	if packageType == model.ValuePackageTypeDay {
		limit7dAmount = 0
	}
	plan := model.SubscriptionPlan{
		Title:                 packageType + " value package",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		SortOrder:             10,
		TotalAmount:           10000,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           packageType,
		PackageLevel:          packageLevel,
		ModelGroup:            packageType + "-card",
		ConcurrencyLimit:      1,
		Limit5hAmount:         1000,
		Limit7dAmount:         limit7dAmount,
		Benefits:              "fast lane",
		LdxpProductUrl:        "https://ldxp.example.test/value-package/" + packageType,
		LdxpProductName:       packageType + " value package product",
		LdxpProductAmount:     9.9,
		LdxpProductRef:        "ref-" + packageType,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	return plan
}

func validAdminValuePackagePlanForTest(packageType string) model.SubscriptionPlan {
	level := model.ValuePackageLevelDay
	limit7dAmount := int64(0)
	switch packageType {
	case model.ValuePackageTypeWeek:
		level = model.ValuePackageLevelWeek
	case model.ValuePackageTypeMonth:
		level = model.ValuePackageLevelMonth
		limit7dAmount = 1000
	}
	return model.SubscriptionPlan{
		Title:                 packageType + " admin value package",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		TotalAmount:           10000,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           packageType,
		PackageLevel:          level,
		ModelGroup:            packageType + "-card",
		ConcurrencyLimit:      1,
		Limit5hAmount:         100,
		Limit7dAmount:         limit7dAmount,
		LdxpProductUrl:        "https://ldxp.example.test/" + packageType,
		LdxpProductName:       packageType + " product",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
}

func TestGetValuePackagePlansReturnsOnlyValuePackagesAndState(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_plans_user")
	day := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	month := seedValuePackageControllerPlan(t, model.ValuePackageTypeMonth, model.ValuePackageLevelMonth)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{Title: "normal subscription", PriceAmount: 1, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindSubscription}).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: month.Id, StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	_, err := model.ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)

	rec := valuePackageControllerRequest(GetValuePackagePlans, http.MethodGet, "/value-packages/plans", nil, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	plans, ok := data["plans"].([]interface{})
	require.True(t, ok)
	require.Len(t, plans, 2)
	assert.Equal(t, float64(day.Id), plans[0].(map[string]interface{})["id"])
	assert.Equal(t, float64(month.Id), plans[1].(map[string]interface{})["id"])
	for _, rawPlan := range plans {
		plan := rawPlan.(map[string]interface{})
		assert.Equal(t, model.SubscriptionPlanKindValuePackage, plan["plan_kind"])
	}
	state, ok := data["state"].(map[string]interface{})
	require.True(t, ok)
	pref, ok := state["preference"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, pref["enabled"])
	assert.Equal(t, float64(sub.Id), pref["active_user_subscription_id"])
	statePlan, ok := state["plan"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(month.Id), statePlan["id"])
}

func TestGetValuePackagePlansReturnsOnlyEnabledValuePackages(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_enabled_only_user")
	enabled := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	disabled := seedValuePackageControllerPlan(t, model.ValuePackageTypeWeek, model.ValuePackageLevelWeek)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", disabled.Id).Update("enabled", false).Error)

	rec := valuePackageControllerRequest(GetValuePackagePlans, http.MethodGet, "/value-packages/plans", nil, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	plans := data["plans"].([]interface{})
	require.Len(t, plans, 1)
	assert.Equal(t, float64(enabled.Id), plans[0].(map[string]interface{})["id"])
}

func TestGetValuePackagePlansHidesDisabledPlanButSelfKeepsActiveSubscription(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_disabled_active_self_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	_, err := model.ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	rec := valuePackageControllerRequest(GetValuePackagePlans, http.MethodGet, "/value-packages/plans", nil, user.Id)
	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	plans := data["plans"].([]interface{})
	require.Empty(t, plans)

	rec = valuePackageControllerRequest(GetValuePackageSelf, http.MethodGet, "/value-packages/self", nil, user.Id)
	body = decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	self := body["data"].(map[string]interface{})
	pref := self["preference"].(map[string]interface{})
	assert.Equal(t, true, pref["enabled"])
	assert.Equal(t, float64(sub.Id), pref["active_user_subscription_id"])
	rawSubscription := self["subscription"]
	require.NotNil(t, rawSubscription)
	subscription := rawSubscription.(map[string]interface{})
	assert.Equal(t, float64(sub.Id), subscription["id"])
	rawPlan := self["plan"]
	require.NotNil(t, rawPlan)
	statePlan := rawPlan.(map[string]interface{})
	assert.Equal(t, float64(plan.Id), statePlan["id"])
	assert.Equal(t, false, statePlan["enabled"])
}

func TestGetSubscriptionPlansExcludesValuePackages(t *testing.T) {
	setupValuePackageControllerTest(t)
	valuePackage := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	normal := model.SubscriptionPlan{Title: "normal subscription", PriceAmount: 1, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindSubscription, SortOrder: 30}
	legacy := model.SubscriptionPlan{Title: "legacy subscription", PriceAmount: 2, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, PlanKind: "", SortOrder: 20}
	disabled := model.SubscriptionPlan{Title: "disabled subscription", PriceAmount: 3, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: false, PlanKind: model.SubscriptionPlanKindSubscription, SortOrder: 10}
	require.NoError(t, model.DB.Create(&normal).Error)
	require.NoError(t, model.DB.Create(&legacy).Error)
	require.NoError(t, model.DB.Create(&disabled).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", disabled.Id).Update("enabled", false).Error)

	rec := valuePackageControllerRequest(GetSubscriptionPlans, http.MethodGet, "/subscription/plans", nil, 0)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	rawPlans := body["data"].([]interface{})
	require.Len(t, rawPlans, 2)
	ids := make(map[float64]bool, len(rawPlans))
	for _, raw := range rawPlans {
		dto := raw.(map[string]interface{})
		plan := dto["plan"].(map[string]interface{})
		ids[plan["id"].(float64)] = true
		assert.NotEqual(t, model.SubscriptionPlanKindValuePackage, plan["plan_kind"])
	}
	assert.True(t, ids[float64(normal.Id)])
	assert.True(t, ids[float64(legacy.Id)])
	assert.False(t, ids[float64(valuePackage.Id)])
	assert.False(t, ids[float64(disabled.Id)])
}

func TestGetValuePackageSelfReturnsCurrentState(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_self_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 3600, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	_, err := model.ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "self-reset-fields", Quota: 10, CreatedAt: now - 1800}))

	rec := valuePackageControllerRequest(GetValuePackageSelf, http.MethodGet, "/value-packages/self", nil, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	pref := data["preference"].(map[string]interface{})
	assert.Equal(t, true, pref["enabled"])
	assert.Equal(t, float64(sub.Id), pref["active_user_subscription_id"])
	statePlan := data["plan"].(map[string]interface{})
	assert.Equal(t, float64(plan.Id), statePlan["id"])
	usage, ok := data["usage"].(map[string]interface{})
	require.True(t, ok)
	requireValuePackageUsageResetFields(t, usage)
	assert.Equal(t, float64(10), usage["used_5h"])
	assert.Equal(t, float64(0), usage["used_7d"])
	assert.Equal(t, float64(0), usage["limit_7d"])
	assert.Greater(t, valuePackageUsageNumber(t, usage, "reset_seconds_5h"), float64(0))
	assert.Equal(t, float64(0), valuePackageUsageNumber(t, usage, "reset_seconds_7d"))
}

func TestValuePackageStatePeriodLimitsByPackageType(t *testing.T) {
	setupValuePackageControllerTest(t)
	now := common.GetTimestamp()

	tests := []struct {
		name          string
		packageType   string
		packageLevel  int
		amountTotal   int64
		limit7dAmount int64
		want          []valuePackagePeriodResponseExpectation
	}{
		{
			name:          "day",
			packageType:   model.ValuePackageTypeDay,
			packageLevel:  model.ValuePackageLevelDay,
			amountTotal:   2400,
			limit7dAmount: 0,
			want: []valuePackagePeriodResponseExpectation{
				{Kind: model.ValuePackagePeriodKindFiveHour, LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true},
				{Kind: model.ValuePackagePeriodKindLifecycle, LabelUnit: "day", LabelValue: 1, Limit: 2400, Used: 600, Remaining: 1800, Percent: 25, Refreshes: false},
			},
		},
		{
			name:          "week ignores legacy seven day limit",
			packageType:   model.ValuePackageTypeWeek,
			packageLevel:  model.ValuePackageLevelWeek,
			amountTotal:   4500,
			limit7dAmount: 5500,
			want: []valuePackagePeriodResponseExpectation{
				{Kind: model.ValuePackagePeriodKindFiveHour, LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true},
				{Kind: model.ValuePackagePeriodKindLifecycle, LabelUnit: "day", LabelValue: 7, Limit: 4500, Used: 600, Remaining: 3900, Percent: 600.0 / 4500 * 100, Refreshes: false},
			},
		},
		{
			name:          "month includes seven day stage",
			packageType:   model.ValuePackageTypeMonth,
			packageLevel:  model.ValuePackageLevelMonth,
			amountTotal:   22000,
			limit7dAmount: 5500,
			want: []valuePackagePeriodResponseExpectation{
				{Kind: model.ValuePackagePeriodKindFiveHour, LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Percent: 100.0 / 900 * 100, Refreshes: true},
				{Kind: model.ValuePackagePeriodKindSevenDayStage, LabelUnit: "day", LabelValue: 7, Limit: 5500, Used: 100, Remaining: 5400, Percent: 100.0 / 5500 * 100, Refreshes: true},
				{Kind: model.ValuePackagePeriodKindLifecycle, LabelUnit: "day", LabelValue: 30, Limit: 22000, Used: 600, Remaining: 21400, Percent: 600.0 / 22000 * 100, Refreshes: false},
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := createLdxpControllerTestUser(t, fmt.Sprintf("vp_period_state_%d", i))
			plan := seedValuePackageControllerPlan(t, tt.packageType, tt.packageLevel)
			require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
				"limit_5h_amount": 900,
				"limit_7d_amount": tt.limit7dAmount,
			}).Error)
			model.InvalidateSubscriptionPlanCache(plan.Id)
			plan.Limit5hAmount = 900
			plan.Limit7dAmount = tt.limit7dAmount
			sub := model.UserSubscription{
				UserId:      user.Id,
				PlanId:      plan.Id,
				AmountTotal: tt.amountTotal,
				AmountUsed:  600,
				StartTime:   now - 3600,
				EndTime:     now + 86400,
				Status:      model.UserSubscriptionStatusActive,
				Source:      "controller-period-test",
			}
			require.NoError(t, model.DB.Create(&sub).Error)
			require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id}).Error)
			require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{
				UserId:             user.Id,
				UserSubscriptionId: sub.Id,
				PlanId:             plan.Id,
				PackageType:        plan.PackageType,
				ModelGroup:         plan.ModelGroup,
				RequestId:          fmt.Sprintf("state-period-%d", i),
				Quota:              100,
				CreatedAt:          now - 1800,
			}))

			recorder := valuePackageControllerRequest(GetValuePackageSelf, http.MethodGet, "/value-packages/self", nil, user.Id)

			require.Equal(t, http.StatusOK, recorder.Code)
			body := decodeTestResponse(t, recorder)
			require.Equal(t, true, body["success"], recorder.Body.String())
			data, ok := body["data"].(map[string]interface{})
			require.True(t, ok)
			usage, ok := data["usage"].(map[string]interface{})
			require.True(t, ok)
			requireValuePackagePeriodResponse(t, decodeValuePackagePeriodResponse(t, usage), tt.want)
			if tt.packageType == model.ValuePackageTypeWeek {
				requireValuePackageLegacyWeek7dZero(t, usage)
			}
		})
	}
}

func TestGetValuePackagePurchaseIntentConfirmedCover(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_intent_user")
	day := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	month := seedValuePackageControllerPlan(t, model.ValuePackageTypeMonth, model.ValuePackageLevelMonth)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: day.Id, StartTime: now - 10, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)

	rec := valuePackageControllerRequest(GetValuePackagePurchaseIntent, http.MethodGet, fmt.Sprintf("/value-packages/plans/%d/purchase-intent?confirmed_cover=true", month.Id), nil, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	assert.Equal(t, model.ValuePackagePurchaseActionUpgrade, data["action"])
	assert.Equal(t, false, data["requires_confirmation"])
}

func TestCreateValuePackageLdxpSessionCreatesPendingOrder(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_ldxp_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	rec := valuePackageControllerRequest(CreateValuePackageLdxpSession, http.MethodPost, fmt.Sprintf("/value-packages/plans/%d/ldxp/session", plan.Id), gin.H{"confirmed_cover": true}, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	session, ok := data["session"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, session["session_id"])
	assert.Equal(t, 9.9, session["money"])
	assert.NotEmpty(t, data["order_id"])
	assert.NotEmpty(t, data["trade_no"])
	assert.NotEqual(t, data["trade_no"], data["order_id"])

	var orders []model.SubscriptionOrder
	require.NoError(t, model.DB.Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Find(&orders).Error)
	require.Len(t, orders, 1)
	assert.Equal(t, common.TopUpStatusPending, orders[0].Status)
	assert.Equal(t, data["trade_no"], orders[0].TradeNo)
	assert.Equal(t, float64(orders[0].Id), data["order_id"])
}

func TestCreateValuePackageLdxpSessionRequiresPaymentComplianceBeforeCreatingRecords(t *testing.T) {
	setupValuePackageControllerTest(t)
	setPaymentComplianceForTest(t, false)
	user := createLdxpControllerTestUser(t, "vp_ldxp_compliance_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	rec := valuePackageControllerRequest(CreateValuePackageLdxpSession, http.MethodPost, fmt.Sprintf("/value-packages/plans/%d/ldxp/session", plan.Id), gin.H{"confirmed_cover": true}, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	require.Contains(t, body["message"], "compliance_required")

	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&orderCount).Error)
	assert.Zero(t, orderCount)
	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("user_id = ?", user.Id).Count(&sessionCount).Error)
	assert.Zero(t, sessionCount)
}

func TestGetSubscriptionSelfExcludesValuePackageSubscriptions(t *testing.T) {
	t.Run("normal and value-package returns only normal subscriptions", func(t *testing.T) {
		setupValuePackageControllerTest(t)
		user := createLdxpControllerTestUser(t, "vp_self_mixed_user")
		valuePackage := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
		normal := model.SubscriptionPlan{Title: "normal subscription", PriceAmount: 1, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindSubscription}
		require.NoError(t, model.DB.Create(&normal).Error)
		model.InvalidateSubscriptionPlanCache(normal.Id)
		now := common.GetTimestamp()
		normalSub := model.UserSubscription{UserId: user.Id, PlanId: normal.Id, AmountTotal: 100, StartTime: now - 10, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
		valuePackageSub := model.UserSubscription{UserId: user.Id, PlanId: valuePackage.Id, AmountTotal: 0, StartTime: now - 10, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
		require.NoError(t, model.DB.Create(&normalSub).Error)
		require.NoError(t, model.DB.Create(&valuePackageSub).Error)

		rec := valuePackageControllerRequest(GetSubscriptionSelf, http.MethodGet, "/subscription/self", nil, user.Id)

		body := decodeTestResponse(t, rec)
		require.Equal(t, true, body["success"], rec.Body.String())
		data := body["data"].(map[string]interface{})
		subscriptions := data["subscriptions"].([]interface{})
		allSubscriptions := data["all_subscriptions"].([]interface{})
		require.Len(t, subscriptions, 1)
		require.Len(t, allSubscriptions, 1)
		assert.Equal(t, float64(normalSub.Id), subscriptions[0].(map[string]interface{})["subscription"].(map[string]interface{})["id"])
		assert.Equal(t, float64(normalSub.Id), allSubscriptions[0].(map[string]interface{})["subscription"].(map[string]interface{})["id"])
	})

	t.Run("only value-package returns empty subscriptions", func(t *testing.T) {
		setupValuePackageControllerTest(t)
		user := createLdxpControllerTestUser(t, "vp_self_only_user")
		valuePackage := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
		now := common.GetTimestamp()
		valuePackageSub := model.UserSubscription{UserId: user.Id, PlanId: valuePackage.Id, AmountTotal: 0, StartTime: now - 10, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
		require.NoError(t, model.DB.Create(&valuePackageSub).Error)

		rec := valuePackageControllerRequest(GetSubscriptionSelf, http.MethodGet, "/subscription/self", nil, user.Id)

		body := decodeTestResponse(t, rec)
		require.Equal(t, true, body["success"], rec.Body.String())
		data := body["data"].(map[string]interface{})
		assert.Empty(t, data["subscriptions"].([]interface{}))
		assert.Empty(t, data["all_subscriptions"].([]interface{}))
	})
}

func TestSubscriptionPaymentHandlersRejectValuePackagePlans(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_payment_guard_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	tests := []struct {
		name    string
		handler gin.HandlerFunc
		path    string
		body    gin.H
	}{
		{
			name:    "epay",
			handler: SubscriptionRequestEpay,
			path:    "/subscription/epay/pay",
			body:    gin.H{"plan_id": plan.Id, "payment_method": "alipay"},
		},
		{
			name:    "stripe",
			handler: SubscriptionRequestStripePay,
			path:    "/subscription/stripe/pay",
			body:    gin.H{"plan_id": plan.Id},
		},
		{
			name:    "creem",
			handler: SubscriptionRequestCreemPay,
			path:    "/subscription/creem/pay",
			body:    gin.H{"plan_id": plan.Id},
		},
		{
			name:    "waffo pancake",
			handler: SubscriptionRequestWaffoPancakePay,
			path:    "/subscription/waffo-pancake/pay",
			body:    gin.H{"plan_id": plan.Id},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := valuePackageControllerRequest(tt.handler, http.MethodPost, tt.path, tt.body, user.Id)

			body := decodeTestResponse(t, rec)
			require.Equal(t, false, body["success"], rec.Body.String())
			require.Contains(t, body["message"], "超值套餐仅支持联动小铺购买")
			var orderCount int64
			require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&orderCount).Error)
			assert.Zero(t, orderCount)
		})
	}
}

func TestSubscriptionBalancePayRejectsValuePackagePlan(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_balance_user")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 1000000).Error)
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	err := model.PurchaseSubscriptionWithBalance(user.Id, plan.Id)

	require.Error(t, err)
	require.Contains(t, err.Error(), "超值套餐仅支持联动小铺购买")
	var subCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", user.Id).Count(&subCount).Error)
	assert.Zero(t, subCount)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&orderCount).Error)
	assert.Zero(t, orderCount)
	var reloaded model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", user.Id).First(&reloaded).Error)
	assert.Equal(t, 1000000, reloaded.Quota)
}

func TestAdminBindSubscriptionHandlersRejectValuePackagePlan(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_admin_bind_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	rec := valuePackageControllerRequest(AdminBindSubscription, http.MethodPost, "/subscription/admin/bind", gin.H{"user_id": user.Id, "plan_id": plan.Id}, 1)
	body := decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	require.Contains(t, body["message"], "超值套餐不能通过普通订阅绑定，请使用超值套餐专用流程")

	rec = valuePackageControllerRequest(AdminCreateUserSubscription, http.MethodPost, fmt.Sprintf("/subscription/admin/users/%d/subscriptions", user.Id), gin.H{"plan_id": plan.Id}, 1)
	body = decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	require.Contains(t, body["message"], "超值套餐不能通过普通订阅绑定，请使用超值套餐专用流程")

	var subCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", user.Id).Count(&subCount).Error)
	assert.Zero(t, subCount)
}

func TestSubscriptionOrderCompletionRejectsValuePackagePlan(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_webhook_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	order := model.SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "value-package-ordinary-webhook",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&order).Error)

	err := model.CompleteSubscriptionOrder(order.TradeNo, "payload", model.PaymentProviderStripe, model.PaymentMethodStripe)

	require.Error(t, err)
	require.Contains(t, err.Error(), "超值套餐仅支持联动小铺购买")
	var subCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", user.Id).Count(&subCount).Error)
	assert.Zero(t, subCount)
	var topupCount int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", order.TradeNo).Count(&topupCount).Error)
	assert.Zero(t, topupCount)
	var reloaded model.SubscriptionOrder
	require.NoError(t, model.DB.Where("trade_no = ?", order.TradeNo).First(&reloaded).Error)
	assert.Equal(t, common.TopUpStatusPending, reloaded.Status)
	assert.Zero(t, reloaded.UserSubscriptionId)
}

func TestActivateAndDeactivateValuePackageAPI(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_active_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 3600, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "activate-reset-fields", Quota: 10, CreatedAt: now - 1800}))

	rec := valuePackageControllerRequest(ActivateValuePackageSelf, http.MethodPost, "/value-packages/activate", gin.H{"user_subscription_id": sub.Id}, user.Id)
	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	pref, ok := data["preference"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, pref["enabled"])
	assert.Equal(t, float64(sub.Id), pref["active_user_subscription_id"])
	usage, ok := data["usage"].(map[string]interface{})
	require.True(t, ok)
	requireValuePackageUsageResetFields(t, usage)
	assert.Equal(t, float64(10), usage["used_5h"])
	assert.Equal(t, float64(0), usage["used_7d"])
	assert.Equal(t, float64(0), usage["limit_7d"])
	assert.Greater(t, valuePackageUsageNumber(t, usage, "reset_seconds_5h"), float64(0))
	assert.Equal(t, float64(0), valuePackageUsageNumber(t, usage, "reset_seconds_7d"))

	rec = valuePackageControllerRequest(DeactivateValuePackageSelf, http.MethodPost, "/value-packages/deactivate", nil, user.Id)
	body = decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data, ok = body["data"].(map[string]interface{})
	require.True(t, ok)
	pref, ok = data["preference"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, pref["enabled"])
	assert.Equal(t, float64(sub.Id), pref["active_user_subscription_id"])
	subscription, ok := data["subscription"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(sub.Id), subscription["id"])
	responsePlan, ok := data["plan"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(plan.Id), responsePlan["id"])
}

func TestResetValuePackageQuotaSelfConsumesResetCount(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_reset_quota_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 100
	plan.Limit7dAmount = 200
	require.NoError(t, model.DB.Save(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, AmountUsed: 300, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "before-reset", Quota: 100, CreatedAt: now - 1800}))

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", gin.H{"user_subscription_id": sub.Id}, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool                    `json:"success"`
		Data    model.ValuePackageState `json:"data"`
		Message string                  `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.EqualValues(t, 0, resp.Data.Preference.ResetCount)
	require.NotNil(t, resp.Data.Usage)
	require.EqualValues(t, 0, resp.Data.Usage.Used5h)
	require.EqualValues(t, 0, resp.Data.Usage.Used7d)
	require.EqualValues(t, 300, resp.Data.Usage.TotalUsed)
}

func TestResetValuePackageQuotaSelfRejectsWithoutCount(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_reset_quota_no_count_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 0}).Error)

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", gin.H{"user_subscription_id": sub.Id}, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "重置次数不足")
}

func TestResetValuePackageQuotaSelfAllowsEmptyBodyForActivePackage(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_reset_quota_empty_body_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", nil, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool                    `json:"success"`
		Data    model.ValuePackageState `json:"data"`
		Message string                  `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.EqualValues(t, 0, resp.Data.Preference.ResetCount)
}

func TestResetValuePackageQuotaSelfRejectsMalformedJSON(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_reset_quota_bad_json_user")

	rec := valuePackageControllerRawRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", `{"user_subscription_id":`, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "参数错误")
}

func TestResetValuePackageQuotaSelfRejectsInvalidSubscriptionIdWithoutConsumingCount(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_reset_quota_invalid_sub_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", gin.H{"user_subscription_id": -1}, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "参数错误")
	var pref model.UserValuePackagePreference
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.EqualValues(t, 1, pref.ResetCount)
	var resetCount int64
	require.NoError(t, model.DB.Model(&model.ValuePackageQuotaReset{}).Where("user_id = ?", user.Id).Count(&resetCount).Error)
	require.EqualValues(t, 0, resetCount)
}

func TestResetValuePackageQuotaSelfRejectsMismatchedSubscriptionIdWithoutConsumingCount(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_reset_quota_mismatch_sub_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	now := common.GetTimestamp()
	activeSub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&activeSub).Error)
	otherSub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&otherSub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: activeSub.Id, ResetCount: 1}).Error)

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", gin.H{"user_subscription_id": otherSub.Id}, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "当前套餐不匹配")
	var pref model.UserValuePackagePreference
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.EqualValues(t, 1, pref.ResetCount)
}

func TestAdminCreateSubscriptionPlanNormalizesValuePackageLevelFromType(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
	plan.PackageLevel = model.ValuePackageLevelMonth

	rec := valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": plan}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	assert.Equal(t, model.ValuePackageTypeDay, data["package_type"])
	assert.Equal(t, float64(model.ValuePackageLevelDay), data["package_level"])
	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.First(&persisted, int(data["id"].(float64))).Error)
	assert.Equal(t, model.ValuePackageLevelDay, persisted.PackageLevel)
}

func TestAdminCreateSubscriptionPlanAllowsEnabledValuePackageWithoutCustomLdxpTTL(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
	plan.Title = "enabled value package with default ldxp ttl"
	plan.Enabled = true
	plan.LdxpSessionTTLSeconds = 0

	rec := valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": plan}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	id := int(data["id"].(float64))
	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.First(&persisted, id).Error)
	assert.True(t, persisted.Enabled)
	assert.Equal(t, "https://ldxp.example.test/"+model.ValuePackageTypeDay, persisted.LdxpProductUrl)
	assert.Equal(t, model.ValuePackageTypeDay+" product", persisted.LdxpProductName)
	assert.Equal(t, 9.9, persisted.LdxpProductAmount)
	assert.Zero(t, persisted.LdxpSessionTTLSeconds)
}

func TestAdminCreateSubscriptionPlanPersistsDisabledValuePackageWithoutLdxpConfig(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
	plan.Title = "disabled value package without ldxp"
	plan.Enabled = false
	plan.LdxpProductUrl = ""
	plan.LdxpProductName = ""
	plan.LdxpProductAmount = 0
	plan.LdxpProductRef = ""
	plan.LdxpSessionTTLSeconds = 0

	rec := valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": plan}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data := body["data"].(map[string]interface{})
	id := int(data["id"].(float64))
	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.First(&persisted, id).Error)
	assert.False(t, persisted.Enabled)

	user := createLdxpControllerTestUser(t, "vp_disabled_created_list_user")
	plans, err := model.GetValuePackagePlansForUser(user.Id)
	require.NoError(t, err)
	for _, visible := range plans {
		assert.NotEqual(t, persisted.Id, visible.Id)
	}
}

func TestAdminCreateSubscriptionPlanRejectsInvalidValuePackageConfig(t *testing.T) {
	setupValuePackageControllerTest(t)

	missingLdxpPlan := model.SubscriptionPlan{
		Title:                 "bad value package",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		Limit5hAmount:         100,
		Limit7dAmount:         1000,
		LdxpProductUrl:        "",
		LdxpProductName:       "",
		LdxpProductAmount:     0,
		LdxpSessionTTLSeconds: 0,
	}
	rec := valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": missingLdxpPlan}, 1)
	body := decodeTestResponse(t, rec)
	assert.Equal(t, false, body["success"], rec.Body.String())

	invalidConcurrencyPlan := missingLdxpPlan
	invalidConcurrencyPlan.Title = "invalid concurrency"
	invalidConcurrencyPlan.ConcurrencyLimit = 3
	invalidConcurrencyPlan.LdxpProductUrl = "https://ldxp.example.test/day"
	invalidConcurrencyPlan.LdxpProductName = "Day product"
	invalidConcurrencyPlan.LdxpProductAmount = 9.9
	invalidConcurrencyPlan.LdxpSessionTTLSeconds = 900
	rec = valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": invalidConcurrencyPlan}, 1)
	body = decodeTestResponse(t, rec)
	assert.Equal(t, false, body["success"], rec.Body.String())
}

func TestAdminCreateSubscriptionPlanRejectsInvalidValuePackageValidationCases(t *testing.T) {
	setupValuePackageControllerTest(t)

	tests := []struct {
		name        string
		packageType string
		mutate      func(*model.SubscriptionPlan)
		message     string
	}{
		{
			name: "invalid package type",
			mutate: func(plan *model.SubscriptionPlan) {
				plan.PackageType = "year"
			},
			message: "套餐类型必须是 day、week 或 month",
		},
		{
			name: "missing model group",
			mutate: func(plan *model.SubscriptionPlan) {
				plan.ModelGroup = " "
			},
			message: "套餐模型分组不能为空",
		},
		{
			name: "negative 5h limit",
			mutate: func(plan *model.SubscriptionPlan) {
				plan.Limit5hAmount = -1
			},
			message: "套餐额度不能为负数",
		},
		{
			name: "negative 7d limit",
			mutate: func(plan *model.SubscriptionPlan) {
				plan.Limit7dAmount = -1
			},
			message: "套餐额度不能为负数",
		},
		{
			name: "day 7d limit unsupported",
			mutate: func(plan *model.SubscriptionPlan) {
				plan.Limit7dAmount = 1
			},
			message: "日卡和周卡不支持7天阶段限额",
		},
		{
			name:        "week 7d limit unsupported",
			packageType: model.ValuePackageTypeWeek,
			mutate: func(plan *model.SubscriptionPlan) {
				plan.Limit7dAmount = 1
			},
			message: "日卡和周卡不支持7天阶段限额",
		},
		{
			name:        "month 7d lower than 5h",
			packageType: model.ValuePackageTypeMonth,
			mutate: func(plan *model.SubscriptionPlan) {
				plan.Limit5hAmount = 1000
				plan.Limit7dAmount = 999
			},
			message: "7天额度不能小于5小时额度",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageType := tt.packageType
			if packageType == "" {
				packageType = model.ValuePackageTypeDay
			}
			plan := validAdminValuePackagePlanForTest(packageType)
			tt.mutate(&plan)

			rec := valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": plan}, 1)

			body := decodeTestResponse(t, rec)
			require.Equal(t, false, body["success"], rec.Body.String())
			assert.Contains(t, body["message"], tt.message)
		})
	}
}

func TestNormalizeAndValidateSubscriptionPlanRequestValidatesValuePackageTotals(t *testing.T) {
	tests := []struct {
		name    string
		plan    model.SubscriptionPlan
		message string
	}{
		{
			name: "value package zero total",
			plan: func() model.SubscriptionPlan {
				plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
				plan.TotalAmount = 0
				return plan
			}(),
			message: "超值套餐总额度必须大于0",
		},
		{
			name: "value package negative total",
			plan: func() model.SubscriptionPlan {
				plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
				plan.TotalAmount = -1
				return plan
			}(),
			message: "超值套餐总额度必须大于0",
		},
		{
			name: "month stage exceeds total",
			plan: func() model.SubscriptionPlan {
				plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeMonth)
				plan.TotalAmount = 999
				plan.Limit7dAmount = 1000
				return plan
			}(),
			message: "7天阶段额度不能大于30天总额度",
		},
		{
			name: "month stage equals total",
			plan: func() model.SubscriptionPlan {
				plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeMonth)
				plan.TotalAmount = 1000
				plan.Limit7dAmount = 1000
				return plan
			}(),
		},
		{
			name: "day stale stage remains invalid",
			plan: func() model.SubscriptionPlan {
				plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
				plan.Limit7dAmount = 1
				return plan
			}(),
			message: "日卡和周卡不支持7天阶段限额",
		},
		{
			name: "week stale stage remains invalid",
			plan: func() model.SubscriptionPlan {
				plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeWeek)
				plan.Limit7dAmount = 1
				return plan
			}(),
			message: "日卡和周卡不支持7天阶段限额",
		},
		{
			name: "ordinary subscription zero total remains valid",
			plan: model.SubscriptionPlan{PlanKind: model.SubscriptionPlanKindSubscription, TotalAmount: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tt.plan
			require.Equal(t, tt.message, normalizeAndValidateSubscriptionPlanRequest(&plan))
		})
	}
}

func TestAdminCreateSubscriptionPlanRejectsUnknownPlanKind(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := model.SubscriptionPlan{
		Title:         "unknown kind",
		PriceAmount:   1,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		PlanKind:      "mystery",
	}

	rec := valuePackageControllerRequest(AdminCreateSubscriptionPlan, http.MethodPost, "/subscription/admin/plans", gin.H{"plan": plan}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	assert.Contains(t, body["message"], "套餐类型无效")
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("title = ?", plan.Title).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAdminUpdateSubscriptionPlanPersistsValuePackageFields(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	updated := plan
	updated.Title = "updated month value package"
	updated.PlanKind = model.SubscriptionPlanKindValuePackage
	updated.PackageType = model.ValuePackageTypeMonth
	updated.PackageLevel = model.ValuePackageLevelMonth
	updated.ModelGroup = "month-card"
	updated.ConcurrencyLimit = 2
	updated.Limit5hAmount = 2000
	updated.Limit7dAmount = 8000
	updated.Benefits = "month benefit"
	updated.LdxpProductUrl = "https://ldxp.example.test/month-updated"
	updated.LdxpProductName = "Month updated product"
	updated.LdxpProductAmount = 29.9
	updated.LdxpProductRef = "ref-month-updated"
	updated.LdxpSessionTTLSeconds = 1200

	rec := valuePackageControllerRequest(AdminUpdateSubscriptionPlan, http.MethodPut, fmt.Sprintf("/subscription/admin/plans/%d", plan.Id), gin.H{"plan": updated}, 1)
	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())

	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.Where("id = ?", plan.Id).First(&persisted).Error)
	assert.Equal(t, model.SubscriptionPlanKindValuePackage, persisted.PlanKind)
	assert.Equal(t, model.ValuePackageTypeMonth, persisted.PackageType)
	assert.Equal(t, model.ValuePackageLevelMonth, persisted.PackageLevel)
	assert.Equal(t, "month-card", persisted.ModelGroup)
	assert.Equal(t, 2, persisted.ConcurrencyLimit)
	assert.EqualValues(t, 2000, persisted.Limit5hAmount)
	assert.EqualValues(t, 8000, persisted.Limit7dAmount)
	assert.Equal(t, "month benefit", persisted.Benefits)
	assert.Equal(t, "https://ldxp.example.test/month-updated", persisted.LdxpProductUrl)
	assert.Equal(t, "Month updated product", persisted.LdxpProductName)
	assert.Equal(t, 29.9, persisted.LdxpProductAmount)
	assert.Equal(t, "ref-month-updated", persisted.LdxpProductRef)
	assert.EqualValues(t, 1200, persisted.LdxpSessionTTLSeconds)
}

func TestAdminUpdateSubscriptionPlanNormalizesValuePackageLevelFromType(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeMonth, model.ValuePackageLevelMonth)
	updated := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
	updated.PackageLevel = model.ValuePackageLevelMonth

	rec := valuePackageControllerRequest(AdminUpdateSubscriptionPlan, http.MethodPut, fmt.Sprintf("/subscription/admin/plans/%d", plan.Id), gin.H{"plan": updated}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.First(&persisted, plan.Id).Error)
	assert.Equal(t, model.ValuePackageTypeDay, persisted.PackageType)
	assert.Equal(t, model.ValuePackageLevelDay, persisted.PackageLevel)
}

func TestAdminUpdateSubscriptionPlanReturnsErrorForMissingPlan(t *testing.T) {
	setupValuePackageControllerTest(t)
	updated := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)

	rec := valuePackageControllerRequest(AdminUpdateSubscriptionPlan, http.MethodPut, "/subscription/admin/plans/999999", gin.H{"plan": updated}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	assert.Contains(t, body["message"], "套餐不存在")
}

func TestAdminUpdateSubscriptionPlanStatusRejectsInvalidValuePackageEnable(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
	plan.Enabled = false
	plan.LdxpProductUrl = ""
	plan.LdxpProductName = ""
	plan.LdxpProductAmount = 0
	plan.LdxpSessionTTLSeconds = 0
	require.NoError(t, model.DB.Create(&plan).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)

	rec := valuePackageControllerRequest(AdminUpdateSubscriptionPlanStatus, http.MethodPatch, fmt.Sprintf("/subscription/admin/plans/%d", plan.Id), gin.H{"enabled": true}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	assert.Contains(t, body["message"], "启用超值套餐时必须配置 LDXP 商品链接、名称和金额")
	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.First(&persisted, plan.Id).Error)
	assert.False(t, persisted.Enabled)
}

func TestAdminUpdateSubscriptionPlanStatusAllowsEnabledValuePackageWithoutCustomLdxpTTL(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := validAdminValuePackagePlanForTest(model.ValuePackageTypeDay)
	plan.Title = "status enable value package with default ldxp ttl"
	plan.Enabled = false
	plan.LdxpSessionTTLSeconds = 0
	require.NoError(t, model.DB.Create(&plan).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)

	rec := valuePackageControllerRequest(AdminUpdateSubscriptionPlanStatus, http.MethodPatch, fmt.Sprintf("/subscription/admin/plans/%d", plan.Id), gin.H{"enabled": true}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	var persisted model.SubscriptionPlan
	require.NoError(t, model.DB.First(&persisted, plan.Id).Error)
	assert.True(t, persisted.Enabled)
	assert.Zero(t, persisted.LdxpSessionTTLSeconds)
}

func TestAdminUpdateSubscriptionPlanStatusReturnsErrorForMissingPlan(t *testing.T) {
	setupValuePackageControllerTest(t)

	rec := valuePackageControllerRequest(AdminUpdateSubscriptionPlanStatus, http.MethodPatch, "/subscription/admin/plans/999999", gin.H{"enabled": true}, 1)

	body := decodeTestResponse(t, rec)
	require.Equal(t, false, body["success"], rec.Body.String())
	assert.Contains(t, body["message"], "套餐不存在")
}

func TestGetValuePackageSelfIncludesBillingState(t *testing.T) {
	setupValuePackageControllerTest(t)
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeMonth, model.ValuePackageLevelMonth)
	user := createLdxpControllerTestUser(t, "vp_billing_state_user")
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: common.GetTimestamp() - 1000, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	_, err := model.ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)

	rec := valuePackageControllerRequest(GetValuePackageSelf, http.MethodGet, "/value-packages/self", nil, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Success bool                    `json:"success"`
		Data    model.ValuePackageState `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.NotNil(t, body.Data.Billing)
	require.True(t, body.Data.Billing.Active)
	require.Equal(t, "", body.Data.Billing.RoutingGroup)
	require.Equal(t, plan.ModelGroup, body.Data.Billing.PackageGroup)
	require.Equal(t, float64(1), body.Data.Billing.EffectiveRatio)
	require.Equal(t, float64(0), body.Data.Billing.OriginalGroupRatio)
	require.Equal(t, plan.Title, body.Data.Billing.PlanTitle)
	require.Equal(t, plan.Id, body.Data.Billing.PlanId)
}

func requireValuePackageUsageResetFields(t *testing.T, usage map[string]interface{}) {
	t.Helper()
	for _, key := range []string{
		"reset_at_5h",
		"reset_seconds_5h",
		"limited_5h",
		"reset_at_7d",
		"reset_seconds_7d",
		"limited_7d",
	} {
		require.Contains(t, usage, key)
	}
	require.IsType(t, float64(0), usage["reset_at_5h"])
	require.IsType(t, float64(0), usage["reset_seconds_5h"])
	require.IsType(t, false, usage["limited_5h"])
	require.IsType(t, float64(0), usage["reset_at_7d"])
	require.IsType(t, float64(0), usage["reset_seconds_7d"])
	require.IsType(t, false, usage["limited_7d"])
}

func valuePackageUsageNumber(t *testing.T, usage map[string]interface{}, key string) float64 {
	t.Helper()
	value, ok := usage[key].(float64)
	require.True(t, ok)
	return value
}

type valuePackagePeriodResponseExpectation struct {
	Kind       string
	LabelUnit  string
	LabelValue int
	Limit      int64
	Used       int64
	Remaining  int64
	Percent    float64
	Refreshes  bool
}

type valuePackagePeriodResponseDTO struct {
	Kind       string  `json:"kind"`
	LabelUnit  string  `json:"label_unit"`
	LabelValue int     `json:"label_value"`
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percent    float64 `json:"percent"`
	Refreshes  bool    `json:"refreshes"`
	ResetAt    int64   `json:"reset_at"`
	Limited    bool    `json:"limited"`
}

func decodeValuePackagePeriodResponse(t *testing.T, usage map[string]interface{}) []valuePackagePeriodResponseDTO {
	t.Helper()
	rawPeriods, exists := usage["period_limits"]
	require.True(t, exists, "usage JSON must contain period_limits")
	require.NotNil(t, rawPeriods, "period_limits must be a non-null array")
	periodArray, ok := rawPeriods.([]interface{})
	require.True(t, ok, "period_limits must be a JSON array")
	payload, err := common.Marshal(periodArray)
	require.NoError(t, err)
	var periods []valuePackagePeriodResponseDTO
	require.NoError(t, common.Unmarshal(payload, &periods))
	return periods
}

func requireValuePackagePeriodResponse(t *testing.T, got []valuePackagePeriodResponseDTO, want []valuePackagePeriodResponseExpectation) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		require.Equal(t, want[i].Kind, got[i].Kind, "period %d kind", i)
		require.Equal(t, want[i].LabelUnit, got[i].LabelUnit, "period %d label unit", i)
		require.Equal(t, want[i].LabelValue, got[i].LabelValue, "period %d label value", i)
		require.Equal(t, want[i].Limit, got[i].Limit, "period %d limit", i)
		require.Equal(t, want[i].Used, got[i].Used, "period %d used", i)
		require.Equal(t, want[i].Remaining, got[i].Remaining, "period %d remaining", i)
		require.InDelta(t, want[i].Percent, got[i].Percent, 0.000001, "period %d percent", i)
		require.Equal(t, want[i].Refreshes, got[i].Refreshes, "period %d refreshes", i)
		require.False(t, got[i].Limited, "period %d limited", i)
		if want[i].Refreshes {
			require.Greater(t, got[i].ResetAt, int64(0), "period %d reset at", i)
		} else {
			require.Zero(t, got[i].ResetAt, "period %d reset at", i)
		}
	}
}

func requireValuePackageLegacyWeek7dZero(t *testing.T, usage map[string]interface{}) {
	t.Helper()
	for _, key := range []string{"used_7d", "limit_7d", "reset_at_7d", "reset_seconds_7d"} {
		value, exists := usage[key]
		require.True(t, exists, "week usage JSON must contain %s", key)
		require.Equal(t, float64(0), value, "week usage JSON %s", key)
	}
	limited7d, exists := usage["limited_7d"]
	require.True(t, exists, "week usage JSON must contain limited_7d")
	require.Equal(t, false, limited7d, "week usage JSON limited_7d")
}
