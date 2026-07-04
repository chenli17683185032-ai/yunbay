package model

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ErrAffiliateWithdrawalAlreadyProcessed keeps the order-management API's
// single-transition semantics while reusing the production affiliate model.
var ErrAffiliateWithdrawalAlreadyProcessed = ErrAffiliateWithdrawalBadStatus

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
	if err := DB.Where("created_time >= ? AND created_time <= ?", startTime, endTime).
		Where("status IN ? AND (topup_id > ? OR redemption_id > ?)", orderManagementSettledLdxpStatuses(), 0, 0).
		Order("created_time ASC, id ASC").Find(&sessions).Error; err != nil {
		return nil, err
	}

	result := &OrderAnalyticsResult{}
	dailyByDate := make(map[string]*OrderAnalyticsDaily)
	for _, session := range sessions {
		siteCents := orderManagementMoneyCents(session.Money)
		externalCents := orderManagementMoneyCents(session.WorkerAmount)
		mailStatus := OrderManagementMailStatusFromSession(session)

		result.Summary.SiteAmountCents += siteCents
		result.Summary.ExternalPaidCents += externalCents
		result.Summary.OrderCount++

		date := time.Unix(session.CreatedTime, 0).UTC().Format("2006-01-02")
		daily := dailyByDate[date]
		if daily == nil {
			daily = &OrderAnalyticsDaily{Date: date}
			dailyByDate[date] = daily
		}
		daily.SiteAmountCents += siteCents
		daily.ExternalPaidCents += externalCents
		daily.OrderCount++

		switch {
		case mailStatus == MailCheckStatusVerified:
			result.Summary.MailVerifiedCount++
			daily.MailVerifiedCount++
		case isOrderManagementMailErrorStatus(mailStatus):
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
	if err := DB.Where("created_time >= ? AND created_time <= ?", startTime, endTime).Find(&commissions).Error; err != nil {
		return nil, err
	}
	inviterSet := make(map[int]struct{})
	for _, commission := range commissions {
		if commission.InviterUserId > 0 {
			inviterSet[commission.InviterUserId] = struct{}{}
		}
		if shouldCountAffiliateCommissionAmount(commission.Status) {
			result.Summary.AffiliateAmountCents += orderManagementMoneyCents(commission.CommissionMoney)
		}
	}
	result.Summary.AffiliateUserCount = len(inviterSet)

	var withdrawals []AffiliateWithdrawal
	if err := DB.Where("created_time >= ? AND created_time <= ? AND status = ?", startTime, endTime, AffiliateWithdrawalStatusPending).Find(&withdrawals).Error; err != nil {
		return nil, err
	}
	result.Summary.PendingWithdrawalCount = len(withdrawals)
	for _, withdrawal := range withdrawals {
		result.Summary.PendingWithdrawalCents += orderManagementMoneyCents(withdrawal.Amount)
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
	pendingByUser := make(map[int]int64)
	paidByUser := make(map[int]int64)

	var periodCommissions []AffiliateCommission
	if err := DB.Where("created_time >= ? AND created_time <= ?", startTime, endTime).Find(&periodCommissions).Error; err != nil {
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
		commissionCents := orderManagementMoneyCents(commission.CommissionMoney)
		periodByUser[commission.InviterUserId] += commissionCents
		result.Summary.PeriodCommissionCents += commissionCents
	}

	var allCommissions []AffiliateCommission
	if err := DB.Find(&allCommissions).Error; err != nil {
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
		if shouldCountAffiliateCommissionAmount(commission.Status) {
			totalByUser[commission.InviterUserId] += orderManagementMoneyCents(commission.CommissionMoney)
		}
	}

	var withdrawals []AffiliateWithdrawal
	if err := DB.Order("created_time DESC, id DESC").Find(&withdrawals).Error; err != nil {
		return nil, err
	}
	latestWithdrawalByUser := make(map[int]AffiliateWithdrawal)
	latestWithdrawalByFilteredUser := make(map[int]AffiliateWithdrawal)
	pendingWithdrawalUsers := make(map[int]struct{})
	for _, withdrawal := range withdrawals {
		if withdrawal.UserId <= 0 {
			continue
		}
		amountCents := orderManagementMoneyCents(withdrawal.Amount)
		summaryUserSet[withdrawal.UserId] = struct{}{}
		if withdrawalStatus == "" {
			itemUserSet[withdrawal.UserId] = struct{}{}
		}
		if _, ok := latestWithdrawalByUser[withdrawal.UserId]; !ok {
			latestWithdrawalByUser[withdrawal.UserId] = withdrawal
		}
		if withdrawalStatus != "" && withdrawal.Status == withdrawalStatus {
			itemUserSet[withdrawal.UserId] = struct{}{}
			if _, ok := latestWithdrawalByFilteredUser[withdrawal.UserId]; !ok {
				latestWithdrawalByFilteredUser[withdrawal.UserId] = withdrawal
			}
		}
		switch withdrawal.Status {
		case AffiliateWithdrawalStatusPending:
			pendingWithdrawalUsers[withdrawal.UserId] = struct{}{}
			pendingByUser[withdrawal.UserId] += amountCents
			result.Summary.PendingWithdrawalCents += amountCents
		case AffiliateWithdrawalStatusPaid:
			paidByUser[withdrawal.UserId] += amountCents
		}
	}
	result.Summary.PendingWithdrawalUserCount = len(pendingWithdrawalUsers)

	summaryIDs := sortedIntKeys(summaryUserSet)
	result.Summary.AffiliateUserCount = len(summaryIDs)
	availableByUser := make(map[int]int64)
	for _, id := range summaryIDs {
		available := totalByUser[id] - paidByUser[id] - pendingByUser[id]
		if available < 0 {
			available = 0
		}
		availableByUser[id] = available
		if available > 0 {
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
			WithdrawnCents:        paidByUser[id],
		}
		if withdrawal, ok := latestWithdrawalByUser[id]; ok {
			if withdrawalStatus != "" {
				withdrawal = latestWithdrawalByFilteredUser[id]
			}
			item.Withdrawal = affiliateWithdrawalInfoFromModel(withdrawal)
		}
		result.Items = append(result.Items, item)
	}

	return result, nil
}

func ListOrderManagementOrders(startTime, endTime int64, mailStatus string, keyword string, offset int, limit int) ([]OrderManagementOrderRow, int64, error) {
	offset, limit = normalizeOffsetLimit(offset, limit, 20, 100)
	query := DB.Model(&LdxpTopupSession{}).
		Where("created_time >= ? AND created_time <= ?", startTime, endTime).
		Where("status IN ? AND (topup_id > ? OR redemption_id > ?)", orderManagementSettledLdxpStatuses(), 0, 0)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("session_id LIKE ? OR worker_order_no LIKE ? OR mail_order_no LIKE ? OR mail_message_id LIKE ?", like, like, like, like)
	}

	var sessions []LdxpTopupSession
	if err := query.Order("created_time DESC, id DESC").Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	filtered := make([]LdxpTopupSession, 0, len(sessions))
	mailStatus = strings.TrimSpace(mailStatus)
	for _, session := range sessions {
		if mailStatus == "" || OrderManagementMailStatusFromSession(session) == mailStatus {
			filtered = append(filtered, session)
		}
	}
	total := int64(len(filtered))
	if offset >= len(filtered) {
		return []OrderManagementOrderRow{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	pageSessions := filtered[offset:end]

	userIDs := make([]int, 0, len(pageSessions))
	tradeNos := make([]string, 0, len(pageSessions))
	topupIDs := make([]int, 0, len(pageSessions))
	for _, session := range pageSessions {
		userIDs = append(userIDs, session.UserId)
		if session.MailMessageId != "" {
			tradeNos = append(tradeNos, session.MailMessageId)
		}
		if session.TopupId > 0 {
			topupIDs = append(topupIDs, session.TopupId)
		}
	}
	usernames, err := getUsernamesByIds(userIDs)
	if err != nil {
		return nil, 0, err
	}
	commissionsByKey, err := getAffiliateCommissionsBySessionKeys(tradeNos, topupIDs)
	if err != nil {
		return nil, 0, err
	}

	rows := make([]OrderManagementOrderRow, 0, len(pageSessions))
	for _, session := range pageSessions {
		row := OrderManagementOrderRow{
			Id:                session.Id,
			SessionId:         session.SessionId,
			UserId:            session.UserId,
			Username:          usernames[session.UserId],
			SiteAmountCents:   orderManagementMoneyCents(session.Money),
			ExternalPaidCents: orderManagementMoneyCents(session.WorkerAmount),
			WorkerOrderNo:     session.WorkerOrderNo,
			MailOrderNo:       session.MailOrderNo,
			MailAmountCents:   orderManagementMoneyCents(session.MailAmount),
			MailStatus:        OrderManagementMailStatusFromSession(session),
			ErrorCode:         session.ErrorCode,
			ErrorMessage:      session.ErrorMessage,
			CreatedTime:       session.CreatedTime,
			VerifiedTime:      session.VerifiedTime,
		}
		if commission, ok := findCommissionForSession(commissionsByKey, session); ok {
			row.AffiliateInviterId = commission.InviterUserId
			row.AffiliateCommissionCents = orderManagementMoneyCents(commission.CommissionMoney)
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
	tradeNos := make([]string, 0, len(commissions))
	topupIDs := make([]int, 0, len(commissions))
	for _, commission := range commissions {
		inviteeIDs = append(inviteeIDs, commission.InviteeUserId)
		if commission.TradeNo != "" {
			tradeNos = append(tradeNos, commission.TradeNo)
		}
		if commission.TopupId > 0 {
			topupIDs = append(topupIDs, commission.TopupId)
		}
	}
	usernames, err := getUsernamesByIds(inviteeIDs)
	if err != nil {
		return nil, err
	}
	sessionsByKey, err := getTopupSessionsByKeys(tradeNos, topupIDs)
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
			BaseMoneyCents:  orderManagementMoneyCents(commission.BaseMoney),
			RateBps:         orderManagementRateBps(commission.Rate),
			CommissionCents: orderManagementMoneyCents(commission.CommissionMoney),
		}
		if session, ok := findSessionForCommission(sessionsByKey, commission); ok {
			row.OrderTime = session.CreatedTime
			row.TradeNo = firstNonEmpty(row.TradeNo, session.MailMessageId)
			row.WorkerOrderNo = session.WorkerOrderNo
			row.MailStatus = OrderManagementMailStatusFromSession(session)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func OrderManagementMailStatusFromSession(session LdxpTopupSession) string {
	code := strings.TrimSpace(session.ErrorCode)
	switch code {
	case "amount_mismatch":
		return MailCheckStatusAmountMismatch
	case "order_mismatch", "mail_order_mismatch":
		return MailCheckStatusOrderMismatch
	case "mail_fetch_failed":
		return MailCheckStatusMailFetchFailed
	case "mail_parse_failed", "missing_data":
		return MailCheckStatusMailParseFailed
	case "waiting_mail", "mail_event_not_found", "pending":
		return MailCheckStatusWaitingMail
	}

	if session.Status == LdxpStatusMailTimeout {
		return MailCheckStatusTimeout
	}
	if session.Status == LdxpStatusVerifyFailed {
		return MailCheckStatusMailParseFailed
	}
	if session.MailOrderNo != "" {
		if session.WorkerOrderNo != "" && session.MailOrderNo != session.WorkerOrderNo {
			return MailCheckStatusOrderMismatch
		}
		if session.WorkerAmount > 0 && session.MailAmount > 0 && orderManagementMoneyCents(session.WorkerAmount) != orderManagementMoneyCents(session.MailAmount) {
			return MailCheckStatusAmountMismatch
		}
		return MailCheckStatusVerified
	}
	switch session.Status {
	case LdxpStatusVerified, LdxpStatusRedeemed, LdxpStatusSuccess:
		return MailCheckStatusVerified
	case LdxpStatusWorkerPaid:
		return MailCheckStatusWaitingMail
	default:
		return MailCheckStatusPending
	}
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
	return status != AffiliateCommissionStatusCanceled
}

func orderManagementSettledLdxpStatuses() []string {
	return []string{LdxpStatusVerified, LdxpStatusRedeemed, LdxpStatusSuccess}
}

func orderManagementMoneyCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

func orderManagementRateBps(rate float64) int {
	return int(math.Round(rate * 10000))
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
		AmountCents:   orderManagementMoneyCents(withdrawal.Amount),
		Contact:       withdrawal.Contact,
		Remark:        withdrawal.Remark,
		Status:        withdrawal.Status,
		CreatedTime:   withdrawal.CreatedTime,
		AdminRemark:   withdrawal.AdminRemark,
		ProcessedTime: withdrawal.ProcessedTime,
	}
}

type orderManagementCommissionMaps struct {
	byTradeNo map[string]AffiliateCommission
	byTopUpId map[int]AffiliateCommission
}

func getAffiliateCommissionsBySessionKeys(tradeNos []string, topupIDs []int) (*orderManagementCommissionMaps, error) {
	maps := &orderManagementCommissionMaps{
		byTradeNo: make(map[string]AffiliateCommission),
		byTopUpId: make(map[int]AffiliateCommission),
	}
	conditions := DB.Session(&gorm.Session{NewDB: true})
	applied := false
	if len(tradeNos) > 0 {
		conditions = conditions.Where("trade_no IN ?", tradeNos)
		applied = true
	}
	if len(topupIDs) > 0 {
		if applied {
			conditions = conditions.Or("topup_id IN ?", topupIDs)
		} else {
			conditions = conditions.Where("topup_id IN ?", topupIDs)
			applied = true
		}
	}
	if !applied {
		return maps, nil
	}
	var commissions []AffiliateCommission
	if err := DB.Where(conditions).Order("created_time DESC, id DESC").Find(&commissions).Error; err != nil {
		return nil, err
	}
	for _, commission := range commissions {
		if commission.TradeNo != "" {
			if _, ok := maps.byTradeNo[commission.TradeNo]; !ok {
				maps.byTradeNo[commission.TradeNo] = commission
			}
		}
		if commission.TopupId > 0 {
			if _, ok := maps.byTopUpId[commission.TopupId]; !ok {
				maps.byTopUpId[commission.TopupId] = commission
			}
		}
	}
	return maps, nil
}

func findCommissionForSession(maps *orderManagementCommissionMaps, session LdxpTopupSession) (AffiliateCommission, bool) {
	if maps == nil {
		return AffiliateCommission{}, false
	}
	if session.TopupId > 0 {
		if commission, ok := maps.byTopUpId[session.TopupId]; ok {
			return commission, true
		}
	}
	if session.MailMessageId != "" {
		if commission, ok := maps.byTradeNo[session.MailMessageId]; ok {
			return commission, true
		}
	}
	return AffiliateCommission{}, false
}

type orderManagementSessionMaps struct {
	byTradeNo map[string]LdxpTopupSession
	byTopUpId map[int]LdxpTopupSession
}

func getTopupSessionsByKeys(tradeNos []string, topupIDs []int) (*orderManagementSessionMaps, error) {
	maps := &orderManagementSessionMaps{
		byTradeNo: make(map[string]LdxpTopupSession),
		byTopUpId: make(map[int]LdxpTopupSession),
	}
	conditions := DB.Session(&gorm.Session{NewDB: true})
	applied := false
	if len(tradeNos) > 0 {
		conditions = conditions.Where("mail_message_id IN ?", tradeNos)
		applied = true
	}
	if len(topupIDs) > 0 {
		if applied {
			conditions = conditions.Or("topup_id IN ?", topupIDs)
		} else {
			conditions = conditions.Where("topup_id IN ?", topupIDs)
			applied = true
		}
	}
	if !applied {
		return maps, nil
	}
	var sessions []LdxpTopupSession
	if err := DB.Where(conditions).Order("created_time DESC, id DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session.MailMessageId != "" {
			if _, ok := maps.byTradeNo[session.MailMessageId]; !ok {
				maps.byTradeNo[session.MailMessageId] = session
			}
		}
		if session.TopupId > 0 {
			if _, ok := maps.byTopUpId[session.TopupId]; !ok {
				maps.byTopUpId[session.TopupId] = session
			}
		}
	}
	return maps, nil
}

func findSessionForCommission(maps *orderManagementSessionMaps, commission AffiliateCommission) (LdxpTopupSession, bool) {
	if maps == nil {
		return LdxpTopupSession{}, false
	}
	if commission.TopupId > 0 {
		if session, ok := maps.byTopUpId[commission.TopupId]; ok {
			return session, true
		}
	}
	if commission.TradeNo != "" {
		if session, ok := maps.byTradeNo[commission.TradeNo]; ok {
			return session, true
		}
	}
	return LdxpTopupSession{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type OrderMailCheckBatchFilter struct {
	StartTime int64
	EndTime   int64
	Limit     int
}

func ListMailCheckCandidates(filter OrderMailCheckBatchFilter) ([]LdxpTopupSession, error) {
	return ListMailCheckCandidatesWithContext(context.Background(), filter)
}

func ListMailCheckCandidatesWithContext(ctx context.Context, filter OrderMailCheckBatchFilter) ([]LdxpTopupSession, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	query := DB.WithContext(ctx)
	if filter.StartTime > 0 {
		query = query.Where("created_time >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_time <= ?", filter.EndTime)
	}
	var sessions []LdxpTopupSession
	if err := query.Order("created_time DESC, id DESC").Limit(limit * 5).Find(&sessions).Error; err != nil {
		return nil, err
	}
	candidates := make([]LdxpTopupSession, 0, limit)
	for _, session := range sessions {
		status := OrderManagementMailStatusFromSession(session)
		if status == MailCheckStatusVerified || status == MailCheckStatusNotRequired || status == MailCheckStatusChecking {
			continue
		}
		candidates = append(candidates, session)
		if len(candidates) >= limit {
			break
		}
	}
	return candidates, nil
}

func TruncateOrderManagementTablesForTest(t interface {
	Helper()
	Cleanup(func())
}) {
	t.Helper()
	truncateOrderManagementTablesForTest()
	t.Cleanup(truncateOrderManagementTablesForTest)
}

func truncateOrderManagementTablesForTest() {
	DB.Exec("DELETE FROM ldxp_topup_sessions")
	DB.Exec("DELETE FROM ldxp_mail_events")
	DB.Exec("DELETE FROM affiliate_commissions")
	DB.Exec("DELETE FROM affiliate_withdrawals")
}

const (
	OrderTypeTopup        = "topup"
	OrderTypeSubscription = "subscription"
)

type OrderDeletionMark struct {
	Id          int    `json:"id"`
	OrderType   string `json:"order_type" gorm:"type:varchar(32);index:idx_order_delete_unique,unique"`
	TradeNo     string `json:"trade_no" gorm:"type:varchar(255);index:idx_order_delete_unique,unique"`
	DeletedBy   int    `json:"deleted_by" gorm:"index"`
	DeletedTime int64  `json:"deleted_time" gorm:"bigint;index"`
	Reason      string `json:"reason" gorm:"type:varchar(255);default:''"`
}

type AdminOrderRecord struct {
	OrderType       string  `json:"order_type"`
	Id              int     `json:"id"`
	TradeNo         string  `json:"trade_no"`
	UserId          int     `json:"user_id"`
	Status          string  `json:"status"`
	PaymentMethod   string  `json:"payment_method"`
	PaymentProvider string  `json:"payment_provider"`
	Amount          int64   `json:"amount"`
	Money           float64 `json:"money"`
	CreateTime      int64   `json:"create_time"`
	CompleteTime    int64   `json:"complete_time"`
	PlanId          int     `json:"plan_id,omitempty"`
	PlanTitle       string  `json:"plan_title,omitempty"`
	DurationUnit    string  `json:"duration_unit,omitempty"`
	DurationValue   int     `json:"duration_value,omitempty"`
	CustomSeconds   int64   `json:"custom_seconds,omitempty"`
}

func ListAdminOrders(pageInfo *common.PageInfo, keyword string) ([]AdminOrderRecord, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}

	deletionMarks, err := loadOrderDeletionMarkSet()
	if err != nil {
		return nil, 0, err
	}

	topupOrders, err := loadAdminTopupOrders(keyword)
	if err != nil {
		return nil, 0, err
	}
	subscriptionOrders, err := loadAdminSubscriptionOrders(keyword)
	if err != nil {
		return nil, 0, err
	}

	records := make([]AdminOrderRecord, 0, len(topupOrders)+len(subscriptionOrders))
	for _, record := range topupOrders {
		if _, deleted := deletionMarks[orderDeletionKey(record.OrderType, record.TradeNo)]; !deleted {
			records = append(records, record)
		}
	}
	for _, record := range subscriptionOrders {
		if _, deleted := deletionMarks[orderDeletionKey(record.OrderType, record.TradeNo)]; !deleted {
			records = append(records, record)
		}
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreateTime == records[j].CreateTime {
			return records[i].Id > records[j].Id
		}
		return records[i].CreateTime > records[j].CreateTime
	})

	total := int64(len(records))
	start := pageInfo.GetStartIdx()
	if start >= len(records) {
		return []AdminOrderRecord{}, total, nil
	}
	end := start + pageInfo.GetPageSize()
	if end > len(records) {
		end = len(records)
	}
	return records[start:end], total, nil
}

func MarkAdminOrderDeleted(orderType, tradeNo string, adminId int, reason string) error {
	orderType = normalizeAdminOrderType(orderType)
	if orderType == "" {
		return errors.New("invalid order_type")
	}
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return errors.New("trade_no is required")
	}
	reason = strings.TrimSpace(reason)
	now := common.GetTimestamp()

	return DB.Transaction(func(tx *gorm.DB) error {
		mark := OrderDeletionMark{
			OrderType:   orderType,
			TradeNo:     tradeNo,
			DeletedBy:   adminId,
			DeletedTime: now,
			Reason:      reason,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "order_type"}, {Name: "trade_no"}},
			DoNothing: true,
		}).Create(&mark).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"deleted_by":   adminId,
			"deleted_time": now,
		}
		if reason != "" {
			updates["reason"] = reason
		}
		return tx.Model(&OrderDeletionMark{}).
			Where("order_type = ? AND trade_no = ?", orderType, tradeNo).
			Updates(updates).Error
	})
}

func normalizeAdminOrderType(orderType string) string {
	switch strings.TrimSpace(orderType) {
	case OrderTypeTopup:
		return OrderTypeTopup
	case OrderTypeSubscription:
		return OrderTypeSubscription
	default:
		return ""
	}
}

func orderDeletionKey(orderType, tradeNo string) string {
	return orderType + "\x00" + tradeNo
}

func loadOrderDeletionMarkSet() (map[string]struct{}, error) {
	var marks []OrderDeletionMark
	if err := DB.Find(&marks).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(marks))
	for _, mark := range marks {
		set[orderDeletionKey(mark.OrderType, mark.TradeNo)] = struct{}{}
	}
	return set, nil
}

func loadAdminTopupOrders(keyword string) ([]AdminOrderRecord, error) {
	query := DB.Model(&TopUp{})
	if keyword != "" {
		query = query.Where("trade_no LIKE ?", "%"+keyword+"%")
	}

	var topups []TopUp
	if err := query.Order("create_time desc").Order("id desc").Find(&topups).Error; err != nil {
		return nil, err
	}

	records := make([]AdminOrderRecord, 0, len(topups))
	for _, topup := range topups {
		records = append(records, AdminOrderRecord{
			OrderType:       OrderTypeTopup,
			Id:              topup.Id,
			TradeNo:         topup.TradeNo,
			UserId:          topup.UserId,
			Status:          topup.Status,
			PaymentMethod:   topup.PaymentMethod,
			PaymentProvider: topup.PaymentProvider,
			Amount:          topup.Amount,
			Money:           topup.Money,
			CreateTime:      topup.CreateTime,
			CompleteTime:    topup.CompleteTime,
		})
	}
	return records, nil
}

func loadAdminSubscriptionOrders(keyword string) ([]AdminOrderRecord, error) {
	query := DB.Model(&SubscriptionOrder{})
	if keyword != "" {
		query = query.Where("trade_no LIKE ?", "%"+keyword+"%")
	}

	var orders []SubscriptionOrder
	if err := query.Order("create_time desc").Order("id desc").Find(&orders).Error; err != nil {
		return nil, err
	}

	records := make([]AdminOrderRecord, 0, len(orders))
	for _, order := range orders {
		records = append(records, AdminOrderRecord{
			OrderType:       OrderTypeSubscription,
			Id:              order.Id,
			TradeNo:         order.TradeNo,
			UserId:          order.UserId,
			Status:          order.Status,
			PaymentMethod:   order.PaymentMethod,
			PaymentProvider: order.PaymentProvider,
			Money:           order.Money,
			CreateTime:      order.CreateTime,
			CompleteTime:    order.CompleteTime,
			PlanId:          order.PlanId,
		})
	}
	records, err := attachSubscriptionPlanFields(records)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func attachSubscriptionPlanFields(records []AdminOrderRecord) ([]AdminOrderRecord, error) {
	planIds := make([]int, 0)
	seen := make(map[int]bool)
	for _, record := range records {
		if record.PlanId > 0 && !seen[record.PlanId] {
			seen[record.PlanId] = true
			planIds = append(planIds, record.PlanId)
		}
	}
	if len(planIds) == 0 {
		return records, nil
	}

	var plans []SubscriptionPlan
	if err := DB.Where("id IN ?", planIds).Find(&plans).Error; err != nil {
		return nil, err
	}
	plansById := make(map[int]SubscriptionPlan, len(plans))
	for _, plan := range plans {
		plansById[plan.Id] = plan
	}

	for i := range records {
		plan, ok := plansById[records[i].PlanId]
		if !ok {
			continue
		}
		records[i].PlanTitle = plan.Title
		records[i].DurationUnit = plan.DurationUnit
		records[i].DurationValue = plan.DurationValue
		records[i].CustomSeconds = plan.CustomSeconds
	}
	return records, nil
}
