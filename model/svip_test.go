package model

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSVIPTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &SVIPValidTopupReconcileReceipt{}))
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

func createSVIPTestUser(t *testing.T, username string) User {
	t.Helper()
	user := User{Username: username, Password: "hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: UserGroupDefault, AffCode: username + "-aff"}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createSVIPTestTopUp(t *testing.T, userID int, tradeNo string, amount int64, money float64, provider string, status string) {
	t.Helper()
	topUp := TopUp{
		UserId:          userID,
		Amount:          amount,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   provider,
		PaymentProvider: provider,
		Status:          status,
	}
	require.NoError(t, DB.Create(&topUp).Error)
}

func TestMoneyToValidTopupCents(t *testing.T) {
	assert.Equal(t, int64(0), MoneyToValidTopupCents(0))
	assert.Equal(t, int64(0), MoneyToValidTopupCents(-10))
	assert.Equal(t, int64(0), MoneyToValidTopupCents(math.NaN()))
	assert.Equal(t, int64(0), MoneyToValidTopupCents(math.Inf(1)))
	assert.Equal(t, int64(0), MoneyToValidTopupCents(float64(math.MaxInt64)))
	assert.Equal(t, int64(20000), MoneyToValidTopupCents(200))
	assert.Equal(t, int64(1999), MoneyToValidTopupCents(19.99))
	// 浮点表示 0.1+0.2 之类的误差应被四舍五入吸收
	assert.Equal(t, int64(30), MoneyToValidTopupCents(0.1+0.2))
	assert.Equal(t, int64(20_000), AmountToValidTopupCents(200))
	assert.Equal(t, int64(0), AmountToValidTopupCents(math.MaxInt64))
}

func TestIsSVIPValidTopupCents(t *testing.T) {
	assert.False(t, IsSVIPValidTopupCents(0))
	assert.False(t, IsSVIPValidTopupCents(19999))
	assert.True(t, IsSVIPValidTopupCents(20000))
	assert.True(t, IsSVIPValidTopupCents(100000))

	user := &User{ValidTopupCents: 20000}
	assert.True(t, user.IsSVIP())
	user.ValidTopupCents = 19999
	assert.False(t, user.IsSVIP())
	var nilUser *User
	assert.False(t, nilUser.IsSVIP())
}

func TestUpdateSettingDoesNotOverwriteConcurrentUserValues(t *testing.T) {
	setupSVIPTestDB(t)
	user := createSVIPTestUser(t, "svip-setting-only")
	stale := user

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"quota":             700,
		"valid_topup_cents": 12_345,
	}).Error)
	setting := stale.GetSetting()
	setting.SvipCelebrationSeen = true
	stale.SetSetting(setting)
	require.NoError(t, stale.UpdateSetting())

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, 700, got.Quota)
	assert.Equal(t, int64(12_345), got.ValidTopupCents)
	assert.True(t, got.GetSetting().SvipCelebrationSeen)
}

func TestAddUserValidTopupCents(t *testing.T) {
	setupSVIPTestDB(t)
	user := createSVIPTestUser(t, "svip-add")

	require.NoError(t, AddUserValidTopupCents(user.Id, 5000))
	require.NoError(t, AddUserValidTopupCents(user.Id, 15000))
	// 非正数与非法用户 ID 应为无操作
	require.NoError(t, AddUserValidTopupCents(user.Id, 0))
	require.NoError(t, AddUserValidTopupCents(user.Id, -100))
	require.NoError(t, AddUserValidTopupCents(0, 100))
	require.ErrorIs(t, AddUserValidTopupCents(user.Id+999, 100), gorm.ErrRecordNotFound)

	var got User
	require.NoError(t, DB.First(&got, "id = ?", user.Id).Error)
	assert.Equal(t, int64(20000), got.ValidTopupCents)
	assert.Equal(t, int64(20000), got.TopupWatermark)
	assert.True(t, got.IsSVIP())

	adminUser := createSVIPTestUser(t, "svip-admin-add")
	require.NoError(t, IncreaseUserQuotaAndValidTopupCents(adminUser.Id, 100, 5000))
	var adminGot User
	require.NoError(t, DB.First(&adminGot, "id = ?", adminUser.Id).Error)
	assert.Equal(t, int64(5000), adminGot.ValidTopupCents)
	assert.Zero(t, adminGot.TopupWatermark)
}

func TestBackfillUserValidTopupCents(t *testing.T) {
	setupSVIPTestDB(t)
	ldxpUser := createSVIPTestUser(t, "svip-backfill-ldxp")
	cardUser := createSVIPTestUser(t, "svip-backfill-card")
	stripeUser := createSVIPTestUser(t, "svip-backfill-stripe")
	prefilledUser := createSVIPTestUser(t, "svip-backfill-prefilled")
	aheadUser := createSVIPTestUser(t, "svip-backfill-ahead")
	adminOnlyUser := createSVIPTestUser(t, "svip-backfill-admin-only")
	require.NoError(t, DB.Model(&User{}).Where("id = ?", prefilledUser.Id).Update("valid_topup_cents", 1234).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", aheadUser.Id).Update("valid_topup_cents", 12000).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", adminOnlyUser.Id).Update("valid_topup_cents", 3000).Error)

	// 联动小铺直充：Amount 为面值（元）
	createSVIPTestTopUp(t, ldxpUser.Id, "ldxp:1", 100, 100, PaymentProviderLDXP, common.TopUpStatusSuccess)
	// 超值套餐：Amount=0，Money 为实付（元）
	createSVIPTestTopUp(t, ldxpUser.Id, "ldxp:2", 0, 128, PaymentProviderLDXP, common.TopUpStatusSuccess)
	// 未成功的流水不计
	createSVIPTestTopUp(t, ldxpUser.Id, "ldxp:3", 500, 500, PaymentProviderLDXP, "pending")
	// 卡密兑换
	createSVIPTestTopUp(t, cardUser.Id, "redemption:1", 50, 50, PaymentProviderRedemptionCode, common.TopUpStatusSuccess)
	// 其他支付渠道不计（Amount 非人民币口径）
	createSVIPTestTopUp(t, stripeUser.Id, "stripe:1", 300, 300, PaymentProviderStripe, common.TopUpStatusSuccess)
	// 低于可靠历史总额的已有累计需要向上补齐；高于历史总额的值不得降低
	createSVIPTestTopUp(t, prefilledUser.Id, "ldxp:prefilled", 90, 90, PaymentProviderLDXP, common.TopUpStatusSuccess)
	createSVIPTestTopUp(t, aheadUser.Id, "ldxp:ahead", 90, 90, PaymentProviderLDXP, common.TopUpStatusSuccess)

	require.NoError(t, backfillUserValidTopupCents())

	fetchUser := func(id int) User {
		var got User
		require.NoError(t, DB.First(&got, "id = ?", id).Error)
		return got
	}

	got := fetchUser(ldxpUser.Id)
	assert.Equal(t, int64(22800), got.ValidTopupCents)
	assert.True(t, got.IsSVIP())

	got = fetchUser(cardUser.Id)
	assert.Equal(t, int64(5000), got.ValidTopupCents)
	assert.False(t, got.IsSVIP())

	assert.Equal(t, int64(0), fetchUser(stripeUser.Id).ValidTopupCents)
	assert.Equal(t, int64(9000), fetchUser(prefilledUser.Id).ValidTopupCents)
	assert.Equal(t, int64(12000), fetchUser(aheadUser.Id).ValidTopupCents)
	assert.Equal(t, int64(3000), fetchUser(adminOnlyUser.Id).ValidTopupCents)
	assert.Equal(t, int64(22800), fetchUser(ldxpUser.Id).TopupWatermark)
	assert.Equal(t, int64(5000), fetchUser(cardUser.Id).TopupWatermark)
	assert.Equal(t, int64(9000), fetchUser(prefilledUser.Id).TopupWatermark)
	assert.Equal(t, int64(9000), fetchUser(aheadUser.Id).TopupWatermark)
	assert.Zero(t, fetchUser(adminOnlyUser.Id).TopupWatermark)

	var receipt SVIPValidTopupReconcileReceipt
	require.NoError(t, DB.First(&receipt, "migration_version = ?", SVIPValidTopupReconcileVersion).Error)

	// 模拟回滚到旧应用期间新增成功流水：再次启动按历史水位差补入，保留管理员额外累计
	createSVIPTestTopUp(t, cardUser.Id, "redemption:2", 50, 50, PaymentProviderRedemptionCode, common.TopUpStatusSuccess)
	createSVIPTestTopUp(t, aheadUser.Id, "ldxp:ahead-after-rollback", 50, 50, PaymentProviderLDXP, common.TopUpStatusSuccess)
	createSVIPTestTopUp(t, adminOnlyUser.Id, "ldxp:admin-only-after-rollback", 40, 40, PaymentProviderLDXP, common.TopUpStatusSuccess)
	require.NoError(t, backfillUserValidTopupCents())
	assert.Equal(t, int64(10000), fetchUser(cardUser.Id).ValidTopupCents)
	assert.Equal(t, int64(9000), fetchUser(prefilledUser.Id).ValidTopupCents)
	assert.Equal(t, int64(17000), fetchUser(aheadUser.Id).ValidTopupCents)
	assert.Equal(t, int64(7000), fetchUser(adminOnlyUser.Id).ValidTopupCents)
	assert.Equal(t, int64(10000), fetchUser(cardUser.Id).TopupWatermark)
	assert.Equal(t, int64(14000), fetchUser(aheadUser.Id).TopupWatermark)
	assert.Equal(t, int64(4000), fetchUser(adminOnlyUser.Id).TopupWatermark)

	// 幂等：没有新增可靠流水时重复执行不再变化
	require.NoError(t, backfillUserValidTopupCents())
	assert.Equal(t, int64(10000), fetchUser(cardUser.Id).ValidTopupCents)
	assert.Equal(t, int64(9000), fetchUser(prefilledUser.Id).ValidTopupCents)
	assert.Equal(t, int64(17000), fetchUser(aheadUser.Id).ValidTopupCents)
	assert.Equal(t, int64(7000), fetchUser(adminOnlyUser.Id).ValidTopupCents)
}

func TestBackfillUserValidTopupCentsOverflowRollsBack(t *testing.T) {
	setupSVIPTestDB(t)
	user := createSVIPTestUser(t, "svip-backfill-overflow")
	largeAmount := int64(math.MaxInt64 / 100)
	createSVIPTestTopUp(t, user.Id, "ldxp:overflow-1", largeAmount, 0, PaymentProviderLDXP, common.TopUpStatusSuccess)
	createSVIPTestTopUp(t, user.Id, "ldxp:overflow-2", largeAmount, 0, PaymentProviderLDXP, common.TopUpStatusSuccess)

	err := backfillUserValidTopupCents()
	require.EqualError(t, err, "valid topup backfill total overflow")

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Zero(t, got.ValidTopupCents)
	assert.Zero(t, got.TopupWatermark)

	var receiptCount int64
	require.NoError(t, DB.Model(&SVIPValidTopupReconcileReceipt{}).Count(&receiptCount).Error)
	assert.Zero(t, receiptCount)
}
