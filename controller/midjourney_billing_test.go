package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failingMidjourneyFlushWriter struct {
	gin.ResponseWriter
}

func (w *failingMidjourneyFlushWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced client flush failure")
}

func TestRelayMidjourneySettlementErrorReturnsOneJSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalSubmit := relayMidjourneySubmitHandler
	originalGenerator := generateMidjourneyRelayInfo
	generateMidjourneyRelayInfo = func(*gin.Context) (*relaycommon.RelayInfo, error) {
		return &relaycommon.RelayInfo{}, nil
	}
	relayMidjourneySubmitHandler = func(c *gin.Context, _ *relaycommon.RelayInfo) *dto.MidjourneyResponse {
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = c.Writer.Write([]byte(`{"code":1,"description":"upstream success"}`))
		return &dto.MidjourneyResponse{Code: 4, Description: "settle_midjourney_billing_failed"}
	}
	t.Cleanup(func() {
		relayMidjourneySubmitHandler = originalSubmit
		generateMidjourneyRelayInfo = originalGenerator
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/submit/imagine", strings.NewReader(`{}`))

	RelayMidjourney(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response map[string]any
	require.NoError(t, common.DecodeJsonStrict(strings.NewReader(recorder.Body.String()), &response))
	require.Equal(t, float64(4), response["code"])
	require.Contains(t, response["description"], "settle_midjourney_billing_failed")
	require.NotContains(t, recorder.Body.String(), "upstream success")
}

func TestRelayMidjourneyClientFlushFailureAuditsAcceptedTaskWithoutRollback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalSubmit := relayMidjourneySubmitHandler
	originalGenerator := generateMidjourneyRelayInfo
	originalAudit := midjourneyFlushErrorLogger
	completed := false
	var audit string
	generateMidjourneyRelayInfo = func(*gin.Context) (*relaycommon.RelayInfo, error) {
		return &relaycommon.RelayInfo{RequestId: "request-flush-1"}, nil
	}
	relayMidjourneySubmitHandler = func(c *gin.Context, _ *relaycommon.RelayInfo) *dto.MidjourneyResponse {
		c.Set("midjourney_task_id", "mj-flush-1")
		_, _ = c.Writer.Write([]byte(`{"code":1,"description":"accepted","result":"mj-flush-1"}`))
		completed = true
		return nil
	}
	midjourneyFlushErrorLogger = func(_ context.Context, message string) {
		audit = message
	}
	t.Cleanup(func() {
		relayMidjourneySubmitHandler = originalSubmit
		generateMidjourneyRelayInfo = originalGenerator
		midjourneyFlushErrorLogger = originalAudit
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Writer = &failingMidjourneyFlushWriter{ResponseWriter: c.Writer}
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/submit/imagine", strings.NewReader(`{}`))
	c.Set("channel_id", 42)

	RelayMidjourney(c)

	require.True(t, completed)
	require.Contains(t, audit, "request_id=request-flush-1")
	require.Contains(t, audit, "task_id=mj-flush-1")
	require.Contains(t, audit, "channel_id=42")
	require.Contains(t, audit, "forced client flush failure")
}

func setupMidjourneyControllerBillingTest(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Midjourney{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedMidjourneyControllerWalletTask(t *testing.T, db *gorm.DB, withContext bool) *model.Midjourney {
	t.Helper()
	require.NoError(t, db.Create(&model.User{Id: 81, Username: "mj-cas", Quota: 100, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 82, UserId: 81, Key: "mj-cas-token", Name: "mj-cas-token",
		Status: common.TokenStatusEnabled, RemainQuota: 200,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 83, Name: "mj-cas-channel", Status: common.ChannelStatusEnabled}).Error)
	task := &model.Midjourney{
		UserId: 81, ChannelId: 83, MjId: "mj-cas-task", Action: "IMAGINE",
		Status: "IN_PROGRESS", Progress: "50%", Quota: 500,
	}
	if withContext {
		task.BillingContext = model.MidjourneyBillingContext{
			BillingSource: service.BillingSourceWallet, TokenId: 82,
			BillingUsingGroup: "group-a", EffectiveGroupRatio: 0.5,
		}
	}
	require.NoError(t, task.Insert())
	return task
}

func TestCommitMidjourneyTaskUpdateRefundsOnlyCASWinner(t *testing.T) {
	db := setupMidjourneyControllerBillingTest(t)
	task := seedMidjourneyControllerWalletTask(t, db, true)
	stale := *task
	task.Status = "FAILURE"
	task.Progress = "100%"
	stale.Status = "FAILURE"
	stale.Progress = "100%"

	won, err := service.CommitMidjourneyTaskUpdate(context.Background(), task, "IN_PROGRESS", true, "terminal failure")
	require.NoError(t, err)
	require.True(t, won)
	won, err = service.CommitMidjourneyTaskUpdate(context.Background(), &stale, "IN_PROGRESS", true, "duplicate poll")
	require.NoError(t, err)
	require.False(t, won)
	won, err = service.CommitMidjourneyTaskUpdate(context.Background(), task, "IN_PROGRESS", true, "repeat")
	require.NoError(t, err)
	require.False(t, won)

	var user model.User
	require.NoError(t, db.First(&user, 81).Error)
	require.Equal(t, 600, user.Quota)
	var token model.Token
	require.NoError(t, db.First(&token, 82).Error)
	require.Equal(t, 700, token.RemainQuota)
	var refundLogs int64
	require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeRefund).Count(&refundLogs).Error)
	require.Equal(t, int64(1), refundLogs)
}

func TestCommitMidjourneyTaskUpdateLegacyRecordRefundsWallet(t *testing.T) {
	db := setupMidjourneyControllerBillingTest(t)
	task := seedMidjourneyControllerWalletTask(t, db, false)
	task.Status = "FAILURE"
	task.Progress = "100%"

	won, err := service.CommitMidjourneyTaskUpdate(context.Background(), task, "IN_PROGRESS", true, "legacy failure")

	require.NoError(t, err)
	require.True(t, won)
	var user model.User
	require.NoError(t, db.First(&user, 81).Error)
	require.Equal(t, 600, user.Quota)
	var token model.Token
	require.NoError(t, db.First(&token, 82).Error)
	require.Equal(t, 200, token.RemainQuota)
}

func TestCommitMidjourneyTaskUpdateRetriesIncompleteRefundLegs(t *testing.T) {
	db := setupMidjourneyControllerBillingTest(t)
	task := seedMidjourneyControllerWalletTask(t, db, true)
	require.NoError(t, db.Unscoped().Delete(&model.Token{}, 82).Error)
	task.Status = "FAILURE"
	task.Progress = "100%"

	won, err := service.CommitMidjourneyTaskUpdate(context.Background(), task, "IN_PROGRESS", true, "terminal failure")
	require.Error(t, err)
	require.True(t, won)
	var pending model.Midjourney
	require.NoError(t, db.First(&pending, task.Id).Error)
	require.Equal(t, "REFUND_PENDING", pending.Progress)
	require.True(t, pending.BillingContext.FundingRefunded)
	require.False(t, pending.BillingContext.TokenRefunded)
	var user model.User
	require.NoError(t, db.First(&user, 81).Error)
	require.Equal(t, 600, user.Quota)

	require.NoError(t, db.Create(&model.Token{
		Id: 82, UserId: 81, Key: "mj-cas-token", Name: "mj-cas-token",
		Status: common.TokenStatusEnabled, RemainQuota: 200,
	}).Error)
	pending.Status = "FAILURE"
	pending.Progress = "100%"
	won, err = service.CommitMidjourneyTaskUpdate(context.Background(), &pending, "FAILURE", true, "retry")
	require.NoError(t, err)
	require.True(t, won)
	require.NoError(t, db.First(&user, 81).Error)
	require.Equal(t, 600, user.Quota)
	var token model.Token
	require.NoError(t, db.First(&token, 82).Error)
	require.Equal(t, 700, token.RemainQuota)
	var completed model.Midjourney
	require.NoError(t, db.First(&completed, task.Id).Error)
	require.Equal(t, "100%", completed.Progress)
	require.True(t, completed.BillingContext.BillingRefunded)
}
