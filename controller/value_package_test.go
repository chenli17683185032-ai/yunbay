package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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

func valuePackageControllerRoutePattern(path string) string {
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
	return path
}

func seedValuePackageControllerPlan(t *testing.T, packageType string, packageLevel int) model.SubscriptionPlan {
	t.Helper()
	plan := model.SubscriptionPlan{
		Title:                 packageType + " value package",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		SortOrder:             10,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           packageType,
		PackageLevel:          packageLevel,
		ModelGroup:            packageType + "-card",
		ConcurrencyLimit:      1,
		Limit5hAmount:         1000,
		Limit7dAmount:         5000,
		Benefits:              "fast lane",
		LdxpProductUrl:        "https://ldxp.example.test/value-package/" + packageType,
		LdxpProductName:       packageType + " value package product",
		LdxpProductAmount:     9.9,
		LdxpProductRef:        "ref-" + packageType,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	return plan
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

func TestActivateAndDeactivateValuePackageAPI(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_active_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)

	rec := valuePackageControllerRequest(ActivateValuePackageSelf, http.MethodPost, "/value-packages/activate", gin.H{"user_subscription_id": sub.Id}, user.Id)
	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	pref, ok := data["preference"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, pref["enabled"])
	assert.Equal(t, float64(sub.Id), pref["active_user_subscription_id"])

	rec = valuePackageControllerRequest(DeactivateValuePackageSelf, http.MethodPost, "/value-packages/deactivate", nil, user.Id)
	body = decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	data, ok = body["data"].(map[string]interface{})
	require.True(t, ok)
	pref, ok = data["preference"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, pref["enabled"])
	assert.Equal(t, float64(0), pref["active_user_subscription_id"])
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
