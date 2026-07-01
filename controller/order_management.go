package controller

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const defaultOrderManagementRangeSeconds = int64(7 * 24 * 60 * 60)

func centsToAmount(cents int64) float64 {
	return float64(cents) / 100
}

func parseOrderManagementRange(rangeValue, startValue, endValue string, now int64) (int64, int64, error) {
	rangeValue = strings.TrimSpace(rangeValue)
	startValue = strings.TrimSpace(startValue)
	endValue = strings.TrimSpace(endValue)

	if rangeValue == "custom" || (rangeValue == "" && (startValue != "" || endValue != "")) {
		if startValue == "" || endValue == "" {
			return 0, 0, errors.New("start_time and end_time are required")
		}
		start, err := strconv.ParseInt(startValue, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid start_time")
		}
		end, err := strconv.ParseInt(endValue, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid end_time")
		}
		if end < start {
			return 0, 0, errors.New("end_time must be greater than or equal to start_time")
		}
		return start, end, nil
	}

	switch rangeValue {
	case "", "7d":
		return now - defaultOrderManagementRangeSeconds, now, nil
	case "30d":
		return now - int64(30*24*60*60), now, nil
	default:
		return 0, 0, errors.New("invalid range")
	}
}

func mailStatusText(status string) string {
	switch status {
	case model.MailCheckStatusPending:
		return "待核对"
	case model.MailCheckStatusWaitingMail:
		return "待邮件"
	case model.MailCheckStatusChecking:
		return "核对中"
	case model.MailCheckStatusVerified:
		return "已核对"
	case model.MailCheckStatusOrderMismatch:
		return "单号异常"
	case model.MailCheckStatusAmountMismatch:
		return "金额异常"
	case model.MailCheckStatusMailParseFailed:
		return "邮件解析失败"
	case model.MailCheckStatusMailFetchFailed:
		return "邮件拉取失败"
	case model.MailCheckStatusTimeout:
		return "核对超时"
	case model.MailCheckStatusNotRequired:
		return "不适用"
	default:
		return "待核对"
	}
}

func AdminOrderManagementAnalytics(c *gin.Context) {
	startTime, endTime, err := parseOrderManagementRange(c.Query("range"), c.Query("start_time"), c.Query("end_time"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := model.GetOrderManagementAnalytics(startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	response := dto.OrderManagementAnalyticsResponse{
		Summary: dto.OrderManagementMoneySummary{
			SiteAmount:              centsToAmount(result.Summary.SiteAmountCents),
			ExternalPaidAmount:      centsToAmount(result.Summary.ExternalPaidCents),
			OrderCount:              result.Summary.OrderCount,
			MailVerifiedCount:       result.Summary.MailVerifiedCount,
			MailPendingCount:        result.Summary.MailPendingCount,
			MailErrorCount:          result.Summary.MailErrorCount,
			MailVerifiedRate:        result.Summary.MailVerifiedRate,
			AffiliateUserCount:      result.Summary.AffiliateUserCount,
			AffiliateAmount:         centsToAmount(result.Summary.AffiliateAmountCents),
			WithdrawalPendingCount:  result.Summary.PendingWithdrawalCount,
			WithdrawalPendingAmount: centsToAmount(result.Summary.PendingWithdrawalCents),
		},
		Daily: make([]dto.OrderManagementDailyPoint, 0, len(result.Daily)),
	}
	for _, daily := range result.Daily {
		response.Daily = append(response.Daily, dto.OrderManagementDailyPoint{
			Date:               daily.Date,
			SiteAmount:         centsToAmount(daily.SiteAmountCents),
			ExternalPaidAmount: centsToAmount(daily.ExternalPaidCents),
			OrderCount:         daily.OrderCount,
			MailVerifiedCount:  daily.MailVerifiedCount,
			MailErrorCount:     daily.MailErrorCount,
		})
	}

	common.ApiSuccess(c, response)
}

func AdminOrderManagementOrders(c *gin.Context) {
	startTime, endTime, err := parseOrderManagementRange(c.Query("range"), c.Query("start_time"), c.Query("end_time"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo := getOrderManagementPageInfo(c)
	rows, total, err := model.ListOrderManagementOrders(startTime, endTime, c.Query("mail_status"), c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.PageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items := make([]dto.OrderManagementOrderItem, 0, len(rows))
	for _, row := range rows {
		item := dto.OrderManagementOrderItem{
			Id:                 row.Id,
			OrderType:          "ldxp",
			SessionId:          row.SessionId,
			UserId:             row.UserId,
			Username:           row.Username,
			SiteAmount:         centsToAmount(row.SiteAmountCents),
			ExternalPaidAmount: centsToAmount(row.ExternalPaidCents),
			WorkerOrderNo:      row.WorkerOrderNo,
			MailOrderNo:        row.MailOrderNo,
			MailPaidAmount:     centsToAmount(row.MailAmountCents),
			MailStatus:         row.MailStatus,
			MailStatusText:     mailStatusText(row.MailStatus),
			ErrorCode:          row.ErrorCode,
			ErrorMessage:       row.ErrorMessage,
			CreatedTime:        row.CreatedTime,
			VerifiedTime:       row.VerifiedTime,
		}
		if row.AffiliateCommissionCents > 0 || row.AffiliateInviterId > 0 {
			item.Affiliate = &dto.OrderManagementAffiliateBrief{
				InviterUserId:   row.AffiliateInviterId,
				CommissionMoney: centsToAmount(row.AffiliateCommissionCents),
				Status:          row.AffiliateStatus,
			}
		}
		items = append(items, item)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminOrderManagementMailCheck(c *gin.Context) {
	req := dto.MailCheckRequest{}
	if _, err := decodeOptionalJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	rangeValue, startValue, endValue := mailCheckRequestRangeValues(c, req)
	if strings.TrimSpace(rangeValue) == "" && strings.TrimSpace(startValue) == "" && strings.TrimSpace(endValue) == "" {
		rangeValue = "7d"
	}

	startTime, endTime, err := parseOrderManagementRange(rangeValue, startValue, endValue, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	job := service.DefaultOrderMailCheckRunner().StartBatch(c.Request.Context(), model.OrderMailCheckBatchFilter{StartTime: startTime, EndTime: endTime, Limit: req.Limit})
	// Audit records that the asynchronous mail-check job was accepted/started;
	// final verification outcome is exposed by the job status endpoint.
	recordManageAudit(c, "order.mail_check_batch", map[string]interface{}{
		"job_id": job.JobId,
		"range":  rangeValue,
		"limit":  req.Limit,
	})
	common.ApiSuccess(c, dto.MailCheckResponse{Started: true, JobId: job.JobId, AffectedCount: job.AffectedCount})
}

func AdminOrderManagementOrderMailCheck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的订单 ID")
		return
	}
	job := service.DefaultOrderMailCheckRunner().StartSingle(c.Request.Context(), id)
	// Audit records that the asynchronous mail-check job was accepted/started;
	// final verification outcome is exposed by the job status endpoint.
	recordManageAudit(c, "order.mail_check_single", map[string]interface{}{
		"order_id": id,
		"job_id":   job.JobId,
	})
	common.ApiSuccess(c, dto.MailCheckResponse{Started: true, JobId: job.JobId, AffectedCount: job.AffectedCount})
}

func AdminOrderManagementMailCheckJob(c *gin.Context) {
	job, ok := service.DefaultOrderMailCheckRunner().GetJob(c.Param("job_id"))
	if !ok {
		common.ApiErrorMsg(c, "任务不存在")
		return
	}
	common.ApiSuccess(c, job)
}

func AdminOrderManagementAffiliateStats(c *gin.Context) {
	startTime, endTime, err := parseOrderManagementRange(c.Query("range"), c.Query("start_time"), c.Query("end_time"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := getOrderManagementPageInfo(c)
	result, err := model.GetAffiliateStats(startTime, endTime, c.Query("withdrawal_status"), pageInfo.GetStartIdx(), pageInfo.PageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, affiliateStatsResponseFromModel(result))
}

func AdminAffiliateSourceOrders(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	startTime, endTime, err := parseOrderManagementRange(c.Query("range"), c.Query("start_time"), c.Query("end_time"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := model.GetAffiliateSourceOrders(userId, startTime, endTime, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]dto.AffiliateSourceOrderDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.AffiliateSourceOrderDTO{
			OrderTime:       row.OrderTime,
			InviteeUserId:   row.InviteeUserId,
			InviteeUsername: row.InviteeUsername,
			TradeNo:         row.TradeNo,
			WorkerOrderNo:   row.WorkerOrderNo,
			BaseMoney:       centsToAmount(row.BaseMoneyCents),
			RateBps:         row.RateBps,
			CommissionMoney: centsToAmount(row.CommissionCents),
			MailStatus:      row.MailStatus,
		})
	}
	common.ApiSuccess(c, items)
}

func AdminAffiliateWithdrawalPaid(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现 ID")
		return
	}
	var req dto.WithdrawalActionRequest
	if _, err := decodeOptionalJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	withdrawal, err := model.MarkAffiliateWithdrawalPaid(id, req.AdminRemark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "affiliate.withdrawal_paid", map[string]interface{}{
		"withdrawal_id": withdrawal.WithdrawalId,
		"user_id":       withdrawal.UserId,
		"amount":        centsToAmount(int64(withdrawal.Amount * 100)),
	})
	common.ApiSuccess(c, affiliateWithdrawalDTOFromModel(withdrawal))
}

func AdminAffiliateWithdrawalReject(c *gin.Context) {
	adminAffiliateWithdrawalReject(c)
}

func adminAffiliateWithdrawalReject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现 ID")
		return
	}
	var req dto.WithdrawalActionRequest
	if _, err := decodeOptionalJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.AdminRemark = strings.TrimSpace(req.AdminRemark)
	if req.AdminRemark == "" {
		common.ApiErrorMsg(c, "驳回提现必须填写管理员备注")
		return
	}
	withdrawal, err := model.RejectAffiliateWithdrawal(id, req.AdminRemark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "affiliate.withdrawal_reject", map[string]interface{}{
		"withdrawal_id": withdrawal.WithdrawalId,
		"user_id":       withdrawal.UserId,
		"amount":        centsToAmount(int64(withdrawal.Amount * 100)),
	})
	common.ApiSuccess(c, affiliateWithdrawalDTOFromModel(withdrawal))
}

func decodeOptionalJSONBody(c *gin.Context, v any) (bool, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return false, nil
	}
	if err := common.DecodeJson(c.Request.Body, v); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func mailCheckRequestRangeValues(c *gin.Context, req dto.MailCheckRequest) (string, string, string) {
	startValue := strings.TrimSpace(req.StartTime)
	if startValue == "" {
		startValue = c.Query("start_time")
	}
	endValue := strings.TrimSpace(req.EndTime)
	if endValue == "" {
		endValue = c.Query("end_time")
	}
	return req.Range, startValue, endValue
}

func getOrderManagementPageInfo(c *gin.Context) *common.PageInfo {
	pageInfo := common.GetPageQuery(c)
	if pageValue := strings.TrimSpace(c.Query("page")); pageValue != "" {
		if page, err := strconv.Atoi(pageValue); err == nil && page > 0 {
			pageInfo.Page = page
		}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize < 1 {
		pageInfo.PageSize = common.ItemsPerPage
	}
	return pageInfo
}

func affiliateStatsResponseFromModel(result *model.AffiliateStatsResult) dto.AffiliateStatsResponse {
	if result == nil {
		return dto.AffiliateStatsResponse{}
	}
	response := dto.AffiliateStatsResponse{
		Summary: dto.AffiliateStatsSummaryDTO{
			AffiliateUserCount:                  result.Summary.AffiliateUserCount,
			PeriodCommissionAmount:              centsToAmount(result.Summary.PeriodCommissionCents),
			PendingWithdrawalUserCount:          result.Summary.PendingWithdrawalUserCount,
			PendingWithdrawalAmount:             centsToAmount(result.Summary.PendingWithdrawalCents),
			AvailableWithoutWithdrawalUserCount: result.Summary.AvailableWithoutWithdrawalUserCount,
		},
		Items: make([]dto.AffiliateStatsItemDTO, 0, len(result.Items)),
		Total: result.Total,
	}
	for _, item := range result.Items {
		response.Items = append(response.Items, dto.AffiliateStatsItemDTO{
			UserId:                 item.UserId,
			Username:               item.Username,
			PeriodCommissionAmount: centsToAmount(item.PeriodCommissionCents),
			TotalCommissionAmount:  centsToAmount(item.TotalCommissionCents),
			AvailableAmount:        centsToAmount(item.AvailableCents),
			WithdrawnAmount:        centsToAmount(item.WithdrawnCents),
			Withdrawal:             affiliateWithdrawalDTOFromInfo(item.Withdrawal),
		})
	}
	return response
}

func affiliateWithdrawalDTOFromInfo(withdrawal *model.AffiliateWithdrawalInfo) *dto.AffiliateWithdrawalDTO {
	if withdrawal == nil {
		return nil
	}
	return &dto.AffiliateWithdrawalDTO{
		Id:            withdrawal.Id,
		WithdrawalId:  withdrawal.WithdrawalId,
		Amount:        centsToAmount(withdrawal.AmountCents),
		Contact:       withdrawal.Contact,
		Remark:        withdrawal.Remark,
		Status:        withdrawal.Status,
		CreatedTime:   withdrawal.CreatedTime,
		AdminRemark:   withdrawal.AdminRemark,
		ProcessedTime: withdrawal.ProcessedTime,
	}
}

func affiliateWithdrawalDTOFromModel(withdrawal *model.AffiliateWithdrawal) *dto.AffiliateWithdrawalDTO {
	if withdrawal == nil {
		return nil
	}
	return &dto.AffiliateWithdrawalDTO{
		Id:            withdrawal.Id,
		WithdrawalId:  withdrawal.WithdrawalId,
		Amount:        centsToAmount(int64(withdrawal.Amount * 100)),
		Contact:       withdrawal.Contact,
		Remark:        withdrawal.Remark,
		Status:        withdrawal.Status,
		CreatedTime:   withdrawal.CreatedTime,
		AdminRemark:   withdrawal.AdminRemark,
		ProcessedTime: withdrawal.ProcessedTime,
	}
}
