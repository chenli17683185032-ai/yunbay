package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	ldxpVerifyCodePending            = "pending"
	ldxpVerifyCodeMissingWorkerOrder = "missing_worker_order"
	ldxpVerifyCodeMailEventNotFound  = "mail_event_not_found"
	ldxpVerifyCodeOrderMismatch      = "order_mismatch"
	ldxpVerifyCodeMailOrderMismatch  = "mail_order_mismatch"
	ldxpVerifyCodeMissingCard        = "missing_card"
	ldxpVerifyCodeCardMismatch       = "card_mismatch"
	ldxpVerifyCodeAmountMismatch     = "amount_mismatch"
	ldxpVerifyCodeStatusNotPaid      = "status_not_paid"
	ldxpVerifyCodeRedeemFailed       = "redeem_failed"
)

var ldxpVerifyMu sync.Mutex

type LdxpVerifyResult struct {
	Verified     bool
	Redeemed     bool
	Status       string
	ErrorCode    string
	ErrorMessage string
}

type ldxpVerifyFieldError struct {
	Code    string
	Message string
}

func (err *ldxpVerifyFieldError) Error() string {
	if err == nil {
		return ""
	}
	if err.Message == "" {
		return err.Code
	}
	return err.Message
}

func TryVerifyAndRedeemLdxpSession(sessionID string) (*LdxpVerifyResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, gorm.ErrInvalidData
	}

	ldxpVerifyMu.Lock()
	defer ldxpVerifyMu.Unlock()

	session, err := model.GetLdxpTopupSessionBySessionId(sessionID)
	if err != nil {
		return nil, err
	}
	if isLdxpSessionAlreadyRedeemed(session) {
		return successLdxpVerifyResult(session), nil
	}

	event, eventErr := getLdxpVerifyMailEvent(session)
	if eventErr != nil && !errors.Is(eventErr, gorm.ErrRecordNotFound) {
		return nil, eventErr
	}
	if errors.Is(eventErr, gorm.ErrRecordNotFound) && !shouldPersistLdxpVerifyMissingMail(session) {
		return &LdxpVerifyResult{
			Verified:     false,
			Redeemed:     false,
			Status:       session.Status,
			ErrorCode:    ldxpVerifyCodePending,
			ErrorMessage: "waiting for matching ldxp mail event",
		}, nil
	}

	if err := VerifyLdxpSessionFields(session, event); err != nil {
		fieldErr := asLdxpVerifyFieldError(err)
		if updateErr := persistLdxpVerifyFailure(session, event, model.LdxpStatusVerifyFailed, fieldErr.Code, fieldErr.Message); updateErr != nil {
			return nil, updateErr
		}
		return &LdxpVerifyResult{
			Verified:     false,
			Redeemed:     false,
			Status:       model.LdxpStatusVerifyFailed,
			ErrorCode:    fieldErr.Code,
			ErrorMessage: fieldErr.Message,
		}, nil
	}

	if err := persistLdxpVerified(session, event); err != nil {
		return nil, err
	}
	if err := RedeemLdxpSessionCard(session); err != nil {
		message := strings.TrimSpace(err.Error())
		if updateErr := persistLdxpRedeemFailure(session, message); updateErr != nil {
			return nil, updateErr
		}
		return &LdxpVerifyResult{
			Verified:     true,
			Redeemed:     false,
			Status:       model.LdxpStatusRedeemFailed,
			ErrorCode:    ldxpVerifyCodeRedeemFailed,
			ErrorMessage: message,
		}, nil
	}
	if err := persistLdxpRedeemSuccess(session); err != nil {
		return nil, err
	}
	return &LdxpVerifyResult{
		Verified: true,
		Redeemed: true,
		Status:   model.LdxpStatusSuccess,
	}, nil
}

func VerifyLdxpSessionFields(session *model.LdxpTopupSession, event *model.LdxpMailEvent) error {
	if session == nil {
		return gorm.ErrInvalidData
	}
	workerOrderNo := strings.TrimSpace(session.WorkerOrderNo)
	if workerOrderNo == "" {
		return newLdxpVerifyFieldError(ldxpVerifyCodeMissingWorkerOrder, "ldxp worker order number is missing")
	}
	if event == nil {
		return newLdxpVerifyFieldError(ldxpVerifyCodeMailEventNotFound, "matching ldxp mail event was not found")
	}
	eventOrderNo := strings.TrimSpace(event.OrderNo)
	if eventOrderNo == "" {
		return newLdxpVerifyFieldError(ldxpVerifyCodeMailEventNotFound, "matching ldxp mail event has no order number")
	}
	if workerOrderNo != eventOrderNo {
		return newLdxpVerifyFieldError(ldxpVerifyCodeOrderMismatch, fmt.Sprintf("worker order %s does not match mail event order %s", workerOrderNo, eventOrderNo))
	}
	mailOrderNo := strings.TrimSpace(session.MailOrderNo)
	if mailOrderNo != "" && mailOrderNo != workerOrderNo {
		return newLdxpVerifyFieldError(ldxpVerifyCodeMailOrderMismatch, fmt.Sprintf("attached mail order %s does not match worker order %s", mailOrderNo, workerOrderNo))
	}

	workerCardKey := strings.TrimSpace(session.WorkerCardKey)
	eventCardKey := strings.TrimSpace(event.CardKey)
	if workerCardKey == "" || eventCardKey == "" {
		return newLdxpVerifyFieldError(ldxpVerifyCodeMissingCard, "ldxp card key is missing")
	}
	if workerCardKey != eventCardKey {
		return newLdxpVerifyFieldError(ldxpVerifyCodeCardMismatch, "worker card key does not match mail event card key")
	}
	mailCardKey := strings.TrimSpace(session.MailCardKey)
	if mailCardKey != "" && mailCardKey != workerCardKey {
		return newLdxpVerifyFieldError(ldxpVerifyCodeCardMismatch, "attached mail card key does not match worker card key")
	}

	if session.Money > 0 && event.Amount > 0 && math.Abs(session.Money-event.Amount) > 0.01 {
		return newLdxpVerifyFieldError(ldxpVerifyCodeAmountMismatch, fmt.Sprintf("session money %.2f does not match mail event amount %.2f", session.Money, event.Amount))
	}
	if session.Money > 0 && session.MailAmount > 0 && math.Abs(session.Money-session.MailAmount) > 0.01 {
		return newLdxpVerifyFieldError(ldxpVerifyCodeAmountMismatch, fmt.Sprintf("session money %.2f does not match attached mail amount %.2f", session.Money, session.MailAmount))
	}

	workerStatusText := strings.TrimSpace(session.WorkerStatusText)
	if !strings.Contains(workerStatusText, "已付款") && !strings.Contains(workerStatusText, "成功") {
		return newLdxpVerifyFieldError(ldxpVerifyCodeStatusNotPaid, "ldxp worker status is not paid or successful")
	}
	return nil
}

func RedeemLdxpSessionCard(session *model.LdxpTopupSession) error {
	if session == nil {
		return gorm.ErrInvalidData
	}
	result, err := model.Redeem(strings.TrimSpace(session.WorkerCardKey), session.UserId)
	if err != nil {
		return err
	}
	if result != nil {
		session.RedemptionId = result.Redemption.Id
	}
	session.TopupId = 0
	return nil
}

func getLdxpVerifyMailEvent(session *model.LdxpTopupSession) (*model.LdxpMailEvent, error) {
	if session == nil {
		return nil, gorm.ErrInvalidData
	}
	workerOrderNo := strings.TrimSpace(session.WorkerOrderNo)
	if workerOrderNo == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return model.GetLdxpMailEventByOrderNo(workerOrderNo)
}

func shouldPersistLdxpVerifyMissingMail(session *model.LdxpTopupSession) bool {
	if session == nil {
		return false
	}
	return hasLdxpMailAttachment(session)
}

func isLdxpSessionAlreadyRedeemed(session *model.LdxpTopupSession) bool {
	if session == nil {
		return false
	}
	if session.Status == model.LdxpStatusSuccess {
		return true
	}
	if (session.Status == model.LdxpStatusVerified || session.Status == model.LdxpStatusRedeemed) && (session.RedemptionId > 0 || session.TopupId > 0) {
		return true
	}
	return false
}

func successLdxpVerifyResult(session *model.LdxpTopupSession) *LdxpVerifyResult {
	status := model.LdxpStatusSuccess
	if session != nil && session.Status == model.LdxpStatusSuccess {
		status = session.Status
	}
	return &LdxpVerifyResult{
		Verified: true,
		Redeemed: true,
		Status:   status,
	}
}

func persistLdxpVerified(session *model.LdxpTopupSession, event *model.LdxpMailEvent) error {
	now := common.GetTimestamp()
	verifiedTime := session.VerifiedTime
	if verifiedTime == 0 {
		verifiedTime = now
	}
	updates := ldxpMailFieldsFromEvent(session, event)
	updates["status"] = model.LdxpStatusVerified
	updates["verified_time"] = verifiedTime
	updates["error_code"] = ""
	updates["error_message"] = ""
	updates["updated_time"] = now
	if err := model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
		return err
	}
	session.Status = model.LdxpStatusVerified
	session.VerifiedTime = verifiedTime
	session.ErrorCode = ""
	session.ErrorMessage = ""
	session.UpdatedTime = now
	applyLdxpMailEventToSession(session, event)
	return nil
}

func persistLdxpRedeemSuccess(session *model.LdxpTopupSession) error {
	now := common.GetTimestamp()
	verifiedTime := session.VerifiedTime
	if verifiedTime == 0 {
		verifiedTime = now
	}
	updates := map[string]interface{}{
		"status":        model.LdxpStatusSuccess,
		"verified_time": verifiedTime,
		"redeemed_time": now,
		"redemption_id": session.RedemptionId,
		"topup_id":      session.TopupId,
		"error_code":    "",
		"error_message": "",
		"updated_time":  now,
	}
	if err := model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
		return err
	}
	session.Status = model.LdxpStatusSuccess
	session.VerifiedTime = verifiedTime
	session.RedeemedTime = now
	session.ErrorCode = ""
	session.ErrorMessage = ""
	session.UpdatedTime = now
	return nil
}

func persistLdxpVerifyFailure(session *model.LdxpTopupSession, event *model.LdxpMailEvent, status string, code string, message string) error {
	now := common.GetTimestamp()
	updates := ldxpMailFieldsFromEvent(session, event)
	updates["status"] = status
	updates["error_code"] = strings.TrimSpace(code)
	updates["error_message"] = strings.TrimSpace(message)
	updates["updated_time"] = now
	if err := model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
		return err
	}
	session.Status = status
	session.ErrorCode = strings.TrimSpace(code)
	session.ErrorMessage = strings.TrimSpace(message)
	session.UpdatedTime = now
	applyLdxpMailEventToSession(session, event)
	return nil
}

func persistLdxpRedeemFailure(session *model.LdxpTopupSession, message string) error {
	now := common.GetTimestamp()
	verifiedTime := session.VerifiedTime
	if verifiedTime == 0 {
		verifiedTime = now
	}
	updates := map[string]interface{}{
		"status":        model.LdxpStatusRedeemFailed,
		"verified_time": verifiedTime,
		"error_code":    ldxpVerifyCodeRedeemFailed,
		"error_message": strings.TrimSpace(message),
		"updated_time":  now,
	}
	if err := model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
		return err
	}
	session.Status = model.LdxpStatusRedeemFailed
	session.VerifiedTime = verifiedTime
	session.ErrorCode = ldxpVerifyCodeRedeemFailed
	session.ErrorMessage = strings.TrimSpace(message)
	session.UpdatedTime = now
	return nil
}

func ldxpMailFieldsFromEvent(session *model.LdxpTopupSession, event *model.LdxpMailEvent) map[string]interface{} {
	updates := make(map[string]interface{})
	if session == nil || event == nil {
		return updates
	}
	if strings.TrimSpace(session.MailOrderNo) == "" {
		updates["mail_order_no"] = strings.TrimSpace(event.OrderNo)
	}
	if session.MailAmount <= 0 && event.Amount > 0 {
		updates["mail_amount"] = event.Amount
	}
	if strings.TrimSpace(session.MailProductName) == "" {
		updates["mail_product_name"] = strings.TrimSpace(event.ProductName)
	}
	if strings.TrimSpace(session.MailCardKey) == "" {
		updates["mail_card_key"] = strings.TrimSpace(event.CardKey)
	}
	if strings.TrimSpace(session.MailFrom) == "" {
		updates["mail_from"] = strings.TrimSpace(event.MailFrom)
	}
	if strings.TrimSpace(session.MailTo) == "" {
		updates["mail_to"] = strings.TrimSpace(event.MailTo)
	}
	if strings.TrimSpace(session.MailSubject) == "" {
		updates["mail_subject"] = strings.TrimSpace(event.Subject)
	}
	if session.MailReceivedTime == 0 && event.ReceivedTime > 0 {
		updates["mail_received_time"] = event.ReceivedTime
	}
	if strings.TrimSpace(session.MailMessageId) == "" && event.MessageId != nil {
		updates["mail_message_id"] = strings.TrimSpace(*event.MessageId)
	}
	return updates
}

func applyLdxpMailEventToSession(session *model.LdxpTopupSession, event *model.LdxpMailEvent) {
	if session == nil || event == nil {
		return
	}
	if strings.TrimSpace(session.MailOrderNo) == "" {
		session.MailOrderNo = strings.TrimSpace(event.OrderNo)
	}
	if session.MailAmount <= 0 && event.Amount > 0 {
		session.MailAmount = event.Amount
	}
	if strings.TrimSpace(session.MailProductName) == "" {
		session.MailProductName = strings.TrimSpace(event.ProductName)
	}
	if strings.TrimSpace(session.MailCardKey) == "" {
		session.MailCardKey = strings.TrimSpace(event.CardKey)
	}
	if strings.TrimSpace(session.MailFrom) == "" {
		session.MailFrom = strings.TrimSpace(event.MailFrom)
	}
	if strings.TrimSpace(session.MailTo) == "" {
		session.MailTo = strings.TrimSpace(event.MailTo)
	}
	if strings.TrimSpace(session.MailSubject) == "" {
		session.MailSubject = strings.TrimSpace(event.Subject)
	}
	if session.MailReceivedTime == 0 && event.ReceivedTime > 0 {
		session.MailReceivedTime = event.ReceivedTime
	}
	if strings.TrimSpace(session.MailMessageId) == "" && event.MessageId != nil {
		session.MailMessageId = strings.TrimSpace(*event.MessageId)
	}
}

func newLdxpVerifyFieldError(code string, message string) error {
	return &ldxpVerifyFieldError{Code: code, Message: message}
}

func asLdxpVerifyFieldError(err error) *ldxpVerifyFieldError {
	var fieldErr *ldxpVerifyFieldError
	if errors.As(err, &fieldErr) {
		return fieldErr
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &ldxpVerifyFieldError{Code: "verify_failed", Message: message}
}
