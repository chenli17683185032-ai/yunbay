package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const ldxpMailFixtureText = `订单号：LD260628UZJ97P
商品名称：0.1 元测试
支付金额：0.10
卡密账号：abcd1234-card-key
付款时间：2026-06-28 03:10:00`

func TestParseLdxpMailExtractsOrderAmountAndCard(t *testing.T) {
	parsed, err := ParseLdxpMailText(ldxpMailFixtureText)

	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "LD260628UZJ97P", parsed.OrderNo)
	assert.Equal(t, 0.10, parsed.Amount)
	assert.Equal(t, "0.1 元测试", parsed.ProductName)
	assert.Equal(t, "abcd1234-card-key", parsed.CardKey)
	assert.Equal(t, time.Date(2026, 6, 28, 3, 10, 0, 0, time.Local).Unix(), parsed.PaidTime)
	assert.NotContains(t, parsed.BodyExcerpt, "abcd1234-card-key")
	assert.Contains(t, parsed.BodyExcerpt, RedactLdxpValue("abcd1234-card-key"))
	assert.LessOrEqual(t, len([]rune(parsed.BodyExcerpt)), 500)
}

func TestParseLdxpMailHandlesHtmlBody(t *testing.T) {
	input := `<html><body><p>订单编号：LD260628UZJ97P</p><br><div>购买内容：0.1 元测试</div><div>金额：¥0.10 元</div><div>兑换码：</div><strong>abcd1234-card-key</strong></body></html>`

	parsed, err := ParseLdxpMailText(input)

	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "LD260628UZJ97P", parsed.OrderNo)
	assert.Equal(t, 0.10, parsed.Amount)
	assert.Equal(t, "0.1 元测试", parsed.ProductName)
	assert.Equal(t, "abcd1234-card-key", parsed.CardKey)
	assert.NotContains(t, NormalizeLdxpMailBody(input), "<strong>")
}

func TestParseLdxpMailDoesNotTreatProductContentAsCard(t *testing.T) {
	body := "订单号：LD260628UZJ97P\n购买内容：OpenAIPro\n卡密信息：SECRET-CARD-999999\n金额：0.10"

	parsed, err := ParseLdxpMailText(body)

	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "OpenAIPro", parsed.ProductName)
	assert.Equal(t, "SECRET-CARD-999999", parsed.CardKey)
	assert.NotContains(t, parsed.BodyExcerpt, "SECRET-CARD-999999")
}

func TestParseLdxpMailDoesNotTreatProductKeywordsAsCard(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		wantProduct string
	}{
		{
			name:        "card_keyword_in_product_name",
			body:        "订单号：LD260628UZJ97P\n商品名称：卡密套餐\n金额：0.10",
			wantProduct: "卡密套餐",
		},
		{
			name:        "english_code_in_product_name",
			body:        "订单号：LD260628UZJ97P\n商品名称：code plan\n金额：0.10",
			wantProduct: "code plan",
		},
		{
			name:        "cdkey_in_purchase_content",
			body:        "订单号：LD260628UZJ97P\n购买内容：CDKEY套餐\n金额：0.10",
			wantProduct: "CDKEY套餐",
		},
		{
			name:        "card_keyword_with_space_in_product_name",
			body:        "订单号：LD260628UZJ97P\n商品名称：卡密 套餐\n金额：0.10",
			wantProduct: "卡密 套餐",
		},
		{
			name:        "cdkey_with_space_in_purchase_content",
			body:        "订单号：LD260628UZJ97P\n购买内容：CDKEY plan\n金额：0.10",
			wantProduct: "CDKEY plan",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseLdxpMailText(tc.body)

			require.NoError(t, err)
			require.NotNil(t, parsed)
			assert.Equal(t, "LD260628UZJ97P", parsed.OrderNo)
			assert.Equal(t, tc.wantProduct, parsed.ProductName)
			assert.Empty(t, parsed.CardKey)
		})
	}
}

func TestParseLdxpMailAcceptsAndRedactsNonSpaceCardTokens(t *testing.T) {
	for _, token := range []string{
		"SECRET.CARD.999999",
		"SECRET/CARD/999999",
		"abc",
	} {
		t.Run(token, func(t *testing.T) {
			body := "订单号：LD260628UZJ97P\n商品名称：0.1 元测试\n金额：0.10\n卡密：\n" + token

			parsed, err := ParseLdxpMailText(body)

			require.NoError(t, err)
			require.NotNil(t, parsed)
			assert.Equal(t, token, parsed.CardKey)
			assert.NotContains(t, parsed.BodyExcerpt, token)
			assert.Contains(t, parsed.BodyExcerpt, RedactLdxpValue(token))
			assert.NotContains(t, parsed.BodyExcerpt, ".CARD")
			assert.NotContains(t, parsed.BodyExcerpt, "/CARD")
		})
	}
}

func TestParseLdxpMailDoesNotUseNextFieldAsCard(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "payment_time",
			body: "订单号：LD260628UZJ97P\n商品名称：0.1 元测试\n金额：0.10\n卡密：\n付款时间：2026-06-28 03:10:00",
		},
		{
			name: "payment_amount",
			body: "订单号：LD260628UZJ97P\n商品名称：0.1 元测试\n金额：0.10\n卡密：\n支付金额：0.10",
		},
		{
			name: "plain_chinese_value",
			body: "订单号：LD260628UZJ97P\n商品名称：0.1 元测试\n金额：0.10\n卡密：\n套餐",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseLdxpMailText(tc.body)

			require.NoError(t, err)
			require.NotNil(t, parsed)
			assert.Empty(t, parsed.CardKey)
		})
	}
}

func TestMatchLdxpMailEventRejectsMissingCardWhenWorkerCardEmpty(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:         "ldxp_mail_missing_card",
		UserId:            1001,
		Amount:            10,
		Money:             0.10,
		Status:            model.LdxpStatusWorkerPaid,
		WorkerId:          "worker-a",
		WorkerOrderNo:     "LD260628UZJ97P",
		WorkerAmount:      0.10,
		WorkerProductName: "0.1 元测试",
		WorkerCardKey:     "",
		CreatedTime:       100,
		UpdatedTime:       100,
		ExpiredTime:       2000,
	})

	body := "订单号：LD260628UZJ97P\n商品名称：0.1 元测试\n金额：0.10\n卡密：\n支付金额：0.10"
	parsed, err := ParseLdxpMailText(body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Empty(t, parsed.CardKey)

	messageID := "message-missing-card"
	event := &model.LdxpMailEvent{
		MessageId:    &messageID,
		RawHash:      HashLdxpMailRaw([]byte(body)),
		OrderNo:      parsed.OrderNo,
		Amount:       parsed.Amount,
		ProductName:  parsed.ProductName,
		CardKey:      parsed.CardKey,
		MailFrom:     "sender@example.test",
		MailTo:       "buyer@example.test",
		Subject:      "paid",
		ReceivedTime: 101,
		CreatedTime:  102,
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))

	session, err := TryMatchLdxpMailEvent(event)

	require.Error(t, err)
	assert.Nil(t, session)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_mail_missing_card")
	require.NoError(t, err)
	assert.Empty(t, persisted.MailOrderNo)
	assert.Empty(t, persisted.MailCardKey)
	assert.Empty(t, persisted.MailMessageId)
	var persistedEvent model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persistedEvent, event.Id).Error)
	assert.False(t, persistedEvent.Processed)
	assert.Empty(t, persistedEvent.MatchedSessionId)
}

func TestUpsertLdxpMailEventDedupesRawHash(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	raw := []byte(ldxpMailFixtureText)
	parsed, err := ParseLdxpMailText(string(raw))
	require.NoError(t, err)
	parsed.MessageID = "   "
	parsed.ImapUID = "uid-1"
	parsed.RawHash = HashLdxpMailRaw(raw)
	parsed.From = "sender@example.test"
	parsed.To = "buyer@example.test"
	parsed.Subject = "paid"
	parsed.ReceivedTime = 100

	first, err := SaveLdxpMailEvent(parsed)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Nil(t, first.MessageId)

	duplicate := *parsed
	duplicate.ImapUID = "uid-2"
	duplicate.Subject = "duplicate should not overwrite"
	second, err := SaveLdxpMailEvent(&duplicate)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, first.Id, second.Id)
	assert.Equal(t, "uid-1", second.ImapUid)
	assert.Equal(t, "paid", second.Subject)
}

func TestMatchLdxpMailEventToWorkerSessionRequiresOrderNo(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	event := &model.LdxpMailEvent{
		RawHash:     HashLdxpMailRaw([]byte("missing-order")),
		OrderNo:     "",
		Amount:      0.10,
		CardKey:     "abcd1234-card-key",
		CreatedTime: 100,
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))

	session, err := TryMatchLdxpMailEvent(event)

	require.Error(t, err)
	assert.Nil(t, session)
	var persisted model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persisted, event.Id).Error)
	assert.False(t, persisted.Processed)
	assert.Empty(t, persisted.MatchedSessionId)
}

func TestMatchLdxpMailEventRejectsMismatchedAmount(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:         "ldxp_mail_mismatch_amount",
		UserId:            1001,
		Amount:            10,
		Money:             0.10,
		Status:            model.LdxpStatusWorkerPaid,
		WorkerId:          "worker-a",
		WorkerOrderNo:     "LD260628UZJ97P",
		WorkerAmount:      0.10,
		WorkerProductName: "0.1 元测试",
		WorkerCardKey:     "abcd1234-card-key",
		CreatedTime:       100,
		UpdatedTime:       100,
		ExpiredTime:       2000,
	})
	messageID := "message-match-mismatch-amount"
	event := &model.LdxpMailEvent{
		MessageId:    &messageID,
		RawHash:      HashLdxpMailRaw([]byte("mismatch-amount")),
		OrderNo:      "LD260628UZJ97P",
		Amount:       0.11,
		ProductName:  "0.1 元测试",
		CardKey:      "abcd1234-card-key",
		MailFrom:     "sender@example.test",
		MailTo:       "buyer@example.test",
		Subject:      "paid",
		ReceivedTime: 101,
		CreatedTime:  102,
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))

	session, err := TryMatchLdxpMailEvent(event)

	require.Error(t, err)
	assert.Nil(t, session)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_mail_mismatch_amount")
	require.NoError(t, err)
	assert.Empty(t, persisted.MailOrderNo)
	assert.Empty(t, persisted.MailCardKey)
	assert.Zero(t, persisted.MailAmount)
	var persistedEvent model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persistedEvent, event.Id).Error)
	assert.False(t, persistedEvent.Processed)
	assert.Empty(t, persistedEvent.MatchedSessionId)
}

func TestMatchLdxpMailEventRejectsMismatchedCard(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:         "ldxp_mail_mismatch_card",
		UserId:            1001,
		Amount:            10,
		Money:             0.10,
		Status:            model.LdxpStatusWorkerPaid,
		WorkerId:          "worker-a",
		WorkerOrderNo:     "LD260628UZJ97P",
		WorkerAmount:      0.10,
		WorkerProductName: "0.1 元测试",
		WorkerCardKey:     "different-card-key",
		CreatedTime:       100,
		UpdatedTime:       100,
		ExpiredTime:       2000,
	})
	messageID := "message-match-mismatch"
	event := &model.LdxpMailEvent{
		MessageId:    &messageID,
		RawHash:      HashLdxpMailRaw([]byte("mismatch-card")),
		OrderNo:      "LD260628UZJ97P",
		Amount:       0.10,
		ProductName:  "0.1 元测试",
		CardKey:      "abcd1234-card-key",
		MailFrom:     "sender@example.test",
		MailTo:       "buyer@example.test",
		Subject:      "paid",
		ReceivedTime: 101,
		CreatedTime:  102,
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))

	session, err := TryMatchLdxpMailEvent(event)

	require.Error(t, err)
	assert.Nil(t, session)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_mail_mismatch_card")
	require.NoError(t, err)
	assert.Empty(t, persisted.MailCardKey)
	assert.Empty(t, persisted.MailOrderNo)
	var persistedEvent model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persistedEvent, event.Id).Error)
	assert.False(t, persistedEvent.Processed)
	assert.Empty(t, persistedEvent.MatchedSessionId)
}

func TestMatchLdxpMailEventRejectsMissingCardOrAmount(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cardKey string
		amount  float64
	}{
		{name: "missing_card", cardKey: "", amount: 0.10},
		{name: "missing_amount", cardKey: "abcd1234-card-key", amount: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupLdxpSessionServiceTest(t)
			sessionID := "ldxp_mail_" + tc.name
			insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
				SessionId:         sessionID,
				UserId:            1001,
				Amount:            10,
				Money:             0.10,
				Status:            model.LdxpStatusWorkerPaid,
				WorkerId:          "worker-a",
				WorkerOrderNo:     "LD260628UZJ97P",
				WorkerAmount:      0.10,
				WorkerProductName: "0.1 元测试",
				WorkerCardKey:     "abcd1234-card-key",
				CreatedTime:       100,
				UpdatedTime:       100,
				ExpiredTime:       2000,
			})
			messageID := "message-" + tc.name
			event := &model.LdxpMailEvent{
				MessageId:    &messageID,
				RawHash:      HashLdxpMailRaw([]byte(tc.name)),
				OrderNo:      "LD260628UZJ97P",
				Amount:       tc.amount,
				ProductName:  "0.1 元测试",
				CardKey:      tc.cardKey,
				MailFrom:     "sender@example.test",
				MailTo:       "buyer@example.test",
				Subject:      "paid",
				ReceivedTime: 101,
				CreatedTime:  102,
			}
			require.NoError(t, model.InsertLdxpMailEvent(event))

			session, err := TryMatchLdxpMailEvent(event)

			require.Error(t, err)
			assert.Nil(t, session)
			persisted, err := model.GetLdxpTopupSessionBySessionId(sessionID)
			require.NoError(t, err)
			assert.Empty(t, persisted.MailCardKey)
			assert.Empty(t, persisted.MailOrderNo)
			assert.Empty(t, persisted.MailMessageId)
			var persistedEvent model.LdxpMailEvent
			require.NoError(t, model.DB.First(&persistedEvent, event.Id).Error)
			assert.False(t, persistedEvent.Processed)
			assert.Empty(t, persistedEvent.MatchedSessionId)
		})
	}
}

func TestMatchLdxpMailEventDoesNotOverwriteExistingMailFields(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:         "ldxp_mail_no_overwrite",
		UserId:            1001,
		Amount:            10,
		Money:             0.10,
		Status:            model.LdxpStatusWorkerPaid,
		WorkerId:          "worker-a",
		WorkerOrderNo:     "LD260628UZJ97P",
		WorkerAmount:      0.10,
		WorkerProductName: "0.1 元测试",
		WorkerCardKey:     "abcd1234-card-key",
		CreatedTime:       100,
		UpdatedTime:       100,
		ExpiredTime:       2000,
	})
	firstMessageID := "message-first"
	firstEvent := &model.LdxpMailEvent{
		MessageId:    &firstMessageID,
		RawHash:      HashLdxpMailRaw([]byte("first-event")),
		MailFrom:     "first@example.test",
		MailTo:       "buyer@example.test",
		Subject:      "first paid",
		ReceivedTime: 101,
		OrderNo:      "LD260628UZJ97P",
		Amount:       0.10,
		ProductName:  "0.1 元测试",
		CardKey:      "abcd1234-card-key",
		CreatedTime:  102,
	}
	require.NoError(t, model.InsertLdxpMailEvent(firstEvent))
	firstSession, err := TryMatchLdxpMailEvent(firstEvent)
	require.NoError(t, err)
	require.NotNil(t, firstSession)

	secondMessageID := "message-second"
	secondEvent := &model.LdxpMailEvent{
		MessageId:    &secondMessageID,
		RawHash:      HashLdxpMailRaw([]byte("second-event")),
		MailFrom:     "second@example.test",
		MailTo:       "other@example.test",
		Subject:      "second paid",
		ReceivedTime: 201,
		OrderNo:      "LD260628UZJ97P",
		Amount:       0.10,
		ProductName:  "changed product",
		CardKey:      "abcd1234-card-key",
		CreatedTime:  202,
	}
	require.NoError(t, model.InsertLdxpMailEvent(secondEvent))

	secondSession, err := TryMatchLdxpMailEvent(secondEvent)

	require.Error(t, err)
	assert.Nil(t, secondSession)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_mail_no_overwrite")
	require.NoError(t, err)
	assert.Equal(t, firstMessageID, persisted.MailMessageId)
	assert.Equal(t, "LD260628UZJ97P", persisted.MailOrderNo)
	assert.Equal(t, "abcd1234-card-key", persisted.MailCardKey)
	assert.Equal(t, "first@example.test", persisted.MailFrom)
	assert.Equal(t, "buyer@example.test", persisted.MailTo)
	assert.Equal(t, "first paid", persisted.MailSubject)
	assert.Equal(t, int64(101), persisted.MailReceivedTime)

	var persistedSecond model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persistedSecond, secondEvent.Id).Error)
	assert.False(t, persistedSecond.Processed)
	assert.Empty(t, persistedSecond.MatchedSessionId)
}

func TestMatchLdxpMailEventAttachesMailFields(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:         "ldxp_mail_match_success",
		UserId:            1001,
		Amount:            10,
		Money:             0.10,
		Status:            model.LdxpStatusWorkerPaid,
		WorkerId:          "worker-a",
		WorkerOrderNo:     "LD260628UZJ97P",
		WorkerAmount:      0.10,
		WorkerProductName: "0.1 元测试",
		WorkerCardKey:     "abcd1234-card-key",
		CreatedTime:       100,
		UpdatedTime:       100,
		ExpiredTime:       2000,
	})
	messageID := "message-match-success"
	event := &model.LdxpMailEvent{
		MessageId:    &messageID,
		RawHash:      HashLdxpMailRaw([]byte("match-success")),
		MailFrom:     "sender@example.test",
		MailTo:       "buyer@example.test",
		Subject:      "paid",
		ReceivedTime: 101,
		OrderNo:      "LD260628UZJ97P",
		Amount:       0.10,
		ProductName:  "0.1 元测试",
		CardKey:      "abcd1234-card-key",
		CreatedTime:  102,
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))

	session, err := TryMatchLdxpMailEvent(event)

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "ldxp_mail_match_success", session.SessionId)
	assert.Equal(t, model.LdxpStatusVerifyFailed, session.Status, "Task 6 verifier runs after mail attach and records non-paid worker status as an audit failure")
	assert.Equal(t, "status_not_paid", session.ErrorCode)
	assert.Contains(t, session.ErrorMessage, "not paid")
	assert.Equal(t, "LD260628UZJ97P", session.MailOrderNo)
	assert.Equal(t, "abcd1234-card-key", session.MailCardKey)
	assert.Equal(t, 0.10, session.MailAmount)
	assert.Equal(t, "sender@example.test", session.MailFrom)
	assert.Equal(t, "buyer@example.test", session.MailTo)
	assert.Equal(t, "paid", session.MailSubject)
	assert.Greater(t, session.UpdatedTime, int64(100))

	var persistedEvent model.LdxpMailEvent
	require.NoError(t, model.DB.First(&persistedEvent, event.Id).Error)
	assert.True(t, persistedEvent.Processed)
	assert.Equal(t, "ldxp_mail_match_success", persistedEvent.MatchedSessionId)
	assert.Empty(t, persistedEvent.ErrorMessage)
}

func TestSaveLdxpMailEventRejectsMissingRawHash(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	parsed, err := ParseLdxpMailText(ldxpMailFixtureText)
	require.NoError(t, err)

	event, err := SaveLdxpMailEvent(parsed)

	require.ErrorIs(t, err, gorm.ErrInvalidData)
	assert.Nil(t, event)
}

func TestParseLdxpMailBodyExcerptDoesNotLeakLongBodies(t *testing.T) {
	body := ldxpMailFixtureText + "\n" + strings.Repeat("填充内容", 300)

	parsed, err := ParseLdxpMailText(body)

	require.NoError(t, err)
	assert.LessOrEqual(t, len([]rune(parsed.BodyExcerpt)), 500)
	assert.NotContains(t, parsed.BodyExcerpt, "abcd1234-card-key")
}

func TestParseLdxpMailDoesNotLeakCardWhenCardLabelChanges(t *testing.T) {
	for _, body := range []string{
		"订单号：LD260628UZJ97P\n购买内容卡号：SECRET-CARD-999999\n金额：0.10",
		"订单号：LD260628UZJ97P\n发货内容：SECRET-CARD-999999\n金额：0.10",
	} {
		parsed, err := ParseLdxpMailText(body)

		require.NoError(t, err)
		require.NotNil(t, parsed)
		assert.NotContains(t, parsed.BodyExcerpt, "SECRET-CARD-999999")
		assert.Contains(t, parsed.BodyExcerpt, RedactLdxpValue("SECRET-CARD-999999"))
	}
}

func TestParseLdxpMailDoesNotGreedilyCaptureStickyFields(t *testing.T) {
	body := "订单号：LD260628UZJ97P\n商品名称：0.1 元测试金额：0.10卡密账号：abcd1234-card-key付款时间：2026-06-28 03:10:00"

	parsed, err := ParseLdxpMailText(body)

	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "0.1 元测试", parsed.ProductName)
	assert.Equal(t, "abcd1234-card-key", parsed.CardKey)
	assert.NotContains(t, parsed.CardKey, "付款时间")
	assert.NotContains(t, parsed.ProductName, "金额")
}

func TestParseLdxpMailRedactsCommonSecretLabelVariants(t *testing.T) {
	body := "订单号：LD260628UZJ97P\n卡密信息：SECRET-CARD-111111\n兑换码为：SECRET-CARD-222222\n金额：0.10"

	parsed, err := ParseLdxpMailText(body)

	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.NotContains(t, parsed.BodyExcerpt, "SECRET-CARD-111111")
	assert.NotContains(t, parsed.BodyExcerpt, "SECRET-CARD-222222")
	assert.Contains(t, parsed.BodyExcerpt, "[redacted]")
}

func TestMatchLdxpMailEventRejectsProcessedOrMatchedEventStates(t *testing.T) {
	for _, tc := range []struct {
		name             string
		processed        bool
		matchedSessionId string
	}{
		{
			name:             "matched_session_set",
			processed:        false,
			matchedSessionId: "other-session",
		},
		{
			name:             "processed_but_unmatched",
			processed:        true,
			matchedSessionId: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupLdxpSessionServiceTest(t)
			insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
				SessionId:         "ldxp_mail_reject_" + tc.name,
				UserId:            1001,
				Amount:            10,
				Money:             0.10,
				Status:            model.LdxpStatusWorkerPaid,
				WorkerId:          "worker-a",
				WorkerOrderNo:     "LD260628UZJ97P",
				WorkerAmount:      0.10,
				WorkerProductName: "0.1 元测试",
				WorkerCardKey:     "abcd1234-card-key",
				CreatedTime:       100,
				UpdatedTime:       100,
				ExpiredTime:       2000,
			})
			messageID := "message-reject-" + tc.name
			event := &model.LdxpMailEvent{
				MessageId:    &messageID,
				RawHash:      HashLdxpMailRaw([]byte("reject-" + tc.name)),
				MailFrom:     "sender@example.test",
				MailTo:       "buyer@example.test",
				Subject:      "paid",
				ReceivedTime: 101,
				OrderNo:      "LD260628UZJ97P",
				Amount:       0.10,
				ProductName:  "0.1 元测试",
				CardKey:      "abcd1234-card-key",
				CreatedTime:  102,
			}
			require.NoError(t, model.InsertLdxpMailEvent(event))
			require.NoError(t, model.DB.Model(&model.LdxpMailEvent{}).
				Where("id = ?", event.Id).
				Updates(map[string]interface{}{
					"processed":          tc.processed,
					"matched_session_id": tc.matchedSessionId,
				}).Error)

			session, err := TryMatchLdxpMailEvent(event)

			require.Error(t, err)
			assert.Nil(t, session)
			persistedSession, err := model.GetLdxpTopupSessionBySessionId("ldxp_mail_reject_" + tc.name)
			require.NoError(t, err)
			assert.Empty(t, persistedSession.MailMessageId)
			assert.Empty(t, persistedSession.MailOrderNo)
			assert.Empty(t, persistedSession.MailCardKey)
			var persistedEvent model.LdxpMailEvent
			require.NoError(t, model.DB.First(&persistedEvent, event.Id).Error)
			assert.Equal(t, tc.processed, persistedEvent.Processed)
			assert.Equal(t, tc.matchedSessionId, persistedEvent.MatchedSessionId)
		})
	}
}

func TestSaveLdxpMailEventDedupesMessageID(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	firstMessageID := "duplicate-message-id"
	first := &LdxpParsedMail{
		MessageID:    firstMessageID,
		ImapUID:      "uid-first",
		RawHash:      HashLdxpMailRaw([]byte("first-message")),
		From:         "first@example.test",
		To:           "buyer@example.test",
		Subject:      "first",
		ReceivedTime: 100,
		OrderNo:      "LD260628UZJ97P",
		Amount:       0.10,
		ProductName:  "0.1 元测试",
		CardKey:      "abcd1234-card-key",
		BodyExcerpt:  "first excerpt",
	}
	firstEvent, err := SaveLdxpMailEvent(first)
	require.NoError(t, err)
	require.NotNil(t, firstEvent)

	second := *first
	second.RawHash = HashLdxpMailRaw([]byte("second-message"))
	second.ImapUID = "uid-second"
	second.Subject = "second should not overwrite"
	secondEvent, err := SaveLdxpMailEvent(&second)

	require.NoError(t, err)
	require.NotNil(t, secondEvent)
	assert.Equal(t, firstEvent.Id, secondEvent.Id)
	assert.Equal(t, "uid-first", secondEvent.ImapUid)
	assert.Equal(t, "first", secondEvent.Subject)
}
