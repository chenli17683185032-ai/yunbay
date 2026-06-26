package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestCreateAliCashierOrderIncludesJeepayRequiredSignatureFields(t *testing.T) {
	originalBaseURL := setting.JeepayBaseUrl
	originalMchNo := setting.JeepayMchNo
	originalAppID := setting.JeepayAppId
	originalAppSecret := setting.JeepayAppSecret
	originalTimeout := setting.JeepayTimeoutMs
	t.Cleanup(func() {
		setting.JeepayBaseUrl = originalBaseURL
		setting.JeepayMchNo = originalMchNo
		setting.JeepayAppId = originalAppID
		setting.JeepayAppSecret = originalAppSecret
		setting.JeepayTimeoutMs = originalTimeout
	})

	const appSecret = "secret_123"
	var captured map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/pay/unifiedOrder", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))

		require.Equal(t, "1.0", captured["version"])
		require.Equal(t, "MD5", captured["signType"])
		require.NotEmpty(t, captured["reqTime"])
		require.NotEmpty(t, captured["sign"])

		signParams := make(map[string]string)
		for key, value := range captured {
			if key == "sign" || value == nil || strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
				continue
			}
			signParams[key] = fmt.Sprintf("%v", value)
		}
		require.Equal(t, SignJeepayParams(signParams, appSecret), captured["sign"])

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"code":"0","data":{"payData":{"payUrl":"https://cashier.example.com/pay/TEST"}}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	setting.JeepayBaseUrl = server.URL
	setting.JeepayMchNo = "mch_123"
	setting.JeepayAppId = "app_123"
	setting.JeepayAppSecret = appSecret
	setting.JeepayTimeoutMs = 5000

	paymentURL, err := NewJeepayClient().CreateAliCashierOrder(context.Background(), JeepayUnifiedOrderParams{
		MchOrderNo: "test-order-001",
		WayCode:    "ALI_QR",
		AmountFen:  100,
		Subject:    "云贝充值",
		Body:       "云贝账户充值",
		NotifyURL:  "https://yunbay.xyz/api/jeepay/notify",
		ReturnURL:  "https://yunbay.xyz/wallet?show_history=true",
	})

	require.NoError(t, err)
	require.Equal(t, "https://cashier.example.com/pay/TEST", paymentURL)
}
