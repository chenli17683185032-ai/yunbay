package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuthGroupSemanticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldSQLitePath := common.SQLitePath
	oldIsMasterNode := common.IsMasterNode
	oldUserUsableGroups := setting.UserUsableGroups2JSONString()
	oldGroupRatio := ratio_setting.GroupRatio2JSONString()

	common.RedisEnabled = false
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.IsMasterNode = false
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-plus":"Plus 模型分组","gpt-pro":"PRO 模型分组"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-plus":0.3,"gpt-pro":0.4}`))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SQLitePath = dsn
	require.NoError(t, model.InitDB())
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}))
	db := model.DB
	model.LOG_DB = db

	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatio))
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.SQLitePath = oldSQLitePath
		common.IsMasterNode = oldIsMasterNode
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})

	return db
}

func TestTokenAuthKeepsUserTagAndUsingGroupSeparate(t *testing.T) {
	setupAuthGroupSemanticsTestDB(t)

	user := model.User{Username: "exp", Password: "hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, Quota: 1000000}
	require.NoError(t, model.DB.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "testtoken", Name: "test", Status: common.TokenStatusEnabled, Group: "gpt-plus", ExpiredTime: -1, UnlimitedQuota: true}
	require.NoError(t, model.DB.Create(&token).Error)

	router := gin.New()
	router.Use(TokenAuth())
	router.GET("/v1/test", func(c *gin.Context) {
		userGroup, _ := common.GetContextKey(c, constant.ContextKeyUserGroup)
		usingGroup, _ := common.GetContextKey(c, constant.ContextKeyUsingGroup)
		tokenGroup, _ := common.GetContextKey(c, constant.ContextKeyTokenGroup)
		c.JSON(http.StatusOK, gin.H{
			"user_group":  userGroup,
			"using_group": usingGroup,
			"token_group": tokenGroup,
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+token.Key)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"user_group":"体验用户"`)
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"gpt-plus"`)
}
