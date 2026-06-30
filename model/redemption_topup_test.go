package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRedemptionTopUpTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Redemption{}, &TopUp{}, &Log{}))
	cleanup := func() {
		DB.Exec("DELETE FROM redemptions")
		DB.Exec("DELETE FROM top_ups")
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM logs")
	}
	cleanup()
	t.Cleanup(cleanup)
}

func insertRedemptionTopUpUser(t *testing.T, userID int, quota int) {
	t.Helper()
	user := &User{
		Id:       userID,
		Username: "redemption_user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}
	require.NoError(t, DB.Create(user).Error)
}

func insertRedemptionCode(t *testing.T, redemption *Redemption) {
	t.Helper()
	if redemption.Status == 0 {
		redemption.Status = common.RedemptionCodeStatusEnabled
	}
	require.NoError(t, DB.Create(redemption).Error)
}

func userQuotaForRedemptionTest(t *testing.T, userID int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", userID).First(&user).Error)
	return user.Quota
}

func userGroupForRedemptionTest(t *testing.T, userID int) string {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("group").Where("id = ?", userID).First(&user).Error)
	return user.Group
}

func topUpsForRedemptionTest(t *testing.T, userID int) []TopUp {
	t.Helper()
	var topUps []TopUp
	require.NoError(t, DB.Where("user_id = ?", userID).Order("id asc").Find(&topUps).Error)
	return topUps
}

func TestRedeemPaidTopupCreatesSuccessfulTopUp(t *testing.T) {
	setupRedemptionTopUpTest(t)

	const userID = 1001
	const originalKey = "paid-topup-key-1001"
	insertRedemptionTopUpUser(t, userID, 50)
	insertRedemptionCode(t, &Redemption{
		Key:          originalKey,
		Name:         "Paid topup card",
		Quota:        700,
		Kind:         RedemptionKindPaidTopUp,
		Amount:       20,
		Money:        9.99,
		CountAsTopUp: true,
		BatchId:      "batch-paid-1",
		Source:       RedemptionSourceLDXP,
	})

	result, err := Redeem(originalKey, userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 700, result.Quota)
	assert.Equal(t, RedemptionKindPaidTopUp, result.Redemption.Kind)
	assert.EqualValues(t, 20, result.Redemption.Amount)
	assert.Equal(t, 9.99, result.Redemption.Money)
	assert.True(t, result.Redemption.CountAsTopUp)
	assert.Equal(t, "batch-paid-1", result.Redemption.BatchId)
	assert.Equal(t, RedemptionSourceLDXP, result.Redemption.Source)
	assert.Equal(t, 750, userQuotaForRedemptionTest(t, userID))

	var redeemed Redemption
	require.NoError(t, DB.Where("key = ?", originalKey).First(&redeemed).Error)

	topUps := topUpsForRedemptionTest(t, userID)
	require.Len(t, topUps, 1)
	assert.EqualValues(t, 20, topUps[0].Amount)
	assert.Equal(t, 9.99, topUps[0].Money)
	assert.Equal(t, common.TopUpStatusSuccess, topUps[0].Status)
	assert.Equal(t, PaymentMethodRedemptionCode, topUps[0].PaymentMethod)
	assert.Equal(t, PaymentProviderRedemptionCode, topUps[0].PaymentProvider)
	assert.Equal(t, CreateRedemptionTopUpTradeNo(redeemed.Id, userID), topUps[0].TradeNo)
	assert.NotContains(t, topUps[0].TradeNo, originalKey)
	assert.NotZero(t, topUps[0].CreateTime)
	assert.Equal(t, topUps[0].CreateTime, topUps[0].CompleteTime)
	assert.Equal(t, common.RedemptionCodeStatusUsed, redeemed.Status)
	assert.Equal(t, userID, redeemed.UsedUserId)
	assert.NotZero(t, redeemed.RedeemedTime)
}

func TestRedeemPromoCreditDoesNotCreateTopUp(t *testing.T) {
	setupRedemptionTopUpTest(t)

	const userID = 1002
	insertRedemptionTopUpUser(t, userID, 10)
	insertRedemptionCode(t, &Redemption{
		Key:          "promo-credit-key",
		Name:         "Promo credit",
		Quota:        300,
		Kind:         RedemptionKindPromoCredit,
		CountAsTopUp: false,
		Source:       RedemptionSourcePromo,
	})

	result, err := Redeem("promo-credit-key", userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 300, result.Quota)
	assert.Equal(t, RedemptionKindPromoCredit, result.Redemption.Kind)
	assert.Equal(t, 310, userQuotaForRedemptionTest(t, userID))
	assert.Empty(t, topUpsForRedemptionTest(t, userID))
}

func TestRedeemLegacyDoesNotCreateTopUp(t *testing.T) {
	setupRedemptionTopUpTest(t)

	const userID = 1003
	insertRedemptionTopUpUser(t, userID, 20)
	insertRedemptionCode(t, &Redemption{
		Key:    "legacy-key",
		Name:   "Legacy code",
		Quota:  400,
		Kind:   "",
		Source: "",
	})

	result, err := Redeem("legacy-key", userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 400, result.Quota)
	assert.Equal(t, RedemptionKindLegacy, result.Redemption.Kind)
	assert.Equal(t, RedemptionSourceManual, result.Redemption.Source)
	assert.Equal(t, 420, userQuotaForRedemptionTest(t, userID))
	assert.Empty(t, topUpsForRedemptionTest(t, userID))
}

func TestRedeemReturnsSpecificErrors(t *testing.T) {
	setupRedemptionTopUpTest(t)
	insertRedemptionTopUpUser(t, 1004, 0)
	insertRedemptionCode(t, &Redemption{Key: "used-key", Name: "Used", Quota: 1, Status: common.RedemptionCodeStatusUsed})
	insertRedemptionCode(t, &Redemption{Key: "disabled-key", Name: "Disabled", Quota: 1, Status: common.RedemptionCodeStatusDisabled})
	insertRedemptionCode(t, &Redemption{Key: "expired-key", Name: "Expired", Quota: 1, ExpiredTime: common.GetTimestamp() - 1})
	insertRedemptionCode(t, &Redemption{Key: "coupon-key", Name: "Coupon", Quota: 1, Kind: RedemptionKindCoupon})
	insertRedemptionCode(t, &Redemption{Key: "unknown-kind-key", Name: "Unknown", Quota: 1, Kind: "gift_card"})

	testCases := []struct {
		name string
		key  string
		user int
		want error
	}{
		{name: "empty key", key: "", user: 1004, want: ErrRedemptionNotProvided},
		{name: "empty user", key: "missing-key", user: 0, want: ErrRedemptionInvalid},
		{name: "missing key", key: "missing-key", user: 1004, want: ErrRedemptionInvalid},
		{name: "used key", key: "used-key", user: 1004, want: ErrRedemptionUsed},
		{name: "disabled key", key: "disabled-key", user: 1004, want: ErrRedemptionUsed},
		{name: "expired key", key: "expired-key", user: 1004, want: ErrRedemptionExpired},
		{name: "coupon key", key: "coupon-key", user: 1004, want: ErrRedemptionUnsupportedKind},
		{name: "unknown kind", key: "unknown-kind-key", user: 1004, want: ErrRedemptionUnsupportedKind},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Redeem(tc.key, tc.user)
			require.Nil(t, result)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "got %v, want %v", err, tc.want)
		})
	}
}

func TestNormalizeRedemptionForCreateDefaultsAndValidates(t *testing.T) {
	t.Run("defaults new create requests to promo credit", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-default", Quota: 100}
		require.NoError(t, NormalizeRedemptionForCreate(redemption))
		assert.Equal(t, RedemptionKindPromoCredit, redemption.Kind)
		assert.Equal(t, RedemptionSourcePromo, redemption.Source)
		assert.NotEmpty(t, redemption.BatchId)
		assert.EqualValues(t, 0, redemption.Amount)
		assert.Equal(t, 0.0, redemption.Money)
		assert.False(t, redemption.CountAsTopUp)
	})

	t.Run("paid topup requires quota amount money and accounting", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-paid", Quota: 100, Kind: RedemptionKindPaidTopUp, Amount: 10, Money: 10, CountAsTopUp: true}
		require.NoError(t, NormalizeRedemptionForCreate(redemption))
		assert.Equal(t, RedemptionSourceLDXP, redemption.Source)
		assert.NotEmpty(t, redemption.BatchId)
	})

	t.Run("paid topup without accounting is invalid", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-paid-no-accounting", Quota: 100, Kind: RedemptionKindPaidTopUp, Amount: 10, Money: 10, CountAsTopUp: false}
		require.ErrorIs(t, NormalizeRedemptionForCreate(redemption), ErrRedemptionInvalid)
	})

	t.Run("paid topup with missing amount or money is invalid", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-paid", Quota: 100, Kind: RedemptionKindPaidTopUp, CountAsTopUp: true}
		require.ErrorIs(t, NormalizeRedemptionForCreate(redemption), ErrRedemptionInvalid)
	})

	t.Run("promo credit cannot count as topup", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-promo", Quota: 100, Kind: RedemptionKindPromoCredit, CountAsTopUp: true}
		require.ErrorIs(t, NormalizeRedemptionForCreate(redemption), ErrRedemptionInvalid)
	})

	t.Run("coupon is not creatable in this iteration", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-coupon", Quota: 100, Kind: RedemptionKindCoupon, Source: RedemptionSourceLDXP}
		require.ErrorIs(t, NormalizeRedemptionForCreate(redemption), ErrRedemptionUnsupportedKind)
	})

	t.Run("legacy is not creatable in this iteration", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-legacy", Quota: 100, Kind: RedemptionKindLegacy}
		require.ErrorIs(t, NormalizeRedemptionForCreate(redemption), ErrRedemptionUnsupportedKind)
	})

	t.Run("unknown kind is rejected", func(t *testing.T) {
		redemption := &Redemption{Key: "normalize-unknown", Quota: 100, Kind: "gift_card"}
		require.ErrorIs(t, NormalizeRedemptionForCreate(redemption), ErrRedemptionUnsupportedKind)
	})

	t.Run("nil redemption is invalid", func(t *testing.T) {
		require.ErrorIs(t, NormalizeRedemptionForCreate(nil), ErrRedemptionInvalid)
	})
}

func TestRedeemDoesNotConsumeCodeWhenUserDoesNotExist(t *testing.T) {
	setupRedemptionTopUpTest(t)
	insertRedemptionCode(t, &Redemption{
		Key:          "missing-user-paid-key",
		Name:         "Paid topup card",
		Quota:        700,
		Kind:         RedemptionKindPaidTopUp,
		Amount:       20,
		Money:        9.99,
		CountAsTopUp: true,
		BatchId:      "batch-missing-user",
		Source:       RedemptionSourceLDXP,
	})

	result, err := Redeem("missing-user-paid-key", 9999)

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrRedemptionInvalid)

	var redemption Redemption
	require.NoError(t, DB.Where("key = ?", "missing-user-paid-key").First(&redemption).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redemption.Status)
	assert.Zero(t, redemption.UsedUserId)
	assert.Zero(t, redemption.RedeemedTime)

	var count int64
	require.NoError(t, DB.Model(&TopUp{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestRedeemRejectsInvalidPaidTopupAccounting(t *testing.T) {
	setupRedemptionTopUpTest(t)
	const userID = 1005
	insertRedemptionTopUpUser(t, userID, 50)
	insertRedemptionCode(t, &Redemption{
		Key:          "invalid-paid-key",
		Name:         "Invalid paid topup card",
		Quota:        700,
		Kind:         RedemptionKindPaidTopUp,
		Amount:       0,
		Money:        9.99,
		CountAsTopUp: true,
		Source:       RedemptionSourceLDXP,
	})

	result, err := Redeem("invalid-paid-key", userID)

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrRedemptionInvalid)
	assert.Equal(t, 50, userQuotaForRedemptionTest(t, userID))
	assert.Empty(t, topUpsForRedemptionTest(t, userID))

	var redemption Redemption
	require.NoError(t, DB.Where("key = ?", "invalid-paid-key").First(&redemption).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redemption.Status)
}

func TestRedeemPaidTopupCannotBeRedeemedTwice(t *testing.T) {
	setupRedemptionTopUpTest(t)
	const userID = 1006
	insertRedemptionTopUpUser(t, userID, 50)
	insertRedemptionCode(t, &Redemption{
		Key:          "repeat-paid-key",
		Name:         "Paid topup card",
		Quota:        700,
		Kind:         RedemptionKindPaidTopUp,
		Amount:       20,
		Money:        9.99,
		CountAsTopUp: true,
		BatchId:      "batch-repeat",
		Source:       RedemptionSourceLDXP,
	})

	first, err := Redeem("repeat-paid-key", userID)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := Redeem("repeat-paid-key", userID)

	require.Nil(t, second)
	require.ErrorIs(t, err, ErrRedemptionUsed)
	assert.Equal(t, 750, userQuotaForRedemptionTest(t, userID))
	require.Len(t, topUpsForRedemptionTest(t, userID), 1)
}

func TestRedeemPaidTopupAtThresholdUpgradesUserToVIP(t *testing.T) {
	setupRedemptionTopUpTest(t)

	const userID = 1010
	const originalKey = "paid-topup-vip-key-1010"
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "redemption_vip_user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    UserGroupTiyan,
		Quota:    0,
	}).Error)
	insertRedemptionCode(t, &Redemption{
		Key:          originalKey,
		Name:         "Paid topup VIP card",
		Quota:        3000,
		Kind:         RedemptionKindPaidTopUp,
		Amount:       30,
		Money:        30,
		CountAsTopUp: true,
		BatchId:      "batch-paid-vip",
		Source:       RedemptionSourceLDXP,
	})

	_, err := Redeem(originalKey, userID)
	require.NoError(t, err)

	assert.Equal(t, UserGroupVIP, userGroupForRedemptionTest(t, userID))
}
