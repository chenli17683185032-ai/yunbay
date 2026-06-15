package channelconsole

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	HealthHealthy   = model.ChannelConsoleStatusHealthy
	HealthWarning   = model.ChannelConsoleStatusWarning
	HealthFailed    = model.ChannelConsoleStatusFailed
	HealthDisabled  = model.ChannelConsoleStatusDisabled
	HealthUnchecked = model.ChannelConsoleStatusUnchecked
)

const (
	manualHealthCheckType    = "manual"
	manualHealthQueuedCode   = "manual_check_queued"
	manualHealthQueuedNotice = "已记录手动验活请求；实时上游调用由后续调度器执行"
)

func AggregateHealthStatus(statuses []string) string {
	if len(statuses) == 0 {
		return HealthUnchecked
	}

	hasHealthy := false
	hasFailed := false
	hasWarningLike := false
	for _, status := range statuses {
		switch strings.TrimSpace(status) {
		case HealthDisabled:
			return HealthDisabled
		case HealthHealthy:
			hasHealthy = true
		case HealthFailed:
			hasFailed = true
		case HealthWarning, HealthUnchecked:
			hasWarningLike = true
		default:
			hasWarningLike = true
		}
	}

	if hasWarningLike {
		return HealthWarning
	}
	if hasHealthy && hasFailed {
		return HealthWarning
	}
	if hasFailed {
		return HealthFailed
	}
	return HealthHealthy
}

func RecordManualHealthCheck(channelID int) (*model.ChannelConsoleHealthCheck, error) {
	check := &model.ChannelConsoleHealthCheck{
		ChannelId:      channelID,
		CheckType:      manualHealthCheckType,
		Status:         HealthUnchecked,
		ErrorCode:      manualHealthQueuedCode,
		ErrorMessage:   manualHealthQueuedNotice,
		ResponseTimeMs: 0,
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var meta model.ChannelConsoleChannel
		if err := tx.Where("channel_id = ?", channelID).First(&meta).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChannelConsoleMetadataNotFound
			}
			return err
		}

		if err := tx.Create(check).Error; err != nil {
			return err
		}

		updates := map[string]any{
			"last_health_check_at": check.CheckedAt,
			"last_error_code":      manualHealthQueuedCode,
			"last_error_message":   manualHealthQueuedNotice,
			"updated_at":           check.CheckedAt,
		}
		if strings.TrimSpace(meta.HealthStatus) == "" {
			updates["health_status"] = HealthUnchecked
		}
		return tx.Model(&model.ChannelConsoleChannel{}).Where("id = ?", meta.Id).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return check, nil
}
