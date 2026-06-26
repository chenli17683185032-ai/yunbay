package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting"
)

type JeepayUnifiedOrderParams struct {
	MchOrderNo string
	WayCode    string
	AmountFen  int64
	Subject    string
	Body       string
	NotifyURL  string
	ReturnURL  string
}

type jeepayUnifiedOrderRequest struct {
	MchNo      string `json:"mchNo"`
	AppId      string `json:"appId"`
	MchOrderNo string `json:"mchOrderNo"`
	WayCode    string `json:"wayCode"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	ClientIp   string `json:"clientIp,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Body       string `json:"body,omitempty"`
	NotifyUrl  string `json:"notifyUrl,omitempty"`
	ReturnUrl  string `json:"returnUrl,omitempty"`
	Version    string `json:"version"`
	SignType   string `json:"signType"`
	ReqTime    string `json:"reqTime"`
	Sign       string `json:"sign"`
}

type JeepayClient struct {
	baseURL    string
	mchNo      string
	appID      string
	appSecret  string
	timeout    time.Duration
	httpClient *http.Client
}

func NewJeepayClient() *JeepayClient {
	timeout := time.Duration(setting.JeepayTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	client := GetHttpClient()
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &JeepayClient{
		baseURL:    strings.TrimRight(setting.JeepayBaseUrl, "/"),
		mchNo:      setting.JeepayMchNo,
		appID:      setting.JeepayAppId,
		appSecret:  setting.JeepayAppSecret,
		timeout:    timeout,
		httpClient: client,
	}
}

func (c *JeepayClient) CreateAliCashierOrder(ctx context.Context, params JeepayUnifiedOrderParams) (string, error) {
	request := jeepayUnifiedOrderRequest{
		MchNo:      c.mchNo,
		AppId:      c.appID,
		MchOrderNo: params.MchOrderNo,
		WayCode:    params.WayCode,
		Amount:     params.AmountFen,
		Currency:   "cny",
		Subject:    params.Subject,
		Body:       params.Body,
		NotifyUrl:  params.NotifyURL,
		ReturnUrl:  params.ReturnURL,
		Version:    "1.0",
		SignType:   "MD5",
		ReqTime:    fmt.Sprintf("%d", time.Now().Unix()),
	}

	signParams := map[string]string{
		"mchNo":      request.MchNo,
		"appId":      request.AppId,
		"mchOrderNo": request.MchOrderNo,
		"wayCode":    request.WayCode,
		"amount":     fmt.Sprintf("%d", request.Amount),
		"currency":   request.Currency,
		"subject":    request.Subject,
		"body":       request.Body,
		"notifyUrl":  request.NotifyUrl,
		"returnUrl":  request.ReturnUrl,
		"version":    request.Version,
		"signType":   request.SignType,
		"reqTime":    request.ReqTime,
	}
	request.Sign = SignJeepayParams(signParams, c.appSecret)

	bodyBytes, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.baseURL+"/api/pay/unifiedOrder", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jeepay request failed: status=%d", resp.StatusCode)
	}

	code := fmt.Sprintf("%v", payload["code"])
	if code != "0" && code != "200" {
		msg := fmt.Sprintf("%v", payload["msg"])
		if strings.TrimSpace(msg) == "" {
			msg = "jeepay create order failed"
		}
		return "", fmt.Errorf("%s", msg)
	}

	data, _ := payload["data"].(map[string]interface{})
	payData, _ := data["payData"].(interface{})
	paymentURL := extractJeepayPaymentURL(payData)
	if paymentURL == "" {
		return "", fmt.Errorf("jeepay payData missing payment url")
	}
	return paymentURL, nil
}

func extractJeepayPaymentURL(payData interface{}) string {
	switch value := payData.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]interface{}:
		for _, key := range []string{"payUrl", "paymentUrl", "cashierUrl", "qrCodeUrl", "codeUrl", "url"} {
			if raw, ok := value[key]; ok {
				if url := strings.TrimSpace(fmt.Sprintf("%v", raw)); url != "" {
					return url
				}
			}
		}
	}
	return ""
}
