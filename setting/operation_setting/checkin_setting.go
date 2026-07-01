package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const legacyCheckinDisplayAmountCompatibilityMax = 100

// CheckinSetting 签到功能配置
type CheckinSetting struct {
	Enabled  bool `json:"enabled"`   // 是否启用签到功能
	MinQuota int  `json:"min_quota"` // 签到最小奖励配置值
	MaxQuota int  `json:"max_quota"` // 签到最大奖励配置值
}

// 默认配置
var checkinSetting = CheckinSetting{
	Enabled:  false, // 默认关闭
	MinQuota: 0,     // 默认最小金额 0
	MaxQuota: 1,     // 默认最大金额 1
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("checkin_setting", &checkinSetting)
}

// GetCheckinSetting 获取签到配置
func GetCheckinSetting() *CheckinSetting {
	return &checkinSetting
}

// IsCheckinEnabled 是否启用签到功能
func IsCheckinEnabled() bool {
	return checkinSetting.Enabled
}

// GetCheckinQuotaRange 获取签到额度范围
func GetCheckinQuotaRange() (min, max int) {
	return NormalizeCheckinQuotaRange(checkinSetting.MinQuota, checkinSetting.MaxQuota)
}

// NormalizeCheckinQuotaRange 返回最终用于发放的内部 quota 范围。
//
// 历史后台表单直接暴露 min_quota / max_quota 原始值，管理员容易按“金额”填写
// 1~5，实际却只发放 1~5 个内部 quota（约 $0.000002~$0.000010）。
// 为兼容这类已保存的小额配置，这里将 1~100 视为当前额度展示类型下的金额并
// 换算为内部 quota；新后台会保存换算后的内部 quota，不受该兼容分支影响。
func NormalizeCheckinQuotaRange(min, max int) (int, int) {
	return normalizeCheckinQuotaValue(min), normalizeCheckinQuotaValue(max)
}

func normalizeCheckinQuotaValue(value int) int {
	if value > 0 && value <= legacyCheckinDisplayAmountCompatibilityMax {
		return checkinDisplayAmountToQuota(value)
	}
	return value
}

func checkinDisplayAmountToQuota(value int) int {
	switch GetQuotaDisplayType() {
	case QuotaDisplayTypeTokens:
		return value
	case QuotaDisplayTypeCNY:
		rate := USDExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return int(float64(value) / rate * common.QuotaPerUnit)
	case QuotaDisplayTypeCustom:
		rate := generalSetting.CustomCurrencyExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return int(float64(value) / rate * common.QuotaPerUnit)
	default:
		return int(float64(value) * common.QuotaPerUnit)
	}
}
