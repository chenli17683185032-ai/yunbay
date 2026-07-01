package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTopupVIPUpgradeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}))
	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})

	return db
}

func createVIPUpgradeUser(t *testing.T, username string, role int, group string) User {
	t.Helper()
	user := User{Username: username, Password: "hash", Role: role, Status: common.UserStatusEnabled, Group: group, AffCode: username + "-aff"}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createSuccessTopUp(t *testing.T, userID int, tradeNo string, money float64) {
	t.Helper()
	require.NoError(t, DB.Create(&TopUp{UserId: userID, TradeNo: tradeNo, Money: money, Amount: int64(money), Status: common.TopUpStatusSuccess}).Error)
}

func TestMaybeUpgradeUserToVIPBelowThresholdDoesNotUpgrade(t *testing.T) {
	setupTopupVIPUpgradeTestDB(t)
	user := createVIPUpgradeUser(t, "below", common.RoleCommonUser, UserGroupTiyan)
	createSuccessTopUp(t, user.Id, "below-1", 29.99)

	upgraded, err := MaybeUpgradeUserToVIP(user.Id)
	require.NoError(t, err)
	require.False(t, upgraded)

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	require.Equal(t, UserGroupTiyan, got.Group)
}

func TestMaybeUpgradeUserToVIPAtThresholdUpgradesTiyanDefaultAndBlank(t *testing.T) {
	for _, tc := range []struct {
		name  string
		group string
	}{
		{name: "tiyan", group: UserGroupTiyan},
		{name: "default", group: UserGroupDefault},
		{name: "blank", group: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupTopupVIPUpgradeTestDB(t)
			user := createVIPUpgradeUser(t, tc.name, common.RoleCommonUser, tc.group)
			createSuccessTopUp(t, user.Id, tc.name+"-1", 10)
			createSuccessTopUp(t, user.Id, tc.name+"-2", 20)

			upgraded, err := MaybeUpgradeUserToVIP(user.Id)
			require.NoError(t, err)
			require.True(t, upgraded)

			var got User
			require.NoError(t, DB.First(&got, user.Id).Error)
			require.Equal(t, UserGroupVIP, got.Group)
		})
	}
}

func TestMaybeUpgradeUserToVIPDoesNotOverrideSpecialRolesOrTags(t *testing.T) {
	for _, tc := range []struct {
		name  string
		role  int
		group string
	}{
		{name: "admin", role: common.RoleAdminUser, group: UserGroupTiyan},
		{name: "root", role: common.RoleRootUser, group: UserGroupTiyan},
		{name: "special", role: common.RoleCommonUser, group: "partner"},
		{name: "vip", role: common.RoleCommonUser, group: UserGroupVIP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupTopupVIPUpgradeTestDB(t)
			user := createVIPUpgradeUser(t, tc.name, tc.role, tc.group)
			createSuccessTopUp(t, user.Id, tc.name+"-1", 30)

			upgraded, err := MaybeUpgradeUserToVIP(user.Id)
			require.NoError(t, err)
			require.False(t, upgraded)

			var got User
			require.NoError(t, DB.First(&got, user.Id).Error)
			require.Equal(t, tc.group, got.Group)
		})
	}
}

func TestMaybeUpgradeUserToVIPUsesAmountForDiscountedLDXPTopup(t *testing.T) {
	setupTopupVIPUpgradeTestDB(t)

	user := User{Username: "ldxp-vip-user", Role: common.RoleCommonUser, Group: UserGroupTiyan}
	require.NoError(t, DB.Create(&user).Error)

	// This deliberately uses money just below the VIP threshold while amount reaches it.
	// It proves VIP qualification uses the book amount, not the discounted real payment price.
	topUp := TopUp{
		UserId:          user.Id,
		Amount:          int64(VIPUpgradeThresholdMoney),
		Money:           VIPUpgradeThresholdMoney - 0.01,
		TradeNo:         "ldxp-discount-vip",
		PaymentMethod:   PaymentMethodLDXP,
		PaymentProvider: PaymentProviderLDXP,
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(&topUp).Error)

	upgraded, err := MaybeUpgradeUserToVIP(user.Id)
	require.NoError(t, err)
	require.True(t, upgraded)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, UserGroupVIP, updated.Group)
}
