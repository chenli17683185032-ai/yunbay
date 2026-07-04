package dto

type OrderManagementMoneySummary struct {
	SiteAmount              float64 `json:"site_amount"`
	ExternalPaidAmount      float64 `json:"external_paid_amount"`
	OrderCount              int     `json:"order_count"`
	MailVerifiedCount       int     `json:"mail_verified_count"`
	MailPendingCount        int     `json:"mail_pending_count"`
	MailErrorCount          int     `json:"mail_error_count"`
	MailVerifiedRate        float64 `json:"mail_verified_rate"`
	AffiliateUserCount      int     `json:"affiliate_user_count"`
	AffiliateAmount         float64 `json:"affiliate_amount"`
	WithdrawalPendingCount  int     `json:"withdrawal_pending_count"`
	WithdrawalPendingAmount float64 `json:"withdrawal_pending_amount"`
}

type OrderManagementDailyPoint struct {
	Date               string  `json:"date"`
	SiteAmount         float64 `json:"site_amount"`
	ExternalPaidAmount float64 `json:"external_paid_amount"`
	OrderCount         int     `json:"order_count"`
	MailVerifiedCount  int     `json:"mail_verified_count"`
	MailErrorCount     int     `json:"mail_error_count"`
}

type OrderManagementAnalyticsResponse struct {
	Summary OrderManagementMoneySummary `json:"summary"`
	Daily   []OrderManagementDailyPoint `json:"daily"`
}

type OrderManagementAffiliateBrief struct {
	InviterUserId   int     `json:"inviter_user_id"`
	CommissionMoney float64 `json:"commission_money"`
	Status          string  `json:"status"`
}

type OrderManagementOrderItem struct {
	Id                 int                            `json:"id"`
	OrderType          string                         `json:"order_type"`
	SessionId          string                         `json:"session_id"`
	UserId             int                            `json:"user_id"`
	Username           string                         `json:"username"`
	SiteAmount         float64                        `json:"site_amount"`
	ExternalPaidAmount float64                        `json:"external_paid_amount"`
	WorkerOrderNo      string                         `json:"worker_order_no"`
	MailOrderNo        string                         `json:"mail_order_no"`
	MailPaidAmount     float64                        `json:"mail_paid_amount"`
	MailStatus         string                         `json:"mail_status"`
	MailStatusText     string                         `json:"mail_status_text"`
	ErrorCode          string                         `json:"error_code"`
	ErrorMessage       string                         `json:"error_message"`
	Affiliate          *OrderManagementAffiliateBrief `json:"affiliate"`
	CreatedTime        int64                          `json:"created_time"`
	VerifiedTime       int64                          `json:"verified_time"`
}

type MailCheckRequest struct {
	Range     string `json:"range"`
	Scope     string `json:"scope"`
	Limit     int    `json:"limit"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type MailCheckResponse struct {
	JobId         string `json:"job_id"`
	Started       bool   `json:"started"`
	AffectedCount int    `json:"affected_count"`
}

type AffiliateWithdrawalDTO struct {
	Id            int     `json:"id"`
	WithdrawalId  string  `json:"withdrawal_id"`
	Amount        float64 `json:"amount"`
	Contact       string  `json:"contact"`
	Remark        string  `json:"remark"`
	Status        string  `json:"status"`
	CreatedTime   int64   `json:"created_time"`
	AdminRemark   string  `json:"admin_remark"`
	ProcessedTime int64   `json:"processed_time"`
}

type AffiliateStatsSummaryDTO struct {
	AffiliateUserCount                  int     `json:"affiliate_user_count"`
	PeriodCommissionAmount              float64 `json:"period_commission_amount"`
	PendingWithdrawalUserCount          int     `json:"pending_withdrawal_user_count"`
	PendingWithdrawalAmount             float64 `json:"pending_withdrawal_amount"`
	AvailableWithoutWithdrawalUserCount int     `json:"available_without_withdrawal_user_count"`
}

type AffiliateStatsItemDTO struct {
	UserId                 int                     `json:"user_id"`
	Username               string                  `json:"username"`
	PeriodCommissionAmount float64                 `json:"period_commission_amount"`
	TotalCommissionAmount  float64                 `json:"total_commission_amount"`
	AvailableAmount        float64                 `json:"available_amount"`
	WithdrawnAmount        float64                 `json:"withdrawn_amount"`
	Withdrawal             *AffiliateWithdrawalDTO `json:"withdrawal"`
}

type AffiliateStatsResponse struct {
	Summary AffiliateStatsSummaryDTO `json:"summary"`
	Items   []AffiliateStatsItemDTO  `json:"items"`
	Total   int64                    `json:"total"`
}

type WithdrawalActionRequest struct {
	AdminRemark string `json:"admin_remark"`
}

type AffiliateSourceOrderDTO struct {
	OrderTime       int64   `json:"order_time"`
	InviteeUserId   int     `json:"invitee_user_id"`
	InviteeUsername string  `json:"invitee_username"`
	TradeNo         string  `json:"trade_no"`
	WorkerOrderNo   string  `json:"worker_order_no"`
	BaseMoney       float64 `json:"base_money"`
	RateBps         int     `json:"rate_bps"`
	CommissionMoney float64 `json:"commission_money"`
	MailStatus      string  `json:"mail_status"`
}

type AdminDeleteOrderRequest struct {
	Reason string `json:"reason"`
}
