package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
	})
	return db
}

func preserveGroupRatioRuntime(t *testing.T) {
	t.Helper()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	common.OptionMapRWMutex.RLock()
	originalOptionGroupRatio, hadOptionGroupRatio := common.OptionMap["GroupRatio"]
	originalOptionGroupGroupRatio, hadOptionGroupGroupRatio := common.OptionMap["GroupGroupRatio"]
	common.OptionMapRWMutex.RUnlock()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if hadOptionGroupRatio {
			common.OptionMap["GroupRatio"] = originalOptionGroupRatio
		} else {
			delete(common.OptionMap, "GroupRatio")
		}
		if hadOptionGroupGroupRatio {
			common.OptionMap["GroupGroupRatio"] = originalOptionGroupGroupRatio
		} else {
			delete(common.OptionMap, "GroupGroupRatio")
		}
	})
}

func setOptionMapValueForTest(key, value string) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[key] = value
}

func optionMapValueForTest(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.Interface2String(common.OptionMap[key])
}

func TestUpdateOptionReturnsFirstOrCreateErrorWithoutChangingRuntime(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"stable":1}`))
	setOptionMapValueForTest("GroupRatio", `{"stable":1}`)

	forcedErr := errors.New("forced option create failure")
	callbackName := "test:fail_option_first_or_create"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Option" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove(callbackName)) })

	err := UpdateOption("GroupRatio", `{"changed":2}`)

	require.ErrorIs(t, err, forcedErr)
	require.Equal(t, `{"stable":1}`, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, `{"stable":1}`, optionMapValueForTest("GroupRatio"))
}

func TestUpdateOptionReturnsSaveErrorWithoutChangingRuntime(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"stable":1}`}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"stable":1}`))
	setOptionMapValueForTest("GroupRatio", `{"stable":1}`)

	forcedErr := errors.New("forced option save failure")
	callbackName := "test:fail_option_save"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Option" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Update().Remove(callbackName)) })

	err := UpdateOption("GroupRatio", `{"changed":2}`)

	require.ErrorIs(t, err, forcedErr)
	require.Equal(t, `{"stable":1}`, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, `{"stable":1}`, optionMapValueForTest("GroupRatio"))
	var stored Option
	require.NoError(t, db.First(&stored, "key = ?", "GroupRatio").Error)
	require.Equal(t, `{"stable":1}`, stored.Value)
}

func TestUpdateGroupRatioOptionsRollsBackFirstWriteWhenSecondWriteFails(t *testing.T) {
	db := setupOptionTestDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"old":{"old":1}}`}).Error)

	forcedErr := errors.New("forced group group ratio save failure")
	callbackName := "test:fail_group_group_ratio_save"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		option, ok := tx.Statement.Dest.(*Option)
		if ok && option.Key == "GroupGroupRatio" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Update().Remove(callbackName)) })

	err := UpdateGroupRatioOptions(`{"new":2}`, `{"new":{"new":2}}`)

	require.ErrorIs(t, err, forcedErr)
	stored, readErr := GetGroupRatioOptions()
	require.NoError(t, readErr)
	require.Equal(t, `{"old":1}`, stored.GroupRatio)
	require.Equal(t, `{"old":{"old":1}}`, stored.GroupGroupRatio)
}
