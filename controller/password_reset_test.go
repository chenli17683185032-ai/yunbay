package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type passwordResetTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func createPasswordResetTestUser(t *testing.T, email string) model.User {
	t.Helper()
	hashedPassword, err := common.Password2Hash("old-password")
	require.NoError(t, err)

	user := model.User{
		Username: "reset-user-" + strings.ReplaceAll(email, "@", "-"),
		Email:    email,
		Password: hashedPassword,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    model.UserGroupTiyan,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func performPasswordResetRequest(t *testing.T, body string) passwordResetTestResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/reset", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	ResetPassword(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response passwordResetTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestResetPasswordAcceptsVerifiedCustomPasswordAndConsumesCode(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	email := "custom-reset@qq.com"
	createPasswordResetTestUser(t, email)
	common.RegisterVerificationCodeWithKey(email, "ABC123", common.PasswordResetPurpose)

	body := `{"email":"custom-reset@qq.com","token":"ABC123","password":"new-password-123"}`
	response := performPasswordResetRequest(t, body)

	require.True(t, response.Success)
	require.Empty(t, response.Data, "custom passwords must not be echoed back")

	var updatedUser model.User
	require.NoError(t, model.DB.Where("email = ?", email).First(&updatedUser).Error)
	require.True(t, common.ValidatePasswordAndHash("new-password-123", updatedUser.Password))

	retry := performPasswordResetRequest(t, body)
	require.False(t, retry.Success, "verification code must be single-use")
}

func TestResetPasswordRejectsInvalidCustomPasswordWithoutConsumingCode(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	email := "invalid-password@qq.com"
	createPasswordResetTestUser(t, email)
	common.RegisterVerificationCodeWithKey(email, "DEF456", common.PasswordResetPurpose)

	for _, password := range []string{"", "short", strings.Repeat("x", 21)} {
		response := performPasswordResetRequest(t, `{"email":"invalid-password@qq.com","token":"DEF456","password":"`+password+`"}`)
		require.False(t, response.Success)
	}

	response := performPasswordResetRequest(t, `{"email":"invalid-password@qq.com","token":"DEF456","password":"valid-password"}`)
	require.True(t, response.Success, "validation errors must not consume the code")
}

func TestResetPasswordKeepsLegacyGeneratedPasswordFlow(t *testing.T) {
	setupRegistrationPolicyTestDB(t)
	email := "legacy-reset@qq.com"
	createPasswordResetTestUser(t, email)
	common.RegisterVerificationCodeWithKey(email, "GHI789", common.PasswordResetPurpose)

	response := performPasswordResetRequest(t, `{"email":"legacy-reset@qq.com","token":"GHI789"}`)

	require.True(t, response.Success)
	require.Len(t, response.Data, 12)

	var updatedUser model.User
	require.NoError(t, model.DB.Where("email = ?", email).First(&updatedUser).Error)
	require.True(t, common.ValidatePasswordAndHash(response.Data, updatedUser.Password))
}
