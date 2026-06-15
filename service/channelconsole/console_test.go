package channelconsole

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelConsoleServiceTestDB(t *testing.T) {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.ChannelConsoleChannel{},
		&model.ChannelConsoleModelPrice{},
		&model.ChannelConsoleHealthCheck{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestCommitImportCreatesChannelAndMetadata(t *testing.T) {
	setupChannelConsoleServiceTestDB(t)

	result, err := CommitImport(ImportCommitRequest{
		RawInput:     `curl https://openrouter.ai/api/v1/chat/completions -H "Authorization: Bearer sk-or-one" sk-or-two`,
		Group:        "vip",
		Models:       []string{"openai/gpt-4o-mini", "anthropic/claude-3.5-sonnet"},
		MultiKeyMode: string(constant.MultiKeyModeRandom),
		Markup:       1.3,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Greater(t, result.ChannelID, 0)
	require.Equal(t, "OpenRouter API 池", result.Name)
	require.Equal(t, ProviderOpenRouter, result.Provider)
	require.Equal(t, 2, result.KeyCount)
	require.Equal(t, 2, result.ModelCount)
	require.Equal(t, model.ChannelConsoleStatusUnchecked, result.HealthStatus)
	require.Equal(t, model.ChannelConsoleStatusUnchecked, result.PriceStatus)

	channel, err := model.GetChannelById(result.ChannelID, true)
	require.NoError(t, err)
	require.Equal(t, constant.ChannelTypeOpenRouter, channel.Type)
	require.Equal(t, "OpenRouter API 池", channel.Name)
	require.Equal(t, common.ChannelStatusEnabled, channel.Status)
	require.Equal(t, "vip", channel.Group)
	require.Equal(t, "https://openrouter.ai/api", channel.GetBaseURL())
	require.Equal(t, "sk-or-one\nsk-or-two", channel.Key)
	require.Equal(t, "openai/gpt-4o-mini,anthropic/claude-3.5-sonnet", channel.Models)
	require.NotNil(t, channel.TestModel)
	require.Equal(t, "openai/gpt-4o-mini", *channel.TestModel)
	require.NotNil(t, channel.Tag)
	require.Equal(t, "yunbay-console", *channel.Tag)
	require.True(t, channel.ChannelInfo.IsMultiKey)
	require.Equal(t, 2, channel.ChannelInfo.MultiKeySize)
	require.Equal(t, constant.MultiKeyModeRandom, channel.ChannelInfo.MultiKeyMode)
	require.Equal(t, common.ChannelStatusEnabled, channel.ChannelInfo.MultiKeyStatusList[0])
	require.Equal(t, common.ChannelStatusEnabled, channel.ChannelInfo.MultiKeyStatusList[1])

	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", result.ChannelID).Order("model asc").Find(&abilities).Error)
	require.Len(t, abilities, 2)
	require.Equal(t, "vip", abilities[0].Group)
	require.True(t, abilities[0].Enabled)

	meta, err := model.GetChannelConsoleChannelByChannelID(result.ChannelID)
	require.NoError(t, err)
	require.Equal(t, ProviderOpenRouter, meta.Provider)
	require.Equal(t, model.ChannelConsoleKindThirdPartyAPI, meta.ProviderKind)
	require.Equal(t, ImportKindCurl, meta.ImportKind)
	require.Equal(t, PriceSourceOpenRouter, meta.PriceSource)
	require.Equal(t, model.ChannelConsoleStatusUnchecked, meta.HealthStatus)
	require.Equal(t, model.ChannelConsoleStatusUnchecked, meta.ModelSyncStatus)
	require.Equal(t, model.ChannelConsoleStatusUnchecked, meta.PriceSyncStatus)
	require.Equal(t, 1.3, meta.Markup)
	require.Equal(t, "mark_only", meta.AutoDisablePolicy)
}

func TestCommitImportAppliesDefaultsAndRejectsMissingKeys(t *testing.T) {
	setupChannelConsoleServiceTestDB(t)

	result, err := CommitImport(ImportCommitRequest{
		RawInput:     "sk-redacted-example",
		MultiKeyMode: "unsupported",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, ProviderOpenAI, result.Provider)
	require.Equal(t, 1, result.KeyCount)
	require.Equal(t, 1, result.ModelCount)

	channel, err := model.GetChannelById(result.ChannelID, true)
	require.NoError(t, err)
	require.Equal(t, "default", channel.Group)
	require.Equal(t, "OpenAI API 池", channel.Name)
	require.Equal(t, "gpt-4o-mini", channel.Models)
	require.False(t, channel.ChannelInfo.IsMultiKey)
	require.Equal(t, constant.MultiKeyModePolling, channel.ChannelInfo.MultiKeyMode)
	require.Equal(t, "https://api.openai.com", channel.GetBaseURL())

	meta, err := model.GetChannelConsoleChannelByChannelID(result.ChannelID)
	require.NoError(t, err)
	require.Equal(t, 1.2, meta.Markup)

	missingKeyResult, err := CommitImport(ImportCommitRequest{RawInput: "Base URL: https://gateway.example.com"})
	require.Error(t, err)
	require.Nil(t, missingKeyResult)
}
