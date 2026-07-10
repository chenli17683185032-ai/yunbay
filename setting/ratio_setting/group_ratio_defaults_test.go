package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestDefaultGroupRatioSettingsAreYunbayModelGroups(t *testing.T) {
	var groupRatio map[string]float64
	require.NoError(t, common.UnmarshalJsonStr(GroupRatio2JSONString(), &groupRatio))
	require.Equal(t, map[string]float64{
		"gpt-plus": 0.3,
		"gpt-pro":  0.4,
	}, groupRatio)
	require.NotContains(t, groupRatio, "default")
	require.NotContains(t, groupRatio, "vip")
	require.NotContains(t, groupRatio, "svip")

	var groupGroupRatio map[string]map[string]float64
	require.NoError(t, common.UnmarshalJsonStr(GroupGroupRatio2JSONString(), &groupGroupRatio))
	require.Equal(t, map[string]map[string]float64{
		"体验用户": {
			"gpt-plus": 0.99,
			"gpt-pro":  1.32,
		},
	}, groupGroupRatio)
	require.NotContains(t, groupGroupRatio, "vip")

	require.Empty(t, GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll())
	exported := config.GlobalConfig.ExportAllConfigs()
	require.NotContains(t, exported, "group_ratio_setting.group_ratio")
	require.NotContains(t, exported, "group_ratio_setting.group_group_ratio")
	require.Contains(t, exported, "group_ratio_setting.group_special_usable_group")
}
