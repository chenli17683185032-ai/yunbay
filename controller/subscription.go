package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- Shared types ----

type SubscriptionPlanDTO struct {
	Plan  model.SubscriptionPlan       `json:"plan"`
	Stats *model.SubscriptionPlanStats `json:"stats,omitempty"`
}

type BillingPreferenceRequest struct {
	BillingPreference string `json:"billing_preference"`
}

type SubscriptionBalancePayRequest struct {
	PlanId int `json:"plan_id"`
}

// ---- User APIs ----

func GetSubscriptionPlans(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiSuccess(c, []SubscriptionPlanDTO{})
		return
	}

	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ? AND (plan_kind = ? OR plan_kind = '' OR plan_kind IS NULL)", true, model.SubscriptionPlanKindSubscription).
		Order("sort_order desc, id desc").
		Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		p.NormalizeDefaults()
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

func GetSubscriptionSelf(c *gin.Context) {
	userId := c.GetInt("id")
	settingMap, _ := model.GetUserSetting(userId, false)
	pref := common.NormalizeBillingPreference(settingMap.BillingPreference)

	// Get all subscriptions (including expired)
	allSubscriptions, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		allSubscriptions = []model.SubscriptionSummary{}
	}

	// Get active subscriptions for backward compatibility
	activeSubscriptions, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		activeSubscriptions = []model.SubscriptionSummary{}
	}

	common.ApiSuccess(c, gin.H{
		"billing_preference": pref,
		"subscriptions":      activeSubscriptions, // all active subscriptions
		"all_subscriptions":  allSubscriptions,    // all subscriptions including expired
	})
}

func UpdateSubscriptionPreference(c *gin.Context) {
	userId := c.GetInt("id")
	var req BillingPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	pref := common.NormalizeBillingPreference(req.BillingPreference)

	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	current := user.GetSetting()
	current.BillingPreference = pref
	user.SetSetting(current)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"billing_preference": pref})
}

func SubscriptionRequestBalancePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userId := c.GetInt("id")
	var req SubscriptionBalancePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if err := model.PurchaseSubscriptionWithBalance(userId, req.PlanId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func rejectValuePackagePlanForSubscriptionPurchase(c *gin.Context, plan *model.SubscriptionPlan) bool {
	if plan != nil && plan.IsValuePackage() {
		common.ApiErrorMsg(c, "超值套餐仅支持联动小铺购买")
		return true
	}
	return false
}

// ---- Admin APIs ----

func AdminListSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	statsMap, err := model.GetSubscriptionPlanStatsMap(common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		p.NormalizeDefaults()
		stat := statsMap[p.Id]
		result = append(result, SubscriptionPlanDTO{
			Plan:  p,
			Stats: &stat,
		})
	}
	common.ApiSuccess(c, result)
}

type AdminUpsertSubscriptionPlanRequest struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

func normalizeAndValidateSubscriptionPlanRequest(plan *model.SubscriptionPlan) string {
	if plan == nil {
		return "参数错误"
	}
	plan.PlanKind = strings.TrimSpace(plan.PlanKind)
	plan.PackageType = strings.TrimSpace(plan.PackageType)
	plan.ModelGroup = strings.TrimSpace(plan.ModelGroup)
	plan.Benefits = strings.TrimSpace(plan.Benefits)
	plan.LdxpProductUrl = strings.TrimSpace(plan.LdxpProductUrl)
	plan.LdxpProductName = strings.TrimSpace(plan.LdxpProductName)
	plan.LdxpProductRef = strings.TrimSpace(plan.LdxpProductRef)
	requestedTotalAmount := plan.TotalAmount
	requestedLimit5hAmount := plan.Limit5hAmount
	requestedLimit7dAmount := plan.Limit7dAmount
	plan.NormalizeDefaults()
	switch plan.PlanKind {
	case model.SubscriptionPlanKindSubscription, model.SubscriptionPlanKindValuePackage:
	default:
		return "套餐类型无效"
	}
	if plan.PlanKind != model.SubscriptionPlanKindValuePackage {
		if requestedTotalAmount < 0 {
			return "总额度不能为负数"
		}
		return ""
	}
	if requestedTotalAmount <= 0 {
		return "超值套餐总额度必须大于0"
	}

	plan.UpgradeGroup = ""
	switch plan.PackageType {
	case model.ValuePackageTypeDay:
		plan.PackageLevel = model.ValuePackageLevelDay
	case model.ValuePackageTypeWeek:
		plan.PackageLevel = model.ValuePackageLevelWeek
	case model.ValuePackageTypeMonth:
		plan.PackageLevel = model.ValuePackageLevelMonth
	default:
		return "套餐类型必须是 day、week 或 month"
	}
	if plan.ModelGroup == "" {
		return "套餐模型分组不能为空"
	}
	if plan.ConcurrencyLimit <= 0 {
		return "并发限制必须大于0"
	}
	if requestedLimit5hAmount < 0 || requestedLimit7dAmount < 0 {
		return "套餐额度不能为负数"
	}
	if (plan.PackageType == model.ValuePackageTypeDay || plan.PackageType == model.ValuePackageTypeWeek) && requestedLimit7dAmount != 0 {
		return "日卡和周卡不支持7天阶段限额"
	}
	if plan.PackageType == model.ValuePackageTypeMonth && requestedLimit7dAmount > requestedTotalAmount {
		return "7天阶段额度不能大于30天总额度"
	}
	if plan.PackageType == model.ValuePackageTypeMonth && requestedLimit5hAmount > 0 && requestedLimit7dAmount > 0 && requestedLimit7dAmount < requestedLimit5hAmount {
		return "7天额度不能小于5小时额度"
	}
	if plan.Enabled {
		if plan.LdxpProductUrl == "" || plan.LdxpProductName == "" || plan.LdxpProductAmount <= 0 {
			return "启用超值套餐时必须配置 LDXP 商品链接、名称和金额"
		}
	}
	return ""
}

func AdminCreateSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Plan.Id = 0
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	if req.Plan.AllowBalancePay == nil {
		req.Plan.AllowBalancePay = common.GetPointer(true)
	}
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.PlanKind != model.SubscriptionPlanKindValuePackage && req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}
	if msg := normalizeAndValidateSubscriptionPlanRequest(&req.Plan); msg != "" {
		common.ApiErrorMsg(c, msg)
		return
	}
	requestedEnabled := req.Plan.Enabled
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Omit("Id").Create(&req.Plan).Error; err != nil {
			return err
		}
		if !requestedEnabled {
			if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", req.Plan.Id).Update("enabled", false).Error; err != nil {
				return err
			}
			req.Plan.Enabled = false
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(req.Plan.Id)
	common.ApiSuccess(c, req.Plan)
}

func AdminUpdateSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	req.Plan.Id = id
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.PlanKind != model.SubscriptionPlanKindValuePackage && req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}
	if msg := normalizeAndValidateSubscriptionPlanRequest(&req.Plan); msg != "" {
		common.ApiErrorMsg(c, msg)
		return
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		// update plan (allow zero values updates with map)
		updateMap := map[string]interface{}{
			"title":                      req.Plan.Title,
			"subtitle":                   req.Plan.Subtitle,
			"price_amount":               req.Plan.PriceAmount,
			"currency":                   req.Plan.Currency,
			"duration_unit":              req.Plan.DurationUnit,
			"duration_value":             req.Plan.DurationValue,
			"custom_seconds":             req.Plan.CustomSeconds,
			"enabled":                    req.Plan.Enabled,
			"sort_order":                 req.Plan.SortOrder,
			"plan_kind":                  req.Plan.PlanKind,
			"package_type":               req.Plan.PackageType,
			"package_level":              req.Plan.PackageLevel,
			"model_group":                req.Plan.ModelGroup,
			"concurrency_limit":          req.Plan.ConcurrencyLimit,
			"limit_5h_amount":            req.Plan.Limit5hAmount,
			"limit_7d_amount":            req.Plan.Limit7dAmount,
			"benefits":                   req.Plan.Benefits,
			"ldxp_product_url":           req.Plan.LdxpProductUrl,
			"ldxp_product_name":          req.Plan.LdxpProductName,
			"ldxp_product_amount":        req.Plan.LdxpProductAmount,
			"ldxp_product_ref":           req.Plan.LdxpProductRef,
			"ldxp_session_ttl_seconds":   req.Plan.LdxpSessionTTLSeconds,
			"stripe_price_id":            req.Plan.StripePriceId,
			"creem_product_id":           req.Plan.CreemProductId,
			"waffo_pancake_product_id":   req.Plan.WaffoPancakeProductId,
			"max_purchase_per_user":      req.Plan.MaxPurchasePerUser,
			"total_amount":               req.Plan.TotalAmount,
			"upgrade_group":              req.Plan.UpgradeGroup,
			"quota_reset_period":         req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
			"updated_at":                 common.GetTimestamp(),
		}
		if req.Plan.AllowBalancePay != nil {
			updateMap["allow_balance_pay"] = *req.Plan.AllowBalancePay
		}
		result := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Updates(updateMap)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var count int64
			if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("套餐不存在")
			}
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminUpdateSubscriptionPlanStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

type AdminCreateSubscriptionRedemptionsRequest struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	ExpiredTime int64  `json:"expired_time"`
}

func AdminUpdateSubscriptionPlanStatus(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var plan model.SubscriptionPlan
	if err := model.DB.Where("id = ?", id).First(&plan).Error; err != nil {
		common.ApiErrorMsg(c, "套餐不存在")
		return
	}
	plan.Enabled = *req.Enabled
	if plan.Enabled {
		if msg := normalizeAndValidateSubscriptionPlanRequest(&plan); msg != "" {
			common.ApiErrorMsg(c, msg)
			return
		}
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Update("enabled", *req.Enabled).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

func AdminCreateSubscriptionRedemptions(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var plan model.SubscriptionPlan
	if err := model.DB.Where("id = ?", id).First(&plan).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	plan.NormalizeDefaults()

	var req AdminCreateSubscriptionRedemptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = plan.Title + "兑换码"
	}
	if utf8.RuneCountInString(req.Name) == 0 || utf8.RuneCountInString(req.Name) > 20 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
		return
	}
	if req.Count <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
		return
	}
	if req.Count > 100 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
		return
	}
	if valid, msg := validateExpiredTime(c, req.ExpiredTime); !valid {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}

	keys := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		key := common.GetUUID()
		redemption := map[string]interface{}{
			"user_id":         c.GetInt("id"),
			"name":            req.Name,
			"key":             key,
			"created_time":    common.GetTimestamp(),
			"quota":           0,
			"type":            model.RedemptionTypeSubscription,
			"plan_id":         plan.Id,
			"kind":            model.RedemptionKindPromoCredit,
			"amount":          0,
			"money":           0,
			"count_as_top_up": false,
			"source":          model.RedemptionSourcePromo,
			"batch_id":        model.GenerateRedemptionBatchId(model.RedemptionSourcePromo, 0, common.GetTimestamp()),
			"expired_time":    req.ExpiredTime,
			"status":          common.RedemptionCodeStatusEnabled,
		}
		if err := model.DB.Model(&model.Redemption{}).Create(redemption).Error; err != nil {
			common.SysError("failed to insert subscription redemption: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgRedemptionCreateFailed),
				"data":    keys,
			})
			return
		}
		keys = append(keys, key)
	}
	recordManageAudit(c, "redemption.subscription.create", map[string]interface{}{
		"name":       req.Name,
		"count":      req.Count,
		"plan_title": plan.Title,
	})
	common.ApiSuccess(c, keys)
}

type AdminBindSubscriptionRequest struct {
	UserId int `json:"user_id"`
	PlanId int `json:"plan_id"`
}

func AdminBindSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req AdminBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(req.UserId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// ---- Admin: user subscription management ----

func AdminListUserSubscriptions(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	subs, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, subs)
}

type AdminCreateUserSubscriptionRequest struct {
	PlanId int `json:"plan_id"`
}

// AdminCreateUserSubscription creates a new user subscription from a plan (no payment).
func AdminCreateUserSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req AdminCreateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(userId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminInvalidateUserSubscription cancels a user subscription immediately.
func AdminInvalidateUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminInvalidateUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminDeleteUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}
