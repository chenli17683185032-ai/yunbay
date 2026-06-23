package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setPaymentComplianceForTest(t *testing.T, confirmed bool) {
	t.Helper()

	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	originalConfirmedAt := paymentSetting.ComplianceConfirmedAt
	originalConfirmedBy := paymentSetting.ComplianceConfirmedBy
	originalConfirmedIP := paymentSetting.ComplianceConfirmedIP

	paymentSetting.ComplianceConfirmed = confirmed
	if confirmed {
		paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	} else {
		paymentSetting.ComplianceTermsVersion = ""
	}
	paymentSetting.ComplianceConfirmedAt = 0
	paymentSetting.ComplianceConfirmedBy = 0
	paymentSetting.ComplianceConfirmedIP = ""

	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
		paymentSetting.ComplianceConfirmedAt = originalConfirmedAt
		paymentSetting.ComplianceConfirmedBy = originalConfirmedBy
		paymentSetting.ComplianceConfirmedIP = originalConfirmedIP
	})
}

func TestTopupAmountRequiresComplianceBeforeParsingBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setPaymentComplianceForTest(t, false)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/amount", strings.NewReader(`{`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestAmount(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "compliance_required")
}

func TestStripePayRequiresComplianceBeforeParsingBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setPaymentComplianceForTest(t, false)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/stripe/pay", strings.NewReader(`{`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestStripePay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "compliance_required")
}
