package controller

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

func GetAllRedemptions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	redemptions, total, err := model.GetAllRedemptions(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
	return
}

func SearchRedemptions(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	redemptions, total, err := model.SearchRedemptions(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	redemption, err := model.GetRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    redemption,
	})
	return
}

func AddRedemption(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}

	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if utf8.RuneCountInString(redemption.Name) == 0 || utf8.RuneCountInString(redemption.Name) > 20 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
		return
	}
	if redemption.Count <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
		return
	}
	if redemption.Count > 100 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
		return
	}
	if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}
	if err := model.NormalizeRedemptionForCreate(&redemption); err != nil {
		switch {
		case errors.Is(err, model.ErrRedemptionUnsupportedKind):
			common.ApiErrorI18n(c, i18n.MsgRedemptionUnsupportedKind)
		case redemption.Kind == model.RedemptionKindPaidTopUp:
			common.ApiErrorI18n(c, i18n.MsgRedemptionPaidTopupInvalid)
		case redemption.Kind == model.RedemptionKindPromoCredit:
			common.ApiErrorI18n(c, i18n.MsgRedemptionPromoInvalid)
		default:
			common.ApiErrorI18n(c, i18n.MsgRedemptionInvalid)
		}
		return
	}
	var keys []string
	for i := 0; i < redemption.Count; i++ {
		key := common.GetUUID()
		cleanRedemption := model.Redemption{
			UserId:       c.GetInt("id"),
			Name:         redemption.Name,
			Key:          key,
			Status:       common.RedemptionCodeStatusEnabled,
			CreatedTime:  common.GetTimestamp(),
			Quota:        redemption.Quota,
			Kind:         redemption.Kind,
			Amount:       redemption.Amount,
			Money:        redemption.Money,
			CountAsTopUp: redemption.CountAsTopUp,
			Source:       redemption.Source,
			BatchId:      redemption.BatchId,
			ExpiredTime:  redemption.ExpiredTime,
		}
		err = cleanRedemption.Insert()
		if err != nil {
			common.SysError("failed to insert redemption: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgRedemptionCreateFailed),
				"data":    keys,
			})
			return
		}
		keys = append(keys, key)
	}
	recordManageAudit(c, "redemption.create", map[string]interface{}{
		"name":           redemption.Name,
		"count":          redemption.Count,
		"quota":          logger.LogQuota(redemption.Quota),
		"kind":           redemption.Kind,
		"amount":         redemption.Amount,
		"money":          redemption.Money,
		"count_as_topup": redemption.CountAsTopUp,
		"source":         redemption.Source,
		"batch_id":       redemption.BatchId,
	})
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "",
		"data":     keys,
		"batch_id": redemption.BatchId,
	})
	return
}

func ExportRedemptions(c *gin.Context) {
	batchId := strings.TrimSpace(c.Query("batch_id"))
	if batchId == "" {
		common.ApiErrorI18n(c, i18n.MsgRedemptionExportBatchRequired)
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "txt")))
	if format == "" {
		format = "txt"
	}
	if format != "txt" && format != "csv" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	redemptions, err := model.GetRedemptionsByBatchId(batchId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(redemptions) == 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionExportEmpty)
		return
	}

	var contentType string
	var payload []byte
	if format == "csv" {
		contentType = "text/csv; charset=utf-8"
		payload, err = buildRedemptionCSV(redemptions)
	} else {
		contentType = "text/plain; charset=utf-8"
		payload = []byte(buildRedemptionTXT(redemptions))
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", `attachment; filename="`+safeRedemptionExportFilename(batchId, format)+`"`)
	c.Status(http.StatusOK)
	if _, err := c.Writer.Write(payload); err != nil {
		common.SysError("failed to export redemptions: " + err.Error())
		return
	}

	exportedTime := common.GetTimestamp()
	if err := model.MarkRedemptionsExported(batchId, exportedTime); err != nil {
		common.SysError("failed to mark redemptions exported: " + err.Error())
		return
	}
	recordManageAudit(c, "redemption.export", map[string]interface{}{
		"batch_id": batchId,
		"count":    len(redemptions),
		"format":   format,
	})
}

func safeRedemptionExportFilename(batchId string, format string) string {
	batchSlug := sanitizeRedemptionFilenamePart(batchId)
	formatSlug := sanitizeRedemptionFilenamePart(format)
	if formatSlug == "" {
		formatSlug = "txt"
	}
	filename := "redemptions-" + batchSlug + "." + formatSlug
	if len(filename) > 128 {
		keepBatchLen := 128 - len("redemptions-") - len(".") - len(formatSlug)
		if keepBatchLen < 0 {
			keepBatchLen = 0
		}
		filename = "redemptions-" + batchSlug[:keepBatchLen] + "." + formatSlug
	}
	return filename
}

func sanitizeRedemptionFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "export"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '_', r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func buildRedemptionTXT(redemptions []*model.Redemption) string {
	keys := make([]string, 0, len(redemptions))
	for _, redemption := range redemptions {
		keys = append(keys, redemption.Key)
	}
	return strings.Join(keys, "\n") + "\n"
}

func buildRedemptionCSV(redemptions []*model.Redemption) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"key", "name", "kind", "amount", "money", "quota", "batch_id", "source", "expired_time"}); err != nil {
		return nil, err
	}
	for _, redemption := range redemptions {
		if err := writer.Write([]string{
			redemption.Key,
			redemption.Name,
			redemption.Kind,
			strconv.FormatInt(redemption.Amount, 10),
			strconv.FormatFloat(redemption.Money, 'f', -1, 64),
			strconv.Itoa(redemption.Quota),
			redemption.BatchId,
			redemption.Source,
			strconv.FormatInt(redemption.ExpiredTime, 10),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DeleteRedemption(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func UpdateRedemption(c *gin.Context) {
	statusOnly := c.Query("status_only")
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cleanRedemption, err := model.GetRedemptionById(redemption.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if statusOnly == "" {
		if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
			return
		}
		// If you add more fields, please also update redemption.Update()
		cleanRedemption.Name = redemption.Name
		cleanRedemption.Quota = redemption.Quota
		cleanRedemption.ExpiredTime = redemption.ExpiredTime
	}
	if statusOnly != "" {
		cleanRedemption.Status = redemption.Status
	}
	err = cleanRedemption.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanRedemption,
	})
	return
}

func DeleteInvalidRedemption(c *gin.Context) {
	rows, err := model.DeleteInvalidRedemptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
	return
}

func validateExpiredTime(c *gin.Context, expired int64) (bool, string) {
	if expired != 0 && expired < common.GetTimestamp() {
		return false, i18n.T(c, i18n.MsgRedemptionExpireTimeInvalid)
	}
	return true, ""
}
