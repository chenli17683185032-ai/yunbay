package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupContextForTokenDefaultsEmptyGroupToGptPlus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	token := &model.Token{UserId: 7, Id: 8, Key: "legacy-empty", Group: "", UnlimitedQuota: true}

	require.NoError(t, SetupContextForToken(ctx, token))

	require.Equal(t, constant.TokenGroupGPTPlus, common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))
	require.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyTokenCrossGroupRetry))
}

func TestSetupContextForTokenClearsRetryOutsideAutoGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	token := &model.Token{UserId: 7, Id: 8, Key: "gpt-plus", Group: constant.TokenGroupGPTPlus, CrossGroupRetry: true, UnlimitedQuota: true}

	require.NoError(t, SetupContextForToken(ctx, token))

	require.Equal(t, constant.TokenGroupGPTPlus, common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))
	require.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyTokenCrossGroupRetry))
}

func withMiddlewareUserUsableGroups(t *testing.T, groups string) {
	t.Helper()
	original := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(groups))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(original))
	})
}

func TestResolvePlaygroundUsingGroupDefaultsToGptPlusTokenGroup(t *testing.T) {
	withMiddlewareUserUsableGroups(t, `{"gpt-plus":"GPT Plus","vip":"VIP"}`)

	group, ok := resolvePlaygroundUsingGroup(model.UserGroupTiyan, "")

	require.True(t, ok)
	require.Equal(t, constant.TokenGroupGPTPlus, group)
}

func TestResolvePlaygroundUsingGroupRejectsUnavailableGptPlusInsteadOfFallingBackToUserGroup(t *testing.T) {
	withMiddlewareUserUsableGroups(t, `{"default":"Default"}`)

	group, ok := resolvePlaygroundUsingGroup(model.UserGroupTiyan, "")

	require.False(t, ok)
	require.Equal(t, constant.TokenGroupGPTPlus, group)
}

func TestResolvePlaygroundUsingGroupKeepsExplicitAllowedModelGroup(t *testing.T) {
	withMiddlewareUserUsableGroups(t, `{"gpt-plus":"GPT Plus","vip":"VIP"}`)

	group, ok := resolvePlaygroundUsingGroup(model.UserGroupTiyan, "vip")

	require.True(t, ok)
	require.Equal(t, "vip", group)
}
