package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createVIPModalTestUser(t *testing.T, username string, group string) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password123",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       username + "@example.test",
		Group:       group,
		AffCode:     "aff_" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func TestMarkVIPUpgradeModalSeen(t *testing.T) {
	setupValuePackageControllerTest(t)

	t.Run("vip user can mark seen", func(t *testing.T) {
		user := createVIPModalTestUser(t, "vip-modal-vip", model.UserGroupVIP)

		recorder := valuePackageControllerRequest(MarkVIPUpgradeModalSeen, http.MethodPost, "/user/vip-upgrade-modal/seen", nil, user.Id)

		assert.Equal(t, http.StatusOK, recorder.Code)
		body := decodeTestResponse(t, recorder)
		assert.Equal(t, true, body["success"])

		data, ok := body["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, data["vip_upgrade_modal_seen"])

		setting, err := model.GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.True(t, setting.VipUpgradeModalSeen)
	})

	t.Run("non vip rejected", func(t *testing.T) {
		user := createVIPModalTestUser(t, "vip-modal-regular", model.UserGroupDefault)

		recorder := valuePackageControllerRequest(MarkVIPUpgradeModalSeen, http.MethodPost, "/user/vip-upgrade-modal/seen", nil, user.Id)

		assert.Equal(t, http.StatusOK, recorder.Code)
		body := decodeTestResponse(t, recorder)
		assert.Equal(t, false, body["success"])

		setting, err := model.GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.False(t, setting.VipUpgradeModalSeen)
	})
}

func TestUpdateUserSettingPreservesVIPUpgradeModalSeen(t *testing.T) {
	setupValuePackageControllerTest(t)

	user := createVIPModalTestUser(t, "vip-modal-setting-preserve", model.UserGroupVIP)
	setting := user.GetSetting()
	setting.VipUpgradeModalSeen = true
	setting.UpstreamModelUpdateNotifyEnabled = true
	user.SetSetting(setting)
	require.NoError(t, user.Update(false))

	recorder := valuePackageControllerRequest(UpdateUserSetting, http.MethodPut, "/user/setting", gin.H{
		"notify_type":                           dto.NotifyTypeEmail,
		"quota_warning_threshold":              12.5,
		"notification_email":                   "vip-modal-setting-preserve@example.test",
		"accept_unset_model_ratio_model":       true,
		"record_ip_log":                        true,
		"upstream_model_update_notify_enabled": false,
	}, user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	assert.Equal(t, true, body["success"])

	savedSetting, err := model.GetUserSetting(user.Id, true)
	require.NoError(t, err)
	assert.True(t, savedSetting.VipUpgradeModalSeen)
	assert.Equal(t, dto.NotifyTypeEmail, savedSetting.NotifyType)
	assert.Equal(t, 12.5, savedSetting.QuotaWarningThreshold)
	assert.Equal(t, "vip-modal-setting-preserve@example.test", savedSetting.NotificationEmail)
	assert.True(t, savedSetting.AcceptUnsetRatioModel)
	assert.True(t, savedSetting.RecordIpLog)
	assert.True(t, savedSetting.UpstreamModelUpdateNotifyEnabled)
}
