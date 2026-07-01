package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApiUserGroupTagsRouteIsRegisteredBeforeDynamicUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("user-group-tags-route-test"))))
	SetApiRouter(r)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/group-tags", nil)
	seedAdminSession(t, r, recorder, req)
	r.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response userGroupTagsRouteTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 2)
	require.Equal(t, model.UserGroupTiyan, response.Data[0].Value)
	require.Equal(t, model.UserGroupVIP, response.Data[1].Value)
}

type userGroupTagsRouteTestResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"data"`
}

func seedAdminSession(t *testing.T, r *gin.Engine, recorder *httptest.ResponseRecorder, req *http.Request) {
	t.Helper()
	seed := httptest.NewRecorder()
	seedReq := httptest.NewRequest(http.MethodGet, "/__seed_session", nil)
	r.GET("/__seed_session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "admin")
		session.Set("role", common.RoleAdminUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	r.ServeHTTP(seed, seedReq)
	for _, cookie := range seed.Result().Cookies() {
		req.AddCookie(cookie)
	}
	req.Header.Set("New-Api-User", "1")
}
