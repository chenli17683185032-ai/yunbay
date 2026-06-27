package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const defaultLdxpPollIntervalMs = 2000

var (
	ErrLdxpTopupDisabled         = errors.New("ldxp topup disabled")
	ErrLdxpUnsupportedAmount     = errors.New("unsupported ldxp topup amount")
	ErrLdxpSessionNotCancelable  = errors.New("ldxp session is not cancelable")
	ErrLdxpInvalidSessionRequest = errors.New("invalid ldxp session request")
)

type LdxpCreateSessionRequest struct {
	Amount int64 `json:"amount"`
}

type LdxpSessionPublicView struct {
	SessionID      string  `json:"session_id"`
	Amount         int64   `json:"amount"`
	Money          float64 `json:"money"`
	Status         string  `json:"status"`
	QRCode         string  `json:"qr_code,omitempty"`
	WorkerOrderNo  string  `json:"worker_order_no,omitempty"`
	ExpiresAt      int64   `json:"expires_at"`
	PollIntervalMs int     `json:"poll_interval_ms"`
	ErrorCode      string  `json:"error_code,omitempty"`
	ErrorMessage   string  `json:"error_message,omitempty"`
}

type LdxpWorkerQrPayload struct {
	WorkerID          string  `json:"worker_id"`
	WorkerOrderNo     string  `json:"worker_order_no"`
	WorkerAmount      float64 `json:"worker_amount"`
	WorkerProductName string  `json:"worker_product_name"`
	QRCode            string  `json:"qr_code"`
	QRPageURL         string  `json:"qr_page_url"`
}

type LdxpWorkerResultPayload struct {
	WorkerID          string  `json:"worker_id"`
	WorkerOrderNo     string  `json:"worker_order_no"`
	WorkerAmount      float64 `json:"worker_amount"`
	WorkerProductName string  `json:"worker_product_name"`
	WorkerCardKey     string  `json:"worker_card_key"`
	WorkerStatusText  string  `json:"worker_status_text"`
	WorkerSuccessURL  string  `json:"worker_success_url"`
}

func CreateLdxpTopupSession(userID int, amount int64, cfg *LdxpConfig) (*LdxpSessionPublicView, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: missing config", ErrLdxpInvalidSessionRequest)
	}
	if !cfg.Enabled {
		return nil, ErrLdxpTopupDisabled
	}
	if userID <= 0 {
		return nil, fmt.Errorf("%w: invalid user", ErrLdxpInvalidSessionRequest)
	}
	product, ok := cfg.Products[amount]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrLdxpUnsupportedAmount, amount)
	}

	now := common.GetTimestamp()
	sessionID, err := generateLdxpSessionID()
	if err != nil {
		return nil, err
	}
	session := &model.LdxpTopupSession{
		SessionId:    sessionID,
		UserId:       userID,
		Amount:       amount,
		Money:        product.Money,
		ProductUrl:   product.ProductURL,
		ProductName:  product.ProductName,
		ContactEmail: cfg.ContactEmail,
		Status:       model.LdxpStatusCreated,
		CreatedTime:  now,
		UpdatedTime:  now,
		ExpiredTime:  now + cfg.SessionTTLSeconds,
	}
	if err := model.InsertLdxpTopupSession(session); err != nil {
		return nil, err
	}
	return publicLdxpSessionView(session), nil
}

func GetLdxpSessionPublicView(sessionID string, userID int) (*LdxpSessionPublicView, error) {
	session, err := model.GetLdxpTopupSessionForUser(sessionID, userID)
	if err != nil {
		return nil, err
	}
	return publicLdxpSessionView(session), nil
}

func CancelLdxpTopupSession(sessionID string, userID int) error {
	session, err := model.GetLdxpTopupSessionForUser(sessionID, userID)
	if err != nil {
		return err
	}
	if !isLdxpCancelableStatus(session.Status) {
		return fmt.Errorf("%w: %s", ErrLdxpSessionNotCancelable, session.Status)
	}
	now := common.GetTimestamp()
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("id = ? AND user_id = ? AND status IN ?", session.Id, userID, []string{model.LdxpStatusCreated, model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady}).
		Updates(map[string]interface{}{
			"status":       model.LdxpStatusCanceled,
			"updated_time": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ClaimLdxpTopupSession(workerID string, cfg *LdxpConfig) (*model.LdxpTopupSession, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	if cfg == nil {
		return nil, fmt.Errorf("%w: missing config", ErrLdxpInvalidSessionRequest)
	}
	if !cfg.Enabled {
		return nil, ErrLdxpTopupDisabled
	}
	now := common.GetTimestamp()
	if _, err := model.MarkExpiredLdxpTopupSessions(now); err != nil {
		return nil, err
	}
	return model.ClaimNextLdxpTopupSession(workerID, now)
}

func RecordLdxpQr(sessionID string, payload LdxpWorkerQrPayload) error {
	workerID := strings.TrimSpace(payload.WorkerID)
	if workerID == "" {
		return fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	now := common.GetTimestamp()
	updates := map[string]interface{}{
		"status":              model.LdxpStatusQrReady,
		"worker_amount":       payload.WorkerAmount,
		"worker_product_name": strings.TrimSpace(payload.WorkerProductName),
		"qr_code":             strings.TrimSpace(payload.QRCode),
		"qr_page_url":         strings.TrimSpace(payload.QRPageURL),
		"qr_ready_time":       now,
		"updated_time":        now,
	}
	if workerOrderNo := strings.TrimSpace(payload.WorkerOrderNo); workerOrderNo != "" {
		updates["worker_order_no"] = workerOrderNo
	}
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status IN ?", sessionID, workerID, []string{model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady}).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func RecordLdxpWorkerResult(sessionID string, payload LdxpWorkerResultPayload) (*model.LdxpTopupSession, error) {
	workerID := strings.TrimSpace(payload.WorkerID)
	if workerID == "" {
		return nil, fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	workerOrderNo := strings.TrimSpace(payload.WorkerOrderNo)
	if workerOrderNo != "" {
		existing, err := model.GetLdxpTopupSessionBySessionId(sessionID)
		if err != nil {
			return nil, err
		}
		if existing.WorkerOrderNo != "" && existing.WorkerOrderNo != workerOrderNo {
			return nil, gorm.ErrRecordNotFound
		}
	}
	now := common.GetTimestamp()
	updates := map[string]interface{}{
		"status":               model.LdxpStatusWorkerPaid,
		"worker_amount":        payload.WorkerAmount,
		"worker_product_name":  strings.TrimSpace(payload.WorkerProductName),
		"worker_card_key":      strings.TrimSpace(payload.WorkerCardKey),
		"worker_status_text":   strings.TrimSpace(payload.WorkerStatusText),
		"worker_success_url":   strings.TrimSpace(payload.WorkerSuccessURL),
		"worker_detected_time": now,
		"updated_time":         now,
	}
	if workerOrderNo != "" {
		updates["worker_order_no"] = workerOrderNo
	}
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status IN ?", sessionID, workerID, []string{model.LdxpStatusQrReady}).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return model.GetLdxpTopupSessionBySessionId(sessionID)
}

func RecordLdxpWorkerError(sessionID string, workerID string, code string, message string, snapshotPath string) error {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	now := common.GetTimestamp()
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status IN ?", sessionID, workerID, []string{model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady}).
		Updates(map[string]interface{}{
			"status":              model.LdxpStatusWorkerFailed,
			"error_code":          strings.TrimSpace(code),
			"error_message":       strings.TrimSpace(message),
			"debug_snapshot_path": strings.TrimSpace(snapshotPath),
			"updated_time":        now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func generateLdxpSessionID() (string, error) {
	key, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return "", err
	}
	return "ldxp_" + strings.ToLower(key), nil
}

func publicLdxpSessionView(session *model.LdxpTopupSession) *LdxpSessionPublicView {
	if session == nil {
		return nil
	}
	view := &LdxpSessionPublicView{
		SessionID:      session.SessionId,
		Amount:         session.Amount,
		Money:          session.Money,
		Status:         session.Status,
		WorkerOrderNo:  session.WorkerOrderNo,
		ExpiresAt:      session.ExpiredTime,
		PollIntervalMs: defaultLdxpPollIntervalMs,
		ErrorCode:      session.ErrorCode,
		ErrorMessage:   session.ErrorMessage,
	}
	if session.Status == model.LdxpStatusQrReady {
		view.QRCode = session.QrCode
	}
	return view
}

func isLdxpCancelableStatus(status string) bool {
	switch status {
	case model.LdxpStatusCreated, model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady:
		return true
	default:
		return false
	}
}

func terminalLdxpStatuses() []string {
	return []string{
		model.LdxpStatusWorkerPaid,
		model.LdxpStatusVerified,
		model.LdxpStatusRedeemed,
		model.LdxpStatusSuccess,
		model.LdxpStatusCanceled,
		model.LdxpStatusExpired,
		model.LdxpStatusWorkerFailed,
		model.LdxpStatusMailTimeout,
		model.LdxpStatusVerifyFailed,
		model.LdxpStatusRedeemFailed,
	}
}
