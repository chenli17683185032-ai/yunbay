package controller

import (
	"net/http"
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
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channelconsole.PreviewImport(req.RawInput),
	})
}

func CommitChannelConsoleImport(c *gin.Context) {
	req := channelconsole.ImportCommitRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := channelconsole.CommitImport(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func ListChannelConsoleChannels(c *gin.Context) {
	channels, err := model.GetAllChannels(0, 100, true, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channels,
	})
}

func GetChannelConsoleChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid channel id",
		})
		return
	}

	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	meta, _ := model.GetChannelConsoleChannelByChannelID(id)
	prices, _ := model.ListChannelConsoleModelPrices(id)
	healthChecks, _ := model.ListChannelConsoleHealthChecks(id, 50)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"channel":       channel,
			"console":       meta,
			"prices":        prices,
			"health_checks": healthChecks,
		},
	})
}
