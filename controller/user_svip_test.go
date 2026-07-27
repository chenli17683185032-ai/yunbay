package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createSVIPControllerTestUser(t *testing.T, username string, validTopupCents int64) *model.User {
	t.Helper()
	user := &model.User{
		Username:        username,
		Password:        "password123",
		DisplayName:     username,
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		Email:           username + "@example.test",
		Group:           model.UserGroupDefault,
		AffCode:         "aff_" + username,
		ValidTopupCents: validTopupCents,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func svipAdminRequest(handler gin.HandlerFunc, body any, adminID int, adminRole int) *httptest.ResponseRecorder {
	router := gin.New()
	router.Handle(http.MethodPost, "/user/manage", func(c *gin.Context) {
		c.Set("id", adminID)
		c.Set("role", adminRole)
		handler(c)
	})

	var reqBody bytes.Buffer
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			panic(err)
		}
		reqBody.Write(payload)
	}
	req := httptest.NewRequest(http.MethodPost, "/user/manage", &reqBody)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestMarkSVIPCelebrationSeen(t *testing.T) {
	setupValuePackageControllerTest(t)

	t.Run("svip user can mark seen", func(t *testing.T) {
		user := createSVIPControllerTestUser(t, "svip-modal-yes", 20000)

		recorder := valuePackageControllerRequest(MarkSVIPCelebrationSeen, http.MethodPost, "/user/svip-celebration/seen", nil, user.Id)

		assert.Equal(t, http.StatusOK, recorder.Code)
		body := decodeTestResponse(t, recorder)
		assert.Equal(t, true, body["success"])

		setting, err := model.GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.True(t, setting.SvipCelebrationSeen)
	})

	t.Run("non svip rejected", func(t *testing.T) {
		user := createSVIPControllerTestUser(t, "svip-modal-no", 19999)

		recorder := valuePackageControllerRequest(MarkSVIPCelebrationSeen, http.MethodPost, "/user/svip-celebration/seen", nil, user.Id)

		assert.Equal(t, http.StatusOK, recorder.Code)
		body := decodeTestResponse(t, recorder)
		assert.Equal(t, false, body["success"])

		setting, err := model.GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.False(t, setting.SvipCelebrationSeen)
	})
}

func TestManageUserAddQuotaCountAsValidTopup(t *testing.T) {
	setupValuePackageControllerTest(t)

	target := createSVIPControllerTestUser(t, "svip-manage-target", 0)
	quotaFor100Yuan := int(100 * common.QuotaPerUnit)

	// 勾选「计入有效充值」：100 元 → 10000 分
	recorder := svipAdminRequest(ManageUser, gin.H{
		"id":                   target.Id,
		"action":               "add_quota",
		"mode":                 "add",
		"value":                quotaFor100Yuan,
		"count_as_valid_topup": true,
	}, 999, common.RoleRootUser)
	assert.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"], body["message"])

	var got model.User
	require.NoError(t, model.DB.First(&got, "id = ?", target.Id).Error)
	assert.Equal(t, int64(10000), got.ValidTopupCents)
	assert.Equal(t, quotaFor100Yuan, got.Quota)

	// 不勾选：余额增加但有效充值不变
	recorder = svipAdminRequest(ManageUser, gin.H{
		"id":     target.Id,
		"action": "add_quota",
		"mode":   "add",
		"value":  quotaFor100Yuan,
	}, 999, common.RoleRootUser)
	assert.Equal(t, http.StatusOK, recorder.Code)
	body = decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"], body["message"])

	require.NoError(t, model.DB.First(&got, "id = ?", target.Id).Error)
	assert.Equal(t, int64(10000), got.ValidTopupCents)
	assert.Equal(t, 2*quotaFor100Yuan, got.Quota)
}

func TestManageUserAddQuotaAndValidTopupRemainAtomicOnFailure(t *testing.T) {
	setupValuePackageControllerTest(t)

	target := createSVIPControllerTestUser(t, "svip-manage-atomic", 500)
	target.Quota = 700
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", target.Id).Update("quota", target.Quota).Error)
	require.NoError(t, model.DB.Exec(`
		CREATE TRIGGER fail_svip_atomic_update
		BEFORE UPDATE OF valid_topup_cents ON users
		BEGIN
			SELECT RAISE(ABORT, 'forced valid topup failure');
		END;
	`).Error)
	t.Cleanup(func() {
		_ = model.DB.Exec("DROP TRIGGER IF EXISTS fail_svip_atomic_update").Error
	})

	recorder := svipAdminRequest(ManageUser, gin.H{
		"id":                   target.Id,
		"action":               "add_quota",
		"mode":                 "add",
		"value":                int(common.QuotaPerUnit),
		"count_as_valid_topup": true,
	}, 999, common.RoleRootUser)

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	assert.Equal(t, false, body["success"])
	var got model.User
	require.NoError(t, model.DB.First(&got, "id = ?", target.Id).Error)
	assert.Equal(t, 700, got.Quota)
	assert.Equal(t, int64(500), got.ValidTopupCents)
}
