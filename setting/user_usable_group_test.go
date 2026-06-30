package setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestDefaultUserUsableGroupsAreYunbayModelGroups(t *testing.T) {
	var groups map[string]string
	require.NoError(t, common.UnmarshalJsonStr(UserUsableGroups2JSONString(), &groups))

	require.Equal(t, map[string]string{
		"gpt-plus": "Plus 模型分组",
		"gpt-pro":  "PRO 模型分组",
	}, groups)
	require.NotContains(t, groups, "default")
	require.NotContains(t, groups, "vip")
	require.NotContains(t, groups, "体验用户")
}
