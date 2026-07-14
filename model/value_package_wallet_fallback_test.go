package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestValuePackageWalletFallbackDefaultsEnabled(t *testing.T) {
	require.True(t, (UserValuePackagePreference{}).AllowsWalletFallback())

	disabled := false
	require.False(t, (UserValuePackagePreference{WalletFallbackEnabled: &disabled}).AllowsWalletFallback())

	enabled := true
	require.True(t, (UserValuePackagePreference{WalletFallbackEnabled: &enabled}).AllowsWalletFallback())
}

func TestUpdateValuePackageWalletFallbackPersistsExplicitChoice(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3980, UserGroupVIP)

	disabledState, err := UpdateValuePackageWalletFallback(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, disabledState)
	require.NotNil(t, disabledState.Preference.WalletFallbackEnabled)
	require.False(t, disabledState.Preference.AllowsWalletFallback())

	var stored UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&stored).Error)
	require.NotNil(t, stored.WalletFallbackEnabled)
	require.False(t, *stored.WalletFallbackEnabled)

	enabledState, err := UpdateValuePackageWalletFallback(user.Id, true)
	require.NoError(t, err)
	require.True(t, enabledState.Preference.AllowsWalletFallback())
	require.Equal(t, common.GetPointer(true), enabledState.Preference.WalletFallbackEnabled)
}

func TestUpdateValuePackageWalletFallbackKeepsLegacyActivePackageEnabled(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3981, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-60, now+valuePackageWeekSeconds)

	state, err := UpdateValuePackageWalletFallback(user.Id, false)

	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.False(t, state.Preference.AllowsWalletFallback())
}
