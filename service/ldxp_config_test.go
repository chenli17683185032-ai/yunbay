package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const ldxpSixProductsJSON = `[
  {"amount":10,"money":0.10,"product_url":"https://example.test/ldxp/10","product_name":"LDXP 10"},
  {"amount":20,"money":20,"product_url":"https://example.test/ldxp/20","product_name":"LDXP 20"},
  {"amount":30,"money":30,"product_url":"https://example.test/ldxp/30","product_name":"LDXP 30"},
  {"amount":50,"money":50,"product_url":"https://example.test/ldxp/50","product_name":"LDXP 50"},
  {"amount":100,"money":100,"product_url":"https://example.test/ldxp/100","product_name":"LDXP 100"},
  {"amount":500,"money":500,"product_url":"https://example.test/ldxp/500","product_name":"LDXP 500"}
]`

func clearLdxpEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"LDXP_AUTO_TOPUP_ENABLED",
		"LDXP_TOPUP_PRODUCTS_JSON",
		"LDXP_CONTACT_EMAIL",
		"LDXP_WORKER_TOKEN",
		"LDXP_WORKER_TOKEN_FILE",
		"LDXP_SESSION_TTL_SECONDS",
		"LDXP_QR_TTL_SECONDS",
		"LDXP_WORKER_JOB_TIMEOUT_SECONDS",
		"LDXP_MAIL_MATCH_WINDOW_SECONDS",
		"LDXP_REQUIRE_MAIL_MATCH",
		"LDXP_DEBUG_SNAPSHOT_DIR",
		"LDXP_ALLOWED_USERNAMES",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadLdxpConfigDisabledByDefault(t *testing.T) {
	clearLdxpEnv(t)

	cfg, err := LoadLdxpConfig()
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Equal(t, "support@yunbay.xyz", cfg.ContactEmail)
	require.Len(t, cfg.Products, 6)
	require.Equal(t, int64(1200), cfg.SessionTTLSeconds)
	require.Equal(t, int64(300), cfg.QrTTLSeconds)
	require.Equal(t, int64(900), cfg.WorkerJobTimeoutSeconds)
	require.Equal(t, int64(1800), cfg.MailMatchWindowSeconds)
	require.True(t, cfg.RequireMailMatch)
	require.Equal(t, "/opt/new-api/logs/ldxp-worker/snapshots", cfg.DebugSnapshotDir)
}

func TestLoadLdxpConfigParsesSixProducts(t *testing.T) {
	clearLdxpEnv(t)
	t.Setenv("LDXP_AUTO_TOPUP_ENABLED", "true")
	t.Setenv("LDXP_TOPUP_PRODUCTS_JSON", ldxpSixProductsJSON)
	t.Setenv("LDXP_WORKER_TOKEN", " token-from-env\n")
	t.Setenv("LDXP_WORKER_TOKEN_FILE", "")

	cfg, err := LoadLdxpConfig()
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, "token-from-env", cfg.WorkerToken)
	require.Len(t, cfg.Products, 6)
	require.Equal(t, 0.10, cfg.Products[10].Money)

	for _, amount := range []int64{10, 20, 30, 50, 100, 500} {
		product, ok := cfg.Products[amount]
		require.True(t, ok, "amount %d must be present", amount)
		require.Equal(t, amount, product.Amount)
		require.NotEmpty(t, product.ProductURL)
		require.NotEmpty(t, product.ProductName)
		require.Positive(t, product.Money)
	}
}

func TestLoadLdxpConfigParsesAllowedUsernames(t *testing.T) {
	clearLdxpEnv(t)
	t.Setenv("LDXP_ALLOWED_USERNAMES", " jiance001, Alice ,jiance001,,BOB ")

	cfg, err := LoadLdxpConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"jiance001", "alice", "bob"}, cfg.AllowedUsernames)
	require.True(t, IsLdxpUserAllowed(cfg, "jiance001"))
	require.True(t, IsLdxpUserAllowed(cfg, "ALICE"))
	require.False(t, IsLdxpUserAllowed(cfg, "charlie"))
}

func TestIsLdxpUserAllowedDefaultsOpenWhenNoAllowlist(t *testing.T) {
	cfg := &LdxpConfig{}
	require.True(t, IsLdxpUserAllowed(cfg, "anyone"))
	require.True(t, IsLdxpUserAllowed(cfg, ""))
}

func TestLoadLdxpConfigRejectsMissingAmounts(t *testing.T) {
	clearLdxpEnv(t)
	t.Setenv("LDXP_TOPUP_PRODUCTS_JSON", `[
  {"amount":10,"money":10,"product_url":"https://example.test/ldxp/10","product_name":"LDXP 10"},
  {"amount":20,"money":20,"product_url":"https://example.test/ldxp/20","product_name":"LDXP 20"},
  {"amount":30,"money":30,"product_url":"https://example.test/ldxp/30","product_name":"LDXP 30"},
  {"amount":50,"money":50,"product_url":"https://example.test/ldxp/50","product_name":"LDXP 50"},
  {"amount":100,"money":100,"product_url":"https://example.test/ldxp/100","product_name":"LDXP 100"}
]`)

	cfg, err := LoadLdxpConfig()
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestReadLdxpSecretPrefersFile(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "worker-token")
	require.NoError(t, os.WriteFile(secretPath, []byte(" file-secret\n"), 0o600))
	t.Setenv("LDXP_WORKER_TOKEN", "env-secret")
	t.Setenv("LDXP_WORKER_TOKEN_FILE", secretPath)

	require.Equal(t, "file-secret", ReadLdxpSecret("LDXP_WORKER_TOKEN", "LDXP_WORKER_TOKEN_FILE"))
}

func TestRedactLdxpSecretMasksCardAndQr(t *testing.T) {
	require.Empty(t, RedactLdxpValue(""))
	require.Equal(t, "[redacted]", RedactLdxpValue("abc"))
	require.Equal(t, "abcd...efgh", RedactLdxpValue("abcdefgh"))
	require.Equal(t, "4242...4242", RedactLdxpValue("4242424242424242"))
	require.Equal(t, "data:image/png;base64,[redacted]", RedactLdxpValue("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUA"))
}

func TestLdxpConfigRedactShortNormalStrings(t *testing.T) {
	require.Equal(t, "[redacted]", RedactLdxpValue("abc"))
	require.Equal(t, "abcd...efgh", RedactLdxpValue("abcdefgh"))
}

func TestLoadLdxpConfigRejectsUnreadableTokenFile(t *testing.T) {
	clearLdxpEnv(t)
	t.Setenv("LDXP_WORKER_TOKEN", "env-token")
	t.Setenv("LDXP_WORKER_TOKEN_FILE", filepath.Join(t.TempDir(), "missing-token"))

	cfg, err := LoadLdxpConfig()
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "LDXP_WORKER_TOKEN_FILE")
	require.NotContains(t, err.Error(), "env-token")
}

func TestLoadLdxpConfigRejectsInvalidTTL(t *testing.T) {
	clearLdxpEnv(t)
	t.Setenv("LDXP_SESSION_TTL_SECONDS", "0")

	cfg, err := LoadLdxpConfig()
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "LDXP_SESSION_TTL_SECONDS")
}

func TestGetLdxpAmountOptionsReturnsSortedAmounts(t *testing.T) {
	options := GetLdxpAmountOptions(&LdxpConfig{Products: map[int64]LdxpProductConfig{
		100: {Amount: 100},
		10:  {Amount: 10},
		50:  {Amount: 50},
	}})

	require.Equal(t, []int64{10, 50, 100}, options)
}

func TestGetLdxpAmountOptionsReturnsEmptyForNilOrEmptyProducts(t *testing.T) {
	require.Empty(t, GetLdxpAmountOptions(nil))
	require.Empty(t, GetLdxpAmountOptions(&LdxpConfig{}))
}
