package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type JeepaySettings struct {
	JeepayEnabled             bool   `json:"JeepayEnabled"`
	JeepayAlipayEnabled       bool   `json:"JeepayAlipayEnabled"`
	JeepayBaseUrl             string `json:"JeepayBaseUrl"`
	JeepayMchNo               string `json:"JeepayMchNo"`
	JeepayAppId               string `json:"JeepayAppId"`
	JeepayAppSecretConfigured bool   `json:"JeepayAppSecretConfigured"`
	JeepayNotifyUrl           string `json:"JeepayNotifyUrl"`
	JeepayReturnUrl           string `json:"JeepayReturnUrl"`
	JeepaySubject             string `json:"JeepaySubject"`
	JeepayBody                string `json:"JeepayBody"`
	JeepayTimeoutMs           int    `json:"JeepayTimeoutMs"`
	JeepayAliDisplayName      string `json:"JeepayAliDisplayName"`
	JeepayAliDisplayColor     string `json:"JeepayAliDisplayColor"`
}

type JeepaySettingsRequest struct {
	JeepayEnabled         bool   `json:"JeepayEnabled"`
	JeepayAlipayEnabled   bool   `json:"JeepayAlipayEnabled"`
	JeepayBaseUrl         string `json:"JeepayBaseUrl"`
	JeepayMchNo           string `json:"JeepayMchNo"`
	JeepayAppId           string `json:"JeepayAppId"`
	JeepayAppSecret       string `json:"JeepayAppSecret"`
	JeepayNotifyUrl       string `json:"JeepayNotifyUrl"`
	JeepayReturnUrl       string `json:"JeepayReturnUrl"`
	JeepaySubject         string `json:"JeepaySubject"`
	JeepayBody            string `json:"JeepayBody"`
	JeepayTimeoutMs       int    `json:"JeepayTimeoutMs"`
	JeepayAliDisplayName  string `json:"JeepayAliDisplayName"`
	JeepayAliDisplayColor string `json:"JeepayAliDisplayColor"`
}

func readJeepaySecretConfigured() bool {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	return strings.TrimSpace(common.Interface2String(common.OptionMap["JeepayAppSecret"])) != ""
}

func GetJeepaySettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": JeepaySettings{
			JeepayEnabled:             setting.JeepayEnabled,
			JeepayAlipayEnabled:       setting.JeepayAlipayEnabled,
			JeepayBaseUrl:             setting.JeepayBaseUrl,
			JeepayMchNo:               setting.JeepayMchNo,
			JeepayAppId:               setting.JeepayAppId,
			JeepayAppSecretConfigured: readJeepaySecretConfigured(),
			JeepayNotifyUrl:           setting.JeepayNotifyUrl,
			JeepayReturnUrl:           setting.JeepayReturnUrl,
			JeepaySubject:             setting.JeepaySubject,
			JeepayBody:                setting.JeepayBody,
			JeepayTimeoutMs:           setting.JeepayTimeoutMs,
			JeepayAliDisplayName:      setting.JeepayAliDisplayName,
			JeepayAliDisplayColor:     setting.JeepayAliDisplayColor,
		},
	})
}

func SaveJeepaySettings(c *gin.Context) {
	var req JeepaySettingsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	updates := map[string]string{
		"JeepayEnabled":         strconv.FormatBool(req.JeepayEnabled),
		"JeepayAlipayEnabled":   strconv.FormatBool(req.JeepayAlipayEnabled),
		"JeepayBaseUrl":         req.JeepayBaseUrl,
		"JeepayMchNo":           req.JeepayMchNo,
		"JeepayAppId":           req.JeepayAppId,
		"JeepayNotifyUrl":       req.JeepayNotifyUrl,
		"JeepayReturnUrl":       req.JeepayReturnUrl,
		"JeepaySubject":         req.JeepaySubject,
		"JeepayBody":            req.JeepayBody,
		"JeepayTimeoutMs":       strconv.Itoa(req.JeepayTimeoutMs),
		"JeepayAliDisplayName":  req.JeepayAliDisplayName,
		"JeepayAliDisplayColor": req.JeepayAliDisplayColor,
	}

	if err := model.UpdateOptionsBulk(updates); err != nil {
		common.ApiError(c, err)
		return
	}

	if strings.TrimSpace(req.JeepayAppSecret) != "" {
		if err := model.UpdateOption("JeepayAppSecret", req.JeepayAppSecret); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "保存成功",
	})
}
