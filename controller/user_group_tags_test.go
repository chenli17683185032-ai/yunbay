package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userGroupTagsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"data"`
}

func TestGetUserGroupTagsReturnsUserTagsNotModelGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/group-tags", nil)

	GetUserGroupTags(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response userGroupTagsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}{
		{Value: model.UserGroupTiyan, Label: "体验用户"},
		{Value: model.UserGroupVIP, Label: "VIP 用户"},
	}, response.Data)
	for _, option := range response.Data {
		require.NotContains(t, []string{model.DefaultModelGroup, "gpt-pro"}, option.Value)
	}
}
