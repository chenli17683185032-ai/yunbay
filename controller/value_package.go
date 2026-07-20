package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type valuePackageLdxpSessionRequest struct {
	ConfirmedCover bool `json:"confirmed_cover"`
}

type valuePackageActivateRequest struct {
	UserSubscriptionId int `json:"user_subscription_id"`
}

type valuePackageResetQuotaRequest struct {
	UserSubscriptionId *int `json:"user_subscription_id"`
}

type valuePackageWalletFallbackRequest struct {
	Enabled *bool `json:"enabled"`
}

func GetValuePackagePlans(c *gin.Context) {
	userId := c.GetInt("id")
	plans, err := model.GetValuePackagePlansForUser(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	state, err := model.GetValuePackageState(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"plans": plans, "state": state})
}

func GetValuePackageSelf(c *gin.Context) {
	userId := c.GetInt("id")
	state, err := model.GetValuePackageState(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func GetValuePackagePurchaseIntent(c *gin.Context) {
	userId := c.GetInt("id")
	planId, _ := strconv.Atoi(c.Param("plan_id"))
	if planId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	confirmedCover := c.Query("confirmed_cover") == "true" || c.Query("confirmed_cover") == "1"
	intent, err := model.CheckValuePackagePurchaseIntent(userId, planId, confirmedCover)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, intent)
}

func CreateValuePackageLdxpSession(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userId := c.GetInt("id")
	planId, _ := strconv.Atoi(c.Param("plan_id"))
	if planId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req valuePackageLdxpSessionRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	cfg, err := service.LoadLdxpConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view, order, err := service.CreateLdxpValuePackageSession(userId, planId, req.ConfirmedCover, cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"session": view, "order_id": order.Id, "trade_no": order.TradeNo})
}

func ActivateValuePackageSelf(c *gin.Context) {
	userId := c.GetInt("id")
	var req valuePackageActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserSubscriptionId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	state, err := model.ActivateValuePackage(userId, req.UserSubscriptionId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func DeactivateValuePackageSelf(c *gin.Context) {
	userId := c.GetInt("id")
	state, err := model.DeactivateValuePackage(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func UpdateValuePackageWalletFallbackSelf(c *gin.Context) {
	userId := c.GetInt("id")
	var req valuePackageWalletFallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	state, err := model.UpdateValuePackageWalletFallback(userId, *req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func ResetValuePackageQuotaSelf(c *gin.Context) {
	userId := c.GetInt("id")
	var req valuePackageResetQuotaRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	userSubscriptionId := 0
	if req.UserSubscriptionId != nil {
		if *req.UserSubscriptionId <= 0 {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
		userSubscriptionId = *req.UserSubscriptionId
	}
	state, err := model.ConsumeValuePackageResetCount(userId, userSubscriptionId, 0, userId, "user reset quota")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}
