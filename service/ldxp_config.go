package service

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type LdxpProductConfig struct {
	// Amount is the Yunbei top-up face value/tier and must be one of 10,20,30,50,100,500.
	Amount int64 `json:"amount"`
	// Money is the actual LDXP checkout amount. Test/POC products may use a different
	// payment amount from Amount (for example amount=10, money=0.10), but it must be positive.
	Money       float64 `json:"money"`
	ProductURL  string  `json:"product_url"`
	ProductName string  `json:"product_name"`
}

type LdxpConfig struct {
	Enabled                 bool
	ContactEmail            string
	Products                map[int64]LdxpProductConfig
	WorkerToken             string
	SessionTTLSeconds       int64
	QrTTLSeconds            int64
	WorkerJobTimeoutSeconds int64
	MailMatchWindowSeconds  int64
	RequireMailMatch        bool
	DebugSnapshotDir        string
}

const defaultLdxpProductsJSON = `[
  {"amount":10,"money":10,"product_url":"https://example.test/ldxp/10","product_name":"LDXP 10"},
  {"amount":20,"money":20,"product_url":"https://example.test/ldxp/20","product_name":"LDXP 20"},
  {"amount":30,"money":30,"product_url":"https://example.test/ldxp/30","product_name":"LDXP 30"},
  {"amount":50,"money":50,"product_url":"https://example.test/ldxp/50","product_name":"LDXP 50"},
  {"amount":100,"money":100,"product_url":"https://example.test/ldxp/100","product_name":"LDXP 100"},
  {"amount":500,"money":500,"product_url":"https://example.test/ldxp/500","product_name":"LDXP 500"}
]`

var requiredLdxpProductAmounts = []int64{10, 20, 30, 50, 100, 500}

func LoadLdxpConfig() (*LdxpConfig, error) {
	productsJSON := common.GetEnvOrDefaultString("LDXP_TOPUP_PRODUCTS_JSON", defaultLdxpProductsJSON)
	var products []LdxpProductConfig
	if err := common.UnmarshalJsonStr(productsJSON, &products); err != nil {
		return nil, fmt.Errorf("parse LDXP_TOPUP_PRODUCTS_JSON: %w", err)
	}

	productMap, err := ValidateLdxpProducts(products)
	if err != nil {
		return nil, err
	}

	workerToken, err := readLdxpSecretStrict("LDXP_WORKER_TOKEN", "LDXP_WORKER_TOKEN_FILE")
	if err != nil {
		return nil, err
	}

	sessionTTLSeconds, err := getPositiveLdxpEnvSeconds("LDXP_SESSION_TTL_SECONDS", 1200)
	if err != nil {
		return nil, err
	}
	qrTTLSeconds, err := getPositiveLdxpEnvSeconds("LDXP_QR_TTL_SECONDS", 300)
	if err != nil {
		return nil, err
	}
	workerJobTimeoutSeconds, err := getPositiveLdxpEnvSeconds("LDXP_WORKER_JOB_TIMEOUT_SECONDS", 900)
	if err != nil {
		return nil, err
	}
	mailMatchWindowSeconds, err := getPositiveLdxpEnvSeconds("LDXP_MAIL_MATCH_WINDOW_SECONDS", 1800)
	if err != nil {
		return nil, err
	}

	return &LdxpConfig{
		Enabled:                 common.GetEnvOrDefaultBool("LDXP_AUTO_TOPUP_ENABLED", false),
		ContactEmail:            common.GetEnvOrDefaultString("LDXP_CONTACT_EMAIL", "support@yunbay.xyz"),
		Products:                productMap,
		WorkerToken:             workerToken,
		SessionTTLSeconds:       sessionTTLSeconds,
		QrTTLSeconds:            qrTTLSeconds,
		WorkerJobTimeoutSeconds: workerJobTimeoutSeconds,
		MailMatchWindowSeconds:  mailMatchWindowSeconds,
		RequireMailMatch:        common.GetEnvOrDefaultBool("LDXP_REQUIRE_MAIL_MATCH", true),
		DebugSnapshotDir:        common.GetEnvOrDefaultString("LDXP_DEBUG_SNAPSHOT_DIR", "/opt/new-api/logs/ldxp-worker/snapshots"),
	}, nil
}

func ReadLdxpSecret(envName string, fileEnvName string) string {
	secret, err := readLdxpSecretStrict(envName, fileEnvName)
	if err == nil {
		return secret
	}
	return strings.TrimSpace(os.Getenv(envName))
}

func GetLdxpAmountOptions(cfg *LdxpConfig) []int64 {
	if cfg == nil || len(cfg.Products) == 0 {
		return []int64{}
	}

	amounts := make([]int64, 0, len(cfg.Products))
	for amount := range cfg.Products {
		amounts = append(amounts, amount)
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] < amounts[j] })
	return amounts
}

func readLdxpSecretStrict(envName string, fileEnvName string) (string, error) {
	filePath := strings.TrimSpace(os.Getenv(fileEnvName))
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read %s path %q: %w", fileEnvName, filePath, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(os.Getenv(envName)), nil
}

func getPositiveLdxpEnvSeconds(envName string, defaultValue int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", envName, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", envName)
	}
	return parsed, nil
}

func RedactLdxpValue(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "data:image/png;base64,") {
		return "data:image/png;base64,[redacted]"
	}
	if len(value) < 4 {
		return "[redacted]"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func ValidateLdxpProducts(products []LdxpProductConfig) (map[int64]LdxpProductConfig, error) {
	if len(products) != len(requiredLdxpProductAmounts) {
		return nil, fmt.Errorf("ldxp products must contain exactly amounts %s", formatLdxpAmounts(requiredLdxpProductAmounts))
	}

	required := make(map[int64]struct{}, len(requiredLdxpProductAmounts))
	for _, amount := range requiredLdxpProductAmounts {
		required[amount] = struct{}{}
	}

	result := make(map[int64]LdxpProductConfig, len(requiredLdxpProductAmounts))
	for _, product := range products {
		if _, ok := required[product.Amount]; !ok {
			return nil, fmt.Errorf("invalid ldxp product amount %d; required amounts are %s", product.Amount, formatLdxpAmounts(requiredLdxpProductAmounts))
		}
		if _, exists := result[product.Amount]; exists {
			return nil, fmt.Errorf("duplicate ldxp product amount %d", product.Amount)
		}
		if product.Money <= 0 {
			return nil, fmt.Errorf("ldxp product amount %d must have positive money", product.Amount)
		}
		if strings.TrimSpace(product.ProductURL) == "" {
			return nil, fmt.Errorf("ldxp product amount %d must have product_url", product.Amount)
		}
		if strings.TrimSpace(product.ProductName) == "" {
			return nil, fmt.Errorf("ldxp product amount %d must have product_name", product.Amount)
		}
		product.ProductURL = strings.TrimSpace(product.ProductURL)
		product.ProductName = strings.TrimSpace(product.ProductName)
		result[product.Amount] = product
	}

	for _, amount := range requiredLdxpProductAmounts {
		if _, ok := result[amount]; !ok {
			return nil, fmt.Errorf("missing ldxp product amount %d", amount)
		}
	}
	return result, nil
}

func formatLdxpAmounts(amounts []int64) string {
	copied := append([]int64(nil), amounts...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	parts := make([]string, 0, len(copied))
	for _, amount := range copied {
		parts = append(parts, fmt.Sprintf("%d", amount))
	}
	return strings.Join(parts, ",")
}
