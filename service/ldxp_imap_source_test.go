package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLdxpIMAPConfigFromEnvDisabledWhenMissing(t *testing.T) {
	t.Setenv("LDXP_MAIL_IMAP_HOST", "")
	cfg := LdxpIMAPConfigFromEnv()
	assert.False(t, cfg.Enabled())
}

func TestConfiguredMailSourceFallsBackToStoredSource(t *testing.T) {
	t.Setenv("LDXP_MAIL_IMAP_HOST", "")
	source := ConfiguredLdxpMailSource()
	_, ok := source.(StoredLdxpMailSource)
	assert.True(t, ok)

	_, err := source.FetchRecent(context.Background())
	require.NoError(t, err)
}

func TestConfiguredMailSourceUsesIMAPWhenEnabled(t *testing.T) {
	t.Setenv("LDXP_MAIL_IMAP_HOST", "imap.example.test")
	t.Setenv("LDXP_MAIL_IMAP_PORT", "1993")
	t.Setenv("LDXP_MAIL_IMAP_USER", "10256345@qq.example")
	t.Setenv("LDXP_MAIL_IMAP_PASSWORD", "secret")
	t.Setenv("LDXP_MAIL_IMAP_MAILBOX", "Orders")

	source := ConfiguredLdxpMailSource()
	imapSource, ok := source.(*LdxpIMAPSource)
	require.True(t, ok)
	assert.Equal(t, "imap.example.test", imapSource.cfg.Host)
	assert.Equal(t, 1993, imapSource.cfg.Port)
	assert.Equal(t, "Orders", imapSource.cfg.Mailbox)
}
