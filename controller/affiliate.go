package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type createAffiliateWithdrawalRequest struct {
	Amount  float64 `json:"amount"`
	Contact string  `json:"contact"`
	Remark  string  `json:"remark"`
}

type processAffiliateWithdrawalRequest struct {
	AdminRemark string `json:"admin_remark"`
}

func GetAffiliateSummary(c *gin.Context) {
	userID := c.GetInt("id")
	summary, err := model.GetAffiliateSummary(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func CreateAffiliateWithdrawal(c *gin.Context) {
	userID := c.GetInt("id")
	var req createAffiliateWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	withdrawal, err := model.CreateAffiliateWithdrawal(userID, req.Amount, req.Contact, req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, withdrawal)
}

func GetAffiliateWithdrawals(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	result, err := model.GetAffiliateWithdrawals(pageInfo, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(result.Total))
	pageInfo.SetItems(result.Items)
	common.ApiSuccess(c, pageInfo)
}

func MarkAffiliateWithdrawalPaid(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	var req processAffiliateWithdrawalRequest
	_ = c.ShouldBindJSON(&req)
	withdrawal, err := model.MarkAffiliateWithdrawalPaid(id, req.AdminRemark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, withdrawal)
}

func RejectAffiliateWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	var req processAffiliateWithdrawalRequest
	_ = c.ShouldBindJSON(&req)
	withdrawal, err := model.RejectAffiliateWithdrawal(id, req.AdminRemark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, withdrawal)
}
