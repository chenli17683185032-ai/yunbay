package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
