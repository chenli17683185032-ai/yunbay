package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminCreateSubscriptionRedemptionsCreatesRequestedCodes(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	setPaymentComplianceForTest(t, true)
	model.DB.Exec("DELETE FROM redemptions")
	model.DB.Exec("DELETE FROM subscription_plans")
	plan := model.SubscriptionPlan{Title: "周卡", DurationUnit: model.SubscriptionDurationDay, DurationValue: 7, Enabled: true, TotalAmount: 7000}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/admin/plans/1/redemptions", strings.NewReader(`{"name":"周卡兑换码","count":2,"expired_time":0}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("id", 99)

	AdminCreateSubscriptionRedemptions(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var codes []model.Redemption
	require.NoError(t, model.DB.Where("type = ? AND plan_id = ?", model.RedemptionTypeSubscription, plan.Id).Find(&codes).Error)
	require.Len(t, codes, 2)
	for _, code := range codes {
		assert.Equal(t, model.RedemptionTypeSubscription, code.Type)
		assert.Equal(t, plan.Id, code.PlanId)
		assert.Equal(t, common.RedemptionCodeStatusEnabled, code.Status)
		assert.Equal(t, 0, code.Quota)
		assert.Equal(t, "周卡兑换码", code.Name)
		assert.Equal(t, 99, code.UserId)
		assert.Equal(t, int64(0), code.ExpiredTime)
	}
}
