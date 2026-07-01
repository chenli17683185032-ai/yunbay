package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setupTopupAffiliateTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &Log{}, &AffiliateCommission{}, &AffiliateWithdrawal{}))
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

func TestRechargeEpayCreatesAffiliateCommissionForInviter(t *testing.T) {
	setupTopupAffiliateTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 12001, Username: "epay-inviter", Password: "hash", Status: common.UserStatusEnabled, AffCode: "epay-inviter-aff"}).Error)
	require.NoError(t, DB.Create(&User{Id: 12002, Username: "epay-invitee", Password: "hash", Status: common.UserStatusEnabled, AffCode: "epay-invitee-aff", InviterId: 12001}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          12002,
		Amount:          20,
		Money:           20,
		TradeNo:         "epay-affiliate-trade",
		PaymentMethod:   "wxpay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}).Error)

	err := RechargeEpay("epay-affiliate-trade", "alipay", "127.0.0.1")

	require.NoError(t, err)
	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", "epay-affiliate-trade").First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.Equal(t, "alipay", topUp.PaymentMethod)
	assert.NotZero(t, topUp.CompleteTime)

	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", 12002).First(&user).Error)
	assert.Equal(t, int(decimalQuotaForTopupAffiliateTest(20)), user.Quota)

	var commission AffiliateCommission
	require.NoError(t, DB.Where("topup_id = ?", topUp.Id).First(&commission).Error)
	assert.Equal(t, 12001, commission.InviterUserId)
	assert.Equal(t, 12002, commission.InviteeUserId)
	assert.Equal(t, 3.0, commission.CommissionMoney)
	assert.Equal(t, AffiliateCommissionStatusAvailable, commission.Status)
}

func TestRechargeEpayIncrementsQuotaCacheAfterTransaction(t *testing.T) {
	setupTopupAffiliateTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 12102, Username: "epay-cache-user", Password: "hash", Status: common.UserStatusEnabled, AffCode: "epay-cache-aff"}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          12102,
		Amount:          20,
		Money:           20,
		TradeNo:         "epay-cache-trade",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}).Error)

	var gotUserID int
	var gotDelta int64
	oldCacheIncr := rechargeEpayCacheIncrUserQuota
	rechargeEpayCacheIncrUserQuota = func(userID int, delta int64) error {
		gotUserID = userID
		gotDelta = delta
		return errors.New("cache unavailable")
	}
	t.Cleanup(func() { rechargeEpayCacheIncrUserQuota = oldCacheIncr })

	err := RechargeEpay("epay-cache-trade", "alipay", "127.0.0.1")

	require.NoError(t, err)
	assert.Equal(t, 12102, gotUserID)
	assert.Equal(t, decimalQuotaForTopupAffiliateTest(20), gotDelta)
}

func TestRechargeEpayTopUpQueryUsesRowLockClauseOutsideSQLite(t *testing.T) {
	oldUsingSQLite := common.UsingSQLite
	common.UsingSQLite = false
	t.Cleanup(func() { common.UsingSQLite = oldUsingSQLite })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	require.NoError(t, err)
	query := rechargeEpayTopUpQueryForUpdate(db, "epay-row-lock-trade")
	stmt := query.Statement

	require.Contains(t, stmt.Clauses, "FOR")
	assert.Equal(t, "UPDATE", stmt.Clauses["FOR"].Expression.(clause.Locking).Strength)
}

func decimalQuotaForTopupAffiliateTest(amount int64) int64 {
	return int64(float64(amount) * common.QuotaPerUnit)
}

func TestAffiliateCommissionUsesMoneyForDiscountedLDXPTopup(t *testing.T) {
	setupTopupAffiliateTestDB(t)

	inviter := User{Username: "ldxp-inviter", Role: common.RoleCommonUser, Group: UserGroupVIP, AffCode: "ldxp-inviter-aff"}
	require.NoError(t, DB.Create(&inviter).Error)
	invitee := User{Username: "ldxp-invitee", Role: common.RoleCommonUser, Group: UserGroupTiyan, AffCode: "ldxp-invitee-aff", InviterId: inviter.Id}
	require.NoError(t, DB.Create(&invitee).Error)

	topUp := TopUp{
		UserId:          invitee.Id,
		Amount:          500,
		Money:           425,
		TradeNo:         "ldxp-discount-affiliate",
		PaymentMethod:   PaymentMethodLDXP,
		PaymentProvider: PaymentProviderLDXP,
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(&topUp).Error)
	require.NoError(t, MaybeCreateAffiliateCommissionForTopUp(topUp.Id))

	var commission AffiliateCommission
	require.NoError(t, DB.Where("topup_id = ?", topUp.Id).First(&commission).Error)
	require.Equal(t, 425.0, commission.BaseMoney)
	require.Equal(t, 63.75, commission.CommissionMoney)
}
