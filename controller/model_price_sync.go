package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func PreviewModelPriceSync(c *gin.Context) {
	req, ok := decodeModelPriceSyncRequest(c)
	if !ok {
		return
	}

	result, err := service.PreviewSelectedModelPriceSync(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func ApplyModelPriceSync(c *gin.Context) {
	req, ok := decodeModelPriceSyncRequest(c)
	if !ok {
		return
	}

	result, err := service.ApplySelectedModelPriceSync(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func decodeModelPriceSyncRequest(c *gin.Context) (service.ModelPriceSyncRequest, bool) {
	var req service.ModelPriceSyncRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求参数格式错误"})
		return req, false
	}
	if req.OpenRouterChannelID <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请选择 OpenRouter 渠道"})
		return req, false
	}
	models := make([]string, 0, len(req.Models))
	seen := make(map[string]struct{}, len(req.Models))
	for _, rawModel := range req.Models {
		modelName := strings.TrimSpace(rawModel)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	if len(models) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请选择要同步的模型"})
		return req, false
	}
	req.Models = models
	return req, true
}
