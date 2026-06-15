package channelconsole

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

	originalDB := model.DB
	originalLOGDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled

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
		&model.ChannelConsolePool{},
		&model.ChannelConsoleCredential{},
		&model.ChannelConsoleModelPrice{},
		&model.ChannelConsoleHealthCheck{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLOGDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
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
	require.Empty(t, channel.ChannelInfo.MultiKeyStatusList)
	nextKey, keyIndex, newAPIError := channel.GetNextEnabledKey()
	require.Nil(t, newAPIError)
	require.Contains(t, []string{"sk-or-one", "sk-or-two"}, nextKey)
	require.Contains(t, []int{0, 1}, keyIndex)

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

func TestBatchDeleteManagedChannelsDeletesOnlyConsoleOwnedRows(t *testing.T) {
	setupChannelConsoleServiceTestDB(t)

	consoleResult, err := CommitImport(ImportCommitRequest{
		RawInput: "sk-redacted-console",
		Models:   []string{"gpt-4o-mini"},
	})
	require.NoError(t, err)

	price := model.ChannelConsoleModelPrice{
		ChannelId:   consoleResult.ChannelID,
		ModelName:   "gpt-4o-mini",
		Source:      PriceSourceOpenAI,
		PriceStatus: model.ChannelConsolePriceStatusSynced,
	}
	require.NoError(t, model.DB.Create(&price).Error)
	require.NoError(t, model.CreateChannelConsoleHealthCheck(&model.ChannelConsoleHealthCheck{
		ChannelId: consoleResult.ChannelID,
		ModelName: "gpt-4o-mini",
		CheckType: HealthCheckTypeManual,
		Status:    model.ChannelConsoleStatusFailed,
	}))

	regularChannel := &model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-regular-channel",
		Name:   "regular channel",
		Status: common.ChannelStatusEnabled,
		Models: "gpt-4o-mini",
		Group:  "default",
	}
	require.NoError(t, model.DB.Create(regularChannel).Error)
	require.NoError(t, regularChannel.AddAbilities(nil))

	deleteResult, err := BatchDeleteManagedChannels([]int{
		consoleResult.ChannelID,
		regularChannel.Id,
		consoleResult.ChannelID,
		0,
	})
	require.NoError(t, err)
	require.Equal(t, 2, deleteResult.Requested)
	require.Equal(t, 1, deleteResult.Deleted)
	require.Equal(t, []int{regularChannel.Id}, deleteResult.SkippedIDs)

	var count int64
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", consoleResult.ChannelID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", consoleResult.ChannelID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, model.DB.Model(&model.ChannelConsoleModelPrice{}).Where("channel_id = ?", consoleResult.ChannelID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, model.DB.Model(&model.ChannelConsoleHealthCheck{}).Where("channel_id = ?", consoleResult.ChannelID).Count(&count).Error)
	require.Zero(t, count)
	_, err = model.GetChannelConsoleChannelByChannelID(consoleResult.ChannelID)
	require.Error(t, err)

	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", regularChannel.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", regularChannel.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestCredentialPoolThirdPartyCredentialDiscoversModelsAndSyncsMultiKeyChannel(t *testing.T) {
	setupChannelConsoleServiceTestDB(t)

	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "Bearer sk-one", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-auto"},{"id":"gpt-plus"}]}`))
	}))
	t.Cleanup(modelServer.Close)

	pool, err := CreateCredentialPool(CreateCredentialPoolRequest{
		Name:         "OpenAI Compatible Pool",
		ProviderKind: model.ChannelConsoleKindThirdPartyAPI,
		BaseURL:      modelServer.URL + "/v1",
	})
	require.NoError(t, err)
	require.Equal(t, model.ChannelConsoleKindThirdPartyAPI, pool.ProviderKind)
	require.Zero(t, pool.NewAPIChannelID)

	firstCredential, err := AddThirdPartyCredentialToPool(t.Context(), pool.Id, AddThirdPartyCredentialRequest{
		APIKey: "sk-one",
	})
	require.NoError(t, err)
	require.Equal(t, model.ChannelConsoleStatusHealthy, firstCredential.Status)
	require.Equal(t, MaskCredential("sk-one"), firstCredential.DisplayName)

	detail, err := GetCredentialPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Equal(t, "gpt-auto,gpt-plus", detail.Pool.Models)
	require.Equal(t, "gpt-auto", detail.Pool.DefaultTestModel)
	require.Greater(t, detail.Pool.NewAPIChannelID, 0)
	require.Len(t, detail.Credentials, 1)

	channel, err := model.GetChannelById(detail.Pool.NewAPIChannelID, true)
	require.NoError(t, err)
	require.Equal(t, "sk-one", channel.Key)
	require.False(t, channel.ChannelInfo.IsMultiKey)
	require.Equal(t, "gpt-auto,gpt-plus", channel.Models)
	require.Equal(t, modelServer.URL, channel.GetBaseURL())
	require.Equal(t, "gpt-auto", *channel.TestModel)

	secondCredential, err := AddThirdPartyCredentialToPool(t.Context(), pool.Id, AddThirdPartyCredentialRequest{
		APIKey: "sk-two",
		Models: []string{"gpt-auto", "gpt-plus"},
	})
	require.NoError(t, err)
	require.Equal(t, model.ChannelConsoleStatusHealthy, secondCredential.Status)

	detail, err = GetCredentialPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, detail.Credentials, 2)

	channel, err = model.GetChannelById(detail.Pool.NewAPIChannelID, true)
	require.NoError(t, err)
	require.Equal(t, "sk-one\nsk-two", channel.Key)
	require.True(t, channel.ChannelInfo.IsMultiKey)
	require.Equal(t, 2, channel.ChannelInfo.MultiKeySize)
	require.Equal(t, constant.MultiKeyModePolling, channel.ChannelInfo.MultiKeyMode)

	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", channel.Id).Order("model asc").Find(&abilities).Error)
	require.Len(t, abilities, 2)
	require.Equal(t, "gpt-auto", abilities[0].Model)
	require.Equal(t, "gpt-plus", abilities[1].Model)
}
