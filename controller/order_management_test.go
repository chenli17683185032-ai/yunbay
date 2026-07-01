package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	c.Request = httptest.NewRequest(http.MethodPost, "/reject", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	adminAffiliateWithdrawalReject(c)

	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
}
