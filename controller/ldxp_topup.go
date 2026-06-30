package controller

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ldxpWorkerClaimRequest struct {
	WorkerID string `json:"worker_id"`
}

type ldxpWorkerErrorRequest struct {
	WorkerID     string `json:"worker_id"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	SnapshotPath string `json:"snapshot_path"`
}

type ldxpWorkerMailEventRequest struct {
	MessageID    string  `json:"message_id"`
	ImapUID      string  `json:"imap_uid"`
	RawHash      string  `json:"raw_hash"`
	From         string  `json:"from"`
	To           string  `json:"to"`
	Subject      string  `json:"subject"`
	ReceivedTime int64   `json:"received_time"`
	OrderNo      string  `json:"order_no"`
	Amount       float64 `json:"amount"`
	ProductName  string  `json:"product_name"`
	CardKey      string  `json:"card_key"`
	PaidTime     int64   `json:"paid_time"`
	BodyExcerpt  string  `json:"body_excerpt"`
}

type ldxpWorkerClaimResponse struct {
	SessionID    string  `json:"session_id"`
	Amount       int64   `json:"amount"`
	Money        float64 `json:"money"`
	ProductURL   string  `json:"product_url"`
	ProductName  string  `json:"product_name"`
	ContactEmail string  `json:"contact_email"`
	ExpiresAt    int64   `json:"expires_at"`
}

type ldxpWorkerPaidWatchResponse struct {
	SessionID         string  `json:"session_id"`
	Amount            int64   `json:"amount"`
	Money             float64 `json:"money"`
	WorkerOrderNo     string  `json:"worker_order_no"`
	WorkerAmount      float64 `json:"worker_amount"`
	WorkerProductName string  `json:"worker_product_name"`
	QRPageURL         string  `json:"qr_page_url"`
	ExpiresAt         int64   `json:"expires_at"`
}

type ldxpSessionStatusResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

type ldxpWorkerSessionActiveResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Active    bool   `json:"active"`
}

type ldxpAdminSessionResponse struct {
	Id                 int     `json:"id"`
	SessionID          string  `json:"session_id"`
	UserID             int     `json:"user_id"`
	Amount             int64   `json:"amount"`
	Money              float64 `json:"money"`
	ProductURL         string  `json:"product_url"`
	ProductName        string  `json:"product_name"`
	ContactEmail       string  `json:"contact_email"`
	Status             string  `json:"status"`
	WorkerID           string  `json:"worker_id"`
	QrPageURL          string  `json:"qr_page_url"`
	QrReadyTime        int64   `json:"qr_ready_time"`
	WorkerOrderNo      string  `json:"worker_order_no"`
	WorkerAmount       float64 `json:"worker_amount"`
	WorkerProductName  string  `json:"worker_product_name"`
	WorkerCardKey      string  `json:"worker_card_key"`
	WorkerStatusText   string  `json:"worker_status_text"`
	WorkerSuccessURL   string  `json:"worker_success_url"`
	WorkerDetectedTime int64   `json:"worker_detected_time"`
	MailMessageID      string  `json:"mail_message_id"`
	MailOrderNo        string  `json:"mail_order_no"`
	MailAmount         float64 `json:"mail_amount"`
	MailProductName    string  `json:"mail_product_name"`
	MailCardKey        string  `json:"mail_card_key"`
	MailFrom           string  `json:"mail_from"`
	MailTo             string  `json:"mail_to"`
	MailSubject        string  `json:"mail_subject"`
	MailReceivedTime   int64   `json:"mail_received_time"`
	VerifiedTime       int64   `json:"verified_time"`
	RedeemedTime       int64   `json:"redeemed_time"`
	TopupID            int     `json:"topup_id"`
	RedemptionID       int     `json:"redemption_id"`
	ErrorCode          string  `json:"error_code"`
	ErrorMessage       string  `json:"error_message"`
	CreatedTime        int64   `json:"created_time"`
	UpdatedTime        int64   `json:"updated_time"`
	ExpiredTime        int64   `json:"expired_time"`
}

func CreateLdxpTopupSession(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		common.ApiErrorMsg(c, "未登录")
		return
	}

	var req service.LdxpCreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	cfg, err := service.LoadLdxpConfig()
	if err != nil {
		common.ApiErrorMsg(c, "ldxp topup unavailable")
		return
	}
	if !service.IsLdxpUserIDAllowed(cfg, userID) {
		common.ApiError(c, service.ErrLdxpTopupDisabled)
		return
	}
	view, err := service.CreateLdxpTopupSession(userID, req.Amount, cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func GetLdxpTopupSession(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		common.ApiErrorMsg(c, "未登录")
		return
	}

	view, err := service.GetLdxpSessionPublicView(c.Param("session_id"), userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func CancelLdxpTopupSession(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		common.ApiErrorMsg(c, "未登录")
		return
	}

	sessionID := c.Param("session_id")
	if err := service.CancelLdxpTopupSession(sessionID, userID); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.GetLdxpSessionPublicView(sessionID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func WorkerClaimLdxpTopupSession(c *gin.Context) {
	cfg, ok := requireLdxpWorkerToken(c)
	if !ok {
		return
	}

	var req ldxpWorkerClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	session, err := service.ClaimLdxpTopupSession(req.WorkerID, cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildLdxpWorkerClaimResponse(session))
}

func WorkerClaimLdxpPaidWatchSession(c *gin.Context) {
	cfg, ok := requireLdxpWorkerToken(c)
	if !ok {
		return
	}

	var req ldxpWorkerClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	session, err := service.ClaimLdxpPaidWatchSession(req.WorkerID, cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildLdxpWorkerPaidWatchResponse(session))
}

func WorkerGetLdxpSessionActive(c *gin.Context) {
	if _, ok := requireLdxpWorkerToken(c); !ok {
		return
	}

	var req ldxpWorkerClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	state, err := service.GetLdxpWorkerSessionState(c.Param("session_id"), req.WorkerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiSuccess(c, &ldxpWorkerSessionActiveResponse{
				SessionID: c.Param("session_id"),
				Active:    false,
			})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &ldxpWorkerSessionActiveResponse{
		SessionID: state.SessionID,
		Status:    state.Status,
		Active:    state.Active,
	})
}

func WorkerRecordLdxpQr(c *gin.Context) {
	if _, ok := requireLdxpWorkerToken(c); !ok {
		return
	}

	var req service.LdxpWorkerQrPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	sessionID := c.Param("session_id")
	if err := service.RecordLdxpQr(sessionID, req); err != nil {
		common.ApiError(c, err)
		return
	}
	session, err := model.GetLdxpTopupSessionBySessionId(sessionID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildLdxpSessionStatusResponse(session))
}

func WorkerRecordLdxpResult(c *gin.Context) {
	if _, ok := requireLdxpWorkerToken(c); !ok {
		return
	}

	var req service.LdxpWorkerResultPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	session, err := service.RecordLdxpWorkerResult(c.Param("session_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildLdxpSessionStatusResponse(session))
}

func WorkerRecordLdxpError(c *gin.Context) {
	if _, ok := requireLdxpWorkerToken(c); !ok {
		return
	}

	var req ldxpWorkerErrorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	err := service.RecordLdxpWorkerError(c.Param("session_id"), req.WorkerID, req.ErrorCode, req.ErrorMessage, req.SnapshotPath)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	session, err := model.GetLdxpTopupSessionBySessionId(c.Param("session_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildLdxpSessionStatusResponse(session))
}

func WorkerRecordLdxpMailEvent(c *gin.Context) {
	if _, ok := requireLdxpWorkerToken(c); !ok {
		return
	}

	var req ldxpWorkerMailEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	parsed := &service.LdxpParsedMail{
		MessageID:    req.MessageID,
		ImapUID:      req.ImapUID,
		RawHash:      req.RawHash,
		From:         req.From,
		To:           req.To,
		Subject:      req.Subject,
		ReceivedTime: req.ReceivedTime,
		OrderNo:      req.OrderNo,
		Amount:       req.Amount,
		ProductName:  req.ProductName,
		CardKey:      req.CardKey,
		PaidTime:     req.PaidTime,
		BodyExcerpt:  req.BodyExcerpt,
	}
	event, err := service.SaveLdxpMailEvent(parsed)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	matched, matchErr := service.TryMatchLdxpMailEvent(event)
	data := gin.H{
		"event_id": event.Id,
		"matched":  matched != nil,
	}
	if matched != nil {
		data["session"] = buildLdxpSessionStatusResponse(matched)
	}
	if matchErr != nil && !errors.Is(matchErr, gorm.ErrRecordNotFound) {
		common.ApiErrorMsg(c, "ldxp mail match failed")
		return
	}
	common.ApiSuccess(c, data)
}

func AdminListLdxpTopupSessions(c *gin.Context) {
	page, size := getLdxpAdminPage(c)
	query := model.DB.Model(&model.LdxpTopupSession{})
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if sessionID := strings.TrimSpace(c.Query("session_id")); sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	if userID, err := strconv.Atoi(strings.TrimSpace(c.Query("user_id"))); err == nil && userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var sessions []model.LdxpTopupSession
	if err := query.Order("id desc").Limit(size).Offset((page - 1) * size).Find(&sessions).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]ldxpAdminSessionResponse, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, buildLdxpAdminSessionResponse(session))
	}
	common.ApiSuccess(c, gin.H{
		"page":      page,
		"page_size": size,
		"total":     total,
		"items":     items,
	})
}

func AdminRetryLdxpTopupVerify(c *gin.Context) {
	result, err := service.TryVerifyAndRedeemLdxpSession(c.Param("session_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func requireLdxpWorkerToken(c *gin.Context) (*service.LdxpConfig, bool) {
	cfg, err := service.LoadLdxpConfig()
	if err != nil {
		common.ApiErrorMsg(c, "worker auth unavailable")
		return nil, false
	}
	if !isValidLdxpWorkerToken(c.GetHeader("X-LDXP-Worker-Token"), cfg.WorkerToken) {
		common.ApiErrorMsg(c, "unauthorized")
		return nil, false
	}
	return cfg, true
}

func isValidLdxpWorkerToken(provided string, expected string) bool {
	provided = strings.TrimSpace(provided)
	expected = strings.TrimSpace(expected)
	if provided == "" || expected == "" {
		return false
	}
	providedHash := sha256.Sum256([]byte(provided))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1
}

func buildLdxpWorkerClaimResponse(session *model.LdxpTopupSession) *ldxpWorkerClaimResponse {
	if session == nil {
		return nil
	}
	return &ldxpWorkerClaimResponse{
		SessionID:    session.SessionId,
		Amount:       session.Amount,
		Money:        session.Money,
		ProductURL:   session.ProductUrl,
		ProductName:  session.ProductName,
		ContactEmail: session.ContactEmail,
		ExpiresAt:    session.ExpiredTime,
	}
}

func buildLdxpWorkerPaidWatchResponse(session *model.LdxpTopupSession) *ldxpWorkerPaidWatchResponse {
	if session == nil {
		return nil
	}
	return &ldxpWorkerPaidWatchResponse{
		SessionID:         session.SessionId,
		Amount:            session.Amount,
		Money:             session.Money,
		WorkerOrderNo:     session.WorkerOrderNo,
		WorkerAmount:      session.WorkerAmount,
		WorkerProductName: session.WorkerProductName,
		QRPageURL:         session.QrPageUrl,
		ExpiresAt:         session.ExpiredTime,
	}
}

func buildLdxpSessionStatusResponse(session *model.LdxpTopupSession) *ldxpSessionStatusResponse {
	if session == nil {
		return nil
	}
	return &ldxpSessionStatusResponse{SessionID: session.SessionId, Status: session.Status}
}

func getLdxpAdminPage(c *gin.Context) (int, int) {
	page := parsePositiveInt(c.Query("page"), 0)
	if page == 0 {
		page = parsePositiveInt(c.Query("p"), 1)
	}
	size := parsePositiveInt(c.Query("limit"), 0)
	if size == 0 {
		size = parsePositiveInt(c.Query("size"), 0)
	}
	if size == 0 {
		size = parsePositiveInt(c.Query("page_size"), common.ItemsPerPage)
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func buildLdxpAdminSessionResponse(session model.LdxpTopupSession) ldxpAdminSessionResponse {
	return ldxpAdminSessionResponse{
		Id:                 session.Id,
		SessionID:          session.SessionId,
		UserID:             session.UserId,
		Amount:             session.Amount,
		Money:              session.Money,
		ProductURL:         session.ProductUrl,
		ProductName:        session.ProductName,
		ContactEmail:       session.ContactEmail,
		Status:             session.Status,
		WorkerID:           session.WorkerId,
		QrPageURL:          session.QrPageUrl,
		QrReadyTime:        session.QrReadyTime,
		WorkerOrderNo:      session.WorkerOrderNo,
		WorkerAmount:       session.WorkerAmount,
		WorkerProductName:  session.WorkerProductName,
		WorkerCardKey:      service.RedactLdxpValue(session.WorkerCardKey),
		WorkerStatusText:   session.WorkerStatusText,
		WorkerSuccessURL:   session.WorkerSuccessUrl,
		WorkerDetectedTime: session.WorkerDetectedTime,
		MailMessageID:      session.MailMessageId,
		MailOrderNo:        session.MailOrderNo,
		MailAmount:         session.MailAmount,
		MailProductName:    session.MailProductName,
		MailCardKey:        service.RedactLdxpValue(session.MailCardKey),
		MailFrom:           session.MailFrom,
		MailTo:             session.MailTo,
		MailSubject:        session.MailSubject,
		MailReceivedTime:   session.MailReceivedTime,
		VerifiedTime:       session.VerifiedTime,
		RedeemedTime:       session.RedeemedTime,
		TopupID:            session.TopupId,
		RedemptionID:       session.RedemptionId,
		ErrorCode:          session.ErrorCode,
		ErrorMessage:       session.ErrorMessage,
		CreatedTime:        session.CreatedTime,
		UpdatedTime:        session.UpdatedTime,
		ExpiredTime:        session.ExpiredTime,
	}
}
