package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestGetUserUsableGroups_DoesNotAutoAddUserTag(t *testing.T) {
	originalJSON := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalJSON))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{
		"gpt-plus": "GPT Plus",
		"gpt-pro": "GPT Pro"
	}`))

	groups := GetUserUsableGroups("体验用户")

	require.NotContains(t, groups, "体验用户")
	require.Equal(t, "GPT Plus", groups["gpt-plus"])
	require.Equal(t, "GPT Pro", groups["gpt-pro"])
	require.Len(t, groups, 2)
}
