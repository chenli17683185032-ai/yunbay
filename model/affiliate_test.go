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

func setupAffiliateTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &AffiliateCommission{}, &AffiliateWithdrawal{}))
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

func insertAffiliateUser(t *testing.T, id int, username string, inviterID int) User {
	t.Helper()
	user := User{
		Id:        id,
		Username:  username,
		Password:  "hash",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     UserGroupDefault,
		AffCode:   fmt.Sprintf("aff-%d", id),
		InviterId: inviterID,
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func insertAffiliateTopUp(t *testing.T, userID int, tradeNo string, money float64, status string) TopUp {
	t.Helper()
	topUp := TopUp{
		UserId:  userID,
		TradeNo: tradeNo,
		Money:   money,
		Amount:  int64(money),
		Status:  status,
	}
	require.NoError(t, DB.Create(&topUp).Error)
	return topUp
}

func TestMaybeCreateAffiliateCommissionForTopUpTxUsesExistingInviterId(t *testing.T) {
	db := setupAffiliateTestDB(t)
	insertAffiliateUser(t, 101, "inviter", 0)
	insertAffiliateUser(t, 102, "invitee", 101)
	topUp := insertAffiliateTopUp(t, 102, "trade-affiliate-success", 30, common.TopUpStatusSuccess)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return MaybeCreateAffiliateCommissionForTopUpTx(tx, &topUp)
	}))

	var commission AffiliateCommission
	require.NoError(t, DB.First(&commission).Error)
	require.Equal(t, 101, commission.InviterUserId)
	require.Equal(t, 102, commission.InviteeUserId)
	require.Equal(t, topUp.Id, commission.TopupId)
	require.Equal(t, topUp.TradeNo, commission.TradeNo)
	require.Equal(t, AffiliateCommissionStatusAvailable, commission.Status)
	require.Equal(t, 4.5, commission.CommissionMoney)
}

func TestMaybeCreateAffiliateCommissionForTopUpTxIsIdempotentByTopUpID(t *testing.T) {
	db := setupAffiliateTestDB(t)
	insertAffiliateUser(t, 201, "idem-inviter", 0)
	insertAffiliateUser(t, 202, "idem-invitee", 201)
	topUp := insertAffiliateTopUp(t, 202, "trade-affiliate-idempotent", 30, common.TopUpStatusSuccess)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return MaybeCreateAffiliateCommissionForTopUpTx(tx, &topUp)
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return MaybeCreateAffiliateCommissionForTopUpTx(tx, &topUp)
	}))

	var count int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Where("topup_id = ?", topUp.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestMaybeCreateAffiliateCommissionForTopUpTxSkipsInvalidCases(t *testing.T) {
	db := setupAffiliateTestDB(t)
	insertAffiliateUser(t, 301, "invalid-inviter", 0)
	insertAffiliateUser(t, 302, "no-inviter", 0)
	insertAffiliateUser(t, 303, "self-inviter", 303)
	insertAffiliateUser(t, 304, "pending-invitee", 301)
	insertAffiliateUser(t, 305, "zero-money-invitee", 301)

	topUps := []TopUp{
		insertAffiliateTopUp(t, 302, "trade-affiliate-no-inviter", 30, common.TopUpStatusSuccess),
		insertAffiliateTopUp(t, 303, "trade-affiliate-self", 30, common.TopUpStatusSuccess),
		insertAffiliateTopUp(t, 304, "trade-affiliate-pending", 30, common.TopUpStatusPending),
		insertAffiliateTopUp(t, 305, "trade-affiliate-zero", 0, common.TopUpStatusSuccess),
	}

	for i := range topUps {
		topUp := topUps[i]
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return MaybeCreateAffiliateCommissionForTopUpTx(tx, &topUp)
		}))
	}

	var count int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func createAvailableCommissionForWithdrawalTest(t *testing.T, inviterID, inviteeID, topupID int, money float64) AffiliateCommission {
	t.Helper()
	insertAffiliateUser(t, inviterID, fmt.Sprintf("withdrawal-inviter-%d", inviterID), 0)
	insertAffiliateUser(t, inviteeID, fmt.Sprintf("withdrawal-invitee-%d", inviteeID), inviterID)
	topUp := TopUp{
		Id:      topupID,
		UserId:  inviteeID,
		TradeNo: fmt.Sprintf("trade-affiliate-withdrawal-%d", topupID),
		Money:   money,
		Amount:  int64(money),
		Status:  common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(&topUp).Error)
	require.NoError(t, MaybeCreateAffiliateCommissionForTopUp(topUp.Id))

	var commission AffiliateCommission
	require.NoError(t, DB.Where("topup_id = ?", topUp.Id).First(&commission).Error)
	require.Equal(t, AffiliateCommissionStatusAvailable, commission.Status)
	return commission
}

func TestCreateAffiliateWithdrawalFreezesAvailableRewards(t *testing.T) {
	setupAffiliateTestDB(t)
	createAvailableCommissionForWithdrawalTest(t, 401, 402, 403, 100)

	withdrawal, err := CreateAffiliateWithdrawal(401, 10, " user@example.com ", " first payout ")
	require.NoError(t, err)
	require.NotNil(t, withdrawal)
	require.NotZero(t, withdrawal.Id)
	require.NotEmpty(t, withdrawal.WithdrawalId)
	require.Equal(t, 401, withdrawal.UserId)
	require.Equal(t, 10.0, withdrawal.Amount)
	require.Equal(t, "user@example.com", withdrawal.Contact)
	require.Equal(t, "first payout", withdrawal.Remark)
	require.Equal(t, AffiliateWithdrawalStatusPending, withdrawal.Status)

	summary, err := GetAffiliateSummary(401)
	require.NoError(t, err)
	require.Equal(t, 15.0, summary.TotalMoney)
	require.Equal(t, 5.0, summary.AvailableMoney)
	require.Equal(t, 10.0, summary.FrozenMoney)
	require.Equal(t, 0.0, summary.WithdrawnMoney)
}

func TestCreateAffiliateWithdrawalRejectsInvalidAndOverdraw(t *testing.T) {
	setupAffiliateTestDB(t)
	createAvailableCommissionForWithdrawalTest(t, 501, 502, 503, 20)

	_, err := CreateAffiliateWithdrawal(501, 0, "user@example.com", "")
	require.ErrorIs(t, err, ErrAffiliateInvalidAmount)

	_, err = CreateAffiliateWithdrawal(501, 1, "   ", "")
	require.ErrorIs(t, err, ErrAffiliateContactRequired)

	_, err = CreateAffiliateWithdrawal(501, 4, "user@example.com", "")
	require.ErrorIs(t, err, ErrAffiliateInsufficientBalance)
}

func TestMarkAffiliateWithdrawalPaidAndRejectedAdjustsComputedBalances(t *testing.T) {
	setupAffiliateTestDB(t)
	createAvailableCommissionForWithdrawalTest(t, 601, 602, 603, 200)

	paidWithdrawal, err := CreateAffiliateWithdrawal(601, 12, "user@example.com", "paid request")
	require.NoError(t, err)
	paidWithdrawal, err = MarkAffiliateWithdrawalPaid(paidWithdrawal.Id, " paid ")
	require.NoError(t, err)
	require.Equal(t, AffiliateWithdrawalStatusPaid, paidWithdrawal.Status)
	require.Equal(t, "paid", paidWithdrawal.AdminRemark)
	require.NotZero(t, paidWithdrawal.ProcessedTime)

	rejectedWithdrawal, err := CreateAffiliateWithdrawal(601, 8, "user@example.com", "rejected request")
	require.NoError(t, err)
	rejectedWithdrawal, err = RejectAffiliateWithdrawal(rejectedWithdrawal.Id, " rejected ")
	require.NoError(t, err)
	require.Equal(t, AffiliateWithdrawalStatusRejected, rejectedWithdrawal.Status)
	require.Equal(t, "rejected", rejectedWithdrawal.AdminRemark)
	require.NotZero(t, rejectedWithdrawal.ProcessedTime)

	summary, err := GetAffiliateSummary(601)
	require.NoError(t, err)
	require.Equal(t, 30.0, summary.TotalMoney)
	require.Equal(t, 18.0, summary.AvailableMoney)
	require.Equal(t, 0.0, summary.FrozenMoney)
	require.Equal(t, 12.0, summary.WithdrawnMoney)
}
