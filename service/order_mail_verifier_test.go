package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestVerifyLdxpMailUsesExternalPaidAmount(t *testing.T) {
	session := &model.LdxpTopupSession{
		SiteAmountCents:   50000,
		ExternalPaidCents: 42500,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 42500, OrderNo: "LD260628UZJ97P"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusVerified, result.Status)
	assert.Empty(t, result.ErrorCode)
	assert.Empty(t, result.ErrorMessage)
}

func TestVerifyLdxpMailAllowsUserPaidFeeOnTop(t *testing.T) {
	session := &model.LdxpTopupSession{
		SiteAmountCents:   1000,
		ExternalPaidCents: 1030,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 1030, OrderNo: "LD260628UZJ97P"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusVerified, result.Status)
	assert.Empty(t, result.ErrorCode)
	assert.Empty(t, result.ErrorMessage)
}

func TestVerifyLdxpMailRejectsAmountMismatch(t *testing.T) {
	session := &model.LdxpTopupSession{
		SiteAmountCents:   1000,
		ExternalPaidCents: 1030,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 1000, OrderNo: "LD260628UZJ97P"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusAmountMismatch, result.Status)
	assert.Equal(t, "amount_mismatch", result.ErrorCode)
}

func TestVerifyLdxpMailRejectsOrderMismatch(t *testing.T) {
	session := &model.LdxpTopupSession{
		ExternalPaidCents: 1030,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 1030, OrderNo: "LD260628OTHER"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusOrderMismatch, result.Status)
	assert.Equal(t, "order_mismatch", result.ErrorCode)
}

func TestVerifyLdxpMailRejectsMissingData(t *testing.T) {
	result := VerifyLdxpMail(nil, &ParsedLdxpMail{PaidCents: 1030, OrderNo: "LD260628UZJ97P"})
	assert.Equal(t, model.MailCheckStatusMailParseFailed, result.Status)
	assert.Equal(t, "missing_data", result.ErrorCode)
	assert.NotEmpty(t, result.ErrorMessage)

	result = VerifyLdxpMail(&model.LdxpTopupSession{ExternalPaidCents: 1030, WorkerOrderNo: "LD260628UZJ97P"}, nil)
	assert.Equal(t, model.MailCheckStatusMailParseFailed, result.Status)
	assert.Equal(t, "missing_data", result.ErrorCode)
	assert.NotEmpty(t, result.ErrorMessage)
}
