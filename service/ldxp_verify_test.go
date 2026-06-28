package service

import (
	"fmt"
	"testing"
	_ "unsafe"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:linkname initLdxpVerifyModelColumns github.com/QuantumNous/new-api/model.initCol
func initLdxpVerifyModelColumns()

func setupLdxpVerifyServiceTest(t *testing.T) {
	t.Helper()
	initLdxpVerifyModelColumns()
	setupLdxpSessionServiceTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Redemption{}, &model.TopUp{}, &model.Log{}))
	cleanup := func() {
		require.NoError(t, model.DB.Exec("DELETE FROM redemptions").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM top_ups").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM logs").Error)
	}
	cleanup()
	t.Cleanup(cleanup)
}

func insertLdxpVerifyUser(t *testing.T, userID int, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userID,
		Username: fmt.Sprintf("ldxp-verify-user-%d", userID),
		AffCode:  fmt.Sprintf("ldxp-verify-aff-%d", userID),
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}).Error)
}

func insertLdxpVerifyPaidTopupCard(t *testing.T, key string, quota int) *model.Redemption {
	t.Helper()
	redemption := &model.Redemption{
		Key:          key,
		Status:       common.RedemptionCodeStatusEnabled,
		Name:         "LDXP paid topup test card",
		Quota:        quota,
		Kind:         model.RedemptionKindPaidTopUp,
		Amount:       10,
		Money:        0.10,
		CountAsTopUp: true,
		BatchId:      "ldxp-verify-batch",
		Source:       model.RedemptionSourceLDXP,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(redemption).Error)
	return redemption
}

func insertLdxpVerifySession(t *testing.T, session *model.LdxpTopupSession) {
	t.Helper()
	if session.CreatedTime == 0 {
		session.CreatedTime = 100
	}
	if session.UpdatedTime == 0 {
		session.UpdatedTime = 100
	}
	if session.ExpiredTime == 0 {
		session.ExpiredTime = common.GetTimestamp() + 1200
	}
	require.NoError(t, model.InsertLdxpTopupSession(session))
}

func insertLdxpVerifyMailEvent(t *testing.T, orderNo string, cardKey string, amount float64) *model.LdxpMailEvent {
	t.Helper()
	event := &model.LdxpMailEvent{
		RawHash:          HashLdxpMailRaw([]byte(orderNo + "|" + cardKey + "|verify")),
		MailFrom:         "sender@example.test",
		MailTo:           "buyer@example.test",
		Subject:          "paid",
		ReceivedTime:     101,
		OrderNo:          orderNo,
		Amount:           amount,
		ProductName:      "0.1 元测试",
		CardKey:          cardKey,
		MatchedSessionId: "",
		Processed:        false,
		CreatedTime:      102,
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))
	return event
}

func completeLdxpVerifySession(sessionID string, userID int, orderNo string, cardKey string, money float64) *model.LdxpTopupSession {
	return &model.LdxpTopupSession{
		SessionId:         sessionID,
		UserId:            userID,
		Amount:            10,
		Money:             money,
		Status:            model.LdxpStatusWorkerPaid,
		WorkerId:          "worker-a",
		WorkerOrderNo:     orderNo,
		WorkerAmount:      money,
		WorkerProductName: "0.1 元测试",
		WorkerCardKey:     cardKey,
		WorkerStatusText:  "已付款",
		MailOrderNo:       orderNo,
		MailAmount:        money,
		MailProductName:   "0.1 元测试",
		MailCardKey:       cardKey,
		MailFrom:          "sender@example.test",
		MailTo:            "buyer@example.test",
		MailSubject:       "paid",
		MailReceivedTime:  101,
	}
}

func ldxpVerifyUserQuota(t *testing.T, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", userID).First(&user).Error)
	return user.Quota
}

func ldxpVerifyTopUps(t *testing.T, userID int) []model.TopUp {
	t.Helper()
	var topUps []model.TopUp
	require.NoError(t, model.DB.Where("user_id = ?", userID).Order("id asc").Find(&topUps).Error)
	return topUps
}

func TestVerifyLdxpSessionRequiresWorkerAndMailOrderMatch(t *testing.T) {
	t.Run("missing worker order is a verification failure once mail is attached", func(t *testing.T) {
		setupLdxpVerifyServiceTest(t)
		insertLdxpVerifyUser(t, 6101, 0)
		insertLdxpVerifySession(t, &model.LdxpTopupSession{
			SessionId:        "ldxp_verify_missing_worker_order",
			UserId:           6101,
			Amount:           10,
			Money:            0.10,
			Status:           model.LdxpStatusWorkerPaid,
			WorkerCardKey:    "verify-missing-worker-card",
			WorkerStatusText: "已付款",
			MailOrderNo:      "LDVERIFYMISSINGWORKER",
			MailAmount:       0.10,
			MailCardKey:      "verify-missing-worker-card",
		})
		insertLdxpVerifyMailEvent(t, "LDVERIFYMISSINGWORKER", "verify-missing-worker-card", 0.10)

		result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_missing_worker_order")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Verified)
		assert.False(t, result.Redeemed)
		assert.Equal(t, model.LdxpStatusVerifyFailed, result.Status)
		assert.Equal(t, "missing_worker_order", result.ErrorCode)

		persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_missing_worker_order")
		require.NoError(t, err)
		assert.Equal(t, model.LdxpStatusVerifyFailed, persisted.Status)
		assert.Equal(t, "missing_worker_order", persisted.ErrorCode)
		assert.Empty(t, persisted.WorkerOrderNo)
		assert.Equal(t, "LDVERIFYMISSINGWORKER", persisted.MailOrderNo)
	})

	t.Run("missing mail event by worker order is a verification failure after mail fields are attached", func(t *testing.T) {
		setupLdxpVerifyServiceTest(t)
		insertLdxpVerifyUser(t, 6102, 0)
		insertLdxpVerifySession(t, completeLdxpVerifySession("ldxp_verify_missing_mail_event", 6102, "LDVERIFYNOEVENT", "verify-no-event-card", 0.10))

		result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_missing_mail_event")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Verified)
		assert.Equal(t, model.LdxpStatusVerifyFailed, result.Status)
		assert.Equal(t, "mail_event_not_found", result.ErrorCode)

		persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_missing_mail_event")
		require.NoError(t, err)
		assert.Equal(t, model.LdxpStatusVerifyFailed, persisted.Status)
		assert.Equal(t, "mail_event_not_found", persisted.ErrorCode)
	})

	t.Run("attached mail order must match worker order", func(t *testing.T) {
		setupLdxpVerifyServiceTest(t)
		insertLdxpVerifyUser(t, 6103, 0)
		session := completeLdxpVerifySession("ldxp_verify_order_mismatch", 6103, "LDVERIFYWORKER", "verify-order-card", 0.10)
		session.MailOrderNo = "LDVERIFYMAIL"
		insertLdxpVerifySession(t, session)
		insertLdxpVerifyMailEvent(t, "LDVERIFYWORKER", "verify-order-card", 0.10)
		insertLdxpVerifyPaidTopupCard(t, "verify-order-card", 100)

		result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_order_mismatch")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Verified)
		assert.False(t, result.Redeemed)
		assert.Equal(t, model.LdxpStatusVerifyFailed, result.Status)
		assert.Equal(t, "mail_order_mismatch", result.ErrorCode)

		persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_order_mismatch")
		require.NoError(t, err)
		assert.Equal(t, model.LdxpStatusVerifyFailed, persisted.Status)
		assert.Equal(t, "mail_order_mismatch", persisted.ErrorCode)
		assert.Zero(t, persisted.RedemptionId)
		assert.Equal(t, common.RedemptionCodeStatusEnabled, redemptionStatusByKey(t, "verify-order-card"))
	})
}

func TestVerifyLdxpSessionRequiresCardMatch(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6201, 0)
	session := completeLdxpVerifySession("ldxp_verify_card_mismatch", 6201, "LDVERIFYCARD", "verify-card-worker", 0.10)
	session.MailCardKey = "verify-card-mail"
	insertLdxpVerifySession(t, session)
	insertLdxpVerifyMailEvent(t, "LDVERIFYCARD", "verify-card-mail", 0.10)
	insertLdxpVerifyPaidTopupCard(t, "verify-card-worker", 100)

	result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_card_mismatch")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Verified)
	assert.False(t, result.Redeemed)
	assert.Equal(t, model.LdxpStatusVerifyFailed, result.Status)
	assert.Equal(t, "card_mismatch", result.ErrorCode)

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_card_mismatch")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusVerifyFailed, persisted.Status)
	assert.Equal(t, "card_mismatch", persisted.ErrorCode)
	assert.Zero(t, persisted.RedemptionId)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redemptionStatusByKey(t, "verify-card-worker"))
}

func TestVerifyLdxpSessionRequiresAmountMatchWhenConfigured(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6301, 0)
	insertLdxpVerifySession(t, completeLdxpVerifySession("ldxp_verify_amount_mismatch", 6301, "LDVERIFYAMOUNT", "verify-amount-card", 0.10))
	insertLdxpVerifyMailEvent(t, "LDVERIFYAMOUNT", "verify-amount-card", 0.12)
	insertLdxpVerifyPaidTopupCard(t, "verify-amount-card", 100)

	result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_amount_mismatch")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Verified)
	assert.False(t, result.Redeemed)
	assert.Equal(t, model.LdxpStatusVerifyFailed, result.Status)
	assert.Equal(t, "amount_mismatch", result.ErrorCode)

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_amount_mismatch")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusVerifyFailed, persisted.Status)
	assert.Equal(t, "amount_mismatch", persisted.ErrorCode)
	assert.Zero(t, persisted.RedemptionId)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redemptionStatusByKey(t, "verify-amount-card"))
}

func TestVerifyLdxpSessionRedeemsPaidTopupCard(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6401, 50)
	redemption := insertLdxpVerifyPaidTopupCard(t, "verify-redeem-card", 700)
	insertLdxpVerifySession(t, completeLdxpVerifySession("ldxp_verify_redeem_success", 6401, "LDVERIFYREDEEM", "verify-redeem-card", 0.10))
	insertLdxpVerifyMailEvent(t, "LDVERIFYREDEEM", "verify-redeem-card", 0.10)

	result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_redeem_success")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Verified)
	assert.True(t, result.Redeemed)
	assert.Equal(t, model.LdxpStatusSuccess, result.Status)
	assert.Empty(t, result.ErrorCode)
	assert.Empty(t, result.ErrorMessage)
	assert.Equal(t, 750, ldxpVerifyUserQuota(t, 6401))

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_redeem_success")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusSuccess, persisted.Status)
	assert.NotZero(t, persisted.VerifiedTime)
	assert.NotZero(t, persisted.RedeemedTime)
	assert.Equal(t, redemption.Id, persisted.RedemptionId)
	assert.Zero(t, persisted.TopupId)
	assert.Empty(t, persisted.ErrorCode)
	assert.Empty(t, persisted.ErrorMessage)

	var used model.Redemption
	require.NoError(t, model.DB.Where("key = ?", "verify-redeem-card").First(&used).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, used.Status)
	assert.Equal(t, 6401, used.UsedUserId)
	assert.NotZero(t, used.RedeemedTime)

	topUps := ldxpVerifyTopUps(t, 6401)
	require.Len(t, topUps, 1)
	assert.EqualValues(t, 10, topUps[0].Amount)
	assert.Equal(t, 0.10, topUps[0].Money)
	assert.Equal(t, model.CreateRedemptionTopUpTradeNo(redemption.Id, 6401), topUps[0].TradeNo)
}

func TestVerifyLdxpSessionUsesMatchedMailEvent(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6451, 0)
	redemption := insertLdxpVerifyPaidTopupCard(t, "verify-matched-current-card", 400)
	session := completeLdxpVerifySession("ldxp_verify_matched_mail_event", 6451, "LDVERIFYMATCHED", "verify-matched-current-card", 0.10)
	session.MailOrderNo = ""
	session.MailAmount = 0
	session.MailProductName = ""
	session.MailCardKey = ""
	session.MailFrom = ""
	session.MailTo = ""
	session.MailSubject = ""
	session.MailReceivedTime = 0
	insertLdxpVerifySession(t, session)
	insertLdxpVerifyMailEvent(t, "LDVERIFYMATCHED", "verify-matched-old-card", 0.10)
	correctEvent := insertLdxpVerifyMailEvent(t, "LDVERIFYMATCHED", "verify-matched-current-card", 0.10)

	matched, err := TryMatchLdxpMailEvent(correctEvent)

	require.NoError(t, err)
	require.NotNil(t, matched)
	assert.Equal(t, model.LdxpStatusSuccess, matched.Status)
	assert.Equal(t, redemption.Id, matched.RedemptionId)
	assert.Empty(t, matched.ErrorCode)
	assert.Empty(t, matched.ErrorMessage)
	assert.Equal(t, 400, ldxpVerifyUserQuota(t, 6451))

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_matched_mail_event")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusSuccess, persisted.Status)
	assert.Equal(t, "verify-matched-current-card", persisted.MailCardKey)
	assert.Equal(t, redemption.Id, persisted.RedemptionId)

	var persistedCorrect model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persistedCorrect, correctEvent.Id).Error)
	assert.True(t, persistedCorrect.Processed)
	assert.Equal(t, "ldxp_verify_matched_mail_event", persistedCorrect.MatchedSessionId)
}

func TestVerifyLdxpSessionRecoversSameUserUsedRedemption(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6452, 900)
	redemption := insertLdxpVerifyPaidTopupCard(t, "verify-recover-used-card", 500)
	redeemedTime := common.GetTimestamp() - 30
	require.NoError(t, model.DB.Model(&model.Redemption{}).
		Where("id = ?", redemption.Id).
		Updates(map[string]interface{}{
			"status":        common.RedemptionCodeStatusUsed,
			"used_user_id":  6452,
			"redeemed_time": redeemedTime,
		}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          6452,
		Amount:          redemption.Amount,
		Money:           redemption.Money,
		TradeNo:         model.CreateRedemptionTopUpTradeNo(redemption.Id, 6452),
		PaymentMethod:   model.PaymentMethodRedemptionCode,
		PaymentProvider: model.PaymentProviderRedemptionCode,
		CreateTime:      redeemedTime,
		CompleteTime:    redeemedTime,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	insertLdxpVerifySession(t, completeLdxpVerifySession("ldxp_verify_recover_used", 6452, "LDVERIFYRECOVER", "verify-recover-used-card", 0.10))
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ?", "ldxp_verify_recover_used").
		Updates(map[string]interface{}{
			"status":        model.LdxpStatusVerified,
			"verified_time": redeemedTime - 10,
		}).Error)
	insertLdxpVerifyMailEvent(t, "LDVERIFYRECOVER", "verify-recover-used-card", 0.10)
	require.Equal(t, 900, ldxpVerifyUserQuota(t, 6452))
	require.Len(t, ldxpVerifyTopUps(t, 6452), 1)

	result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_recover_used")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Verified)
	assert.True(t, result.Redeemed)
	assert.Equal(t, model.LdxpStatusSuccess, result.Status)
	assert.Empty(t, result.ErrorCode)
	assert.Empty(t, result.ErrorMessage)
	assert.Equal(t, 900, ldxpVerifyUserQuota(t, 6452))
	assert.Len(t, ldxpVerifyTopUps(t, 6452), 1)

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_recover_used")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusSuccess, persisted.Status)
	assert.Equal(t, redemption.Id, persisted.RedemptionId)
	assert.Equal(t, redeemedTime, persisted.RedeemedTime)
	assert.Empty(t, persisted.ErrorCode)
	assert.Empty(t, persisted.ErrorMessage)
}

func TestVerifyLdxpSessionIsIdempotent(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6501, 10)
	redemption := insertLdxpVerifyPaidTopupCard(t, "verify-idempotent-card", 300)
	insertLdxpVerifySession(t, completeLdxpVerifySession("ldxp_verify_idempotent", 6501, "LDVERIFYIDEMPOTENT", "verify-idempotent-card", 0.10))
	insertLdxpVerifyMailEvent(t, "LDVERIFYIDEMPOTENT", "verify-idempotent-card", 0.10)

	first, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_idempotent")
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, model.LdxpStatusSuccess, first.Status)
	assert.Equal(t, 310, ldxpVerifyUserQuota(t, 6501))

	second, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_idempotent")

	require.NoError(t, err)
	require.NotNil(t, second)
	assert.True(t, second.Verified)
	assert.True(t, second.Redeemed)
	assert.Equal(t, model.LdxpStatusSuccess, second.Status)
	assert.Empty(t, second.ErrorCode)
	assert.Equal(t, 310, ldxpVerifyUserQuota(t, 6501))
	assert.Len(t, ldxpVerifyTopUps(t, 6501), 1)

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_idempotent")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusSuccess, persisted.Status)
	assert.Equal(t, redemption.Id, persisted.RedemptionId)
}

func TestVerifyLdxpSessionStoresRedeemFailure(t *testing.T) {
	setupLdxpVerifyServiceTest(t)
	insertLdxpVerifyUser(t, 6601, 0)
	insertLdxpVerifySession(t, completeLdxpVerifySession("ldxp_verify_redeem_failure", 6601, "LDVERIFYFAIL", "verify-missing-redeem-card", 0.10))
	insertLdxpVerifyMailEvent(t, "LDVERIFYFAIL", "verify-missing-redeem-card", 0.10)

	result, err := TryVerifyAndRedeemLdxpSession("ldxp_verify_redeem_failure")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Verified)
	assert.False(t, result.Redeemed)
	assert.Equal(t, model.LdxpStatusRedeemFailed, result.Status)
	assert.Equal(t, "redeem_failed", result.ErrorCode)
	assert.Contains(t, result.ErrorMessage, "redemption.invalid")

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_verify_redeem_failure")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusRedeemFailed, persisted.Status)
	assert.NotZero(t, persisted.VerifiedTime)
	assert.Zero(t, persisted.RedeemedTime)
	assert.Zero(t, persisted.RedemptionId)
	assert.Zero(t, persisted.TopupId)
	assert.Equal(t, "redeem_failed", persisted.ErrorCode)
	assert.Contains(t, persisted.ErrorMessage, "redemption.invalid")
	assert.Equal(t, "LDVERIFYFAIL", persisted.WorkerOrderNo)
	assert.Equal(t, "verify-missing-redeem-card", persisted.WorkerCardKey)
	assert.Equal(t, "LDVERIFYFAIL", persisted.MailOrderNo)
	assert.Equal(t, "verify-missing-redeem-card", persisted.MailCardKey)
	assert.Equal(t, 0, ldxpVerifyUserQuota(t, 6601))
	assert.Empty(t, ldxpVerifyTopUps(t, 6601))
}

func redemptionStatusByKey(t *testing.T, key string) int {
	t.Helper()
	var redemption model.Redemption
	require.NoError(t, model.DB.Select("status").Where("key = ?", key).First(&redemption).Error)
	return redemption.Status
}

func TestLdxpVerifyTask6Suite(t *testing.T) {
	t.Run("RequiresWorkerAndMailOrderMatch", TestVerifyLdxpSessionRequiresWorkerAndMailOrderMatch)
	t.Run("RequiresCardMatch", TestVerifyLdxpSessionRequiresCardMatch)
	t.Run("RequiresAmountMatchWhenConfigured", TestVerifyLdxpSessionRequiresAmountMatchWhenConfigured)
	t.Run("RedeemsPaidTopupCard", TestVerifyLdxpSessionRedeemsPaidTopupCard)
	t.Run("UsesMatchedMailEvent", TestVerifyLdxpSessionUsesMatchedMailEvent)
	t.Run("RecoversSameUserUsedRedemption", TestVerifyLdxpSessionRecoversSameUserUsedRedemption)
	t.Run("IsIdempotent", TestVerifyLdxpSessionIsIdempotent)
	t.Run("StoresRedeemFailure", TestVerifyLdxpSessionStoresRedeemFailure)
}
