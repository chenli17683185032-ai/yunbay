package channelconsole

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestAggregateHealthStatus(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		want     string
	}{
		{"empty", nil, HealthUnchecked},
		{"all healthy", []string{HealthHealthy, HealthHealthy}, HealthHealthy},
		{"one failed", []string{HealthHealthy, HealthFailed}, HealthWarning},
		{"all failed", []string{HealthFailed, HealthFailed}, HealthFailed},
		{"disabled wins", []string{HealthDisabled, HealthHealthy}, HealthDisabled},
		{"disabled after warning", []string{HealthWarning, HealthDisabled}, HealthDisabled},
		{"disabled after unchecked", []string{HealthUnchecked, HealthDisabled}, HealthDisabled},
		{"disabled after unknown", []string{"unknown", HealthDisabled}, HealthDisabled},
		{"warning", []string{HealthHealthy, HealthWarning}, HealthWarning},
		{"unchecked", []string{HealthHealthy, HealthUnchecked}, HealthWarning},
		{"unknown status", []string{"unknown"}, HealthWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AggregateHealthStatus(tc.statuses); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestRecordHealthCheckResultUpdatesMetadata(t *testing.T) {
	setupChannelConsoleServiceTestDB(t)

	meta := &model.ChannelConsoleChannel{
		ChannelId:        101,
		Provider:         ProviderOpenAI,
		ProviderKind:     model.ChannelConsoleKindThirdPartyAPI,
		ImportKind:       ImportKindKeyOnly,
		PriceSource:      PriceSourceOpenAI,
		HealthStatus:     HealthFailed,
		LastErrorCode:    "old_error",
		LastErrorMessage: "old message",
	}
	require.NoError(t, model.UpsertChannelConsoleChannel(meta))

	check, err := RecordHealthCheckResult(HealthCheckRecord{
		ChannelID:      101,
		CheckType:      HealthCheckTypeAutomatic,
		Status:         HealthHealthy,
		ModelName:      "gpt-4o-mini",
		ResponseTimeMs: 1234,
	})
	require.NoError(t, err)
	require.Equal(t, 101, check.ChannelId)
	require.Equal(t, HealthCheckTypeAutomatic, check.CheckType)
	require.Equal(t, HealthHealthy, check.Status)
	require.Equal(t, "gpt-4o-mini", check.ModelName)
	require.Equal(t, 1234, check.ResponseTimeMs)
	require.Empty(t, check.ErrorCode)
	require.Empty(t, check.ErrorMessage)

	updated, err := model.GetChannelConsoleChannelByChannelID(101)
	require.NoError(t, err)
	require.Equal(t, HealthHealthy, updated.HealthStatus)
	require.Equal(t, check.CheckedAt, updated.LastHealthCheckAt)
	require.Empty(t, updated.LastErrorCode)
	require.Empty(t, updated.LastErrorMessage)
}

func TestRecordHealthCheckResultRecordsFailureAndRejectsNonConsoleChannel(t *testing.T) {
	setupChannelConsoleServiceTestDB(t)

	meta := &model.ChannelConsoleChannel{
		ChannelId:    202,
		Provider:     ProviderOpenAI,
		ProviderKind: model.ChannelConsoleKindThirdPartyAPI,
		ImportKind:   ImportKindKeyOnly,
		PriceSource:  PriceSourceOpenAI,
	}
	require.NoError(t, model.UpsertChannelConsoleChannel(meta))

	check, err := RecordHealthCheckResult(HealthCheckRecord{
		ChannelID:      202,
		CheckType:      HealthCheckTypeManual,
		Status:         HealthFailed,
		ErrorCode:      "upstream_error",
		ErrorMessage:   "upstream rejected request",
		ResponseTimeMs: -1,
	})
	require.NoError(t, err)
	require.Equal(t, HealthFailed, check.Status)
	require.Equal(t, 0, check.ResponseTimeMs)
	require.Equal(t, "upstream_error", check.ErrorCode)

	updated, err := model.GetChannelConsoleChannelByChannelID(202)
	require.NoError(t, err)
	require.Equal(t, HealthFailed, updated.HealthStatus)
	require.Equal(t, "upstream_error", updated.LastErrorCode)
	require.Equal(t, "upstream rejected request", updated.LastErrorMessage)

	_, err = RecordHealthCheckResult(HealthCheckRecord{
		ChannelID: 999,
		Status:    HealthHealthy,
	})
	require.ErrorIs(t, err, ErrChannelConsoleMetadataNotFound)
}
