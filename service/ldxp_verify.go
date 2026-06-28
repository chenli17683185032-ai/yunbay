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
	"gorm.io/gorm/clause"
)

const (
	ldxpRedeemSavePointName          = "ldxp_redeem"
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
	ldxpVerifyCodeDuplicateOrder     = "duplicate_order"
	ldxpVerifyCodeDuplicateCard      = "duplicate_card"
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
	return tryVerifyAndRedeemLdxpSession(sessionID, nil)
}

func tryVerifyAndRedeemLdxpSession(sessionID string, preferredEvent *model.LdxpMailEvent) (*LdxpVerifyResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, gorm.ErrInvalidData
	}

	ldxpVerifyMu.Lock()
	defer ldxpVerifyMu.Unlock()

	var result *LdxpVerifyResult
	var redeemResult *model.RedeemResult
	var redeemLogUserID int
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		session, err := getLdxpVerifySessionForUpdateTx(tx, sessionID)
		if err != nil {
			return err
		}
		if isLdxpSessionAlreadyRedeemed(session) {
			result = successLdxpVerifyResult(session)
			return nil
		}

		event, eventErr := getLdxpVerifyMailEventTx(tx, session, preferredEvent)
		if eventErr != nil && !errors.Is(eventErr, gorm.ErrRecordNotFound) {
			return eventErr
		}
		if errors.Is(eventErr, gorm.ErrRecordNotFound) && !shouldPersistLdxpVerifyMissingMail(session) {
			result = &LdxpVerifyResult{
				Verified:     false,
				Redeemed:     false,
				Status:       session.Status,
				ErrorCode:    ldxpVerifyCodePending,
				ErrorMessage: "waiting for matching ldxp mail event",
			}
			return nil
		}

		if err := VerifyLdxpSessionFields(session, event); err != nil {
			fieldErr := asLdxpVerifyFieldError(err)
			if updateErr := persistLdxpVerifyFailureTx(tx, session, event, model.LdxpStatusVerifyFailed, fieldErr.Code, fieldErr.Message); updateErr != nil {
				return updateErr
			}
			result = &LdxpVerifyResult{
				Verified:     false,
				Redeemed:     false,
				Status:       model.LdxpStatusVerifyFailed,
				ErrorCode:    fieldErr.Code,
				ErrorMessage: fieldErr.Message,
			}
			return nil
		}

		if duplicateErr := rejectDuplicateSuccessfulLdxpSessionTx(tx, session, event); duplicateErr != nil {
			fieldErr := asLdxpVerifyFieldError(duplicateErr)
			if updateErr := persistLdxpVerifyFailureTx(tx, session, event, model.LdxpStatusVerifyFailed, fieldErr.Code, fieldErr.Message); updateErr != nil {
				return updateErr
			}
			result = &LdxpVerifyResult{
				Verified:     false,
				Redeemed:     false,
				Status:       model.LdxpStatusVerifyFailed,
				ErrorCode:    fieldErr.Code,
				ErrorMessage: fieldErr.Message,
			}
			return nil
		}

		if err := persistLdxpVerifiedTx(tx, session, event); err != nil {
			return err
		}
		if err := beginLdxpRedeemSavePointTx(tx); err != nil {
			return err
		}
		redeem, err := RedeemLdxpSessionCardTx(tx, session)
		if err != nil {
			if rollbackErr := rollbackLdxpRedeemSavePointTx(tx); rollbackErr != nil {
				return rollbackErr
			}
			session.RedemptionId = 0
			session.TopupId = 0
			if errors.Is(err, model.ErrRedemptionUsed) {
				recovered, recoverErr := recoverLdxpSameUserUsedRedemptionTx(tx, session)
				if recoverErr != nil {
					return recoverErr
				}
				if recovered {
					if err := persistLdxpRedeemSuccessTx(tx, session); err != nil {
						return err
					}
					result = &LdxpVerifyResult{
						Verified: true,
						Redeemed: true,
						Status:   model.LdxpStatusSuccess,
					}
					return nil
				}
			}
			message := strings.TrimSpace(err.Error())
			if updateErr := persistLdxpRedeemFailureTx(tx, session, message); updateErr != nil {
				return updateErr
			}
			result = &LdxpVerifyResult{
				Verified:     true,
				Redeemed:     false,
				Status:       model.LdxpStatusRedeemFailed,
				ErrorCode:    ldxpVerifyCodeRedeemFailed,
				ErrorMessage: message,
			}
			return nil
		}
		if err := persistLdxpRedeemSuccessTx(tx, session); err != nil {
			return err
		}
		redeemResult = redeem
		redeemLogUserID = session.UserId
		result = &LdxpVerifyResult{
			Verified: true,
			Redeemed: true,
			Status:   model.LdxpStatusSuccess,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result != nil && result.Redeemed && redeemResult != nil {
		model.RecordRedeemLog(redeemLogUserID, redeemResult)
	}
	return result, nil
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

func beginLdxpRedeemSavePointTx(tx *gorm.DB) error {
	if tx == nil {
		return gorm.ErrInvalidData
	}
	return tx.SavePoint(ldxpRedeemSavePointName).Error
}

func rollbackLdxpRedeemSavePointTx(tx *gorm.DB) error {
	if tx == nil {
		return gorm.ErrInvalidData
	}
	return tx.RollbackTo(ldxpRedeemSavePointName).Error
}

func getLdxpVerifySessionForUpdateTx(tx *gorm.DB, sessionID string) (*model.LdxpTopupSession, error) {
	if tx == nil || strings.TrimSpace(sessionID) == "" {
		return nil, gorm.ErrInvalidData
	}
	var session model.LdxpTopupSession
	query := tx.Where("session_id = ?", strings.TrimSpace(sessionID))
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func RedeemLdxpSessionCardTx(tx *gorm.DB, session *model.LdxpTopupSession) (*model.RedeemResult, error) {
	if tx == nil || session == nil {
		return nil, gorm.ErrInvalidData
	}
	result, err := model.RedeemWithTx(tx, strings.TrimSpace(session.WorkerCardKey), session.UserId)
	if err != nil {
		return nil, err
	}
	if result != nil {
		session.RedemptionId = result.Redemption.Id
	}
	session.TopupId = 0
	return result, nil
}

func getLdxpVerifyMailEventTx(tx *gorm.DB, session *model.LdxpTopupSession, preferredEvent *model.LdxpMailEvent) (*model.LdxpMailEvent, error) {
	if tx == nil || session == nil {
		return nil, gorm.ErrInvalidData
	}
	if preferredEvent != nil {
		var event model.LdxpMailEvent
		if preferredEvent.Id > 0 {
			if err := tx.Where("id = ?", preferredEvent.Id).First(&event).Error; err != nil {
				return nil, err
			}
			return &event, nil
		}
		return preferredEvent, nil
	}
	if strings.TrimSpace(session.SessionId) != "" {
		var event model.LdxpMailEvent
		err := tx.
			Where(&model.LdxpMailEvent{MatchedSessionId: session.SessionId, Processed: true}).
			Order("id DESC").
			First(&event).Error
		if err == nil {
			return &event, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if messageID := strings.TrimSpace(session.MailMessageId); messageID != "" {
		var event model.LdxpMailEvent
		err := tx.
			Where(&model.LdxpMailEvent{MessageId: &messageID}).
			First(&event).Error
		if err == nil {
			return &event, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	workerOrderNo := strings.TrimSpace(session.WorkerOrderNo)
	if workerOrderNo == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if workerCardKey := strings.TrimSpace(session.WorkerCardKey); workerCardKey != "" {
		var event model.LdxpMailEvent
		err := tx.
			Where(&model.LdxpMailEvent{OrderNo: workerOrderNo, CardKey: workerCardKey}).
			Order("id DESC").
			First(&event).Error
		if err == nil {
			return &event, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var event model.LdxpMailEvent
	err := tx.Where("order_no = ?", workerOrderNo).First(&event).Error
	return &event, err
}

func rejectDuplicateSuccessfulLdxpSessionTx(tx *gorm.DB, session *model.LdxpTopupSession, event *model.LdxpMailEvent) error {
	if tx == nil || session == nil {
		return gorm.ErrInvalidData
	}
	orderValues := []string{session.WorkerOrderNo, session.MailOrderNo}
	cardValues := []string{session.WorkerCardKey, session.MailCardKey}
	if event != nil {
		orderValues = append(orderValues, event.OrderNo)
		cardValues = append(cardValues, event.CardKey)
	}
	for _, value := range orderValues {
		if err := rejectDuplicateSuccessfulLdxpValueTx(tx, session.Id, ldxpVerifyCodeDuplicateOrder, []string{"worker_order_no", "mail_order_no"}, strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	for _, value := range cardValues {
		if err := rejectDuplicateSuccessfulLdxpValueTx(tx, session.Id, ldxpVerifyCodeDuplicateCard, []string{"worker_card_key", "mail_card_key"}, strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return nil
}

func rejectDuplicateSuccessfulLdxpValueTx(tx *gorm.DB, currentID int, code string, columns []string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var duplicate model.LdxpTopupSession
	query := tx.Where("id <> ? AND status = ?", currentID, model.LdxpStatusSuccess)
	if len(columns) == 2 {
		query = query.Where(columns[0]+" = ? OR "+columns[1]+" = ?", value, value)
	} else if len(columns) == 1 {
		query = query.Where(columns[0]+" = ?", value)
	} else {
		return gorm.ErrInvalidData
	}
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&duplicate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if code == ldxpVerifyCodeDuplicateCard {
		return newLdxpVerifyFieldError(code, fmt.Sprintf("ldxp card key is already used by successful session %s", duplicate.SessionId))
	}
	return newLdxpVerifyFieldError(code, fmt.Sprintf("ldxp order number is already used by successful session %s", duplicate.SessionId))
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

func getLdxpVerifyMailEvent(session *model.LdxpTopupSession, preferredEvent *model.LdxpMailEvent) (*model.LdxpMailEvent, error) {
	if session == nil {
		return nil, gorm.ErrInvalidData
	}
	if preferredEvent != nil {
		return preferredEvent, nil
	}
	if strings.TrimSpace(session.SessionId) != "" {
		var event model.LdxpMailEvent
		err := model.DB.
			Where(&model.LdxpMailEvent{MatchedSessionId: session.SessionId, Processed: true}).
			Order("id DESC").
			First(&event).Error
		if err == nil {
			return &event, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if messageID := strings.TrimSpace(session.MailMessageId); messageID != "" {
		var event model.LdxpMailEvent
		err := model.DB.
			Where(&model.LdxpMailEvent{MessageId: &messageID}).
			First(&event).Error
		if err == nil {
			return &event, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	workerOrderNo := strings.TrimSpace(session.WorkerOrderNo)
	if workerOrderNo == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if workerCardKey := strings.TrimSpace(session.WorkerCardKey); workerCardKey != "" {
		var event model.LdxpMailEvent
		err := model.DB.
			Where(&model.LdxpMailEvent{OrderNo: workerOrderNo, CardKey: workerCardKey}).
			Order("id DESC").
			First(&event).Error
		if err == nil {
			return &event, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return model.GetLdxpMailEventByOrderNo(workerOrderNo)
}

func recoverLdxpSameUserUsedRedemptionTx(tx *gorm.DB, session *model.LdxpTopupSession) (bool, error) {
	if tx == nil || session == nil {
		return false, gorm.ErrInvalidData
	}
	key := strings.TrimSpace(session.WorkerCardKey)
	if key == "" {
		return false, nil
	}

	var redemption model.Redemption
	query := tx.Where(&model.Redemption{Key: key})
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&redemption).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if redemption.Status != common.RedemptionCodeStatusUsed || redemption.UsedUserId != session.UserId {
		return false, nil
	}
	if !ldxpRecoveredRedemptionHasTopUpTx(tx, &redemption, session.UserId) {
		return false, nil
	}

	session.RedemptionId = redemption.Id
	if redemption.RedeemedTime > 0 {
		session.RedeemedTime = redemption.RedeemedTime
	}
	return true, nil
}

func ldxpRecoveredRedemptionHasTopUpTx(tx *gorm.DB, redemption *model.Redemption, userID int) bool {
	if tx == nil || redemption == nil {
		return false
	}
	normalized := *redemption
	if normalized.Kind == "" {
		normalized.Kind = model.RedemptionKindLegacy
	}
	if normalized.Kind != model.RedemptionKindPaidTopUp || !normalized.CountAsTopUp {
		return true
	}
	var count int64
	err := tx.Model(&model.TopUp{}).
		Where("user_id = ? AND trade_no = ? AND status = ?", userID, model.CreateRedemptionTopUpTradeNo(normalized.Id, userID), common.TopUpStatusSuccess).
		Count(&count).Error
	return err == nil && count > 0
}

func recoverLdxpSameUserUsedRedemption(session *model.LdxpTopupSession) (bool, error) {
	return recoverLdxpSameUserUsedRedemptionTx(model.DB, session)
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
	return persistLdxpVerifiedWithDB(model.DB, session, event)
}

func persistLdxpRedeemSuccess(session *model.LdxpTopupSession) error {
	return persistLdxpRedeemSuccessWithDB(model.DB, session)
}

func persistLdxpVerifyFailure(session *model.LdxpTopupSession, event *model.LdxpMailEvent, status string, code string, message string) error {
	return persistLdxpVerifyFailureWithDB(model.DB, session, event, status, code, message)
}

func persistLdxpRedeemFailure(session *model.LdxpTopupSession, message string) error {
	return persistLdxpRedeemFailureWithDB(model.DB, session, message)
}

func persistLdxpVerifiedTx(tx *gorm.DB, session *model.LdxpTopupSession, event *model.LdxpMailEvent) error {
	return persistLdxpVerifiedWithDB(tx, session, event)
}

func persistLdxpVerifiedWithDB(db *gorm.DB, session *model.LdxpTopupSession, event *model.LdxpMailEvent) error {
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
	if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
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

func persistLdxpRedeemSuccessTx(tx *gorm.DB, session *model.LdxpTopupSession) error {
	return persistLdxpRedeemSuccessWithDB(tx, session)
}

func persistLdxpRedeemSuccessWithDB(db *gorm.DB, session *model.LdxpTopupSession) error {
	now := common.GetTimestamp()
	verifiedTime := session.VerifiedTime
	if verifiedTime == 0 {
		verifiedTime = now
	}
	redeemedTime := session.RedeemedTime
	if redeemedTime == 0 {
		redeemedTime = now
	}
	updates := map[string]interface{}{
		"status":        model.LdxpStatusSuccess,
		"verified_time": verifiedTime,
		"redeemed_time": redeemedTime,
		"redemption_id": session.RedemptionId,
		"topup_id":      session.TopupId,
		"error_code":    "",
		"error_message": "",
		"updated_time":  now,
	}
	if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
		return err
	}
	session.Status = model.LdxpStatusSuccess
	session.VerifiedTime = verifiedTime
	session.RedeemedTime = redeemedTime
	session.ErrorCode = ""
	session.ErrorMessage = ""
	session.UpdatedTime = now
	return nil
}

func persistLdxpVerifyFailureTx(tx *gorm.DB, session *model.LdxpTopupSession, event *model.LdxpMailEvent, status string, code string, message string) error {
	return persistLdxpVerifyFailureWithDB(tx, session, event, status, code, message)
}

func persistLdxpVerifyFailureWithDB(db *gorm.DB, session *model.LdxpTopupSession, event *model.LdxpMailEvent, status string, code string, message string) error {
	now := common.GetTimestamp()
	updates := ldxpMailFieldsFromEvent(session, event)
	updates["status"] = status
	updates["error_code"] = strings.TrimSpace(code)
	updates["error_message"] = strings.TrimSpace(message)
	updates["updated_time"] = now
	if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
		return err
	}
	session.Status = status
	session.ErrorCode = strings.TrimSpace(code)
	session.ErrorMessage = strings.TrimSpace(message)
	session.UpdatedTime = now
	applyLdxpMailEventToSession(session, event)
	return nil
}

func persistLdxpRedeemFailureTx(tx *gorm.DB, session *model.LdxpTopupSession, message string) error {
	return persistLdxpRedeemFailureWithDB(tx, session, message)
}

func persistLdxpRedeemFailureWithDB(db *gorm.DB, session *model.LdxpTopupSession, message string) error {
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
	if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
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
