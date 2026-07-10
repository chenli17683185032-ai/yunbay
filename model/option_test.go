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
	firstAttemptStarted := make(chan struct{})
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		close(firstAttemptStarted)
		firstDone <- WithGroupRatioOptionsLock(func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstAttemptStarted
	<-firstEntered

	secondAttemptStarted := make(chan struct{})
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondAttemptStarted)
		secondDone <- WithGroupRatioOptionsLock(func() error {
			close(secondEntered)
			return nil
		})
	}()
	<-secondAttemptStarted

	enteredEarly := false
	select {
	case <-secondEntered:
		enteredEarly = true
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
	require.False(t, enteredEarly, "second group ratio operation entered while the first still held the lock")
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
			lockDone := make(chan error, 1)
			go func() {
				lockDone <- WithGroupRatioOptionsLock(func() error {
					close(lockEntered)
					<-releaseLock
					return nil
				})
			}()
			<-lockEntered

			attemptStarted := make(chan struct{})
			operationDone := make(chan error, 1)
			go func() {
				close(attemptStarted)
				operationDone <- tt.operation()
			}()
			<-attemptStarted
			completedEarly := false
			select {
			case err := <-operationDone:
				require.NoError(t, err)
				completedEarly = true
			case <-time.After(100 * time.Millisecond):
			}
			close(releaseLock)
			require.NoError(t, <-lockDone)
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
	pairAttemptStarted := make(chan struct{})
	pairDone := make(chan error, 1)
	go func() {
		close(pairAttemptStarted)
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
	<-pairAttemptStarted
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

	queryCount := 0
	structuredINPredicate := false
	callbackName := "test:inspect_group_ratio_option_predicates"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Option" {
			return
		}
		queryCount++
		whereClause, ok := tx.Statement.Clauses["WHERE"]
		structuredINPredicate = structuredINPredicate || ok && containsStructuredOptionKeyINPredicate(whereClause.Expression)
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(callbackName)) })

	_, err := GetGroupRatioOptions()

	require.NoError(t, err)
	require.Equal(t, 1, queryCount)
	require.True(t, structuredINPredicate)
}

func TestGetGroupRatioOptionsReturnsErrorWhenEitherKeyIsMissing(t *testing.T) {
	db := setupOptionTestDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)

	_, err := GetGroupRatioOptions()

	require.Error(t, err)
}

func TestDeprecatedGroupRatioAliasesAreRejectedBeforeModelWrites(t *testing.T) {
	tests := []struct {
		name      string
		operation func() error
		keys      []string
	}{
		{
			name: "single alias",
			operation: func() error {
				return UpdateOption("group_ratio_setting.group_ratio", `{"changed":2}`)
			},
			keys: []string{"group_ratio_setting.group_ratio"},
		},
		{
			name: "bulk alias rejects all keys",
			operation: func() error {
				return UpdateOptionsBulk(map[string]string{
					"AliasSafeKey":                          "must-not-commit",
					"group_ratio_setting.group_group_ratio": `{"changed":{"child":2}}`,
				})
			},
			keys: []string{"AliasSafeKey", "group_ratio_setting.group_group_ratio"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupOptionTestDB(t)
			common.OptionMapRWMutex.Lock()
			if common.OptionMap == nil {
				common.OptionMap = make(map[string]string)
			}
			common.OptionMapRWMutex.Unlock()
			err := tt.operation()
			require.Error(t, err)
			for _, key := range tt.keys {
				var count int64
				require.NoError(t, db.Model(&Option{}).Where(&Option{Key: key}).Count(&count).Error)
				require.Zero(t, count, key)
			}
		})
	}
}

func TestLoadOptionsFromDatabaseIgnoresDeprecatedGroupRatioAliases(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	const groupAlias = "group_ratio_setting.group_ratio"
	const nestedAlias = "group_ratio_setting.group_group_ratio"
	require.NoError(t, db.Create(&Option{Key: groupAlias, Value: `{"alias":9}`}).Error)
	require.NoError(t, db.Create(&Option{Key: nestedAlias, Value: `{"alias":{"child":9}}`}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioPairByJSONString(`{"stable":1}`, `{"stable":{"child":2}}`))
	common.OptionMapRWMutex.Lock()
	delete(common.OptionMap, groupAlias)
	delete(common.OptionMap, nestedAlias)
	common.OptionMapRWMutex.Unlock()

	loadOptionsFromDatabase()

	groupRatio, groupGroupRatio := ratio_setting.GroupRatioPair2JSONStrings()
	require.Equal(t, `{"stable":1}`, groupRatio)
	require.Equal(t, `{"stable":{"child":2}}`, groupGroupRatio)
	common.OptionMapRWMutex.RLock()
	_, hasGroupAlias := common.OptionMap[groupAlias]
	_, hasNestedAlias := common.OptionMap[nestedAlias]
	common.OptionMapRWMutex.RUnlock()
	require.False(t, hasGroupAlias)
	require.False(t, hasNestedAlias)
}

func TestUpdateOptionValidatesCanonicalGroupRatioBeforeWriting(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "top null entry", key: "GroupRatio", value: `{"bad":null}`},
		{name: "nested zero", key: "GroupGroupRatio", value: `{"bad":{"child":0}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupOptionTestDB(t)
			preserveGroupRatioRuntime(t)
			require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"stable":1}`}).Error)
			require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"stable":{"child":2}}`}).Error)
			require.NoError(t, ratio_setting.UpdateGroupRatioPairByJSONString(`{"stable":1}`, `{"stable":{"child":2}}`))
			setOptionMapValueForTest("GroupRatio", `{"stable":1}`)
			setOptionMapValueForTest("GroupGroupRatio", `{"stable":{"child":2}}`)

			err := UpdateOption(tt.key, tt.value)

			require.Error(t, err)
			stored, readErr := GetGroupRatioOptions()
			require.NoError(t, readErr)
			require.Equal(t, `{"stable":1}`, stored.GroupRatio)
			require.Equal(t, `{"stable":{"child":2}}`, stored.GroupGroupRatio)
			groupRatio, groupGroupRatio := ratio_setting.GroupRatioPair2JSONStrings()
			require.Equal(t, stored.GroupRatio, groupRatio)
			require.Equal(t, stored.GroupGroupRatio, groupGroupRatio)
			require.Equal(t, stored.GroupRatio, optionMapValueForTest("GroupRatio"))
			require.Equal(t, stored.GroupGroupRatio, optionMapValueForTest("GroupGroupRatio"))
		})
	}
}

func TestUpdateOptionStoresNormalizedCanonicalGroupRatio(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":1}`}).Error)
	setOptionMapValueForTest("GroupRatio", `{"old":1}`)

	err := UpdateOption("GroupRatio", `{" zero ":0," paid ":1.5}`)

	require.NoError(t, err)
	var stored Option
	require.NoError(t, db.Where(&Option{Key: "GroupRatio"}).First(&stored).Error)
	require.Equal(t, `{"paid":1.5,"zero":0}`, stored.Value)
	require.Equal(t, stored.Value, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, stored.Value, optionMapValueForTest("GroupRatio"))
}

func TestUpdateOptionsBulkRejectsInvalidCanonicalRatioBeforeAnyWrite(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"stable":1}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"stable":{"child":2}}`}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioPairByJSONString(`{"stable":1}`, `{"stable":{"child":2}}`))
	setOptionMapValueForTest("GroupRatio", `{"stable":1}`)
	setOptionMapValueForTest("GroupGroupRatio", `{"stable":{"child":2}}`)

	err := UpdateOptionsBulk(map[string]string{
		"BulkSafeKey": "must-not-commit",
		"GroupRatio":  `{"bad":null}`,
	})

	require.Error(t, err)
	var safeCount int64
	require.NoError(t, db.Model(&Option{}).Where(&Option{Key: "BulkSafeKey"}).Count(&safeCount).Error)
	require.Zero(t, safeCount)
	stored, readErr := GetGroupRatioOptions()
	require.NoError(t, readErr)
	require.Equal(t, `{"stable":1}`, stored.GroupRatio)
	require.Equal(t, `{"stable":{"child":2}}`, stored.GroupGroupRatio)
	groupRatio, groupGroupRatio := ratio_setting.GroupRatioPair2JSONStrings()
	require.Equal(t, stored.GroupRatio, groupRatio)
	require.Equal(t, stored.GroupGroupRatio, groupGroupRatio)
	require.Equal(t, stored.GroupRatio, optionMapValueForTest("GroupRatio"))
	require.Equal(t, stored.GroupGroupRatio, optionMapValueForTest("GroupGroupRatio"))
}

func TestUpdateGroupRatioOptionsValidatesAndNormalizesBeforeTransaction(t *testing.T) {
	db := setupOptionTestDB(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"stable":1}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"stable":{"child":2}}`}).Error)

	err := UpdateGroupRatioOptions(`{" changed ":3}`, `{"bad":{"child":0}}`)
	require.Error(t, err)
	stored, readErr := GetGroupRatioOptions()
	require.NoError(t, readErr)
	require.Equal(t, `{"stable":1}`, stored.GroupRatio)
	require.Equal(t, `{"stable":{"child":2}}`, stored.GroupGroupRatio)

	require.NoError(t, UpdateGroupRatioOptions(`{" zero ":0}`, `{" user ":{" child ":1.25}}`))
	stored, readErr = GetGroupRatioOptions()
	require.NoError(t, readErr)
	require.Equal(t, `{"zero":0}`, stored.GroupRatio)
	require.Equal(t, `{"user":{"child":1.25}}`, stored.GroupGroupRatio)
}

func TestLoadOptionsFromDatabaseKeepsRuntimePairOnInvalidHistoricalCanonicalValue(t *testing.T) {
	db := setupOptionTestDB(t)
	preserveGroupRatioRuntime(t)
	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"bad":null}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "GroupGroupRatio", Value: `{"changed":{"child":3}}`}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioPairByJSONString(`{"stable":1}`, `{"stable":{"child":2}}`))
	setOptionMapValueForTest("GroupRatio", `{"stable":1}`)
	setOptionMapValueForTest("GroupGroupRatio", `{"stable":{"child":2}}`)

	loadOptionsFromDatabase()

	groupRatio, groupGroupRatio := ratio_setting.GroupRatioPair2JSONStrings()
	require.Equal(t, `{"stable":1}`, groupRatio)
	require.Equal(t, `{"stable":{"child":2}}`, groupGroupRatio)
	require.Equal(t, groupRatio, optionMapValueForTest("GroupRatio"))
	require.Equal(t, groupGroupRatio, optionMapValueForTest("GroupGroupRatio"))
}

func containsStructuredOptionKeyINPredicate(expression clause.Expression) bool {
	switch predicate := expression.(type) {
	case clause.Where:
		for _, child := range predicate.Exprs {
			if containsStructuredOptionKeyINPredicate(child) {
				return true
			}
		}
	case clause.AndConditions:
		for _, child := range predicate.Exprs {
			if containsStructuredOptionKeyINPredicate(child) {
				return true
			}
		}
	case clause.OrConditions:
		for _, child := range predicate.Exprs {
			if containsStructuredOptionKeyINPredicate(child) {
				return true
			}
		}
	case clause.IN:
		switch column := predicate.Column.(type) {
		case clause.Column:
			return column.Name == "key"
		case string:
			return column == "key"
		}
	}
	return false
}
