package model

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	MailCheckStatusNotRequired     = "not_required"
	MailCheckStatusPending         = "pending"
	MailCheckStatusWaitingMail     = "waiting_mail"
	MailCheckStatusChecking        = "checking"
	MailCheckStatusVerified        = "verified"
	MailCheckStatusOrderMismatch   = "order_mismatch"
	MailCheckStatusAmountMismatch  = "amount_mismatch"
	MailCheckStatusMailParseFailed = "mail_parse_failed"
	MailCheckStatusMailFetchFailed = "mail_fetch_failed"
	MailCheckStatusTimeout         = "timeout"
)

const (
	AffiliateCommissionStatusPending   = "pending"
	AffiliateCommissionStatusAvailable = "available"
	AffiliateCommissionStatusWithdrawn = "withdrawn"
	AffiliateCommissionStatusRejected  = "rejected"
)

const (
	AffiliateWithdrawalStatusPending  = "pending"
	AffiliateWithdrawalStatusPaid     = "paid"
	AffiliateWithdrawalStatusRejected = "rejected"
)

type LdxpTopupSession struct {
	Id                int    `json:"id"`
	SessionId         string `json:"session_id" gorm:"uniqueIndex;type:varchar(64)"`
	UserId            int    `json:"user_id" gorm:"index"`
	TopUpId           int    `json:"topup_id" gorm:"index;default:0"`
	TradeNo           string `json:"trade_no" gorm:"type:varchar(255);index"`
	SiteAmountCents   int64  `json:"site_amount_cents" gorm:"type:bigint;not null;default:0"`
	ExternalPaidCents int64  `json:"external_paid_cents" gorm:"type:bigint;not null;default:0"`
	WorkerOrderNo     string `json:"worker_order_no" gorm:"type:varchar(64);index"`
	WorkerAmountCents int64  `json:"worker_amount_cents" gorm:"type:bigint;not null;default:0"`
	MailOrderNo       string `json:"mail_order_no" gorm:"type:varchar(64);index"`
	MailAmountCents   int64  `json:"mail_amount_cents" gorm:"type:bigint;not null;default:0"`
	MailStatus        string `json:"mail_status" gorm:"type:varchar(32);index;default:'pending'"`
	MailEventId       int    `json:"mail_event_id" gorm:"index;default:0"`
	ErrorCode         string `json:"error_code" gorm:"type:varchar(64);default:''"`
	ErrorMessage      string `json:"error_message" gorm:"type:varchar(512);default:''"`
	CreatedTime       int64  `json:"created_time" gorm:"index"`
	PaidTime          int64  `json:"paid_time" gorm:"index;default:0"`
	VerifiedTime      int64  `json:"verified_time" gorm:"index;default:0"`
	UpdatedTime       int64  `json:"updated_time" gorm:"autoUpdateTime"`
}

type LdxpMailEvent struct {
	Id            int    `json:"id"`
	SourceAccount string `json:"source_account" gorm:"type:varchar(128);index"`
	MessageId     string `json:"message_id" gorm:"type:varchar(255);index"`
	ImapUid       string `json:"imap_uid" gorm:"type:varchar(64);index"`
	RawHash       string `json:"raw_hash" gorm:"type:char(64);uniqueIndex"`
	Subject       string `json:"subject" gorm:"type:varchar(255);default:''"`
	FromAddress   string `json:"from_address" gorm:"type:varchar(255);default:''"`
	ProductName   string `json:"product_name" gorm:"type:varchar(255);default:''"`
	OrderNo       string `json:"order_no" gorm:"type:varchar(64);index"`
	PaidCents     int64  `json:"paid_cents" gorm:"type:bigint;not null;default:0"`
	Quantity      int    `json:"quantity" gorm:"type:int;not null;default:0"`
	PaymentTime   int64  `json:"payment_time" gorm:"index;default:0"`
	ContentMasked string `json:"content_masked" gorm:"type:text"`
	ParseStatus   string `json:"parse_status" gorm:"type:varchar(32);index;default:'parsed'"`
	ParseError    string `json:"parse_error" gorm:"type:varchar(512);default:''"`
	CreatedTime   int64  `json:"created_time" gorm:"index"`
}

type AffiliateCommission struct {
	Id              int    `json:"id"`
	InviterUserId   int    `json:"inviter_user_id" gorm:"index"`
	InviteeUserId   int    `json:"invitee_user_id" gorm:"index"`
	TopUpId         int    `json:"topup_id" gorm:"index;default:0"`
	SessionId       string `json:"session_id" gorm:"type:varchar(64);index"`
	TradeNo         string `json:"trade_no" gorm:"type:varchar(255);index"`
	BaseMoneyCents  int64  `json:"base_money_cents" gorm:"type:bigint;not null;default:0"`
	RateBps         int    `json:"rate_bps" gorm:"type:int;not null;default:0"`
	CommissionCents int64  `json:"commission_cents" gorm:"type:bigint;not null;default:0"`
	Status          string `json:"status" gorm:"type:varchar(32);index;default:'available'"`
	CreatedTime     int64  `json:"created_time" gorm:"index"`
	ConfirmedTime   int64  `json:"confirmed_time" gorm:"index;default:0"`
	// WithdrawalId links to affiliate_withdrawals.id as an internal numeric ID.
	WithdrawalId int `json:"withdrawal_id" gorm:"index;default:0"`
}

type AffiliateWithdrawal struct {
	Id int `json:"id"`
	// WithdrawalId is the withdrawal request business number for external display.
	WithdrawalId  string `json:"withdrawal_id" gorm:"uniqueIndex;type:varchar(64)"`
	UserId        int    `json:"user_id" gorm:"index"`
	AmountCents   int64  `json:"amount_cents" gorm:"type:bigint;not null;default:0"`
	Contact       string `json:"contact" gorm:"type:varchar(255);not null"`
	Remark        string `json:"remark" gorm:"type:varchar(512);default:''"`
	Status        string `json:"status" gorm:"type:varchar(32);index;default:'pending'"`
	AdminRemark   string `json:"admin_remark" gorm:"type:varchar(512);default:''"`
	CreatedTime   int64  `json:"created_time" gorm:"index"`
	ProcessedTime int64  `json:"processed_time" gorm:"index;default:0"`
	ProcessedBy   int    `json:"processed_by" gorm:"index;default:0"`
}

var (
	ErrAffiliateWithdrawalNotFound         = errors.New("affiliate withdrawal not found")
	ErrAffiliateWithdrawalAlreadyProcessed = errors.New("affiliate withdrawal already processed")
)

func MarkAffiliateWithdrawalPaid(id int, adminId int, adminRemark string) (*AffiliateWithdrawal, error) {
	return transitionAffiliateWithdrawal(id, adminId, adminRemark, AffiliateWithdrawalStatusPaid)
}

func RejectAffiliateWithdrawal(id int, adminId int, adminRemark string) (*AffiliateWithdrawal, error) {
	return transitionAffiliateWithdrawal(id, adminId, adminRemark, AffiliateWithdrawalStatusRejected)
}

func transitionAffiliateWithdrawal(id int, adminId int, adminRemark string, status string) (*AffiliateWithdrawal, error) {
	if id <= 0 {
		return nil, ErrAffiliateWithdrawalNotFound
	}

	var withdrawal AffiliateWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		processedTime := common.GetTimestamp()
		result := tx.Model(&AffiliateWithdrawal{}).
			Where("id = ? AND status = ?", id, AffiliateWithdrawalStatusPending).
			Updates(map[string]interface{}{
				"status":         status,
				"processed_by":   adminId,
				"processed_time": processedTime,
				"admin_remark":   strings.TrimSpace(adminRemark),
			})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			if err := tx.First(&withdrawal, id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAffiliateWithdrawalNotFound
				}
				return err
			}
			return ErrAffiliateWithdrawalAlreadyProcessed
		}

		return tx.First(&withdrawal, id).Error
	})
	if err != nil {
		return nil, err
	}

	return &withdrawal, nil
}

type OrderAnalyticsSummary struct {
	SiteAmountCents        int64
	ExternalPaidCents      int64
	OrderCount             int
	MailVerifiedCount      int
	MailPendingCount       int
	MailErrorCount         int
	MailVerifiedRate       float64
	AffiliateUserCount     int
	AffiliateAmountCents   int64
	PendingWithdrawalCount int
	PendingWithdrawalCents int64
}

type OrderAnalyticsDaily struct {
	Date              string
	SiteAmountCents   int64
	ExternalPaidCents int64
	OrderCount        int
	MailVerifiedCount int
	MailErrorCount    int
}

type OrderAnalyticsResult struct {
	Summary OrderAnalyticsSummary
	Daily   []OrderAnalyticsDaily
}

type AffiliateStatsSummary struct {
	AffiliateUserCount                  int
	PeriodCommissionCents               int64
	PendingWithdrawalUserCount          int
	PendingWithdrawalCents              int64
	AvailableWithoutWithdrawalUserCount int
}

type AffiliateWithdrawalInfo struct {
	Id            int
	WithdrawalId  string
	AmountCents   int64
	Contact       string
	Remark        string
	Status        string
	CreatedTime   int64
	AdminRemark   string
	ProcessedTime int64
}

type AffiliateStatsItem struct {
	UserId                int
	Username              string
	PeriodCommissionCents int64
	TotalCommissionCents  int64
	AvailableCents        int64
	WithdrawnCents        int64
	Withdrawal            *AffiliateWithdrawalInfo
}

type AffiliateStatsResult struct {
	Summary AffiliateStatsSummary
	Items   []AffiliateStatsItem
	Total   int64
}

type AffiliateSourceOrderRow struct {
	OrderTime       int64
	InviteeUserId   int
	InviteeUsername string
	TradeNo         string
	WorkerOrderNo   string
	BaseMoneyCents  int64
	RateBps         int
	CommissionCents int64
	MailStatus      string
}

type OrderManagementOrderRow struct {
	Id                       int
	SessionId                string
	UserId                   int
	Username                 string
	SiteAmountCents          int64
	ExternalPaidCents        int64
	WorkerOrderNo            string
	MailOrderNo              string
	MailAmountCents          int64
	MailStatus               string
	ErrorCode                string
	ErrorMessage             string
	CreatedTime              int64
	VerifiedTime             int64
	AffiliateInviterId       int
	AffiliateCommissionCents int64
	AffiliateStatus          string
}

func GetOrderManagementAnalytics(startTime, endTime int64) (*OrderAnalyticsResult, error) {
	var sessions []LdxpTopupSession
	if err := DB.Select("id", "site_amount_cents", "external_paid_cents", "mail_status", "created_time").
		Where("created_time >= ? AND created_time <= ?", startTime, endTime).
		Order("created_time ASC, id ASC").Find(&sessions).Error; err != nil {
		return nil, err
	}

	result := &OrderAnalyticsResult{}
	dailyByDate := make(map[string]*OrderAnalyticsDaily)
	for _, session := range sessions {
		result.Summary.SiteAmountCents += session.SiteAmountCents
		result.Summary.ExternalPaidCents += session.ExternalPaidCents
		result.Summary.OrderCount++

		date := time.Unix(session.CreatedTime, 0).UTC().Format("2006-01-02")
		daily := dailyByDate[date]
		if daily == nil {
			daily = &OrderAnalyticsDaily{Date: date}
			dailyByDate[date] = daily
		}
		daily.SiteAmountCents += session.SiteAmountCents
		daily.ExternalPaidCents += session.ExternalPaidCents
		daily.OrderCount++

		switch {
		case session.MailStatus == MailCheckStatusVerified:
			result.Summary.MailVerifiedCount++
			daily.MailVerifiedCount++
		case isOrderManagementMailErrorStatus(session.MailStatus):
			result.Summary.MailErrorCount++
			daily.MailErrorCount++
		default:
			result.Summary.MailPendingCount++
		}
	}
	if result.Summary.OrderCount > 0 {
		result.Summary.MailVerifiedRate = float64(result.Summary.MailVerifiedCount) / float64(result.Summary.OrderCount)
	}

	result.Daily = make([]OrderAnalyticsDaily, 0, len(dailyByDate))
	for _, daily := range dailyByDate {
		result.Daily = append(result.Daily, *daily)
	}
	sort.Slice(result.Daily, func(i, j int) bool { return result.Daily[i].Date < result.Daily[j].Date })

	var commissions []AffiliateCommission
	if err := DB.Select("inviter_user_id", "commission_cents", "status", "created_time").
		Where("created_time >= ? AND created_time <= ?", startTime, endTime).Find(&commissions).Error; err != nil {
		return nil, err
	}
	inviterSet := make(map[int]struct{})
	for _, commission := range commissions {
		if commission.InviterUserId > 0 {
			inviterSet[commission.InviterUserId] = struct{}{}
		}
		if shouldCountAffiliateCommissionAmount(commission.Status) {
			result.Summary.AffiliateAmountCents += commission.CommissionCents
		}
	}
	result.Summary.AffiliateUserCount = len(inviterSet)

	var withdrawals []AffiliateWithdrawal
	if err := DB.Select("amount_cents").
		Where("created_time >= ? AND created_time <= ? AND status = ?", startTime, endTime, AffiliateWithdrawalStatusPending).Find(&withdrawals).Error; err != nil {
		return nil, err
	}
	result.Summary.PendingWithdrawalCount = len(withdrawals)
	for _, withdrawal := range withdrawals {
		result.Summary.PendingWithdrawalCents += withdrawal.AmountCents
	}

	return result, nil
}

func GetAffiliateStats(startTime, endTime int64, withdrawalStatus string, offset int, limit int) (*AffiliateStatsResult, error) {
	offset, limit = normalizeOffsetLimit(offset, limit, 20, 100)
	result := &AffiliateStatsResult{}
	summaryUserSet := make(map[int]struct{})
	itemUserSet := make(map[int]struct{})
	periodByUser := make(map[int]int64)
	totalByUser := make(map[int]int64)
	availableByUser := make(map[int]int64)
	withdrawnByUser := make(map[int]int64)

	var periodCommissions []AffiliateCommission
	if err := DB.Select("inviter_user_id", "commission_cents", "status", "created_time").
		Where("created_time >= ? AND created_time <= ?", startTime, endTime).Find(&periodCommissions).Error; err != nil {
		return nil, err
	}
	for _, commission := range periodCommissions {
		if commission.InviterUserId <= 0 {
			continue
		}
		summaryUserSet[commission.InviterUserId] = struct{}{}
		if withdrawalStatus == "" {
			itemUserSet[commission.InviterUserId] = struct{}{}
		}
		if !shouldCountAffiliateCommissionAmount(commission.Status) {
			continue
		}
		periodByUser[commission.InviterUserId] += commission.CommissionCents
		result.Summary.PeriodCommissionCents += commission.CommissionCents
	}

	var allCommissions []AffiliateCommission
	if err := DB.Select("inviter_user_id", "commission_cents", "status").Find(&allCommissions).Error; err != nil {
		return nil, err
	}
	for _, commission := range allCommissions {
		if commission.InviterUserId <= 0 {
			continue
		}
		summaryUserSet[commission.InviterUserId] = struct{}{}
		if withdrawalStatus == "" {
			itemUserSet[commission.InviterUserId] = struct{}{}
		}
		if !shouldCountAffiliateCommissionAmount(commission.Status) {
			continue
		}
		totalByUser[commission.InviterUserId] += commission.CommissionCents
		switch commission.Status {
		case AffiliateCommissionStatusAvailable:
			availableByUser[commission.InviterUserId] += commission.CommissionCents
		case AffiliateCommissionStatusWithdrawn:
			withdrawnByUser[commission.InviterUserId] += commission.CommissionCents
		}
	}

	withdrawalQuery := DB.Select("id", "withdrawal_id", "user_id", "amount_cents", "contact", "remark", "status", "created_time", "admin_remark", "processed_time").
		Order("created_time DESC, id DESC")
	if withdrawalStatus != "" {
		withdrawalQuery = withdrawalQuery.Where("status = ?", withdrawalStatus)
	}
	var withdrawals []AffiliateWithdrawal
	if err := withdrawalQuery.Find(&withdrawals).Error; err != nil {
		return nil, err
	}
	latestWithdrawalByUser := make(map[int]AffiliateWithdrawal)
	for _, withdrawal := range withdrawals {
		if withdrawal.UserId <= 0 {
			continue
		}
		summaryUserSet[withdrawal.UserId] = struct{}{}
		itemUserSet[withdrawal.UserId] = struct{}{}
		if _, ok := latestWithdrawalByUser[withdrawal.UserId]; !ok {
			latestWithdrawalByUser[withdrawal.UserId] = withdrawal
		}
	}

	var pendingWithdrawals []AffiliateWithdrawal
	if err := DB.Select("user_id", "amount_cents").
		Where("status = ?", AffiliateWithdrawalStatusPending).Find(&pendingWithdrawals).Error; err != nil {
		return nil, err
	}
	pendingWithdrawalUsers := make(map[int]struct{})
	for _, withdrawal := range pendingWithdrawals {
		if withdrawal.UserId <= 0 {
			continue
		}
		summaryUserSet[withdrawal.UserId] = struct{}{}
		if withdrawalStatus == "" {
			itemUserSet[withdrawal.UserId] = struct{}{}
		}
		pendingWithdrawalUsers[withdrawal.UserId] = struct{}{}
		result.Summary.PendingWithdrawalCents += withdrawal.AmountCents
	}
	result.Summary.PendingWithdrawalUserCount = len(pendingWithdrawalUsers)

	summaryIDs := sortedIntKeys(summaryUserSet)
	result.Summary.AffiliateUserCount = len(summaryIDs)
	for _, id := range summaryIDs {
		if availableByUser[id] > 0 {
			if _, hasPendingWithdrawal := pendingWithdrawalUsers[id]; !hasPendingWithdrawal {
				result.Summary.AvailableWithoutWithdrawalUserCount++
			}
		}
	}

	ids := sortedIntKeys(itemUserSet)
	result.Total = int64(len(ids))

	pageIDs := paginateIntSlice(ids, offset, limit)
	usernames, err := getUsernamesByIds(pageIDs)
	if err != nil {
		return nil, err
	}
	result.Items = make([]AffiliateStatsItem, 0, len(pageIDs))
	for _, id := range pageIDs {
		item := AffiliateStatsItem{
			UserId:                id,
			Username:              usernames[id],
			PeriodCommissionCents: periodByUser[id],
			TotalCommissionCents:  totalByUser[id],
			AvailableCents:        availableByUser[id],
			WithdrawnCents:        withdrawnByUser[id],
		}
		if withdrawal, ok := latestWithdrawalByUser[id]; ok {
			item.Withdrawal = affiliateWithdrawalInfoFromModel(withdrawal)
		}
		result.Items = append(result.Items, item)
	}

	return result, nil
}

func ListOrderManagementOrders(startTime, endTime int64, mailStatus string, keyword string, offset int, limit int) ([]OrderManagementOrderRow, int64, error) {
	offset, limit = normalizeOffsetLimit(offset, limit, 20, 100)
	query := DB.Model(&LdxpTopupSession{}).Where("created_time >= ? AND created_time <= ?", startTime, endTime)
	if mailStatus != "" {
		query = query.Where("mail_status = ?", mailStatus)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("session_id LIKE ? OR trade_no LIKE ? OR worker_order_no LIKE ? OR mail_order_no LIKE ?", like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var sessions []LdxpTopupSession
	if err := query.Order("created_time DESC, id DESC").Offset(offset).Limit(limit).Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	userIDs := make([]int, 0, len(sessions))
	sessionIDs := make([]string, 0, len(sessions))
	tradeNos := make([]string, 0, len(sessions))
	topupIDs := make([]int, 0, len(sessions))
	for _, session := range sessions {
		userIDs = append(userIDs, session.UserId)
		if session.SessionId != "" {
			sessionIDs = append(sessionIDs, session.SessionId)
		}
		if session.TradeNo != "" {
			tradeNos = append(tradeNos, session.TradeNo)
		}
		if session.TopUpId > 0 {
			topupIDs = append(topupIDs, session.TopUpId)
		}
	}
	usernames, err := getUsernamesByIds(userIDs)
	if err != nil {
		return nil, 0, err
	}
	commissionsByKey, err := getAffiliateCommissionsBySessionKeys(sessionIDs, tradeNos, topupIDs)
	if err != nil {
		return nil, 0, err
	}

	rows := make([]OrderManagementOrderRow, 0, len(sessions))
	for _, session := range sessions {
		row := OrderManagementOrderRow{
			Id:                session.Id,
			SessionId:         session.SessionId,
			UserId:            session.UserId,
			Username:          usernames[session.UserId],
			SiteAmountCents:   session.SiteAmountCents,
			ExternalPaidCents: session.ExternalPaidCents,
			WorkerOrderNo:     session.WorkerOrderNo,
			MailOrderNo:       session.MailOrderNo,
			MailAmountCents:   session.MailAmountCents,
			MailStatus:        session.MailStatus,
			ErrorCode:         session.ErrorCode,
			ErrorMessage:      session.ErrorMessage,
			CreatedTime:       session.CreatedTime,
			VerifiedTime:      session.VerifiedTime,
		}
		if commission, ok := findCommissionForSession(commissionsByKey, session); ok {
			row.AffiliateInviterId = commission.InviterUserId
			row.AffiliateCommissionCents = commission.CommissionCents
			row.AffiliateStatus = commission.Status
		}
		rows = append(rows, row)
	}
	return rows, total, nil
}

func GetAffiliateSourceOrders(inviterUserId int, startTime int64, endTime int64, limit int) ([]AffiliateSourceOrderRow, error) {
	_, limit = normalizeOffsetLimit(0, limit, 50, 200)
	var commissions []AffiliateCommission
	if err := DB.Where("inviter_user_id = ? AND created_time >= ? AND created_time <= ?", inviterUserId, startTime, endTime).
		Order("created_time DESC, id DESC").Limit(limit).Find(&commissions).Error; err != nil {
		return nil, err
	}

	inviteeIDs := make([]int, 0, len(commissions))
	sessionIDs := make([]string, 0, len(commissions))
	tradeNos := make([]string, 0, len(commissions))
	topupIDs := make([]int, 0, len(commissions))
	for _, commission := range commissions {
		inviteeIDs = append(inviteeIDs, commission.InviteeUserId)
		if commission.SessionId != "" {
			sessionIDs = append(sessionIDs, commission.SessionId)
		}
		if commission.TradeNo != "" {
			tradeNos = append(tradeNos, commission.TradeNo)
		}
		if commission.TopUpId > 0 {
			topupIDs = append(topupIDs, commission.TopUpId)
		}
	}
	usernames, err := getUsernamesByIds(inviteeIDs)
	if err != nil {
		return nil, err
	}
	sessionsByKey, err := getTopupSessionsByKeys(sessionIDs, tradeNos, topupIDs)
	if err != nil {
		return nil, err
	}

	rows := make([]AffiliateSourceOrderRow, 0, len(commissions))
	for _, commission := range commissions {
		row := AffiliateSourceOrderRow{
			OrderTime:       commission.CreatedTime,
			InviteeUserId:   commission.InviteeUserId,
			InviteeUsername: usernames[commission.InviteeUserId],
			TradeNo:         commission.TradeNo,
			BaseMoneyCents:  commission.BaseMoneyCents,
			RateBps:         commission.RateBps,
			CommissionCents: commission.CommissionCents,
		}
		if session, ok := findSessionForCommission(sessionsByKey, commission); ok {
			row.OrderTime = session.CreatedTime
			row.TradeNo = firstNonEmpty(row.TradeNo, session.TradeNo)
			row.WorkerOrderNo = session.WorkerOrderNo
			row.MailStatus = session.MailStatus
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func isOrderManagementMailErrorStatus(status string) bool {
	switch status {
	case MailCheckStatusAmountMismatch, MailCheckStatusOrderMismatch, MailCheckStatusMailParseFailed, MailCheckStatusMailFetchFailed, MailCheckStatusTimeout:
		return true
	default:
		return false
	}
}

func shouldCountAffiliateCommissionAmount(status string) bool {
	return status != AffiliateCommissionStatusRejected
}

func normalizeOffsetLimit(offset int, limit int, defaultLimit int, maxLimit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return offset, limit
}

func sortedIntKeys(values map[int]struct{}) []int {
	ids := make([]int, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func paginateIntSlice(ids []int, offset int, limit int) []int {
	if offset >= len(ids) {
		return []int{}
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}
	return ids[offset:end]
}

func getUsernamesByIds(ids []int) (map[int]string, error) {
	usernames := make(map[int]string)
	ids = uniquePositiveInts(ids)
	if len(ids) == 0 {
		return usernames, nil
	}
	var users []User
	if err := DB.Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		usernames[user.Id] = user.Username
	}
	return usernames, nil
}

func uniquePositiveInts(values []int) []int {
	seen := make(map[int]struct{})
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func affiliateWithdrawalInfoFromModel(withdrawal AffiliateWithdrawal) *AffiliateWithdrawalInfo {
	return &AffiliateWithdrawalInfo{
		Id:            withdrawal.Id,
		WithdrawalId:  withdrawal.WithdrawalId,
		AmountCents:   withdrawal.AmountCents,
		Contact:       withdrawal.Contact,
		Remark:        withdrawal.Remark,
		Status:        withdrawal.Status,
		CreatedTime:   withdrawal.CreatedTime,
		AdminRemark:   withdrawal.AdminRemark,
		ProcessedTime: withdrawal.ProcessedTime,
	}
}

type orderManagementCommissionMaps struct {
	bySessionId map[string]AffiliateCommission
	byTradeNo   map[string]AffiliateCommission
	byTopUpId   map[int]AffiliateCommission
}

func getAffiliateCommissionsBySessionKeys(sessionIDs []string, tradeNos []string, topupIDs []int) (*orderManagementCommissionMaps, error) {
	maps := &orderManagementCommissionMaps{
		bySessionId: make(map[string]AffiliateCommission),
		byTradeNo:   make(map[string]AffiliateCommission),
		byTopUpId:   make(map[int]AffiliateCommission),
	}
	query := DB.Model(&AffiliateCommission{})
	conditions := DB.Session(&gorm.Session{NewDB: true})
	applied := false
	if len(sessionIDs) > 0 {
		conditions = conditions.Where("session_id IN ?", sessionIDs)
		applied = true
	}
	if len(tradeNos) > 0 {
		if applied {
			conditions = conditions.Or("trade_no IN ?", tradeNos)
		} else {
			conditions = conditions.Where("trade_no IN ?", tradeNos)
			applied = true
		}
	}
	if len(topupIDs) > 0 {
		if applied {
			conditions = conditions.Or("top_up_id IN ?", topupIDs)
		} else {
			conditions = conditions.Where("top_up_id IN ?", topupIDs)
			applied = true
		}
	}
	if !applied {
		return maps, nil
	}
	var commissions []AffiliateCommission
	if err := query.Where(conditions).Order("created_time DESC, id DESC").Find(&commissions).Error; err != nil {
		return nil, err
	}
	for _, commission := range commissions {
		if commission.SessionId != "" {
			if _, ok := maps.bySessionId[commission.SessionId]; !ok {
				maps.bySessionId[commission.SessionId] = commission
			}
		}
		if commission.TradeNo != "" {
			if _, ok := maps.byTradeNo[commission.TradeNo]; !ok {
				maps.byTradeNo[commission.TradeNo] = commission
			}
		}
		if commission.TopUpId > 0 {
			if _, ok := maps.byTopUpId[commission.TopUpId]; !ok {
				maps.byTopUpId[commission.TopUpId] = commission
			}
		}
	}
	return maps, nil
}

func findCommissionForSession(maps *orderManagementCommissionMaps, session LdxpTopupSession) (AffiliateCommission, bool) {
	if session.SessionId != "" {
		if commission, ok := maps.bySessionId[session.SessionId]; ok {
			if orderManagementKeysMatchWithoutConflict(session.SessionId, session.TradeNo, session.TopUpId, commission.SessionId, commission.TradeNo, commission.TopUpId) {
				return commission, true
			}
		}
	}
	if session.TradeNo != "" {
		if commission, ok := maps.byTradeNo[session.TradeNo]; ok {
			if orderManagementKeysMatchWithoutConflict(session.SessionId, session.TradeNo, session.TopUpId, commission.SessionId, commission.TradeNo, commission.TopUpId) {
				return commission, true
			}
		}
	}
	if session.TopUpId > 0 {
		if commission, ok := maps.byTopUpId[session.TopUpId]; ok {
			if orderManagementKeysMatchWithoutConflict(session.SessionId, session.TradeNo, session.TopUpId, commission.SessionId, commission.TradeNo, commission.TopUpId) {
				return commission, true
			}
		}
	}
	return AffiliateCommission{}, false
}

type orderManagementSessionMaps struct {
	bySessionId map[string]LdxpTopupSession
	byTradeNo   map[string]LdxpTopupSession
	byTopUpId   map[int]LdxpTopupSession
}

func getTopupSessionsByKeys(sessionIDs []string, tradeNos []string, topupIDs []int) (*orderManagementSessionMaps, error) {
	maps := &orderManagementSessionMaps{
		bySessionId: make(map[string]LdxpTopupSession),
		byTradeNo:   make(map[string]LdxpTopupSession),
		byTopUpId:   make(map[int]LdxpTopupSession),
	}
	query := DB.Model(&LdxpTopupSession{})
	conditions := DB.Session(&gorm.Session{NewDB: true})
	applied := false
	if len(sessionIDs) > 0 {
		conditions = conditions.Where("session_id IN ?", sessionIDs)
		applied = true
	}
	if len(tradeNos) > 0 {
		if applied {
			conditions = conditions.Or("trade_no IN ?", tradeNos)
		} else {
			conditions = conditions.Where("trade_no IN ?", tradeNos)
			applied = true
		}
	}
	if len(topupIDs) > 0 {
		if applied {
			conditions = conditions.Or("top_up_id IN ?", topupIDs)
		} else {
			conditions = conditions.Where("top_up_id IN ?", topupIDs)
			applied = true
		}
	}
	if !applied {
		return maps, nil
	}
	var sessions []LdxpTopupSession
	if err := query.Where(conditions).Order("created_time DESC, id DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session.SessionId != "" {
			if _, ok := maps.bySessionId[session.SessionId]; !ok {
				maps.bySessionId[session.SessionId] = session
			}
		}
		if session.TradeNo != "" {
			if _, ok := maps.byTradeNo[session.TradeNo]; !ok {
				maps.byTradeNo[session.TradeNo] = session
			}
		}
		if session.TopUpId > 0 {
			if _, ok := maps.byTopUpId[session.TopUpId]; !ok {
				maps.byTopUpId[session.TopUpId] = session
			}
		}
	}
	return maps, nil
}

func findSessionForCommission(maps *orderManagementSessionMaps, commission AffiliateCommission) (LdxpTopupSession, bool) {
	if commission.SessionId != "" {
		if session, ok := maps.bySessionId[commission.SessionId]; ok {
			if orderManagementKeysMatchWithoutConflict(session.SessionId, session.TradeNo, session.TopUpId, commission.SessionId, commission.TradeNo, commission.TopUpId) {
				return session, true
			}
		}
	}
	if commission.TradeNo != "" {
		if session, ok := maps.byTradeNo[commission.TradeNo]; ok {
			if orderManagementKeysMatchWithoutConflict(session.SessionId, session.TradeNo, session.TopUpId, commission.SessionId, commission.TradeNo, commission.TopUpId) {
				return session, true
			}
		}
	}
	if commission.TopUpId > 0 {
		if session, ok := maps.byTopUpId[commission.TopUpId]; ok {
			if orderManagementKeysMatchWithoutConflict(session.SessionId, session.TradeNo, session.TopUpId, commission.SessionId, commission.TradeNo, commission.TopUpId) {
				return session, true
			}
		}
	}
	return LdxpTopupSession{}, false
}

func orderManagementKeysMatchWithoutConflict(sessionID1 string, tradeNo1 string, topUpID1 int, sessionID2 string, tradeNo2 string, topUpID2 int) bool {
	hasMatch := false
	if sessionID1 != "" && sessionID2 != "" {
		if sessionID1 != sessionID2 {
			return false
		}
		hasMatch = true
	}
	if tradeNo1 != "" && tradeNo2 != "" {
		if tradeNo1 != tradeNo2 {
			return false
		}
		hasMatch = true
	}
	if topUpID1 > 0 && topUpID2 > 0 {
		if topUpID1 != topUpID2 {
			return false
		}
		hasMatch = true
	}
	return hasMatch
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
