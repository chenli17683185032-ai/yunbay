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

func setupUserGroupInviteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldQuotaForNewUser := common.QuotaForNewUser
	oldQuotaForInvitee := common.QuotaForInvitee
	oldQuotaForInviter := common.QuotaForInviter

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Log{}))
	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.QuotaForNewUser = oldQuotaForNewUser
		common.QuotaForInvitee = oldQuotaForInvitee
		common.QuotaForInviter = oldQuotaForInviter
	})

	return db
}

func TestUserInsertDefaultsCommonUserToTiyanTag(t *testing.T) {
	setupUserGroupInviteTestDB(t)

	user := &User{Username: "exp-user", Password: "password123", DisplayName: "Exp", Role: common.RoleCommonUser}
	require.NoError(t, user.Insert(0))

	var got User
	require.NoError(t, DB.Where("username = ?", "exp-user").First(&got).Error)
	require.Equal(t, UserGroupTiyan, got.Group)
}

func TestUserInsertKeepsExplicitAdminGroup(t *testing.T) {
	setupUserGroupInviteTestDB(t)

	user := &User{Username: "admin-user", Password: "password123", DisplayName: "Admin", Role: common.RoleAdminUser, Group: "ops"}
	require.NoError(t, user.Insert(0))

	var got User
	require.NoError(t, DB.Where("username = ?", "admin-user").First(&got).Error)
	require.Equal(t, "ops", got.Group)
}

func TestUserInsertWithTxDefaultsCommonUserToTiyanTag(t *testing.T) {
	setupUserGroupInviteTestDB(t)

	err := DB.Transaction(func(tx *gorm.DB) error {
		user := &User{Username: "oauth-user", DisplayName: "OAuth", Role: common.RoleCommonUser}
		return user.InsertWithTx(tx, 0)
	})
	require.NoError(t, err)

	var got User
	require.NoError(t, DB.Where("username = ?", "oauth-user").First(&got).Error)
	require.Equal(t, UserGroupTiyan, got.Group)
}

func TestCountInviteesByInviterIDExcludesSoftDeletedUsers(t *testing.T) {
	setupUserGroupInviteTestDB(t)

	inviter := User{Username: "inviter", Password: "hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: UserGroupTiyan}
	require.NoError(t, DB.Create(&inviter).Error)
	active := User{Username: "active", Password: "hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: UserGroupTiyan, AffCode: "ACT1", InviterId: inviter.Id}
	deleted := User{Username: "deleted", Password: "hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: UserGroupTiyan, AffCode: "DEL1", InviterId: inviter.Id}
	require.NoError(t, DB.Create(&active).Error)
	require.NoError(t, DB.Create(&deleted).Error)
	require.NoError(t, DB.Delete(&deleted).Error)

	count, err := CountInviteesByInviterID(inviter.Id)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	counts, err := CountInviteesByInviterIDs([]int{inviter.Id, 999999})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts[inviter.Id])
	require.Equal(t, int64(0), counts[999999])
}
