package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db

	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&model.Task{},
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Channel{},
		&model.TopUp{},
		&model.OrderDeletionMark{},
		&model.LdxpTopupSession{},
		&model.LdxpMailEvent{},
		&model.AffiliateCommission{},
		&model.AffiliateWithdrawal{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.UserValuePackagePreference{},
		&model.ValuePackageUsageRecord{},
		&model.ValuePackageQuotaReset{},
		&model.ValuePackageResetCountLedger{},
		&model.SubscriptionPreConsumeRecord{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func truncate(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM tasks")
		model.DB.Exec("DELETE FROM users")
		model.DB.Exec("DELETE FROM tokens")
		model.DB.Exec("DELETE FROM logs")
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM top_ups")
		model.DB.Exec("DELETE FROM order_deletion_marks")
		model.DB.Exec("DELETE FROM ldxp_topup_sessions")
		model.DB.Exec("DELETE FROM ldxp_mail_events")
		model.DB.Exec("DELETE FROM affiliate_commissions")
		model.DB.Exec("DELETE FROM affiliate_withdrawals")
		model.DB.Exec("DELETE FROM subscription_orders")
		model.DB.Exec("DELETE FROM subscription_plans")
		model.DB.Exec("DELETE FROM subscription_pre_consume_records")
		model.DB.Exec("DELETE FROM user_subscriptions")
		model.DB.Exec("DELETE FROM user_value_package_preferences")
		model.DB.Exec("DELETE FROM value_package_usage_records")
	})
}

func seedUser(t *testing.T, id int, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: "test_user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
}

func seedToken(t *testing.T, id int, userId int, key string, remainQuota int) {
	t.Helper()
	token := &model.Token{
		Id:          id,
		UserId:      userId,
		Key:         key,
		Name:        "test_token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: remainQuota,
		UsedQuota:   0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedSubscription(t *testing.T, id int, userId int, amountTotal int64, amountUsed int64) {
	t.Helper()
	sub := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		Status:      "active",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
}

func seedChannel(t *testing.T, id int) {
	t.Helper()
	ch := &model.Channel{Id: id, Name: "test_channel", Key: "sk-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(ch).Error)
}

func makeTask(userId, channelId, quota, tokenId int, billingSource string, subscriptionId int) *model.Task {
	return &model.Task{
		TaskID:    "task_" + time.Now().Format("150405.000"),
		UserId:    userId,
		ChannelId: channelId,
		Quota:     quota,
		Status:    model.TaskStatus(model.TaskStatusInProgress),
		Group:     "default",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: "test-model",
		},
		PrivateData: model.TaskPrivateData{
			BillingSource:  billingSource,
			SubscriptionId: subscriptionId,
			TokenId:        tokenId,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      0.02,
				GroupRatio:      1.0,
				OriginModelName: "test-model",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Read-back helpers
// ---------------------------------------------------------------------------

func getUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getTokenRemainQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota").Where("id = ?", id).First(&token).Error)
	return token.RemainQuota
}

func getTokenUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&token).Error)
	return token.UsedQuota
}

func getSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func getLastLog(t *testing.T) *model.Log {
	t.Helper()
	var log model.Log
	err := model.LOG_DB.Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	model.LOG_DB.Model(&model.Log{}).Count(&count)
	return count
}

func withTaskBillingRatios(t *testing.T, modelRatios string, groupRatios string, groupGroupRatios string) {
	t.Helper()
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(modelRatios))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(groupRatios))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(groupGroupRatios))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatios))
	})
}

// ===========================================================================
// RefundTaskQuota tests
// ===========================================================================

func TestRefundTaskQuota_Wallet(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 1, 1, 1
	const initQuota, preConsumed = 10000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-key", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "task failed: upstream error")

	// User quota should increase by preConsumed
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Token remain_quota should increase, used_quota should decrease
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, -preConsumed, getTokenUsedQuota(t, tokenID))

	// A refund log should be created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Equal(t, "test-model", log.ModelName)
}

func TestRefundTaskQuota_Subscription(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 2, 2, 2, 1
	const preConsumed = 2000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-key", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RefundTaskQuota(ctx, task, "subscription task failed")

	// Subscription used should decrease by preConsumed
	assert.Equal(t, subUsed-int64(preConsumed), getSubscriptionUsed(t, subID))

	// Token should also be refunded
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRefundTaskQuota_ZeroQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 3
	seedUser(t, userID, 5000)

	task := makeTask(userID, 0, 0, 0, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "zero quota task")

	// No change to user quota
	assert.Equal(t, 5000, getUserQuota(t, userID))

	// No log created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRefundTaskQuota_NoToken(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 4, 4
	const initQuota, preConsumed = 10000, 1500

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0) // TokenId=0

	RefundTaskQuota(ctx, task, "no token task failed")

	// User quota refunded
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Log created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

// ===========================================================================
// RecalculateTaskQuota tests
// ===========================================================================

func TestRecalculate_PositiveDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 10, 10, 10
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000 // under-charged by 1000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-pos", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should decrease by the delta (1000 additional charge)
	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))

	// Token should also be charged the delta
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)

	// Log type should be Consume (additional charge)
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Equal(t, actualQuota-preConsumed, log.Quota)
}

func TestRecalculate_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 11, 11, 11
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged by 2000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-neg", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should increase by abs(delta) = 2000 (refund overpayment)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))

	// Token should be refunded the difference
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota updated
	assert.Equal(t, actualQuota, task.Quota)

	// Log type should be Refund
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-actualQuota, log.Quota)
}

func TestRecalculate_ZeroDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 12
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, preConsumed, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, preConsumed, "exact match")

	// No change to user quota
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No log created (delta is zero)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_ActualQuotaZero(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 13
	const initQuota = 10000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, 5000, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, 0, "zero actual")

	// No change (early return)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_Subscription_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 14, 14, 14, 2
	const preConsumed = 5000
	const actualQuota = 2000 // over-charged by 3000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-recalc", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RecalculateTaskQuota(ctx, task, actualQuota, "subscription over-charge")

	// Subscription used should decrease by delta (refund 3000)
	assert.Equal(t, subUsed-int64(preConsumed-actualQuota), getSubscriptionUsed(t, subID))

	// Token refunded
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	assert.Equal(t, actualQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRecalculateTaskQuotaByTokensDefaultsEmptyTaskGroupToGptPlus(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	withTaskBillingRatios(
		t,
		`{"test-model":2}`,
		`{"gpt-plus":3,"体验用户":11}`,
		`{"体验用户":{"体验用户":13}}`,
	)

	const userID, tokenID, channelID = 15, 15, 15
	const initQuota, preConsumed = 10000, 1000
	const tokenRemain = 5000

	user := &model.User{
		Id:       userID,
		Username: "task-empty-group-user",
		Group:    model.UserGroupTiyan,
		Quota:    initQuota,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(user).Error)
	seedToken(t, tokenID, userID, "sk-task-empty-group", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Group = ""
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		OriginModelName: "test-model",
	}

	RecalculateTaskQuotaByTokens(ctx, task, 100)

	require.Equal(t, constant.TokenGroupGPTPlus, resolveTaskBillingGroup(""))
	require.Equal(t, 600, task.Quota)
	assert.Equal(t, initQuota+(preConsumed-600), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-600), getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, constant.TokenGroupGPTPlus, log.Group)
}

// ===========================================================================
// CAS + Billing integration tests
// Simulates the flow in updateVideoSingleTask (service/task_polling.go)
// ===========================================================================

// simulatePollBilling reproduces the CAS + billing logic from updateVideoSingleTask.
// It takes a persisted task (already in DB), applies the new status, and performs
// the conditional update + billing exactly as the polling loop does.
func simulatePollBilling(ctx context.Context, task *model.Task, newStatus model.TaskStatus, actualQuota int) {
	snap := task.Snapshot()

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = newStatus
	switch string(newStatus) {
	case model.TaskStatusSuccess:
		task.Progress = "100%"
		task.FinishTime = 9999
		shouldSettle = true
	case model.TaskStatusFailure:
		task.Progress = "100%"
		task.FinishTime = 9999
		task.FailReason = "upstream error"
		if quota != 0 {
			shouldRefund = true
		}
	default:
		task.Progress = "50%"
	}

	isDone := task.Status == model.TaskStatus(model.TaskStatusSuccess) || task.Status == model.TaskStatus(model.TaskStatusFailure)
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		_, _ = task.UpdateWithStatus(snap.Status)
	}

	if shouldSettle && actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "test settle")
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
}

func TestCASGuardedRefund_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 20, 20, 20
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS wins: task in DB should now be FAILURE
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)

	// Refund should have happened
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestCASGuardedRefund_Lose(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 21, 21, 21
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-lose", tokenRemain)
	seedChannel(t, channelID)

	// Create task with IN_PROGRESS in DB
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate another process already transitioning to FAILURE
	model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("status", model.TaskStatusFailure)

	// Our process still has the old in-memory state (IN_PROGRESS) and tries to transition
	// task.Status is still IN_PROGRESS in the snapshot
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS lost: user quota should NOT change (no double refund)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	// No billing log should be created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCASGuardedSettle_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 22, 22, 22
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged, should get partial refund
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-settle-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusSuccess), actualQuota)

	// CAS wins: task should be SUCCESS
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)

	// Settlement should refund the over-charge (5000 - 3000 = 2000 back to user)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)
}

func TestNonTerminalUpdate_NoBilling(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 23, 23
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	task.Progress = "20%"
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate a non-terminal poll update (still IN_PROGRESS, progress changed)
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusInProgress), 0)

	// User quota should NOT change
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No billing log
	assert.Equal(t, int64(0), countLogs(t))

	// Task progress should be updated in DB
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "50%", reloaded.Progress)
}

// ===========================================================================
// Mock adaptor for settleTaskBillingOnComplete tests
// ===========================================================================

type mockAdaptor struct {
	adjustReturn int
}

func (m *mockAdaptor) Init(_ *relaycommon.RelayInfo) {}
func (m *mockAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return nil, nil
}
func (m *mockAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) { return nil, nil }
func (m *mockAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return m.adjustReturn
}

// ===========================================================================
// PerCallBilling tests — settleTaskBillingOnComplete
// ===========================================================================

func TestSettle_PerCallBilling_SkipsAdaptorAdjust(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, preConsumed = 10000, 5000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-adaptor", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 2000}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no adjustment despite adaptor returning 2000
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_PerCallBilling_SkipsTotalTokens(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 7000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-tokens", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 0}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, TotalTokens: 9999}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no recalculation by tokens
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_NonPerCall_AdaptorAdjustWorks(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 32, 32, 32
	const initQuota, preConsumed = 10000, 5000
	const adaptorQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-nonpercall-adj", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	// PerCallBilling defaults to false

	adaptor := &mockAdaptor{adjustReturn: adaptorQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Non-per-call: adaptor adjustment applies (refund 2000)
	assert.Equal(t, initQuota+(preConsumed-adaptorQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-adaptorQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, adaptorQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRecalculateTaskQuotaByTokensUsesBillingContextGroupRatioSnapshot(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 40, 40, 40, 40
	const subTotal, subUsed int64 = 100000, 300
	const preConsumed = 300
	const tokenRemain = 10000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-snapshot", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)
	task.Group = "default"
	require.NoError(t, task.PrivateData.Scan([]byte(`{
		"billing_source":"subscription",
		"subscription_id":40,
		"token_id":40,
		"billing_context":{
			"model_ratio":1,
			"has_group_ratio":true,
			"group_ratio":0.45,
			"origin_model_name":"test-model",
			"subscription_ratio_applied":true,
			"subscription_ratio_source":"configured",
			"value_package_billing_group":"month-card",
			"billing_using_group":"group-a",
			"has_original_group_ratio":true,
			"original_group_ratio":0.3
		}
	}`)))
	withTaskBillingRatios(
		t,
		`{"test-model":1}`,
		`{"default":4}`,
		`{"default":{"default":4},"month-card":{"default":3}}`,
	)

	RecalculateTaskQuotaByTokens(ctx, task, 1000)

	assert.Equal(t, 450, task.Quota)
	assert.Equal(t, subUsed+150, getSubscriptionUsed(t, subID))
	assert.Equal(t, tokenRemain-150, getTokenRemainQuota(t, tokenID))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Contains(t, log.Content, "groupRatio=0.45")
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "month-card", other["value_package_billing_group"])
	assert.Equal(t, "group-a", other["value_package_billing_using_group"])
	assert.Equal(t, 0.45, other["value_package_effective_ratio"])
	assert.Equal(t, SubscriptionRatioSourceConfigured, other["value_package_ratio_source"])
}

func TestTaskBillingOtherValuePackageAndRegularSubscriptionRatioAudit(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		billingContextJSON   string
		wantValuePackage     bool
		wantApplied          bool
		wantEffectiveRatio   float64
		wantSubscriptionFrom string
	}{
		{
			name: "configured value package",
			billingContextJSON: `{
				"model_price":0.02,"group_ratio":0.45,"origin_model_name":"test-model",
				"subscription_ratio_applied":true,"subscription_ratio_source":"configured",
				"value_package_billing_group":"month-card",
				"has_original_group_ratio":true,"original_group_ratio":0.3,
				"has_original_user_group_ratio":true,"original_user_group_ratio":-1
			}`,
			wantValuePackage:     true,
			wantApplied:          true,
			wantEffectiveRatio:   0.45,
			wantSubscriptionFrom: SubscriptionRatioSourceConfigured,
		},
		{
			name: "default value package",
			billingContextJSON: `{
				"model_price":0.02,"group_ratio":1,"origin_model_name":"test-model",
				"subscription_ratio_applied":true,"subscription_ratio_source":"default_1x",
				"value_package_billing_group":"month-card",
				"has_original_group_ratio":true,"original_group_ratio":0.3
			}`,
			wantValuePackage:     true,
			wantApplied:          true,
			wantEffectiveRatio:   1,
			wantSubscriptionFrom: SubscriptionRatioSourceDefault,
		},
		{
			name: "regular subscription",
			billingContextJSON: `{
				"model_price":0.02,"group_ratio":1,"origin_model_name":"test-model",
				"subscription_ratio_applied":true,"subscription_ratio_source":"regular_subscription_1x",
				"value_package_billing_group":"month-card",
				"has_original_group_ratio":true,"original_group_ratio":0.3,
				"has_original_user_group_ratio":true,"original_user_group_ratio":-1
			}`,
			wantValuePackage: false,
			wantApplied:      true,
		},
		{
			name: "configured package source not applied",
			billingContextJSON: `{
				"model_price":0.02,"group_ratio":0.45,"origin_model_name":"test-model",
				"subscription_ratio_source":"configured",
				"value_package_billing_group":"month-card",
				"has_original_group_ratio":true,"original_group_ratio":0.3
			}`,
			wantValuePackage: false,
		},
		{
			name: "default package source not applied",
			billingContextJSON: `{
				"model_price":0.02,"group_ratio":1,"origin_model_name":"test-model",
				"subscription_ratio_source":"default_1x",
				"value_package_billing_group":"month-card",
				"has_original_group_ratio":true,"original_group_ratio":0.3
			}`,
			wantValuePackage: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			task := makeTask(41, 0, 1000, 0, BillingSourceSubscription, 41)
			require.NoError(t, task.PrivateData.Scan([]byte(`{"billing_context":`+tt.billingContextJSON+`}`)))

			other := taskBillingOther(task)

			if tt.wantApplied {
				assert.Equal(t, true, other["subscription_ratio_applied"])
			} else {
				assert.NotContains(t, other, "subscription_ratio_applied")
			}
			assert.Equal(t, 0.3, other["original_group_ratio"])
			if tt.wantValuePackage {
				assert.Equal(t, "month-card", other["value_package_billing_group"])
				assert.Equal(t, tt.wantEffectiveRatio, other["value_package_effective_ratio"])
				assert.Equal(t, tt.wantSubscriptionFrom, other["value_package_ratio_source"])
			} else {
				assert.NotContains(t, other, "value_package_billing_group")
				assert.NotContains(t, other, "value_package_effective_ratio")
				assert.NotContains(t, other, "value_package_ratio_source")
			}
		})
	}
}

func TestTaskBillingContextValueScanRoundTripPreservesValuePackageRatioAudit(t *testing.T) {
	var privateData model.TaskPrivateData
	require.NoError(t, privateData.Scan([]byte(`{
		"billing_context":{
			"has_group_ratio":true,
			"group_ratio":0.45,
			"subscription_ratio_source":"configured",
			"value_package_billing_group":"month-card",
			"billing_using_group":"group-a"
		}
	}`)))

	value, err := privateData.Value()
	require.NoError(t, err)
	privateJSON, ok := value.([]byte)
	require.True(t, ok)
	var decoded map[string]interface{}
	require.NoError(t, common.Unmarshal(privateJSON, &decoded))
	billingContext, ok := decoded["billing_context"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, billingContext["has_group_ratio"])
	assert.Equal(t, 0.45, billingContext["group_ratio"])
	assert.Equal(t, "configured", billingContext["subscription_ratio_source"])
	assert.Equal(t, "month-card", billingContext["value_package_billing_group"])
	assert.Equal(t, "group-a", billingContext["billing_using_group"])
}

func TestLogTaskConsumptionValuePackageRatioAudit(t *testing.T) {
	truncate(t)
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldDataExportEnabled := common.DataExportEnabled
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	common.DataExportEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		common.DataExportEnabled = oldDataExportEnabled
	})

	const userID, channelID = 43, 43
	seedUser(t, userID, 10000)
	seedChannel(t, channelID)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/videos", nil)
	ctx.Set("token_name", "task-ratio-audit")

	packageInfo := &relaycommon.RelayInfo{
		UserId:                     userID,
		UsingGroup:                 "group-b",
		BillingUsingGroup:          "group-a",
		OriginModelName:            "test-model",
		ChannelMeta:                &relaycommon.ChannelMeta{ChannelId: channelID},
		TaskRelayInfo:              &relaycommon.TaskRelayInfo{Action: "submit"},
		ValuePackageSubscriptionId: 123,
		ValuePackageBillingGroup:   "month-card",
		PriceData: types.PriceData{
			Quota:                    450,
			GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: 0.45},
			SubscriptionRatioApplied: true,
			SubscriptionRatioSource:  SubscriptionRatioSourceConfigured,
		},
	}
	LogTaskConsumption(ctx, packageInfo)

	log := getLastLog(t)
	require.NotNil(t, log)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, "month-card", other["value_package_billing_group"])
	assert.Equal(t, "group-a", other["value_package_billing_using_group"])
	assert.Equal(t, 0.45, other["value_package_effective_ratio"])
	assert.Equal(t, SubscriptionRatioSourceConfigured, other["value_package_ratio_source"])
	assert.Equal(t, "group-a", log.Group)

	require.NoError(t, model.DB.Exec("DELETE FROM logs").Error)
	regularInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		UsingGroup:      "default",
		OriginModelName: "test-model",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: "submit"},
		PriceData: types.PriceData{
			Quota:                    1000,
			GroupRatioInfo:           types.GroupRatioInfo{GroupRatio: 1},
			SubscriptionRatioApplied: true,
			SubscriptionRatioSource:  SubscriptionRatioSourceRegular,
		},
	}
	LogTaskConsumption(ctx, regularInfo)

	log = getLastLog(t)
	require.NotNil(t, log)
	other, err = common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, true, other["subscription_ratio_applied"])
	assert.NotContains(t, other, "value_package_billing_group")
	assert.NotContains(t, other, "value_package_effective_ratio")
	assert.NotContains(t, other, "value_package_ratio_source")
}

func TestRecalculateTaskQuotaByTokensPreservesExplicitZeroGroupRatioSnapshot(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	oldModelRatio := ratio_setting.ModelRatio2JSONString()
	oldGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"test-model":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModelRatio))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatio))
	})

	const userID, tokenID, channelID = 42, 42, 42
	const initQuota, tokenRemain = 10000, 10000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-zero-ratio-snapshot", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, 0, tokenID, BillingSourceWallet, 0)
	task.Group = "default"
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelRatio:      1,
		HasGroupRatio:   true,
		GroupRatio:      0,
		OriginModelName: "test-model",
	}

	RecalculateTaskQuotaByTokens(ctx, task, 1000)

	assert.Equal(t, 0, task.Quota)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(0), countLogs(t))
}
