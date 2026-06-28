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

type ldxpSessionStatusResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
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
		common.ApiError(c, err)
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
		data["match_error"] = matchErr.Error()
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
	items := make([]model.LdxpTopupSession, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, redactLdxpAdminSession(session))
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
		common.ApiError(c, err)
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

func redactLdxpAdminSession(session model.LdxpTopupSession) model.LdxpTopupSession {
	session.WorkerCardKey = service.RedactLdxpValue(session.WorkerCardKey)
	session.MailCardKey = service.RedactLdxpValue(session.MailCardKey)
	return session
}
