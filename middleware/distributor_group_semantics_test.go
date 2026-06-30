package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupDistributorGroupSemanticsTestDB(t *testing.T) {
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
}

func TestDistributePlaygroundGroupPermissionUsesUserTag(t *testing.T) {
	require.NoError(t, i18n.Init())
	setupDistributorGroupSemanticsTestDB(t)

	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldSpecialUsableGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	common.MemoryCacheEnabled = false
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Set(model.UserGroupTiyan, map[string]string{
		"+:gpt-pro": "PRO 模型分组",
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-plus":"Plus 模型分组"}`))
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(oldSpecialUsableGroups)
	})

	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	priority := int64(0)
	channel := model.Channel{
		Type:     constant.ChannelTypeOpenAI,
		Key:      "sk-test",
		Status:   common.ChannelStatusEnabled,
		Name:     "gpt-pro-channel",
		Models:   "gpt-4o",
		Group:    "gpt-pro",
		Priority: &priority,
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "gpt-pro",
		Model:     "gpt-4o",
		ChannelId: channel.Id,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupTiyan)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"channel_id":  common.GetContextKeyString(c, constant.ContextKeyChannelId),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-4o","group":"gpt-pro","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-pro"`)
}
