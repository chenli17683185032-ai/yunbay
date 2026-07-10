package relay

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type recordingMidjourneyBilling struct {
	preConsumed int
	settled     int
	refunded    int
	needsRefund bool
}

type failingMidjourneyResponseWriter struct {
	gin.ResponseWriter
}

func (w *failingMidjourneyResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced response copy failure")
}

func (b *recordingMidjourneyBilling) Settle(actualQuota int) error {
	b.settled++
	b.preConsumed = actualQuota
	b.needsRefund = false
	return nil
}

func (b *recordingMidjourneyBilling) Refund(*gin.Context) {
	if !b.needsRefund {
		return
	}
	b.refunded++
	b.needsRefund = false
}

func (b *recordingMidjourneyBilling) NeedsRefund() bool { return b.needsRefund }
func (b *recordingMidjourneyBilling) GetPreConsumedQuota() int {
	return b.preConsumed
}
func (b *recordingMidjourneyBilling) Reserve(int) error { return nil }
func (b *recordingMidjourneyBilling) ReserveRealtime(int) (int, error) {
	return b.preConsumed, nil
}

func setupMidjourneyBillingTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldPriceHelper := midjourneyModelPriceHelperPerCall
	oldPreConsume := midjourneyPreConsumeBilling
	oldSettle := midjourneySettleBilling

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.LogConsumeEnabled = false
	common.BatchUpdateEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Midjourney{},
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Log{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{},
		&model.ValuePackageUsageRecord{},
		&model.ValuePackageQuotaReset{},
	))
	model.DB = db
	model.LOG_DB = db
	service.InitHttpClient()

	t.Cleanup(func() {
		midjourneyModelPriceHelperPerCall = oldPriceHelper
		midjourneyPreConsumeBilling = oldPreConsume
		midjourneySettleBilling = oldSettle
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

func newMidjourneyBillingContext(path, body, baseURL string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("base_url", baseURL)
	c.Set("channel_id", 42)
	c.Set("channel_name", "mj-test")
	c.Set("token_name", "mj-token")
	c.Set("token_quota", 100000)
	return c, recorder
}

func newMidjourneyUpstream(t *testing.T, status int, response string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server
}

func installRecordingMidjourneyBillingHooks(t *testing.T, frozenQuota int) (*recordingMidjourneyBilling, *int, *int) {
	t.Helper()
	billing := &recordingMidjourneyBilling{preConsumed: frozenQuota, needsRefund: true}
	prepareCalls := 0
	settleCalls := 0
	midjourneyModelPriceHelperPerCall = func(*gin.Context, *relaycommon.RelayInfo) (types.PriceData, error) {
		return types.PriceData{
			ModelPrice:       0.003,
			UsePrice:         true,
			Quota:            300,
			QuotaBeforeGroup: 1000,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 0.3,
			},
		}, nil
	}
	midjourneyPreConsumeBilling = func(_ *gin.Context, quota int, info *relaycommon.RelayInfo) *types.NewAPIError {
		prepareCalls++
		require.Equal(t, 300, quota)
		require.Equal(t, 300, info.PriceData.Quota)
		info.Billing = billing
		info.BillingSource = service.BillingSourceSubscription
		info.SubscriptionId = 77
		info.BillingUsingGroup = info.UsingGroup
		info.PriceData.Quota = frozenQuota
		info.PriceData.GroupRatioInfo.GroupRatio = float64(frozenQuota) / info.PriceData.QuotaBeforeGroup
		info.PriceData.SubscriptionRatioApplied = true
		info.PriceData.SubscriptionRatioSource = service.SubscriptionRatioSourceConfigured
		return nil
	}
	midjourneySettleBilling = func(_ *gin.Context, info *relaycommon.RelayInfo, actualQuota int) error {
		settleCalls++
		require.Equal(t, frozenQuota, info.PriceData.Quota)
		require.Equal(t, frozenQuota, actualQuota)
		return info.Billing.Settle(actualQuota)
	}
	return billing, &prepareCalls, &settleCalls
}

func TestMidjourneyChargeableEntrypointsUseBillingSession(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		mode int
		call func(*gin.Context, *relaycommon.RelayInfo) *dto.MidjourneyResponse
	}{
		{
			name: "submit",
			path: "/mj/submit/imagine",
			body: `{"prompt":"billing test"}`,
			mode: relayconstant.RelayModeMidjourneyImagine,
			call: RelayMidjourneySubmit,
		},
		{
			name: "swap face",
			path: "/mj/insight-face/swap",
			body: `{"sourceBase64":"source","targetBase64":"target"}`,
			mode: relayconstant.RelayModeSwapFace,
			call: RelaySwapFace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupMidjourneyBillingTest(t)
			upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-123"}`)
			billing, prepareCalls, settleCalls := installRecordingMidjourneyBillingHooks(t, 450)
			c, _ := newMidjourneyBillingContext(tt.path, tt.body, upstream.URL)
			info := &relaycommon.RelayInfo{
				UserId:                     901,
				TokenId:                    902,
				UsingGroup:                 "gpt-plus",
				OriginModelName:            "mj_imagine",
				RelayMode:                  tt.mode,
				RequestId:                  "mj-success-" + tt.name,
				StartTime:                  time.Now(),
				IsPlayground:               true,
				ValuePackageSubscriptionId: 88,
				ValuePackagePlanId:         99,
				ValuePackageBillingGroup:   "month-card",
				ValuePackageModelGroup:     "month-card",
				ValuePackagePackageType:    model.ValuePackageTypeMonth,
			}

			mjErr := tt.call(c, info)

			require.Nil(t, mjErr)
			require.Equal(t, 1, *prepareCalls)
			require.Equal(t, 1, *settleCalls)
			require.Equal(t, 1, billing.settled)
			require.Zero(t, billing.refunded)
			require.Equal(t, 450, info.PriceData.Quota)
			require.Equal(t, 0.45, info.PriceData.GroupRatioInfo.GroupRatio)
			var task model.Midjourney
			require.NoError(t, db.Where("mj_id = ?", "mj-123").First(&task).Error)
			require.Equal(t, 450, task.Quota)
			require.Equal(t, service.BillingSourceSubscription, task.BillingContext.BillingSource)
			require.Equal(t, 77, task.BillingContext.SubscriptionId)
			require.Equal(t, info.RequestId, task.BillingContext.RequestId)
			require.Equal(t, info.TokenId, task.BillingContext.TokenId)
			require.Equal(t, 88, task.BillingContext.ValuePackageSubscriptionId)
			require.Equal(t, 99, task.BillingContext.ValuePackagePlanId)
			require.Equal(t, "month-card", task.BillingContext.ValuePackageBillingGroup)
			require.Equal(t, "gpt-plus", task.BillingContext.BillingUsingGroup)
			require.Equal(t, 0.45, task.BillingContext.EffectiveGroupRatio)
			require.Equal(t, service.SubscriptionRatioSourceConfigured, task.BillingContext.SubscriptionRatioSource)
		})
	}
}

func TestRelayMidjourneySubmitAcceptedBusinessCodesSettle(t *testing.T) {
	for _, code := range []int{1, 21, 22} {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			setupMidjourneyBillingTest(t)
			upstream := newMidjourneyUpstream(t, http.StatusOK, fmt.Sprintf(`{"code":%d,"description":"accepted","result":"mj-%d"}`, code, code))
			billing, prepareCalls, settleCalls := installRecordingMidjourneyBillingHooks(t, 450)
			c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{"prompt":"billing test"}`, upstream.URL)
			info := &relaycommon.RelayInfo{
				UserId: 901, TokenId: 902, UsingGroup: "gpt-plus", OriginModelName: "mj_imagine",
				RelayMode: relayconstant.RelayModeMidjourneyImagine,
				RequestId: fmt.Sprintf("mj-accepted-%d", code), StartTime: time.Now(), IsPlayground: true,
			}

			mjErr := RelayMidjourneySubmit(c, info)

			require.Nil(t, mjErr)
			require.Equal(t, 1, *prepareCalls)
			require.Equal(t, 1, *settleCalls)
			require.Equal(t, 1, billing.settled)
			require.Zero(t, billing.refunded)
		})
	}
}

func TestRelayMidjourneySubmitFreeModelDoesNotUseLegacySettlement(t *testing.T) {
	setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-free"}`)
	prepareCalls := 0
	settleCalls := 0
	midjourneyModelPriceHelperPerCall = func(*gin.Context, *relaycommon.RelayInfo) (types.PriceData, error) {
		return types.PriceData{FreeModel: true, Quota: 0, ModelPrice: 0, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}, nil
	}
	midjourneyPreConsumeBilling = func(*gin.Context, int, *relaycommon.RelayInfo) *types.NewAPIError {
		prepareCalls++
		return nil
	}
	midjourneySettleBilling = func(*gin.Context, *relaycommon.RelayInfo, int) error {
		settleCalls++
		return nil
	}
	c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{"prompt":"free test"}`, upstream.URL)
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, UsingGroup: "gpt-plus", OriginModelName: "mj_imagine",
		RelayMode: relayconstant.RelayModeMidjourneyImagine,
		RequestId: "mj-free", StartTime: time.Now(), IsPlayground: true,
	}

	mjErr := RelayMidjourneySubmit(c, info)

	require.Nil(t, mjErr)
	require.Zero(t, prepareCalls)
	require.Zero(t, settleCalls)
	require.Nil(t, info.Billing)
}

func TestRelayMidjourneySubmitNonChargeableTaskCannotRefundPhantomQuota(t *testing.T) {
	db := setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-inpaint"}`)
	baseURL := upstream.URL
	require.NoError(t, db.Create(&model.User{Id: 901, Username: "mj-inpaint", Quota: 1000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 902, UserId: 901, Key: "mj-inpaint-token", Name: "mj-inpaint-token",
		Status: common.TokenStatusEnabled, RemainQuota: 1000,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 42, Name: "mj-inpaint", Key: "secret", BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled,
	}).Error)
	origin := &model.Midjourney{
		UserId: 901, ChannelId: 42, MjId: "mj-origin", Action: constant.MjActionImagine,
		Status: "SUCCESS", Progress: "100%", Prompt: "origin",
	}
	require.NoError(t, origin.Insert())
	preConsumeCalls := 0
	midjourneyModelPriceHelperPerCall = func(*gin.Context, *relaycommon.RelayInfo) (types.PriceData, error) {
		return types.PriceData{Quota: 300, QuotaBeforeGroup: 1000, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3}}, nil
	}
	midjourneyPreConsumeBilling = func(*gin.Context, int, *relaycommon.RelayInfo) *types.NewAPIError {
		preConsumeCalls++
		return nil
	}
	c, _ := newMidjourneyBillingContext(
		"/mj/submit/change",
		`{"taskId":"mj-origin","action":"INPAINT","index":1}`,
		upstream.URL,
	)
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, UsingGroup: "gpt-plus", OriginModelName: "mj_inpaint",
		RelayMode: relayconstant.RelayModeMidjourneyChange, RequestId: "mj-inpaint-request",
		StartTime: time.Now(), IsPlayground: false,
	}

	require.Nil(t, RelayMidjourneySubmit(c, info))
	require.Zero(t, preConsumeCalls)
	var task model.Midjourney
	require.NoError(t, db.Where("mj_id = ?", "mj-inpaint").First(&task).Error)
	require.Zero(t, task.Quota)

	task.Status = "FAILURE"
	task.Progress = "100%"
	require.NoError(t, task.Update())
	require.NoError(t, service.RefundMidjourneyQuota(c, &task, "terminal failure"))
	var user model.User
	require.NoError(t, db.First(&user, 901).Error)
	require.Equal(t, 1000, user.Quota)
	var token model.Token
	require.NoError(t, db.First(&token, 902).Error)
	require.Equal(t, 1000, token.RemainQuota)
}

func TestMidjourneyChargeableEntrypointsRefundUnsettledFailures(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		mode       int
		status     int
		response   string
		closeEarly bool
		insertFail bool
		persisted  bool
		call       func(*gin.Context, *relaycommon.RelayInfo) *dto.MidjourneyResponse
	}{
		{
			name: "submit transport failure",
			path: "/mj/submit/imagine", body: `{"prompt":"billing test"}`,
			mode: relayconstant.RelayModeMidjourneyImagine, closeEarly: true, call: RelayMidjourneySubmit,
		},
		{
			name: "submit non success http",
			path: "/mj/submit/imagine", body: `{"prompt":"billing test"}`,
			mode: relayconstant.RelayModeMidjourneyImagine, status: http.StatusBadGateway,
			response: `{"code":1,"description":"bad gateway","result":"mj-http"}`, persisted: true, call: RelayMidjourneySubmit,
		},
		{
			name: "submit business failure",
			path: "/mj/submit/imagine", body: `{"prompt":"billing test"}`,
			mode: relayconstant.RelayModeMidjourneyImagine, status: http.StatusOK,
			response: `{"code":23,"description":"queue full","result":"mj-business"}`, persisted: true, call: RelayMidjourneySubmit,
		},
		{
			name: "submit policy business failure",
			path: "/mj/submit/imagine", body: `{"prompt":"billing test"}`,
			mode: relayconstant.RelayModeMidjourneyImagine, status: http.StatusOK,
			response: `{"code":24,"description":"blocked","result":"mj-policy"}`, persisted: true, call: RelayMidjourneySubmit,
		},
		{
			name: "swap local insert failure",
			path: "/mj/insight-face/swap", body: `{"sourceBase64":"source","targetBase64":"target"}`,
			mode: relayconstant.RelayModeSwapFace, status: http.StatusOK,
			response: `{"code":1,"description":"ok","result":"mj-insert"}`, insertFail: true, call: RelaySwapFace,
		},
		{
			name: "swap business failure",
			path: "/mj/insight-face/swap", body: `{"sourceBase64":"source","targetBase64":"target"}`,
			mode: relayconstant.RelayModeSwapFace, status: http.StatusOK,
			response: `{"code":4,"description":"failed","result":"mj-swap-business"}`, persisted: true, call: RelaySwapFace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupMidjourneyBillingTest(t)
			upstream := newMidjourneyUpstream(t, tt.status, tt.response)
			if tt.closeEarly {
				upstream.Close()
			}
			if tt.insertFail {
				callbackName := "test:fail_midjourney_insert"
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Midjourney" {
						tx.AddError(errors.New("forced midjourney insert failure"))
					}
				}))
			}
			billing, prepareCalls, settleCalls := installRecordingMidjourneyBillingHooks(t, 450)
			c, _ := newMidjourneyBillingContext(tt.path, tt.body, upstream.URL)
			info := &relaycommon.RelayInfo{
				UserId:          901,
				TokenId:         902,
				UsingGroup:      "gpt-plus",
				OriginModelName: "mj_imagine",
				RelayMode:       tt.mode,
				RequestId:       "mj-failure-" + tt.name,
				StartTime:       time.Now(),
				IsPlayground:    true,
			}

			_ = tt.call(c, info)

			require.Equal(t, 1, *prepareCalls)
			require.Zero(t, *settleCalls)
			require.Zero(t, billing.settled)
			if tt.persisted {
				require.Zero(t, billing.refunded)
				require.True(t, billing.NeedsRefund())
				var task model.Midjourney
				require.NoError(t, db.Order("id desc").First(&task).Error)
				require.Equal(t, "REFUND_PENDING", task.Progress)
				require.False(t, task.BillingContext.BillingRefunded)
			} else {
				require.Equal(t, 1, billing.refunded)
				require.False(t, billing.NeedsRefund())
			}
		})
	}
}

func TestRelaySwapFaceRefundsWhenResponseCopyFails(t *testing.T) {
	setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-copy"}`)
	billing, prepareCalls, settleCalls := installRecordingMidjourneyBillingHooks(t, 450)
	c, _ := newMidjourneyBillingContext(
		"/mj/insight-face/swap",
		`{"sourceBase64":"source","targetBase64":"target"}`,
		upstream.URL,
	)
	c.Writer = &failingMidjourneyResponseWriter{ResponseWriter: c.Writer}
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, UsingGroup: "gpt-plus", OriginModelName: "mj_swap_face",
		RelayMode: relayconstant.RelayModeSwapFace, RequestId: "mj-copy-failure",
		StartTime: time.Now(), IsPlayground: true,
	}

	mjErr := RelaySwapFace(c, info)

	require.NotNil(t, mjErr)
	require.Equal(t, "copy_response_body_failed", mjErr.Description)
	require.Equal(t, 1, *prepareCalls)
	require.Zero(t, *settleCalls)
	require.Zero(t, billing.settled)
	require.Zero(t, billing.refunded)
	var task model.Midjourney
	require.NoError(t, model.DB.Where("mj_id = ?", "mj-copy").First(&task).Error)
	require.Equal(t, "REFUND_PENDING", task.Progress)
	require.False(t, task.BillingContext.BillingRefunded)
}

func TestRelayMidjourneyInsertedFailurePersistsIncompleteWalletRefund(t *testing.T) {
	db := setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":23,"description":"queue full","result":"mj-persisted-refund"}`)
	require.NoError(t, db.Create(&model.User{Id: 901, Username: "mj-wallet-refund", Quota: 1000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 902, UserId: 901, Key: "mj-wallet-refund-token", Name: "mj-wallet-refund-token",
		Status: common.TokenStatusEnabled, RemainQuota: 1000,
	}).Error)
	midjourneyModelPriceHelperPerCall = func(*gin.Context, *relaycommon.RelayInfo) (types.PriceData, error) {
		return types.PriceData{Quota: 500, QuotaBeforeGroup: 1000, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.5}}, nil
	}
	billing := &recordingMidjourneyBilling{preConsumed: 500, needsRefund: true}
	callbackName := "test:fail_first_inserted_mj_token_refund"
	failed := false
	midjourneyPreConsumeBilling = func(c *gin.Context, quota int, info *relaycommon.RelayInfo) *types.NewAPIError {
		require.Equal(t, 500, quota)
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", info.UserId).
			Update("quota", gorm.Expr("quota - ?", quota)).Error)
		require.NoError(t, db.Model(&model.Token{}).Where("id = ?", info.TokenId).Updates(map[string]any{
			"remain_quota": gorm.Expr("remain_quota - ?", quota),
			"used_quota":   gorm.Expr("used_quota + ?", quota),
		}).Error)
		info.Billing = billing
		info.BillingSource = service.BillingSourceWallet
		info.BillingUsingGroup = info.UsingGroup
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
			if !failed && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Token" {
				failed = true
				tx.AddError(errors.New("forced inserted token refund failure"))
			}
		}))
		return nil
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })
	c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{"prompt":"billing test"}`, upstream.URL)
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, TokenKey: "mj-wallet-refund-token",
		UsingGroup: "group-a", OriginModelName: "mj_imagine",
		RelayMode: relayconstant.RelayModeMidjourneyImagine, RequestId: "mj-persisted-refund",
		StartTime: time.Now(), IsPlayground: false,
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}

	require.Nil(t, RelayMidjourneySubmit(c, info))
	var pending model.Midjourney
	require.NoError(t, db.Where("mj_id = ?", "mj-persisted-refund").First(&pending).Error)
	require.Equal(t, "REFUND_PENDING", pending.Progress)
	require.True(t, pending.BillingContext.FundingRefunded)
	require.False(t, pending.BillingContext.TokenRefunded)
	require.False(t, pending.BillingContext.BillingRefunded)
	var user model.User
	require.NoError(t, db.First(&user, 901).Error)
	require.Equal(t, 1000, user.Quota)
	var token model.Token
	require.NoError(t, db.First(&token, 902).Error)
	require.Equal(t, 500, token.RemainQuota)

	require.NoError(t, service.RefundMidjourneyQuota(c, &pending, "retry"))
	require.NoError(t, db.First(&token, 902).Error)
	require.Equal(t, 1000, token.RemainQuota)
	var completed model.Midjourney
	require.NoError(t, db.First(&completed, pending.Id).Error)
	require.True(t, completed.BillingContext.BillingRefunded)
	require.Zero(t, billing.refunded)
}

func TestRelayMidjourneySubmitRefundsUnsettledSessionWhenSettlementFails(t *testing.T) {
	db := setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-settle"}`)
	billing, prepareCalls, _ := installRecordingMidjourneyBillingHooks(t, 450)
	settleCalls := 0
	midjourneySettleBilling = func(*gin.Context, *relaycommon.RelayInfo, int) error {
		settleCalls++
		return errors.New("forced settlement failure")
	}
	c, recorder := newMidjourneyBillingContext("/mj/submit/imagine", `{"prompt":"billing test"}`, upstream.URL)
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, UsingGroup: "gpt-plus", OriginModelName: "mj_imagine",
		RelayMode: relayconstant.RelayModeMidjourneyImagine, RequestId: "mj-settle-failure",
		StartTime: time.Now(), IsPlayground: true,
	}

	mjErr := RelayMidjourneySubmit(c, info)

	require.NotNil(t, mjErr)
	require.Equal(t, "settle_midjourney_billing_failed", mjErr.Description)
	require.Equal(t, 1, *prepareCalls)
	require.Equal(t, 1, settleCalls)
	require.Zero(t, billing.settled)
	require.Zero(t, billing.refunded)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":1`)
	var task model.Midjourney
	require.NoError(t, db.Where("mj_id = ?", "mj-settle").First(&task).Error)
	require.Equal(t, "FAILURE", task.Status)
	require.Equal(t, "REFUND_PENDING", task.Progress)
	require.False(t, task.BillingContext.BillingRefunded)
}

func TestRelayMidjourneySubmitKeepsPendingWhenPersistedFundingCannotRefund(t *testing.T) {
	db := setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-funded-settle"}`)
	billing, _, _ := installRecordingMidjourneyBillingHooks(t, 450)
	midjourneySettleBilling = func(*gin.Context, *relaycommon.RelayInfo, int) error {
		billing.needsRefund = false
		return errors.New("funding already committed")
	}
	c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{"prompt":"billing test"}`, upstream.URL)
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, UsingGroup: "gpt-plus", OriginModelName: "mj_imagine",
		RelayMode: relayconstant.RelayModeMidjourneyImagine, RequestId: "mj-funded-settle",
		StartTime: time.Now(), IsPlayground: true,
	}

	mjErr := RelayMidjourneySubmit(c, info)

	require.NotNil(t, mjErr)
	require.Zero(t, billing.refunded)
	var task model.Midjourney
	require.NoError(t, db.Where("mj_id = ?", "mj-funded-settle").First(&task).Error)
	require.Equal(t, "FAILURE", task.Status)
	require.Equal(t, "REFUND_PENDING", task.Progress)
	require.False(t, task.BillingContext.BillingRefunded)
}

func TestRelayMidjourneySettlementErrorUsesPersistedRefundWhenSessionNeedsRefundIsFalse(t *testing.T) {
	db := setupMidjourneyBillingTest(t)
	upstream := newMidjourneyUpstream(t, http.StatusOK, `{"code":1,"description":"ok","result":"mj-funded-settle-persisted"}`)
	require.NoError(t, db.Create(&model.User{Id: 901, Username: "mj-funded-settle", Quota: 1000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 902, UserId: 901, Key: "mj-funded-settle-token", Name: "mj-funded-settle-token",
		Status: common.TokenStatusEnabled, RemainQuota: 1000,
	}).Error)
	midjourneyModelPriceHelperPerCall = func(*gin.Context, *relaycommon.RelayInfo) (types.PriceData, error) {
		return types.PriceData{Quota: 500, QuotaBeforeGroup: 1000, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.5}}, nil
	}
	billing := &recordingMidjourneyBilling{preConsumed: 500, needsRefund: true}
	midjourneyPreConsumeBilling = func(_ *gin.Context, quota int, info *relaycommon.RelayInfo) *types.NewAPIError {
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", info.UserId).
			Update("quota", gorm.Expr("quota - ?", quota)).Error)
		require.NoError(t, db.Model(&model.Token{}).Where("id = ?", info.TokenId).Updates(map[string]any{
			"remain_quota": gorm.Expr("remain_quota - ?", quota),
			"used_quota":   gorm.Expr("used_quota + ?", quota),
		}).Error)
		info.Billing = billing
		info.BillingSource = service.BillingSourceWallet
		info.BillingUsingGroup = info.UsingGroup
		return nil
	}
	midjourneySettleBilling = func(*gin.Context, *relaycommon.RelayInfo, int) error {
		billing.needsRefund = false
		return errors.New("funding already committed")
	}
	c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{"prompt":"billing test"}`, upstream.URL)
	info := &relaycommon.RelayInfo{
		UserId: 901, TokenId: 902, UsingGroup: "group-a", OriginModelName: "mj_imagine",
		RelayMode: relayconstant.RelayModeMidjourneyImagine, RequestId: "mj-funded-settle-persisted",
		StartTime: time.Now(), IsPlayground: false,
	}

	mjErr := RelayMidjourneySubmit(c, info)

	require.NotNil(t, mjErr)
	require.Equal(t, "settle_midjourney_billing_failed", mjErr.Description)
	require.Zero(t, billing.refunded)
	var task model.Midjourney
	require.NoError(t, db.Where("mj_id = ?", "mj-funded-settle-persisted").First(&task).Error)
	require.Equal(t, "100%", task.Progress)
	require.True(t, task.BillingContext.FundingRefunded)
	require.True(t, task.BillingContext.TokenRefunded)
	require.True(t, task.BillingContext.BillingRefunded)
	var user model.User
	require.NoError(t, db.First(&user, 901).Error)
	require.Equal(t, 1000, user.Quota)
	var token model.Token
	require.NoError(t, db.First(&token, 902).Error)
	require.Equal(t, 1000, token.RemainQuota)
}

func TestPrepareMidjourneyBillingSkipsFreeOrNonChargeableRequests(t *testing.T) {
	setupMidjourneyBillingTest(t)
	preConsumeCalls := 0
	midjourneyPreConsumeBilling = func(*gin.Context, int, *relaycommon.RelayInfo) *types.NewAPIError {
		preConsumeCalls++
		return nil
	}
	c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{}`, "http://example.test")

	for _, tt := range []struct {
		name         string
		consumeQuota bool
		priceData    types.PriceData
	}{
		{name: "free model", consumeQuota: true, priceData: types.PriceData{FreeModel: true}},
		{name: "non chargeable action", consumeQuota: false, priceData: types.PriceData{Quota: 300}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{}
			require.Nil(t, prepareMidjourneyBilling(c, info, tt.priceData, tt.consumeQuota))
			require.Equal(t, tt.priceData, info.PriceData)
			require.Nil(t, info.Billing)
		})
	}
	require.Zero(t, preConsumeCalls)
}

func TestPrepareMidjourneyBillingValuePackageUsesFrozenRatioWithoutWallet(t *testing.T) {
	tests := []struct {
		name       string
		special    types.GroupRatioInfo
		wantQuota  int
		wantRatio  float64
		wantSource string
	}{
		{
			name:      "configured",
			special:   types.GroupRatioInfo{GroupRatio: 0.45, GroupSpecialRatio: 0.45, HasSpecialRatio: true},
			wantQuota: 450, wantRatio: 0.45, wantSource: service.SubscriptionRatioSourceConfigured,
		},
		{
			name:      "default one x",
			special:   types.GroupRatioInfo{GroupRatio: 0.3, GroupSpecialRatio: -1, HasSpecialRatio: false},
			wantQuota: 1000, wantRatio: 1, wantSource: service.SubscriptionRatioSourceDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupMidjourneyBillingTest(t)
			user := &model.User{Id: 901, Username: "mj-zero-wallet", Quota: 0, Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(user).Error)
			plan := &model.SubscriptionPlan{
				Title: "mj value package", PlanKind: model.SubscriptionPlanKindValuePackage,
				PackageType: model.ValuePackageTypeMonth, ModelGroup: "month-card",
				DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
				Enabled: true, TotalAmount: 100000,
			}
			require.NoError(t, db.Create(plan).Error)
			t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
			subscription := &model.UserSubscription{
				UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount,
				Status:    model.UserSubscriptionStatusActive,
				StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(),
			}
			require.NoError(t, db.Create(subscription).Error)
			c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{}`, "http://example.test")
			info := &relaycommon.RelayInfo{
				UserId: user.Id, OriginModelName: "mj_imagine", RequestId: "mj-real-" + tt.name,
				UsingGroup: "gpt-plus", BillingUserGroup: "month-card", IsPlayground: true,
				ValuePackageSubscriptionId: subscription.Id, ValuePackagePlanId: plan.Id,
				ValuePackageBillingGroup: "month-card",
			}
			priceData := types.PriceData{
				Quota: 300, QuotaBeforeGroup: 1000, GroupRatioInfo: tt.special,
			}

			mjErr := prepareMidjourneyBilling(c, info, priceData, true)

			require.Nil(t, mjErr)
			require.NotNil(t, info.Billing)
			require.Equal(t, service.BillingSourceSubscription, info.BillingSource)
			require.Equal(t, tt.wantQuota, info.PriceData.Quota)
			require.Equal(t, tt.wantRatio, info.PriceData.GroupRatioInfo.GroupRatio)
			require.Equal(t, tt.wantSource, info.PriceData.SubscriptionRatioSource)
			var gotUser model.User
			require.NoError(t, db.First(&gotUser, user.Id).Error)
			require.Zero(t, gotUser.Quota)
			require.NoError(t, service.SettleBilling(c, info, info.PriceData.Quota))
		})
	}
}

func TestPrepareMidjourneyBillingConvertsQuotaFailureToMidjourneyResponse(t *testing.T) {
	setupMidjourneyBillingTest(t)
	midjourneyPreConsumeBilling = func(*gin.Context, int, *relaycommon.RelayInfo) *types.NewAPIError {
		return types.NewErrorWithStatusCode(
			errors.New("wallet depleted"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
		)
	}
	c, _ := newMidjourneyBillingContext("/mj/submit/imagine", `{}`, "http://example.test")

	mjErr := prepareMidjourneyBilling(c, &relaycommon.RelayInfo{}, types.PriceData{Quota: 300}, true)

	require.NotNil(t, mjErr)
	require.Equal(t, constant.MjRequestError, mjErr.Code)
	require.Equal(t, "quota_not_enough", mjErr.Description)
}
