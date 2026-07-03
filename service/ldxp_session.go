package service

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultLdxpPollIntervalMs = 2000

const (
	MaxLdxpQRCodeLength             = 512 * 1024
	MaxLdxpWorkerCardKeyLength      = 255
	MaxLdxpWorkerErrorMessageLength = 2048
	MaxLdxpWorkerSnapshotPathLength = 512
	ldxpPublicWorkerFailedMessage   = "Worker failed, please contact support"
)

var ldxpSafeQRDataPrefixes = []string{
	"data:image/png;base64,",
	"data:image/jpeg;base64,",
	"data:image/jpg;base64,",
	"data:image/webp;base64,",
	"data:image/gif;base64,",
}

var (
	ErrLdxpTopupDisabled         = errors.New("ldxp topup disabled")
	ErrLdxpUnsupportedAmount     = errors.New("unsupported ldxp topup amount")
	ErrLdxpSessionNotCancelable  = errors.New("ldxp session is not cancelable")
	ErrLdxpInvalidSessionRequest = errors.New("invalid ldxp session request")
)

var ldxpCreateSessionMu sync.Mutex

type LdxpCreateSessionRequest struct {
	Amount int64 `json:"amount"`
}

type LdxpCreateValuePackageSessionRequest struct {
	PlanId         int  `json:"plan_id"`
	ConfirmedCover bool `json:"confirmed_cover"`
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

type LdxpWorkerSessionState struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Active    bool   `json:"active"`
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
	ldxpCreateSessionMu.Lock()
	defer ldxpCreateSessionMu.Unlock()

	var session *model.LdxpTopupSession
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockLdxpUserRowIfPossible(tx, userID); err != nil {
			return err
		}

		if activeSession, err := findActiveLdxpTopupSessionForUserTx(tx, userID, now); err == nil {
			session = activeSession
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		sessionID, err := generateLdxpSessionID()
		if err != nil {
			return err
		}
		session = &model.LdxpTopupSession{
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
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return publicLdxpSessionView(session), nil
}

func CreateLdxpValuePackageSession(userID int, planID int, confirmedCover bool, cfg *LdxpConfig) (*LdxpSessionPublicView, *model.SubscriptionOrder, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("%w: missing config", ErrLdxpInvalidSessionRequest)
	}
	if !cfg.Enabled {
		return nil, nil, ErrLdxpTopupDisabled
	}
	if userID <= 0 || planID <= 0 {
		return nil, nil, fmt.Errorf("%w: invalid value package request", ErrLdxpInvalidSessionRequest)
	}
	plan, err := model.GetSubscriptionPlanById(planID)
	if err != nil {
		return nil, nil, err
	}
	if !plan.IsValuePackage() {
		return nil, nil, fmt.Errorf("%w: not value package", ErrLdxpInvalidSessionRequest)
	}
	if strings.TrimSpace(plan.LdxpProductUrl) == "" || strings.TrimSpace(plan.LdxpProductName) == "" || plan.LdxpProductAmount <= 0 {
		return nil, nil, fmt.Errorf("%w: ldxp product incomplete", ErrLdxpInvalidSessionRequest)
	}
	if _, err := model.CheckValuePackagePurchaseIntent(userID, planID, confirmedCover); err != nil {
		return nil, nil, err
	}

	now := common.GetTimestamp()
	ldxpCreateSessionMu.Lock()
	defer ldxpCreateSessionMu.Unlock()

	var session *model.LdxpTopupSession
	var order *model.SubscriptionOrder
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockLdxpUserRowIfPossible(tx, userID); err != nil {
			return err
		}
		if activeSession, err := findActiveLdxpValuePackageSessionForUserTx(tx, userID, now); err == nil {
			session = activeSession
			var existingOrder model.SubscriptionOrder
			if activeSession.SubscriptionOrderId > 0 {
				_ = tx.Where("id = ?", activeSession.SubscriptionOrderId).First(&existingOrder).Error
				order = &existingOrder
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		tradeNo := fmt.Sprintf("LDXP_VP-%d-%d-%s", userID, time.Now().UnixMilli(), common.GetRandomString(6))
		order = &model.SubscriptionOrder{UserId: userID, PlanId: plan.Id, Money: plan.LdxpProductAmount, TradeNo: tradeNo, PaymentMethod: model.PaymentMethodLDXP, PaymentProvider: model.PaymentProviderLDXP, CreateTime: now, Status: common.TopUpStatusPending}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		sessionID, err := generateLdxpSessionID()
		if err != nil {
			return err
		}
		ttl := cfg.SessionTTLSeconds
		if plan.LdxpSessionTTLSeconds > 0 {
			ttl = plan.LdxpSessionTTLSeconds
		}
		session = &model.LdxpTopupSession{SessionId: sessionID, UserId: userID, Amount: 0, Money: plan.LdxpProductAmount, ProductUrl: plan.LdxpProductUrl, ProductName: plan.LdxpProductName, ContactEmail: cfg.ContactEmail, Status: model.LdxpStatusCreated, Purpose: model.LdxpPurposeValuePackage, SubscriptionOrderId: order.Id, SubscriptionPlanId: plan.Id, ConfirmedCover: confirmedCover, CreatedTime: now, UpdatedTime: now, ExpiredTime: now + ttl}
		return tx.Create(session).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return publicLdxpSessionView(session), order, nil
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

func GetLdxpWorkerSessionState(sessionID string, workerID string) (*LdxpWorkerSessionState, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	session, err := model.GetLdxpTopupSessionBySessionId(sessionID)
	if err != nil {
		return nil, err
	}
	return &LdxpWorkerSessionState{
		SessionID: session.SessionId,
		Status:    session.Status,
		Active:    isLdxpWorkerSessionActiveForWorker(session, workerID, common.GetTimestamp()),
	}, nil
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

func ClaimLdxpPaidWatchSession(workerID string, cfg *LdxpConfig) (*model.LdxpTopupSession, error) {
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
	return model.ClaimNextLdxpPaidWatchSession(workerID, now)
}

func RecordLdxpQr(sessionID string, payload LdxpWorkerQrPayload) error {
	workerID := strings.TrimSpace(payload.WorkerID)
	if workerID == "" {
		return fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	workerOrderNo := strings.TrimSpace(payload.WorkerOrderNo)
	if workerOrderNo == "" {
		return fmt.Errorf("%w: missing worker order no", ErrLdxpInvalidSessionRequest)
	}
	if err := validateLdxpStringMax("qr_code", payload.QRCode, MaxLdxpQRCodeLength); err != nil {
		return err
	}
	qrCode := strings.TrimSpace(payload.QRCode)
	if !isSafeLdxpQRCodeSource(qrCode) {
		return fmt.Errorf("%w: invalid qr code source", ErrLdxpInvalidSessionRequest)
	}
	now := common.GetTimestamp()
	updates := map[string]interface{}{
		"status":              model.LdxpStatusQrReady,
		"worker_amount":       payload.WorkerAmount,
		"worker_product_name": strings.TrimSpace(payload.WorkerProductName),
		"qr_code":             qrCode,
		"qr_page_url":         strings.TrimSpace(payload.QRPageURL),
		"worker_order_no":     workerOrderNo,
		"qr_ready_time":       now,
		"updated_time":        now,
	}
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status IN ? AND (worker_order_no = ? OR worker_order_no = '' OR worker_order_no IS NULL)", sessionID, workerID, []string{model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady}, workerOrderNo).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		if isLdxpSessionAlreadyCanceledByWorker(sessionID, workerID) {
			return nil
		}
		return gorm.ErrRecordNotFound
	}
	return nil
}

func isSafeLdxpQRCodeSource(qrCode string) bool {
	if qrCode == "" {
		return false
	}
	for _, prefix := range ldxpSafeQRDataPrefixes {
		if strings.HasPrefix(qrCode, prefix) {
			return true
		}
	}
	return strings.HasPrefix(qrCode, "https://")
}

func RecordLdxpWorkerResult(sessionID string, payload LdxpWorkerResultPayload) (*model.LdxpTopupSession, error) {
	workerID := strings.TrimSpace(payload.WorkerID)
	if workerID == "" {
		return nil, fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	workerOrderNo := strings.TrimSpace(payload.WorkerOrderNo)
	if workerOrderNo == "" {
		return nil, fmt.Errorf("%w: missing worker order no", ErrLdxpInvalidSessionRequest)
	}
	if err := validateLdxpStringMax("worker_card_key", payload.WorkerCardKey, MaxLdxpWorkerCardKeyLength); err != nil {
		return nil, err
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
	updates["worker_order_no"] = workerOrderNo
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status IN ? AND worker_order_no = ?", sessionID, workerID, []string{model.LdxpStatusQrReady}, workerOrderNo).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	if _, err := TryVerifyAndRedeemLdxpSession(sessionID); err != nil {
		return nil, err
	}
	return model.GetLdxpTopupSessionBySessionId(sessionID)
}

func RecordLdxpWorkerError(sessionID string, workerID string, code string, message string, snapshotPath string) error {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return fmt.Errorf("%w: missing worker", ErrLdxpInvalidSessionRequest)
	}
	if err := validateLdxpStringMax("error_message", message, MaxLdxpWorkerErrorMessageLength); err != nil {
		return err
	}
	if err := validateLdxpStringMax("snapshot_path", snapshotPath, MaxLdxpWorkerSnapshotPathLength); err != nil {
		return err
	}
	now := common.GetTimestamp()
	result := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status IN ?", sessionID, workerID, []string{model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady}).
		Updates(map[string]interface{}{
			"status":              model.LdxpStatusWorkerFailed,
			"error_code":          sanitizeLdxpWorkerErrorCode(code),
			"error_message":       PublicLdxpWorkerFailedMessage(),
			"debug_snapshot_path": sanitizeLdxpSnapshotPath(snapshotPath),
			"updated_time":        now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		if isLdxpSessionAlreadyCanceledByWorker(sessionID, workerID) {
			return nil
		}
		return gorm.ErrRecordNotFound
	}
	return nil
}

func isLdxpSessionAlreadyCanceledByWorker(sessionID string, workerID string) bool {
	var count int64
	err := model.DB.Model(&model.LdxpTopupSession{}).
		Where("session_id = ? AND worker_id = ? AND status = ?", sessionID, workerID, model.LdxpStatusCanceled).
		Count(&count).Error
	return err == nil && count > 0
}

func validateLdxpStringMax(field string, value string, max int) error {
	if max <= 0 {
		return nil
	}
	if len(strings.TrimSpace(value)) > max {
		return fmt.Errorf("%w: %s too large", ErrLdxpInvalidSessionRequest, field)
	}
	return nil
}

func sanitizeLdxpWorkerErrorCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) > 64 {
		return code[:64]
	}
	return code
}

func PublicLdxpWorkerFailedMessage() string {
	return ldxpPublicWorkerFailedMessage
}

func publicLdxpErrorMessage(session *model.LdxpTopupSession) string {
	if session == nil {
		return ""
	}
	if session.Status == model.LdxpStatusWorkerFailed && strings.TrimSpace(session.ErrorMessage) != "" {
		return PublicLdxpWorkerFailedMessage()
	}
	return session.ErrorMessage
}

func sanitizeLdxpSnapshotPath(snapshotPath string) string {
	snapshotPath = strings.TrimSpace(snapshotPath)
	if snapshotPath == "" {
		return ""
	}
	normalized := strings.ReplaceAll(snapshotPath, "\\", "/")
	base := path.Base(normalized)
	if base == "." || base == "/" {
		return ""
	}
	if len(base) > MaxLdxpWorkerSnapshotPathLength {
		base = base[:MaxLdxpWorkerSnapshotPathLength]
	}
	return base
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
		ErrorMessage:   publicLdxpErrorMessage(session),
	}
	if session.Status == model.LdxpStatusQrReady {
		view.QRCode = session.QrCode
	}
	return view
}

func isLdxpWorkerSessionActiveForWorker(session *model.LdxpTopupSession, workerID string, now int64) bool {
	if session == nil || strings.TrimSpace(workerID) == "" {
		return false
	}
	if session.WorkerId != strings.TrimSpace(workerID) {
		return false
	}
	if session.ExpiredTime <= now {
		return false
	}
	switch session.Status {
	case model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady:
		return true
	default:
		return false
	}
}

func isLdxpCancelableStatus(status string) bool {
	switch status {
	case model.LdxpStatusCreated, model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady:
		return true
	default:
		return false
	}
}

func activeLdxpSessionStatuses() []string {
	return []string{
		model.LdxpStatusCreated,
		model.LdxpStatusWorkerClaimed,
		model.LdxpStatusQrReady,
		model.LdxpStatusWorkerPaid,
		model.LdxpStatusVerified,
		model.LdxpStatusRedeemed,
	}
}

func findActiveLdxpTopupSessionForUser(userID int, now int64) (*model.LdxpTopupSession, error) {
	var session model.LdxpTopupSession
	err := model.DB.
		Where("user_id = ? AND status IN ? AND expired_time > ? AND (purpose = ? OR purpose = '' OR purpose IS NULL)", userID, activeLdxpSessionStatuses(), now, model.LdxpPurposeTopup).
		Order("created_time ASC, id ASC").
		First(&session).Error
	return &session, err
}

func findActiveLdxpTopupSessionForUserTx(tx *gorm.DB, userID int, now int64) (*model.LdxpTopupSession, error) {
	var session model.LdxpTopupSession
	query := tx.
		Where("user_id = ? AND status IN ? AND expired_time > ? AND (purpose = ? OR purpose = '' OR purpose IS NULL)", userID, activeLdxpSessionStatuses(), now, model.LdxpPurposeTopup).
		Order("created_time ASC, id ASC")
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&session).Error
	return &session, err
}

func findActiveLdxpValuePackageSessionForUserTx(tx *gorm.DB, userID int, now int64) (*model.LdxpTopupSession, error) {
	var session model.LdxpTopupSession
	query := tx.
		Where("user_id = ? AND status IN ? AND expired_time > ? AND purpose = ?", userID, activeLdxpSessionStatuses(), now, model.LdxpPurposeValuePackage).
		Order("created_time ASC, id ASC")
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&session).Error
	return &session, err
}

func lockLdxpUserRowIfPossible(tx *gorm.DB, userID int) error {
	if tx == nil {
		return gorm.ErrInvalidData
	}
	var user model.User
	query := tx.Model(&model.User{}).Where("id = ?", userID)
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: user not found", ErrLdxpInvalidSessionRequest)
	}
	return err
}
