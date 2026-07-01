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

func TestUserGroupTagsRouteDoesNotFallThroughToUserIDRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	userRoute := router.Group("/api/user")
	{
		// Keep the same ordering shape as the API router: static routes must be
		// registered before the dynamic /:id admin route, otherwise Gin will route
		// /api/user/group-tags to GetUser and it will try strconv.Atoi("group-tags").
		userRoute.GET("/group-tags", GetUserGroupTags)
		userRoute.GET("/:id", GetUser)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/group-tags", nil)
	router.ServeHTTP(recorder, req)

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
}
