package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestParseOrderManagementRange7dAnd30d(t *testing.T) {
	now := int64(1782887783)

	start, end, err := parseOrderManagementRange("7d", "", "", now)
	require.NoError(t, err)
	require.Equal(t, int64(1782282983), start)
	require.Equal(t, now, end)

	start, end, err = parseOrderManagementRange("30d", "", "", now)
	require.NoError(t, err)
	require.Equal(t, int64(1780295783), start)
	require.Equal(t, now, end)
}

func TestParseOrderManagementRangeCustom(t *testing.T) {
	now := int64(1782887783)

	start, end, err := parseOrderManagementRange("", "1782518400", "1782604800", now)
	require.NoError(t, err)
	require.Equal(t, int64(1782518400), start)
	require.Equal(t, int64(1782604800), end)

	start, end, err = parseOrderManagementRange("custom", "1782518400", "1782604800", now)
	require.NoError(t, err)
	require.Equal(t, int64(1782518400), start)
	require.Equal(t, int64(1782604800), end)
}

func TestAffiliateWithdrawalActionRejectsEmptyRemarkOnReject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "123"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/reject", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	adminAffiliateWithdrawalReject(c)

	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Equal(t, "驳回提现必须填写管理员备注", body["message"])
}

func TestAffiliateWithdrawalActionRejectsWhitespaceRemarkOnReject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "123"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/reject", strings.NewReader(`{"admin_remark":" \n\t "}`))
	c.Request.Header.Set("Content-Type", "application/json")

	adminAffiliateWithdrawalReject(c)

	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Equal(t, "驳回提现必须填写管理员备注", body["message"])
}

func TestDecodeOptionalJSONBodyAllowsUnknownLengthEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/optional", io.NopCloser(strings.NewReader("")))
	require.Equal(t, int64(-1), c.Request.ContentLength)

	var req dto.WithdrawalActionRequest
	decoded, err := decodeOptionalJSONBody(c, &req)
	require.NoError(t, err)
	require.False(t, decoded)
}

func TestMailCheckRequestCustomRangeFromBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/mail-check?start_time=1&end_time=2", strings.NewReader(`{"range":"custom","start_time":"1782518400","end_time":"1782604800"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	var req dto.MailCheckRequest
	decoded, err := decodeOptionalJSONBody(c, &req)
	require.NoError(t, err)
	require.True(t, decoded)

	rangeValue, startValue, endValue := mailCheckRequestRangeValues(c, req)
	require.Equal(t, "custom", rangeValue)
	require.Equal(t, "1782518400", startValue)
	require.Equal(t, "1782604800", endValue)

	start, end, err := parseOrderManagementRange(rangeValue, startValue, endValue, 1782887783)
	require.NoError(t, err)
	require.Equal(t, int64(1782518400), start)
	require.Equal(t, int64(1782604800), end)
}

func setupOrderManagementControllerTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldLogConsumeEnabled := common.LogConsumeEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.TopUp{},
		&model.Redemption{},
		&model.AffiliateCommission{},
		&model.OrderDeletionMark{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.UserValuePackagePreference{},
		&model.ValuePackageUsageRecord{},
		&model.ValuePackageQuotaReset{},
		&model.ValuePackageResetCountLedger{},
		&model.LdxpTopupSession{},
		&model.LdxpMailEvent{},
		&model.SubscriptionPreConsumeRecord{},
	))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		common.LogConsumeEnabled = oldLogConsumeEnabled
	})
}

func newOrderManagementContext(method string, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 99)
	return ctx, recorder
}

func createOrderManagementRouter() *gin.Engine {
	router := gin.New()
	router.DELETE("/billing-orders/:order_type/*trade_no", func(c *gin.Context) {
		c.Set("id", 99)
		AdminDeleteBillingOrder(c)
	})
	return router
}

type adminListOrdersResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Page     int                      `json:"page"`
		PageSize int                      `json:"page_size"`
		Total    int                      `json:"total"`
		Items    []model.AdminOrderRecord `json:"items"`
	} `json:"data"`
}

type adminOrderManagementOrdersResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Page     int                            `json:"page"`
		PageSize int                            `json:"page_size"`
		Total    int                            `json:"total"`
		Items    []dto.OrderManagementOrderItem `json:"items"`
	} `json:"data"`
}

type adminOrderManagementValuePackageUsageResponse struct {
	Success bool                         `json:"success"`
	Data    []model.ValuePackageUsageRow `json:"data"`
}

func TestAdminDeleteOrderRejectsInvalidType(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	ctx, recorder := newOrderManagementContext(http.MethodDelete, "/api/order-management/admin/billing-orders/invoice/A", `{}`)
	ctx.Params = gin.Params{
		{Key: "order_type", Value: "invoice"},
		{Key: "trade_no", Value: "A"},
	}

	AdminDeleteBillingOrder(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "参数错误")
}

func TestAdminDeleteOrderRejectsMalformedJSONWithoutDeletionMark(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	ctx, recorder := newOrderManagementContext(http.MethodDelete, "/api/order-management/admin/billing-orders/topup/TRADE-BAD-JSON", `{"reason":`)
	ctx.Params = gin.Params{
		{Key: "order_type", Value: model.OrderTypeTopup},
		{Key: "trade_no", Value: "TRADE-BAD-JSON"},
	}

	AdminDeleteBillingOrder(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "参数错误")
	var count int64
	require.NoError(t, model.DB.Model(&model.OrderDeletionMark{}).
		Where("order_type = ? AND trade_no = ?", model.OrderTypeTopup, "TRADE-BAD-JSON").
		Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestAdminDeleteOrderRouterAllowsSlashContainingTradeNo(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:        1,
		Amount:        100,
		Money:         1,
		TradeNo:       "TRADE/A",
		PaymentMethod: model.PaymentMethodStripe,
		CreateTime:    1000,
		Status:        common.TopUpStatusPending,
	}).Error)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/billing-orders/topup/TRADE%2FA", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	createOrderManagementRouter().ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var mark model.OrderDeletionMark
	require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.OrderTypeTopup, "TRADE/A").First(&mark).Error)
	assert.Equal(t, 99, mark.DeletedBy)
}

func TestAdminDeleteOrderRouterDoesNotDoubleDecodeTradeNo(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:        1,
		Amount:        100,
		Money:         1,
		TradeNo:       "TRADE%2FA",
		PaymentMethod: model.PaymentMethodStripe,
		CreateTime:    1000,
		Status:        common.TopUpStatusPending,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:        2,
		Amount:        200,
		Money:         2,
		TradeNo:       "TRADE/A",
		PaymentMethod: model.PaymentMethodStripe,
		CreateTime:    1001,
		Status:        common.TopUpStatusPending,
	}).Error)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/billing-orders/topup/TRADE%252FA", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	createOrderManagementRouter().ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var literalEscapedMark model.OrderDeletionMark
	require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.OrderTypeTopup, "TRADE%2FA").First(&literalEscapedMark).Error)
	assert.Equal(t, 99, literalEscapedMark.DeletedBy)

	var slashCount int64
	require.NoError(t, model.DB.Model(&model.OrderDeletionMark{}).
		Where("order_type = ? AND trade_no = ?", model.OrderTypeTopup, "TRADE/A").
		Count(&slashCount).Error)
	assert.Equal(t, int64(0), slashCount)
}

func TestAdminDeleteOrderWritesDeletionMarkKeepsOriginalAndHidesFromList(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	model.DB.Exec("DELETE FROM order_deletion_marks")
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:        1,
		Amount:        100,
		Money:         1,
		TradeNo:       "TRADE-1",
		PaymentMethod: model.PaymentMethodStripe,
		CreateTime:    1000,
		Status:        common.TopUpStatusPending,
	}).Error)
	ctx, recorder := newOrderManagementContext(http.MethodDelete, "/api/order-management/admin/billing-orders/topup/TRADE-1", `{"reason":"测试订单"}`)
	ctx.Params = gin.Params{
		{Key: "order_type", Value: model.OrderTypeTopup},
		{Key: "trade_no", Value: "TRADE-1"},
	}

	AdminDeleteBillingOrder(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var mark model.OrderDeletionMark
	require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.OrderTypeTopup, "TRADE-1").First(&mark).Error)
	assert.Equal(t, 99, mark.DeletedBy)
	assert.Equal(t, "测试订单", mark.Reason)

	var topup model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", "TRADE-1").First(&topup).Error)
	assert.Equal(t, int64(100), topup.Amount)

	listCtx, listRecorder := newOrderManagementContext(http.MethodGet, "/api/order-management/admin/billing-orders?p=1&page_size=10", "")
	AdminListBillingOrders(listCtx)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	var response adminListOrdersResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 0, response.Data.Total)
	assert.Empty(t, response.Data.Items)
}

func TestAdminListOrdersReturnsPageInfo(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	model.DB.Exec("DELETE FROM top_ups")
	model.DB.Exec("DELETE FROM order_deletion_marks")
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:        1,
		Amount:        100,
		Money:         1,
		TradeNo:       "ORDER-LIST-1",
		PaymentMethod: model.PaymentMethodStripe,
		CreateTime:    1000,
		Status:        common.TopUpStatusPending,
	}).Error)

	ctx, recorder := newOrderManagementContext(http.MethodGet, "/api/order-management/admin/billing-orders?p=1&page_size=10", "")
	AdminListBillingOrders(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response adminListOrdersResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Page)
	assert.Equal(t, 10, response.Data.PageSize)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "ORDER-LIST-1", response.Data.Items[0].TradeNo)
	assert.Equal(t, model.OrderTypeTopup, response.Data.Items[0].OrderType)
}

func TestAdminOrderManagementOrdersReturnsValuePackageBillingFields(t *testing.T) {
	setupOrderManagementControllerTestDB(t)

	plan := &model.SubscriptionPlan{
		Title:        "月卡",
		PlanKind:     model.SubscriptionPlanKindValuePackage,
		PackageType:  model.ValuePackageTypeMonth,
		PackageLevel: model.ValuePackageLevelMonth,
		TotalAmount:  30000,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	order := &model.SubscriptionOrder{
		UserId:          88,
		PlanId:          plan.Id,
		Money:           19.9,
		TradeNo:         "LDXP_VP-admin-month",
		PaymentMethod:   model.PaymentMethodLDXP,
		PaymentProvider: model.PaymentProviderLDXP,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      1782518300,
		CompleteTime:    1782518400,
	}
	require.NoError(t, model.DB.Create(order).Error)
	require.NoError(t, model.DB.Create(&model.LdxpTopupSession{
		SessionId:           "vp-admin-month-session",
		UserId:              88,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  plan.Id,
		Money:               19.9,
		WorkerAmount:        19.9,
		WorkerOrderNo:       "LDADMINMONTH",
		Status:              model.LdxpStatusSuccess,
		CreatedTime:         1782518500,
	}).Error)

	ctx, recorder := newOrderManagementContext(http.MethodGet, "/api/order-management/admin/orders?range=custom&start_time=1782518400&end_time=1782604800&p=1&page_size=10", "")

	AdminOrderManagementOrders(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response adminOrderManagementOrdersResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	item := response.Data.Items[0]
	assert.Equal(t, model.OrderTypeSubscription, item.BillingOrderType)
	assert.Equal(t, "LDXP_VP-admin-month", item.TradeNo)
	assert.Equal(t, plan.Id, item.PlanId)
	assert.Equal(t, "月卡", item.PlanTitle)
	assert.Equal(t, model.PaymentMethodLDXP, item.PaymentMethod)
	assert.Equal(t, common.TopUpStatusSuccess, item.OrderStatus)
}

func TestAdminOrderManagementValuePackageUsageReturnsActiveUsers(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	now := common.GetTimestamp()
	user := &model.User{Id: 8891, Username: "vp-admin-usage", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, AffCode: "vp-admin-usage-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:           "日卡",
		Enabled:         true,
		PlanKind:        model.SubscriptionPlanKindValuePackage,
		PackageType:     model.ValuePackageTypeDay,
		PackageLevel:    model.ValuePackageLevelDay,
		Currency:        "CNY",
		DurationUnit:    model.SubscriptionDurationDay,
		DurationValue:   1,
		ModelGroup:      "day-card",
		TotalAmount:     2000,
		Limit5hAmount:   500,
		Limit7dAmount:   1000,
		PriceAmount:     3.9,
		AllowBalancePay: nil,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	sub := &model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, AmountUsed: 350, StartTime: now - 100, EndTime: now + 3600, Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id}).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "admin-usage-5h", Quota: 50, CreatedAt: now - 1800}))
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "admin-usage-7d", Quota: 70, CreatedAt: now - 6*3600}))

	ctx, recorder := newOrderManagementContext(http.MethodGet, "/api/order-management/admin/value-package-usage", "")

	AdminOrderManagementValuePackageUsage(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response adminOrderManagementValuePackageUsageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Len(t, response.Data, 1)
	row := response.Data[0]
	assert.Equal(t, user.Id, row.UserId)
	assert.Equal(t, user.Username, row.Username)
	assert.Equal(t, sub.Id, row.Subscription.Id)
	assert.Equal(t, model.ValuePackageTypeDay, row.Plan.PackageType)
	require.NotNil(t, row.Usage)
	assert.EqualValues(t, 50, row.Usage.Used5h)
	assert.EqualValues(t, 120, row.Usage.Used7d)
	assert.EqualValues(t, 500, row.Usage.Limit5h)
	assert.EqualValues(t, 1000, row.Usage.Limit7d)
	assert.Greater(t, row.Usage.ResetSeconds5h, int64(0))
	assert.Greater(t, row.Usage.ResetSeconds7d, int64(0))
	assert.False(t, row.Usage.Limited5h)
	assert.False(t, row.Usage.Limited7d)

	var raw map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &raw))
	rawData, ok := raw["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, rawData, 1)
	rawRow, ok := rawData[0].(map[string]interface{})
	require.True(t, ok)
	usage, ok := rawRow["usage"].(map[string]interface{})
	require.True(t, ok)
	requireValuePackageUsageResetFields(t, usage)
	assert.Greater(t, valuePackageUsageNumber(t, usage, "reset_seconds_5h"), float64(0))
	assert.Greater(t, valuePackageUsageNumber(t, usage, "reset_seconds_7d"), float64(0))
	assert.Equal(t, false, usage["limited_5h"])
	assert.Equal(t, false, usage["limited_7d"])
}

func TestAdminValuePackageManagementUsersReturnsRows(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	now := common.GetTimestamp()
	user := &model.User{Id: 8893, Username: "vp-management-user", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, AffCode: "vp-management-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:         "月卡",
		Enabled:       true,
		PlanKind:      model.SubscriptionPlanKindValuePackage,
		PackageType:   model.ValuePackageTypeMonth,
		PackageLevel:  model.ValuePackageLevelMonth,
		Currency:      "CNY",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		ModelGroup:    "month-card",
		TotalAmount:   10000,
		Limit5hAmount: 500,
		Limit7dAmount: 5000,
		PriceAmount:   29.9,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	sub := &model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, AmountUsed: 350, StartTime: now - 100, EndTime: now + 86400, Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 3}).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "management-usage", Quota: 50, CreatedAt: now - 1800}))

	ctx, recorder := newOrderManagementContext(http.MethodGet, "/api/order-management/admin/value-packages/users?page=1&page_size=20", "")

	AdminOrderManagementValuePackageUsers(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"reset_count":3`)
	require.Contains(t, recorder.Body.String(), `"items"`)
	require.Contains(t, recorder.Body.String(), "vp-management-user")
	require.Contains(t, recorder.Body.String(), `"used_5h":50`)
}

func TestAdminAdjustValuePackageResetCount(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	user := &model.User{Id: 8892, Username: "vp-reset-adjust", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, AffCode: "vp-reset-adjust-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, ResetCount: 1}).Error)

	ctx, recorder := newOrderManagementContext(http.MethodPost, fmt.Sprintf("/api/order-management/admin/value-packages/users/%d/reset-count", user.Id), `{"mode":"add","value":2,"reason":"test"}`)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}

	AdminAdjustValuePackageResetCount(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"old_count":1`)
	require.Contains(t, recorder.Body.String(), `"new_count":3`)
	var pref model.UserValuePackagePreference
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.EqualValues(t, 3, pref.ResetCount)
}

func TestAdminAdjustValuePackageResetCountRejectsInvalidRequestsWithoutLedger(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	user := &model.User{Id: 8894, Username: "vp-reset-invalid", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, AffCode: "vp-reset-invalid-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, ResetCount: 2}).Error)

	cases := []struct {
		name   string
		userID int
		body   string
	}{
		{name: "missing value", userID: user.Id, body: `{"mode":"add"}`},
		{name: "negative value", userID: user.Id, body: `{"mode":"add","value":-1}`},
		{name: "zero add", userID: user.Id, body: `{"mode":"add","value":0}`},
		{name: "invalid mode", userID: user.Id, body: `{"mode":"bad","value":1}`},
		{name: "missing user", userID: 999999, body: `{"mode":"add","value":1}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newOrderManagementContext(http.MethodPost, fmt.Sprintf("/api/order-management/admin/value-packages/users/%d/reset-count", tc.userID), tc.body)
			ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", tc.userID)}}

			AdminAdjustValuePackageResetCount(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			body := decodeTestResponse(t, recorder)
			require.Equal(t, false, body["success"])
		})
	}

	var pref model.UserValuePackagePreference
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.EqualValues(t, 2, pref.ResetCount)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.ValuePackageResetCountLedger{}).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
	var orphanPrefCount int64
	require.NoError(t, model.DB.Model(&model.UserValuePackagePreference{}).Where("user_id = ?", 999999).Count(&orphanPrefCount).Error)
	require.Zero(t, orphanPrefCount)
}
