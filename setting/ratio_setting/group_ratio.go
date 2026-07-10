package ratio_setting

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"gpt-plus": 0.3,
	"gpt-pro":  0.4,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"体验用户": {
		"gpt-plus": 0.99,
		"gpt-pro":  1.32,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	normalized, _, err := ParseAndNormalizeGroupRatioJSON(jsonStr)
	if err != nil {
		return err
	}
	groupRatioMap.ReplaceAll(normalized)
	return nil
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok || !isFiniteRatio(ratio) || ratio <= 0 {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	normalized, _, err := ParseAndNormalizeGroupGroupRatioJSON(jsonStr)
	if err != nil {
		return err
	}
	groupGroupRatioMap.ReplaceAll(normalized)
	return nil
}

func CheckGroupRatio(jsonStr string) error {
	_, _, err := ParseAndNormalizeGroupRatioJSON(jsonStr)
	return err
}

func CheckGroupGroupRatio(jsonStr string) error {
	_, _, err := ParseAndNormalizeGroupGroupRatioJSON(jsonStr)
	return err
}

func NormalizeGroupRatio(groupRatios map[string]float64) (map[string]float64, error) {
	normalized := make(map[string]float64, len(groupRatios))
	for rawName, ratio := range groupRatios {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, errors.New("group ratio name must not be empty")
		}
		if _, exists := normalized[name]; exists {
			return nil, fmt.Errorf("group ratio name conflicts after trimming: %s", name)
		}
		if !isFiniteRatio(ratio) {
			return nil, fmt.Errorf("group ratio must be finite: %s", name)
		}
		if ratio < 0 {
			return nil, errors.New("group ratio must be not less than 0: " + name)
		}
		normalized[name] = ratio
	}
	return normalized, nil
}

func ParseAndNormalizeGroupRatioJSON(jsonStr string) (map[string]float64, string, error) {
	parsedPointers := make(map[string]*float64)
	if err := common.UnmarshalJsonStr(jsonStr, &parsedPointers); err != nil {
		return nil, "", err
	}
	if parsedPointers == nil {
		return nil, "", errors.New("group ratio must be a JSON object")
	}
	parsed := make(map[string]float64, len(parsedPointers))
	for name, ratio := range parsedPointers {
		if ratio == nil {
			return nil, "", fmt.Errorf("group ratio must not be null: %s", strings.TrimSpace(name))
		}
		parsed[name] = *ratio
	}
	normalized, err := NormalizeGroupRatio(parsed)
	if err != nil {
		return nil, "", err
	}
	normalizedBytes, err := common.Marshal(normalized)
	if err != nil {
		return nil, "", err
	}
	return normalized, string(normalizedBytes), nil
}

func NormalizeGroupGroupRatio(groupGroupRatios map[string]map[string]float64) (map[string]map[string]float64, error) {
	normalized := make(map[string]map[string]float64, len(groupGroupRatios))
	seenParents := make(map[string]struct{}, len(groupGroupRatios))
	for rawParent, childRatios := range groupGroupRatios {
		parent := strings.TrimSpace(rawParent)
		if parent == "" {
			return nil, errors.New("group group ratio parent must not be empty")
		}
		if _, exists := seenParents[parent]; exists {
			return nil, fmt.Errorf("group group ratio parent conflicts after trimming: %s", parent)
		}
		seenParents[parent] = struct{}{}
		if childRatios == nil {
			return nil, fmt.Errorf("group group ratio children must be a JSON object: %s", parent)
		}

		normalizedChildren := make(map[string]float64, len(childRatios))
		for rawChild, ratio := range childRatios {
			child := strings.TrimSpace(rawChild)
			if child == "" {
				return nil, fmt.Errorf("group group ratio child must not be empty: %s", parent)
			}
			if _, exists := normalizedChildren[child]; exists {
				return nil, fmt.Errorf("group group ratio child conflicts after trimming: %s/%s", parent, child)
			}
			if !isFiniteRatio(ratio) || ratio <= 0 {
				return nil, fmt.Errorf("group group ratio must be finite and greater than 0: %s/%s", parent, child)
			}
			normalizedChildren[child] = ratio
		}
		if len(normalizedChildren) > 0 {
			normalized[parent] = normalizedChildren
		}
	}
	return normalized, nil
}

func ParseAndNormalizeGroupGroupRatioJSON(jsonStr string) (map[string]map[string]float64, string, error) {
	parsed := make(map[string]map[string]float64)
	if err := common.UnmarshalJsonStr(jsonStr, &parsed); err != nil {
		return nil, "", err
	}
	if parsed == nil {
		return nil, "", errors.New("group group ratio must be a JSON object")
	}
	normalized, err := NormalizeGroupGroupRatio(parsed)
	if err != nil {
		return nil, "", err
	}
	normalizedBytes, err := common.Marshal(normalized)
	if err != nil {
		return nil, "", err
	}
	return normalized, string(normalizedBytes), nil
}

func isFiniteRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0)
}
