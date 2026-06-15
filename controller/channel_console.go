package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/channelconsole"
	"github.com/gin-gonic/gin"
)

type channelConsolePreviewRequest struct {
	RawInput string `json:"raw_input"`
}

func PreviewChannelConsoleImport(c *gin.Context) {
	req := channelConsolePreviewRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, channelconsole.PreviewImport(req.RawInput))
}

func CommitChannelConsoleImport(c *gin.Context) {
	req := channelconsole.ImportCommitRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := channelconsole.CommitImport(req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.ApiSuccess(c, result)
}

func ListChannelConsoleChannels(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	channels, err := channelconsole.ListManagedChannels(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), pageInfo.GetPage())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, channels)
}

func GetChannelConsoleChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}

	detail, err := channelconsole.GetManagedChannelDetail(id)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.ApiSuccess(c, detail)
}

func CheckChannelConsoleHealth(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}

	check, err := RunChannelConsoleHealthCheck(id, channelconsole.HealthCheckTypeManual)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.ApiSuccess(c, check)
}
