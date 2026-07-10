package router

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionGroupRatioRouteTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	common.OptionMapRWMutex.RLock()
	originalOptionGroupRatio, hadOptionGroupRatio := common.OptionMap["GroupRatio"]
	originalOptionGroupGroupRatio, hadOptionGroupGroupRatio := common.OptionMap["GroupGroupRatio"]
	common.OptionMapRWMutex.RUnlock()

	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SubscriptionPlan{}, &model.User{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "option-group-ratio-root",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}).Error)

	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("option-group-ratio-route-test"))))
	SetApiRouter(r)
	r.GET("/__seed_option_group_ratio_session", func(c *gin.Context) {
		role, err := strconv.Atoi(c.Query("role"))
		require.NoError(t, err)
		session := sessions.Default(c)
		session.Set("username", "option-group-ratio-user")
		session.Set("role", role)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})

	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		common.OptionMapRWMutex.Lock()
		if hadOptionGroupRatio {
			common.OptionMap["GroupRatio"] = originalOptionGroupRatio
		} else {
			delete(common.OptionMap, "GroupRatio")
		}
		if hadOptionGroupGroupRatio {
			common.OptionMap["GroupGroupRatio"] = originalOptionGroupGroupRatio
		} else {
			delete(common.OptionMap, "GroupGroupRatio")
		}
		common.OptionMapRWMutex.Unlock()
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return r, db
}

func addOptionGroupRatioSession(t *testing.T, r *gin.Engine, req *http.Request, role int) {
	t.Helper()
	seed := httptest.NewRecorder()
	seedReq := httptest.NewRequest(http.MethodGet, "/__seed_option_group_ratio_session?role="+strconv.Itoa(role), nil)
	r.ServeHTTP(seed, seedReq)
	require.Equal(t, http.StatusNoContent, seed.Code)
	for _, sessionCookie := range seed.Result().Cookies() {
		req.AddCookie(sessionCookie)
	}
	req.Header.Set("New-Api-User", "1")
}

func TestOptionGroupRatioRouteRequiresRootAndAuditOmitsRatioJSON(t *testing.T) {
	r, db := setupOptionGroupRatioRouteTest(t)

	adminRecorder := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodGet, "/api/option/group-ratios", nil)
	addOptionGroupRatioSession(t, r, adminRequest, common.RoleAdminUser)
	r.ServeHTTP(adminRecorder, adminRequest)
	var adminResponse struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminResponse))
	require.False(t, adminResponse.Success)

	const groupSentinel = "group-ratio-audit-sentinel"
	const nestedSentinel = "nested-ratio-audit-sentinel"
	body, err := common.Marshal(map[string]string{
		"group_ratio":       `{"` + groupSentinel + `":1}`,
		"group_group_ratio": `{"package":{"` + nestedSentinel + `":1.25}}`,
	})
	require.NoError(t, err)
	rootRecorder := httptest.NewRecorder()
	rootRequest := httptest.NewRequest(http.MethodPut, "/api/option/group-ratios", bytes.NewReader(body))
	rootRequest.Header.Set("Content-Type", "application/json")
	addOptionGroupRatioSession(t, r, rootRequest, common.RoleRootUser)
	r.ServeHTTP(rootRecorder, rootRequest)
	var rootResponse struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(rootRecorder.Body.Bytes(), &rootResponse))
	require.True(t, rootResponse.Success, rootRecorder.Body.String())

	deadline := time.Now().Add(2 * time.Second)
	var audit model.Log
	for time.Now().Before(deadline) {
		err = db.Where("type = ?", model.LogTypeManage).Order("id DESC").First(&audit).Error
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, err)
	require.NotContains(t, audit.Other, groupSentinel)
	require.NotContains(t, audit.Other, nestedSentinel)
	require.Contains(t, audit.Other, "/api/option/group-ratios")
}
