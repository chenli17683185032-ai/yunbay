package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

const jeepayAliCashierMethod = "jeepay_ali_cashier"

var jeepaySign = service.SignJeepayParams

func RequestJeepayPay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isJeepayAlipayTopUpEnabled() {
		common.ApiErrorMsg(c, "Jeepay 支付未启用")
		return
	}

	var req EpayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.PaymentMethod != jeepayAliCashierMethod {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}
	if req.Amount < getMinTopup() {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", getMinTopup()))
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}

	tradeNo := fmt.Sprintf("JEPAY-%d-%d-%s", id, time.Now().UnixMilli(), randstr.String(6))
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   jeepayAliCashierMethod,
		PaymentProvider: model.PaymentProviderJeepay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Jeepay 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	client := service.NewJeepayClient()
	notifyURL := strings.TrimSpace(setting.JeepayNotifyUrl)
	if notifyURL == "" {
		notifyURL = strings.TrimRight(service.GetCallbackAddress(), "/") + "/api/jeepay/notify"
	}
	returnURL := strings.TrimSpace(setting.JeepayReturnUrl)
	if returnURL == "" {
		returnURL = paymentReturnPath("/console/topup?show_history=true")
	}
	paymentURL, err := client.CreateAliCashierOrder(c.Request.Context(), service.JeepayUnifiedOrderParams{
		MchOrderNo: tradeNo,
		WayCode:    "QR_CASHIER",
		AmountFen:  decimal.NewFromFloat(payMoney).Mul(decimal.NewFromInt(100)).IntPart(),
		Subject:    strings.TrimSpace(setting.JeepaySubject),
		Body:       strings.TrimSpace(setting.JeepayBody),
		NotifyURL:  notifyURL,
		ReturnURL:  returnURL,
	})
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		logger.LogError(c.Request.Context(), fmt.Sprintf("Jeepay 下单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "success",
		"data": gin.H{
			"payment_url": paymentURL,
		},
	})
}

func JeepayNotify(c *gin.Context) {
	if !isJeepayAlipayTopUpEnabled() {
		c.String(http.StatusOK, "FAIL")
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Jeepay 回调解析失败 client_ip=%s error=%q", c.ClientIP(), err.Error()))
		c.String(http.StatusOK, "FAIL")
		return
	}
	if !service.VerifyJeepayParams(payload, setting.JeepayAppSecret) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Jeepay 回调验签失败 trade_no=%s client_ip=%s", payload["mchOrderNo"], c.ClientIP()))
		c.String(http.StatusOK, "FAIL")
		return
	}
	if payload["mchNo"] != setting.JeepayMchNo || payload["appId"] != setting.JeepayAppId {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Jeepay 回调商户不匹配 trade_no=%s mch_no=%s app_id=%s client_ip=%s", payload["mchOrderNo"], payload["mchNo"], payload["appId"], c.ClientIP()))
		c.String(http.StatusOK, "FAIL")
		return
	}

	tradeNo := strings.TrimSpace(payload["mchOrderNo"])
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderJeepay {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Jeepay 回调订单不存在或网关不匹配 trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		c.String(http.StatusOK, "FAIL")
		return
	}

	expectedAmount := decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromInt(100)).IntPart()
	if payload["amount"] != fmt.Sprintf("%d", expectedAmount) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Jeepay 回调金额不匹配 trade_no=%s expected=%d actual=%s client_ip=%s", tradeNo, expectedAmount, payload["amount"], c.ClientIP()))
		c.String(http.StatusOK, "FAIL")
		return
	}
	if payload["state"] != "2" {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("Jeepay 回调忽略非成功状态 trade_no=%s state=%s client_ip=%s", tradeNo, payload["state"], c.ClientIP()))
		c.String(http.StatusOK, "FAIL")
		return
	}

	if err := model.RechargeJeepay(tradeNo, payload, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Jeepay 入账失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		c.String(http.StatusOK, "FAIL")
		return
	}

	c.String(http.StatusOK, "SUCCESS")
}
