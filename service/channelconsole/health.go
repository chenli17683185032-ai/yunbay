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
	HealthCheckTypeManual    = "manual"
	HealthCheckTypeAutomatic = "automatic"

	manualHealthCheckType    = HealthCheckTypeManual
	manualHealthQueuedCode   = "manual_check_queued"
	manualHealthQueuedNotice = "已记录手动验活请求；实时上游调用由后续调度器执行"
)

type HealthCheckRecord struct {
	ChannelID      int
	KeyIndex       *int
	ModelName      string
	CheckType      string
	Status         string
	ResponseTimeMs int
	ErrorCode      string
	ErrorMessage   string
}

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

func RecordHealthCheckResult(record HealthCheckRecord) (*model.ChannelConsoleHealthCheck, error) {
	status := normalizeHealthCheckStatus(record.Status)
	checkType := strings.TrimSpace(record.CheckType)
	if checkType == "" {
		checkType = HealthCheckTypeAutomatic
	}
	responseTimeMs := record.ResponseTimeMs
	if responseTimeMs < 0 {
		responseTimeMs = 0
	}

	errorCode := strings.TrimSpace(record.ErrorCode)
	errorMessage := strings.TrimSpace(record.ErrorMessage)
	if status == HealthHealthy {
		errorCode = ""
		errorMessage = ""
	}

	check := &model.ChannelConsoleHealthCheck{
		ChannelId:      record.ChannelID,
		KeyIndex:       record.KeyIndex,
		ModelName:      strings.TrimSpace(record.ModelName),
		CheckType:      checkType,
		Status:         status,
		ErrorCode:      errorCode,
		ErrorMessage:   errorMessage,
		ResponseTimeMs: responseTimeMs,
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var meta model.ChannelConsoleChannel
		if err := tx.Where("channel_id = ?", record.ChannelID).First(&meta).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChannelConsoleMetadataNotFound
			}
			return err
		}

		if err := tx.Create(check).Error; err != nil {
			return err
		}

		return tx.Model(&model.ChannelConsoleChannel{}).Where("id = ?", meta.Id).Updates(map[string]any{
			"health_status":        status,
			"last_health_check_at": check.CheckedAt,
			"last_error_code":      errorCode,
			"last_error_message":   errorMessage,
			"updated_at":           check.CheckedAt,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return check, nil
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

func normalizeHealthCheckStatus(status string) string {
	switch strings.TrimSpace(status) {
	case HealthHealthy:
		return HealthHealthy
	case HealthWarning:
		return HealthWarning
	case HealthFailed:
		return HealthFailed
	case HealthDisabled:
		return HealthDisabled
	case HealthUnchecked:
		return HealthUnchecked
	default:
		return HealthWarning
	}
}
