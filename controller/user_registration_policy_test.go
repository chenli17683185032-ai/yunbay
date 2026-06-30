package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRegistrationPolicyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldRegisterEnabled := common.RegisterEnabled
	oldPasswordRegisterEnabled := common.PasswordRegisterEnabled
	oldEmailVerificationEnabled := common.EmailVerificationEnabled
	oldQuotaForNewUser := common.QuotaForNewUser
	oldQuotaForInvitee := common.QuotaForInvitee
	oldQuotaForInviter := common.QuotaForInviter
	oldGenerateDefaultToken := constant.GenerateDefaultToken
	oldDefaultUseAutoGroup := setting.DefaultUseAutoGroup

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = true
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	constant.GenerateDefaultToken = false
	setting.DefaultUseAutoGroup = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.RegisterEnabled = oldRegisterEnabled
		common.PasswordRegisterEnabled = oldPasswordRegisterEnabled
		common.EmailVerificationEnabled = oldEmailVerificationEnabled
		common.QuotaForNewUser = oldQuotaForNewUser
		common.QuotaForInvitee = oldQuotaForInvitee
		common.QuotaForInviter = oldQuotaForInviter
		constant.GenerateDefaultToken = oldGenerateDefaultToken
		setting.DefaultUseAutoGroup = oldDefaultUseAutoGroup
	})

	return db
}

func performRegisterPolicyRequest(body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)
	return recorder
}

func TestRegisterRejectsNonQQEmail(t *testing.T) {
	setupRegistrationPolicyTestDB(t)

	recorder := performRegisterPolicyRequest(`{"username":"gmailuser","password":"password123","email":"abc@gmail.com","verification_code":"123456"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "仅支持 QQ 邮箱注册")
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "gmailuser").Count(&count).Error)
	require.Zero(t, count)
}

func TestRegisterAcceptsQQEmailCaseInsensitiveAndDefaultsTiyanTag(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	common.EmailVerificationEnabled = false

	recorder := performRegisterPolicyRequest(`{"username":"qquser","password":"password123","email":"ABC@qq.com"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "qquser").First(&user).Error)
	require.Equal(t, model.UserGroupTiyan, user.Group)
	require.Equal(t, "ABC@qq.com", user.Email)
}

func TestRegisterRejectsInvalidAffCode(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	common.EmailVerificationEnabled = false

	recorder := performRegisterPolicyRequest(`{"username":"badinvite","password":"password123","email":"badinvite@qq.com","aff_code":"missing"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "邀请码无效")
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "badinvite").Count(&count).Error)
	require.Zero(t, count)
}

func TestRegisterWithValidAffCodeSetsInviterID(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	common.EmailVerificationEnabled = false

	inviter := model.User{Username: "inviter", Password: "hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, AffCode: "GOOD"}
	require.NoError(t, model.DB.Create(&inviter).Error)

	recorder := performRegisterPolicyRequest(`{"username":"invitee","password":"password123","email":"invitee@qq.com","aff_code":"GOOD"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var invitee model.User
	require.NoError(t, model.DB.Where("username = ?", "invitee").First(&invitee).Error)
	require.Equal(t, inviter.Id, invitee.InviterId)
}

func TestRegisterDefaultTokenUsesDefaultModelGroupWhenAutoGroupDisabled(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = true
	setting.DefaultUseAutoGroup = false

	recorder := performRegisterPolicyRequest(`{"username":"tokengroup","password":"password123","email":"tokengroup@qq.com"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "tokengroup").First(&user).Error)
	var token model.Token
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&token).Error)
	require.Equal(t, "gpt-plus", token.Group)
}
