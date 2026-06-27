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
	assert.Equal(t, model.LdxpStatusWorkerPaid, session.Status, "Task 5 must not verify/redeem yet")
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
