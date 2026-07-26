package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderTopUpSuccessResponse(t *testing.T, result *model.RedeemResult) map[string]interface{} {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.JSON(http.StatusOK, buildTopUpSuccessResponse(result))
	return decodeTestResponse(t, recorder)
}

func TestTopUpResponseContractByRedemptionType(t *testing.T) {
	quotaBody := renderTopUpSuccessResponse(t, &model.RedeemResult{
		Type:       model.RedemptionTypeQuota,
		Quota:      250,
		Redemption: model.RedeemRedemptionMeta{Type: model.RedemptionTypeQuota},
	})
	require.Equal(t, true, quotaBody["success"])
	assert.Equal(t, float64(250), quotaBody["data"], "quota remains a bare number for legacy clients")

	subscriptionBody := renderTopUpSuccessResponse(t, &model.RedeemResult{
		Type:       model.RedemptionTypeSubscription,
		PlanId:     42,
		PlanTitle:  "contract plan",
		Redemption: model.RedeemRedemptionMeta{Type: model.RedemptionTypeSubscription, PlanId: 42},
	})
	subscriptionData, ok := subscriptionBody["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, model.RedemptionTypeSubscription, subscriptionData["type"])
	assert.Equal(t, "contract plan", subscriptionData["plan_title"])

	resetBody := renderTopUpSuccessResponse(t, &model.RedeemResult{
		Type:           model.RedemptionTypeResetCard,
		ResetCardCount: 2,
		Redemption:     model.RedeemRedemptionMeta{Type: model.RedemptionTypeResetCard, ResetCardCount: 2},
	})
	resetData, ok := resetBody["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, model.RedemptionTypeResetCard, resetData["type"])
	assert.Equal(t, float64(2), resetData["reset_card_count"])
}
