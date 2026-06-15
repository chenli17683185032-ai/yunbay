package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resetChannelConsoleTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&ChannelConsoleChannel{},
		&ChannelConsoleModelPrice{},
		&ChannelConsoleHealthCheck{},
	))
	cleanup := func() {
		DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ChannelConsoleChannel{})
		DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelConsoleModelPrice{})
		DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelConsoleHealthCheck{})
	}
	cleanup()
	t.Cleanup(cleanup)
}

func floatPtrForChannelConsoleTest(v float64) *float64 {
	return &v
}

func TestChannelConsoleModelPriceUsesExpectedPriceColumnNames(t *testing.T) {
	resetChannelConsoleTables(t)

	for _, column := range []string{
		"input_usd_per_1m_tokens",
		"output_usd_per_1m_tokens",
		"cached_input_usd_per_1m_tokens",
		"cache_write_5m_usd_per_1m_tokens",
		"cache_write_1h_usd_per_1m_tokens",
	} {
		require.Truef(t, DB.Migrator().HasColumn(&ChannelConsoleModelPrice{}, column), "expected column %q to exist", column)
	}
}

func TestChannelConsoleModelPriceCompositeUniqueIndex(t *testing.T) {
	resetChannelConsoleTables(t)

	require.True(t, DB.Migrator().HasIndex(&ChannelConsoleModelPrice{}, "idx_channel_console_model_price"))

	first := &ChannelConsoleModelPrice{
		ChannelId:   505,
		ModelName:   "same-model",
		Source:      "test",
		PriceStatus: ChannelConsolePriceStatusUnknown,
	}
	require.NoError(t, DB.Create(first).Error)

	duplicate := &ChannelConsoleModelPrice{
		ChannelId:   505,
		ModelName:   "same-model",
		Source:      "test",
		PriceStatus: ChannelConsolePriceStatusUnknown,
	}
	require.Error(t, DB.Create(duplicate).Error)
}

func TestUpsertChannelConsoleChannelRejectsNil(t *testing.T) {
	resetChannelConsoleTables(t)

	require.Error(t, UpsertChannelConsoleChannel(nil))
}

func TestUpsertChannelConsoleChannelRestoresSoftDeletedAndMergesPartial(t *testing.T) {
	resetChannelConsoleTables(t)

	existing := &ChannelConsoleChannel{
		ChannelId:         101,
		Provider:          ChannelConsoleProviderOpenRouter,
		ProviderKind:      ChannelConsoleKindThirdPartyAPI,
		ImportKind:        "manual",
		PriceSource:       "openrouter",
		HealthStatus:      ChannelConsoleStatusWarning,
		ModelSyncStatus:   ChannelConsoleStatusHealthy,
		PriceSyncStatus:   ChannelConsoleStatusFailed,
		LastHealthCheckAt: 11,
		LastModelSyncAt:   22,
		LastPriceSyncAt:   33,
		LastErrorCode:     "rate_limit",
		LastErrorMessage:  "rate limited",
		Markup:            1.7,
		AutoDisablePolicy: "disable_after_failures",
	}
	require.NoError(t, DB.Create(existing).Error)
	require.NoError(t, DB.Delete(existing).Error)

	require.NoError(t, UpsertChannelConsoleChannel(&ChannelConsoleChannel{
		ChannelId: 101,
		Provider:  ChannelConsoleProviderOpenAI,
	}))

	var restored ChannelConsoleChannel
	require.NoError(t, DB.Where("channel_id = ?", 101).First(&restored).Error)
	assert.Equal(t, existing.Id, restored.Id)
	assert.False(t, restored.DeletedAt.Valid)
	assert.Equal(t, ChannelConsoleProviderOpenAI, restored.Provider)
	assert.Equal(t, ChannelConsoleKindThirdPartyAPI, restored.ProviderKind)
	assert.Equal(t, "manual", restored.ImportKind)
	assert.Equal(t, "openrouter", restored.PriceSource)
	assert.Equal(t, ChannelConsoleStatusWarning, restored.HealthStatus)
	assert.Equal(t, ChannelConsoleStatusHealthy, restored.ModelSyncStatus)
	assert.Equal(t, ChannelConsoleStatusFailed, restored.PriceSyncStatus)
	assert.EqualValues(t, 11, restored.LastHealthCheckAt)
	assert.EqualValues(t, 22, restored.LastModelSyncAt)
	assert.EqualValues(t, 33, restored.LastPriceSyncAt)
	assert.Equal(t, "rate_limit", restored.LastErrorCode)
	assert.Equal(t, "rate limited", restored.LastErrorMessage)
	assert.Equal(t, 1.7, restored.Markup)
	assert.Equal(t, "disable_after_failures", restored.AutoDisablePolicy)
}

func TestSaveChannelConsoleModelPricesMergesWithoutClearingExisting(t *testing.T) {
	resetChannelConsoleTables(t)

	existing := &ChannelConsoleModelPrice{
		ChannelId:            202,
		ModelName:            "model-a",
		ProviderModelName:    "provider-model-a",
		Source:               "provider-source",
		InputUSDPer1MTokens:  floatPtrForChannelConsoleTest(1.25),
		OutputUSDPer1MTokens: floatPtrForChannelConsoleTest(2.5),
		CompiledModelRatio:   floatPtrForChannelConsoleTest(0.5),
		ManualOverride:       false,
		Enabled:              true,
		PriceStatus:          ChannelConsolePriceStatusSynced,
		SourceUpdatedAt:      123,
		SyncedAt:             456,
	}
	require.NoError(t, DB.Create(existing).Error)

	beforeSync := time.Now().Unix()
	require.NoError(t, SaveChannelConsoleModelPrices(202, []ChannelConsoleModelPrice{
		{
			ModelName:            "model-a",
			OutputUSDPer1MTokens: floatPtrForChannelConsoleTest(3.75),
		},
		{
			ModelName: "model-b",
		},
	}))

	var updated ChannelConsoleModelPrice
	require.NoError(t, DB.Where("channel_id = ? AND model_name = ?", 202, "model-a").First(&updated).Error)
	assert.Equal(t, existing.Id, updated.Id)
	assert.Equal(t, "provider-model-a", updated.ProviderModelName)
	assert.Equal(t, "provider-source", updated.Source)
	require.NotNil(t, updated.InputUSDPer1MTokens)
	assert.Equal(t, 1.25, *updated.InputUSDPer1MTokens)
	require.NotNil(t, updated.OutputUSDPer1MTokens)
	assert.Equal(t, 3.75, *updated.OutputUSDPer1MTokens)
	require.NotNil(t, updated.CompiledModelRatio)
	assert.Equal(t, 0.5, *updated.CompiledModelRatio)
	assert.True(t, updated.Enabled)
	assert.False(t, updated.ManualOverride)
	assert.Equal(t, ChannelConsolePriceStatusSynced, updated.PriceStatus)
	assert.EqualValues(t, 123, updated.SourceUpdatedAt)
	assert.GreaterOrEqual(t, updated.SyncedAt, beforeSync)

	var created ChannelConsoleModelPrice
	require.NoError(t, DB.Where("channel_id = ? AND model_name = ?", 202, "model-b").First(&created).Error)
	assert.Equal(t, ChannelConsolePriceStatusUnknown, created.PriceStatus)
	assert.GreaterOrEqual(t, created.SyncedAt, beforeSync)
}

func TestSaveChannelConsoleModelPricesSkipsManualOverride(t *testing.T) {
	resetChannelConsoleTables(t)

	existing := &ChannelConsoleModelPrice{
		ChannelId:           303,
		ModelName:           "model-manual",
		ProviderModelName:   "manual-provider",
		Source:              "manual-source",
		ManualOverride:      true,
		Enabled:             true,
		PriceStatus:         ChannelConsolePriceStatusManual,
		InputUSDPer1MTokens: floatPtrForChannelConsoleTest(9.99),
		SyncedAt:            100,
	}
	require.NoError(t, DB.Create(existing).Error)

	require.NoError(t, SaveChannelConsoleModelPrices(303, []ChannelConsoleModelPrice{
		{
			ModelName:           "model-manual",
			ProviderModelName:   "incoming-provider",
			Source:              "incoming-source",
			PriceStatus:         ChannelConsolePriceStatusSynced,
			InputUSDPer1MTokens: floatPtrForChannelConsoleTest(1.11),
		},
	}))

	var updated ChannelConsoleModelPrice
	require.NoError(t, DB.Where("channel_id = ? AND model_name = ?", 303, "model-manual").First(&updated).Error)
	assert.Equal(t, "manual-provider", updated.ProviderModelName)
	assert.Equal(t, "manual-source", updated.Source)
	assert.True(t, updated.ManualOverride)
	assert.True(t, updated.Enabled)
	assert.Equal(t, ChannelConsolePriceStatusManual, updated.PriceStatus)
	require.NotNil(t, updated.InputUSDPer1MTokens)
	assert.Equal(t, 9.99, *updated.InputUSDPer1MTokens)
	assert.EqualValues(t, 100, updated.SyncedAt)
}

func TestCreateChannelConsoleHealthCheckRejectsNilAndHookSetsCheckedAt(t *testing.T) {
	resetChannelConsoleTables(t)

	require.Error(t, CreateChannelConsoleHealthCheck(nil))

	check := &ChannelConsoleHealthCheck{
		ChannelId: 404,
		CheckType: "model_probe",
		Status:    ChannelConsoleStatusHealthy,
	}
	beforeCreate := time.Now().Unix()
	require.NoError(t, DB.Create(check).Error)
	assert.GreaterOrEqual(t, check.CheckedAt, beforeCreate)
}
