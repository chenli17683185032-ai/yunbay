package service

import "github.com/QuantumNous/new-api/model"

type MailVerificationResult struct {
	Status       string
	ErrorCode    string
	ErrorMessage string
}

func VerifyLdxpMail(session *model.LdxpTopupSession, mail *ParsedLdxpMail) MailVerificationResult {
	if session == nil || mail == nil {
		return MailVerificationResult{
			Status:       model.MailCheckStatusMailParseFailed,
			ErrorCode:    "missing_data",
			ErrorMessage: "session or parsed mail is missing",
		}
	}

	if session.WorkerOrderNo == "" || mail.OrderNo == "" || session.WorkerOrderNo != mail.OrderNo {
		return MailVerificationResult{
			Status:       model.MailCheckStatusOrderMismatch,
			ErrorCode:    "order_mismatch",
			ErrorMessage: "worker order number does not match mail order number",
		}
	}

	if session.ExternalPaidCents != mail.PaidCents {
		return MailVerificationResult{
			Status:       model.MailCheckStatusAmountMismatch,
			ErrorCode:    "amount_mismatch",
			ErrorMessage: "external paid amount does not match mail paid amount",
		}
	}

	return MailVerificationResult{Status: model.MailCheckStatusVerified}
}
