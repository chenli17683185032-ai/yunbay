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
