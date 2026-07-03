package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
