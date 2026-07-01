package controller

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTopUpInfoShowsLdxpWhenAllowlistIsConfigured(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	t.Setenv("LDXP_ALLOWED_USERNAMES", "jiance001")
	user := createLdxpControllerTestUser(t, "ordinary_user")

	recorder := performLdxpControllerRequest(GetTopUpInfo, http.MethodGet, "/topup/info", nil, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, data["enable_ldxp_topup"])
}

func TestGetTopUpInfoStillShowsLdxpForPreviouslyAllowedUsername(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	t.Setenv("LDXP_ALLOWED_USERNAMES", "jiance001")
	user := createLdxpControllerTestUser(t, "jiance001")

	recorder := performLdxpControllerRequest(GetTopUpInfo, http.MethodGet, "/topup/info", nil, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, data["enable_ldxp_topup"])
}
