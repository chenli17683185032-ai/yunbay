package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func TestWithGroupRatioOptionsLockSerializesOperations(t *testing.T) {
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		require.NoError(t, WithGroupRatioOptionsLock(func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		}))
	}()
	<-firstEntered

	secondEntered := make(chan struct{})
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		require.NoError(t, WithGroupRatioOptionsLock(func() error {
			close(secondEntered)
			return nil
		}))
	}()

	select {
	case <-secondEntered:
		t.Fatal("second group ratio operation entered while the first still held the lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	<-firstDone
	<-secondDone
}

func TestGroupRatioOptionWritesUseSharedOperationLock(t *testing.T) {
	tests := []struct {
		name      string
		operation func() error
	}{
		{name: "single option", operation: func() error {
			return UpdateOption("GroupRatio", `{"new":2}`)
		}},
		{name: "bulk options", operation: func() error {
			return UpdateOptionsBulk(map[string]string{
				"GroupRatio":      `{"new":2}`,
				"GroupGroupRatio": `{"new":{"child":1.25}}`,
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupOptionTestDB(t)
			preserveGroupRatioRuntime(t)
			require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
			require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"old":{"child":1}}`}).Error)
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"old":1}`))
			require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"old":{"child":1}}`))
			setOptionMapValueForTest("GroupRatio", `{"old":1}`)
			setOptionMapValueForTest("GroupGroupRatio", `{"old":{"child":1}}`)

			lockEntered := make(chan struct{})
			releaseLock := make(chan struct{})
			lockDone := make(chan struct{})
			go func() {
				defer close(lockDone)
				_ = WithGroupRatioOptionsLock(func() error {
					close(lockEntered)
					<-releaseLock
					return nil
				})
			}()
			<-lockEntered

			operationDone := make(chan error, 1)
			go func() { operationDone <- tt.operation() }()
			completedEarly := false
			select {
			case err := <-operationDone:
				require.NoError(t, err)
				completedEarly = true
			case <-time.After(100 * time.Millisecond):
			}
			close(releaseLock)
			<-lockDone
			if !completedEarly {
				require.NoError(t, <-operationDone)
			}
			require.False(t, completedEarly, "ratio write completed while the shared operation lock was held")
		})
	}
}

func TestLoadOptionsFromDatabasePreventsStaleSnapshotFromOverwritingPairUpdate(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"old":{"child":1}}`}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"old":1}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"old":{"child":1}}`))
	setOptionMapValueForTest("GroupRatio", `{"old":1}`)
	setOptionMapValueForTest("GroupGroupRatio", `{"old":{"child":1}}`)

	snapshotRead := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	var snapshotOnce sync.Once
	callbackName := "test:pause_option_sync_after_snapshot_read"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]*Option); ok {
			snapshotOnce.Do(func() {
				close(snapshotRead)
				<-releaseSnapshot
			})
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(callbackName)) })

	syncDone := make(chan struct{})
	go func() {
		defer close(syncDone)
		loadOptionsFromDatabase()
	}()
	<-snapshotRead

	pairLockAcquired := make(chan struct{})
	pairDone := make(chan error, 1)
	go func() {
		pairDone <- WithGroupRatioOptionsLock(func() error {
			close(pairLockAcquired)
			if err := UpdateGroupRatioOptions(`{"new":2}`, `{"new":{"child":1.25}}`); err != nil {
				return err
			}
			if err := updateOptionMap("GroupRatio", `{"new":2}`); err != nil {
				return err
			}
			return updateOptionMap("GroupGroupRatio", `{"new":{"child":1.25}}`)
		})
	}()
	acquiredEarly := false
	select {
	case <-pairLockAcquired:
		acquiredEarly = true
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseSnapshot)
	<-syncDone
	require.NoError(t, <-pairDone)
	require.False(t, acquiredEarly, "pair update acquired the shared lock after sync read a stale snapshot but before sync applied it")

	stored, err := GetGroupRatioOptions()
	require.NoError(t, err)
	require.Equal(t, `{"new":2}`, stored.GroupRatio)
	require.Equal(t, `{"new":{"child":1.25}}`, stored.GroupGroupRatio)
	require.Equal(t, stored.GroupRatio, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, stored.GroupGroupRatio, ratio_setting.GroupGroupRatio2JSONString())
	require.Equal(t, stored.GroupRatio, optionMapValueForTest("GroupRatio"))
	require.Equal(t, stored.GroupGroupRatio, optionMapValueForTest("GroupGroupRatio"))
}

func TestGetGroupRatioOptionsUsesStructuredKeyPredicates(t *testing.T) {
	db := setupOptionTestDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"old":{"child":1}}`}).Error)

	structuredPredicates := make([]bool, 0, 2)
	callbackName := "test:inspect_group_ratio_option_predicates"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*Option); !ok || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Option" {
			return
		}
		whereClause, ok := tx.Statement.Clauses["WHERE"]
		structuredPredicates = append(structuredPredicates, ok && containsStructuredOptionKeyPredicate(whereClause.Expression))
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(callbackName)) })

	_, err := GetGroupRatioOptions()

	require.NoError(t, err)
	require.Equal(t, []bool{true, true}, structuredPredicates)
}

func containsStructuredOptionKeyPredicate(expression clause.Expression) bool {
	switch predicate := expression.(type) {
	case clause.Where:
		for _, child := range predicate.Exprs {
			if containsStructuredOptionKeyPredicate(child) {
				return true
			}
		}
	case clause.AndConditions:
		for _, child := range predicate.Exprs {
			if containsStructuredOptionKeyPredicate(child) {
				return true
			}
		}
	case clause.OrConditions:
		for _, child := range predicate.Exprs {
			if containsStructuredOptionKeyPredicate(child) {
				return true
			}
		}
	case clause.Eq:
		switch column := predicate.Column.(type) {
		case clause.Column:
			return column.Name == "key"
		case string:
			return column == "key"
		}
	}
	return false
}
