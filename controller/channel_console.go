package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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

func GetChannelConsoleCliProxyStatus(c *gin.Context) {
	result, err := channelconsole.GetCliProxyStatus(c.Request.Context())
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func ListChannelConsoleCliProxyAuthFiles(c *gin.Context) {
	result, err := channelconsole.ListCliProxyAuthFiles(c.Request.Context())
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func UploadChannelConsoleCliProxyAuthFile(c *gin.Context) {
	req := channelconsole.CliProxyUploadAuthFileRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := channelconsole.UploadCliProxyAuthFile(c.Request.Context(), req); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"status": "ok"})
}

func DeleteChannelConsoleCliProxyAuthFiles(c *gin.Context) {
	req := channelconsole.CliProxyDeleteAuthFilesRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := channelconsole.DeleteCliProxyAuthFiles(c.Request.Context(), req.Names)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func GetChannelConsoleCliProxyAuthURL(c *gin.Context) {
	result, err := channelconsole.GetCliProxyAuthURL(c.Request.Context(), c.Query("provider"))
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

func CreateChannelConsoleCredentialPool(c *gin.Context) {
	req := channelconsole.CreateCredentialPoolRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	pool, err := channelconsole.CreateCredentialPool(req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, pool)
}

func ListChannelConsoleCredentialPools(c *gin.Context) {
	result, err := channelconsole.ListCredentialPools()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetChannelConsoleCredentialPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid pool id")
		return
	}
	result, err := channelconsole.GetCredentialPoolDetail(id)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func AddChannelConsoleThirdPartyCredential(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid pool id")
		return
	}
	req := channelconsole.AddThirdPartyCredentialRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := channelconsole.AddThirdPartyCredentialToPool(c.Request.Context(), id, req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func AddChannelConsoleCliProxyCredential(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid pool id")
		return
	}
	req := channelconsole.AddCliProxyCredentialRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := channelconsole.AddCliProxyCredentialToPool(c.Request.Context(), id, req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func BatchDeleteChannelConsoleCredentials(c *gin.Context) {
	req := channelconsole.CredentialBatchDeleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := channelconsole.BatchDeleteCredentials(req.IDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func BatchDeleteChannelConsoleChannels(c *gin.Context) {
	req := channelconsole.ManagedChannelBatchDeleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	result, err := channelconsole.BatchDeleteManagedChannels(req.IDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if result.Deleted > 0 {
		model.InitChannelCache()
	}

	common.ApiSuccess(c, result)
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
