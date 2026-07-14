package controller

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func setupUserGroupControllerTest(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldSQLitePath := common.SQLitePath
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.SQLitePath = filepath.Join(t.TempDir(), "group-controller.db")
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Setenv("SQL_DSN", "")
	require.NoError(t, model.InitDBWithoutMigrations())
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.UserValuePackagePreference{},
	))

	db := model.DB
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.SQLitePath = oldSQLitePath
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})
}

func preserveUserGroupRatioSettings(t *testing.T) {
	t.Helper()
	oldUsableGroups := setting.UserUsableGroups2JSONString()
	oldGroupRatio, oldGroupGroupRatio := ratio_setting.GroupRatioPair2JSONStrings()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioPairByJSONString(oldGroupRatio, oldGroupGroupRatio))
	})
}

func configureUserGroupRatioTest(t *testing.T, groupGroupRatio string) {
	t.Helper()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"claude-max":"Claude models"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioPairByJSONString(
		`{"claude-max":2.5}`,
		groupGroupRatio,
	))
}

func createUserGroupRatioTestUser(t *testing.T, username string) *model.User {
	t.Helper()
	user := &model.User{
		Username: username,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    model.UserGroupTiyan,
		AffCode:  "aff_" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func activateUserGroupRatioTestWeekCard(t *testing.T, userId int) {
	t.Helper()
	plan := model.SubscriptionPlan{
		Title:            "week value package",
		PriceAmount:      9.9,
		Currency:         "CNY",
		DurationUnit:     model.SubscriptionDurationDay,
		DurationValue:    7,
		Enabled:          true,
		PlanKind:         model.SubscriptionPlanKindValuePackage,
		PackageType:      model.ValuePackageTypeWeek,
		PackageLevel:     model.ValuePackageLevelWeek,
		ModelGroup:       "week-card",
		ConcurrencyLimit: 1,
		TotalAmount:      10000,
		Limit5hAmount:    1000,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	subscription := model.UserSubscription{
		UserId:      userId,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      model.UserSubscriptionStatusActive,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{
		UserId:                   userId,
		Enabled:                  true,
		ActiveUserSubscriptionId: subscription.Id,
		CreatedAt:                now,
		UpdatedAt:                now,
	}).Error)
}

func getUserGroupRatioFromResponse(t *testing.T, userId int, group string) float64 {
	t.Helper()
	recorder := valuePackageControllerRequest(GetUserGroups, http.MethodGet, "/api/user/self/groups", nil, userId)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"], recorder.Body.String())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	groupInfo, ok := data[group].(map[string]interface{})
	require.True(t, ok)
	ratio, ok := groupInfo["ratio"].(float64)
	require.True(t, ok)
	return ratio
}

func TestGetUserGroupsUsesConfiguredValuePackageGroupRatio(t *testing.T) {
	setupUserGroupControllerTest(t)
	preserveUserGroupRatioSettings(t)
	configureUserGroupRatioTest(t, `{"体验用户":{"claude-max":3.6},"week-card":{"claude-max":2.5}}`)
	user := createUserGroupRatioTestUser(t, "group_ratio_package_configured")
	activateUserGroupRatioTestWeekCard(t, user.Id)

	require.Equal(t, 2.5, getUserGroupRatioFromResponse(t, user.Id, "claude-max"))
}

func TestGetUserGroupsUsesOneForValuePackageWithoutConfiguredPair(t *testing.T) {
	setupUserGroupControllerTest(t)
	preserveUserGroupRatioSettings(t)
	configureUserGroupRatioTest(t, `{"体验用户":{"claude-max":3.6}}`)
	user := createUserGroupRatioTestUser(t, "group_ratio_package_default")
	activateUserGroupRatioTestWeekCard(t, user.Id)

	require.Equal(t, float64(1), getUserGroupRatioFromResponse(t, user.Id, "claude-max"))
}

func TestGetUserGroupsKeepsRegularUserRatioWithoutActivePackage(t *testing.T) {
	setupUserGroupControllerTest(t)
	preserveUserGroupRatioSettings(t)
	configureUserGroupRatioTest(t, `{"体验用户":{"claude-max":3.6},"week-card":{"claude-max":2.5}}`)
	user := createUserGroupRatioTestUser(t, "group_ratio_regular")

	require.Equal(t, 3.6, getUserGroupRatioFromResponse(t, user.Id, "claude-max"))
}

func TestGetUserGroupsKeepsRegularUserRatioWhenPackageIsDisabled(t *testing.T) {
	setupUserGroupControllerTest(t)
	preserveUserGroupRatioSettings(t)
	configureUserGroupRatioTest(t, `{"体验用户":{"claude-max":3.6},"week-card":{"claude-max":2.5}}`)
	user := createUserGroupRatioTestUser(t, "group_ratio_package_disabled")
	activateUserGroupRatioTestWeekCard(t, user.Id)
	require.NoError(t, model.DB.Model(&model.UserValuePackagePreference{}).
		Where("user_id = ?", user.Id).
		Updates(map[string]interface{}{
			"enabled":    false,
			"updated_at": common.GetTimestamp() + 1,
		}).Error)

	require.Equal(t, 3.6, getUserGroupRatioFromResponse(t, user.Id, "claude-max"))
}
