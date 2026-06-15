package controller

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/channelconsole"
	"gorm.io/gorm"
)

const (
	channelConsoleHealthCheckDefaultInterval = 6 * time.Hour
	channelConsoleHealthCheckMaxBatch        = 100
)

type channelConsoleHealthCheckOutcome struct {
	status         string
	errorCode      string
	errorMessage   string
	modelName      string
	responseTimeMs int
}

var (
	channelConsoleHealthCheckRunner = defaultChannelConsoleHealthCheckRunner
	channelConsoleHealthCheckOnce   sync.Once
)

func RunChannelConsoleHealthCheck(channelID int, checkType string) (*model.ChannelConsoleHealthCheck, error) {
	if _, err := model.GetChannelConsoleChannelByChannelID(channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, channelconsole.ErrChannelConsoleMetadataNotFound
		}
		return nil, err
	}

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, channelconsole.ErrChannelConsoleChannelNotFound
		}
		return nil, err
	}

	outcome := channelConsoleHealthCheckRunner(channel, checkType)
	return channelconsole.RecordHealthCheckResult(channelconsole.HealthCheckRecord{
		ChannelID:      channelID,
		ModelName:      outcome.modelName,
		CheckType:      checkType,
		Status:         outcome.status,
		ErrorCode:      outcome.errorCode,
		ErrorMessage:   outcome.errorMessage,
		ResponseTimeMs: outcome.responseTimeMs,
	})
}

func RunChannelConsoleHealthChecksOnce(limit int) (int, int, error) {
	if limit <= 0 || limit > channelConsoleHealthCheckMaxBatch {
		limit = channelConsoleHealthCheckMaxBatch
	}

	var metas []model.ChannelConsoleChannel
	if err := model.DB.
		Order("last_health_check_at ASC, id ASC").
		Limit(limit).
		Find(&metas).Error; err != nil {
		return 0, 0, err
	}

	checked := 0
	failed := 0
	for _, meta := range metas {
		check, err := RunChannelConsoleHealthCheck(meta.ChannelId, channelconsole.HealthCheckTypeAutomatic)
		if err != nil {
			failed++
			common.SysError(fmt.Sprintf("channel console health check failed before record: channel_id=%d err=%v", meta.ChannelId, err))
			continue
		}
		checked++
		if check.Status != channelconsole.HealthHealthy {
			failed++
		}
		time.Sleep(common.RequestInterval)
	}

	return checked, failed, nil
}

func StartChannelConsoleHealthCheckTask() {
	if !common.IsMasterNode {
		return
	}
	if !channelConsoleHealthCheckTaskEnabled() {
		common.SysLog("channel console health check task disabled")
		return
	}

	channelConsoleHealthCheckOnce.Do(func() {
		go func() {
			interval := channelConsoleHealthCheckInterval()
			common.SysLog(fmt.Sprintf("channel console health check task started: interval=%s", interval))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				checked, failed, err := RunChannelConsoleHealthChecksOnce(channelConsoleHealthCheckMaxBatch)
				if err != nil {
					common.SysError(fmt.Sprintf("channel console health check task query failed: %v", err))
					continue
				}
				common.SysLog(fmt.Sprintf("channel console health check task done: checked_channels=%d failed_channels=%d", checked, failed))
			}
		}()
	})
}

func defaultChannelConsoleHealthCheckRunner(channel *model.Channel, _ string) channelConsoleHealthCheckOutcome {
	modelName := channelConsoleHealthCheckModelName(channel)
	if channel.Status != common.ChannelStatusEnabled {
		return channelConsoleHealthCheckOutcome{
			status:       channelconsole.HealthDisabled,
			errorCode:    "channel_disabled",
			errorMessage: "渠道当前不是启用状态，已跳过上游验活",
			modelName:    modelName,
		}
	}

	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return channelConsoleHealthCheckOutcome{
			status:       channelconsole.HealthFailed,
			errorCode:    "channel_test_user_unavailable",
			errorMessage: err.Error(),
			modelName:    modelName,
		}
	}

	startedAt := time.Now()
	result := testChannel(channel, testUserID, modelName, "", shouldUseStreamForAutomaticChannelTest(channel))
	responseTimeMs := int(time.Since(startedAt).Milliseconds())
	go channel.UpdateResponseTime(int64(responseTimeMs))

	if result.localErr == nil && result.newAPIError == nil {
		return channelConsoleHealthCheckOutcome{
			status:         channelconsole.HealthHealthy,
			modelName:      modelName,
			responseTimeMs: responseTimeMs,
		}
	}

	errorCode := "channel_console_health_check_failed"
	if result.newAPIError != nil && strings.TrimSpace(string(result.newAPIError.GetErrorCode())) != "" {
		errorCode = string(result.newAPIError.GetErrorCode())
	}
	errorMessage := ""
	if result.localErr != nil {
		errorMessage = result.localErr.Error()
	} else if result.newAPIError != nil {
		errorMessage = result.newAPIError.Error()
	}

	return channelConsoleHealthCheckOutcome{
		status:         channelconsole.HealthFailed,
		errorCode:      errorCode,
		errorMessage:   errorMessage,
		modelName:      modelName,
		responseTimeMs: responseTimeMs,
	}
}

func channelConsoleHealthCheckModelName(channel *model.Channel) string {
	if channel == nil {
		return ""
	}
	if channel.TestModel != nil && strings.TrimSpace(*channel.TestModel) != "" {
		return strings.TrimSpace(*channel.TestModel)
	}
	models := channel.GetModels()
	if len(models) > 0 {
		return strings.TrimSpace(models[0])
	}
	return ""
}

func channelConsoleHealthCheckTaskEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("CHANNEL_CONSOLE_HEALTH_CHECK_ENABLED")))
	return value != "0" && value != "false" && value != "off" && value != "no"
}

func channelConsoleHealthCheckInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("CHANNEL_CONSOLE_HEALTH_CHECK_FREQUENCY"))
	if raw == "" {
		return channelConsoleHealthCheckDefaultInterval
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes <= 0 {
		return channelConsoleHealthCheckDefaultInterval
	}
	return time.Duration(minutes) * time.Minute
}
