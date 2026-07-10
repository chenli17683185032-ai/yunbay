package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type groupRatioOptionsTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		GroupRatio      string   `json:"group_ratio"`
		GroupGroupRatio string   `json:"group_group_ratio"`
		PackageGroups   []string `json:"package_groups"`
	} `json:"data"`
}

func setupGroupRatioOptionsControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	common.OptionMapRWMutex.RLock()
	originalOptionGroupRatio, hadOptionGroupRatio := common.OptionMap["GroupRatio"]
	originalOptionGroupGroupRatio, hadOptionGroupGroupRatio := common.OptionMap["GroupGroupRatio"]
	common.OptionMapRWMutex.RUnlock()

	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SubscriptionPlan{}, &model.User{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		common.OptionMapRWMutex.Lock()
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
		common.OptionMapRWMutex.Unlock()
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedGroupRatioOptionsState(t *testing.T, db *gorm.DB, groupRatio, groupGroupRatio string) {
	t.Helper()
	require.NoError(t, db.Create(&model.Option{Key: "GroupRatio", Value: groupRatio}).Error)
	require.NoError(t, db.Create(&model.Option{Key: "GroupGroupRatio", Value: groupGroupRatio}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(groupRatio))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(groupGroupRatio))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["GroupRatio"] = groupRatio
	common.OptionMap["GroupGroupRatio"] = groupGroupRatio
	common.OptionMapRWMutex.Unlock()
}

func addEnabledValuePackagePlanForGroupRatioTest(t *testing.T, db *gorm.DB, title, group string) {
	t.Helper()
	plan := model.SubscriptionPlan{
		Title:       title,
		Enabled:     true,
		PlanKind:    model.SubscriptionPlanKindValuePackage,
		PackageType: model.ValuePackageTypeDay,
		ModelGroup:  group,
	}
	require.NoError(t, db.Create(&plan).Error)
}

func performGroupRatioOptionsRequest(t *testing.T, method string, body any, handler gin.HandlerFunc) (*httptest.ResponseRecorder, groupRatioOptionsTestResponse) {
	t.Helper()
	var requestBody []byte
	if body != nil {
		var err error
		requestBody, err = common.Marshal(body)
		require.NoError(t, err)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/option/group-ratios", bytes.NewReader(requestBody))
	handler(c)
	var response groupRatioOptionsTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func readStoredGroupRatioOptions(t *testing.T) model.GroupRatioOptions {
	t.Helper()
	stored, err := model.GetGroupRatioOptions()
	require.NoError(t, err)
	return stored
}

func readGroupRatioOptionMapForControllerTest() (string, string) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.Interface2String(common.OptionMap["GroupRatio"]), common.Interface2String(common.OptionMap["GroupGroupRatio"])
}

func TestGetGroupRatioOptionsReturnsNormalizedSnapshotAndPackageGroups(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{" z ":0,"a":1.5}`, `{" empty ":{}," month-card ":{" gpt-pro ":1.3}}`)
	addEnabledValuePackagePlanForGroupRatioTest(t, db, "Month", " month-card ")
	addEnabledValuePackagePlanForGroupRatioTest(t, db, "Duplicate", "month-card")
	addEnabledValuePackagePlanForGroupRatioTest(t, db, "Day", "day-card")

	recorder, response := performGroupRatioOptionsRequest(t, http.MethodGet, nil, GetGroupRatioOptions)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	require.Equal(t, `{"a":1.5,"z":0}`, response.Data.GroupRatio)
	require.Equal(t, `{"month-card":{"gpt-pro":1.3}}`, response.Data.GroupGroupRatio)
	require.Equal(t, []string{"day-card", "month-card"}, response.Data.PackageGroups)
}

func TestUpdateGroupRatioOptionsPersistsAppliesAndReturnsNormalizedReadback(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)
	addEnabledValuePackagePlanForGroupRatioTest(t, db, "Month", " month-card ")

	recorder, response := performGroupRatioOptionsRequest(t, http.MethodPut, map[string]string{
		"group_ratio":       `{" z ":0,"a":1.5}`,
		"group_group_ratio": `{" empty ":{}," month-card ":{" gpt-pro ":1.3}}`,
	}, UpdateGroupRatioOptions)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	require.Equal(t, `{"a":1.5,"z":0}`, response.Data.GroupRatio)
	require.Equal(t, `{"month-card":{"gpt-pro":1.3}}`, response.Data.GroupGroupRatio)
	require.Equal(t, []string{"month-card"}, response.Data.PackageGroups)
	stored := readStoredGroupRatioOptions(t)
	require.Equal(t, response.Data.GroupRatio, stored.GroupRatio)
	require.Equal(t, response.Data.GroupGroupRatio, stored.GroupGroupRatio)
	require.Equal(t, response.Data.GroupRatio, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, response.Data.GroupGroupRatio, ratio_setting.GroupGroupRatio2JSONString())
	optionGroupRatio, optionGroupGroupRatio := readGroupRatioOptionMapForControllerTest()
	require.Equal(t, response.Data.GroupRatio, optionGroupRatio)
	require.Equal(t, response.Data.GroupGroupRatio, optionGroupGroupRatio)
}

func TestUpdateGroupRatioOptionsRejectsInvalidPayloadWithoutChangingState(t *testing.T) {
	tests := []struct {
		name            string
		groupRatio      string
		groupGroupRatio string
	}{
		{name: "invalid top ratio", groupRatio: `{"bad":-1}`, groupGroupRatio: `{"old":{"old":1}}`},
		{name: "invalid nested ratio", groupRatio: `{"new":2}`, groupGroupRatio: `{"bad":{"bad":0}}`},
		{name: "null top entry", groupRatio: `{"paid":null}`, groupGroupRatio: `{"new":{"new":2}}`},
		{name: "null top map", groupRatio: `null`, groupGroupRatio: `{"new":{"new":2}}`},
		{name: "null nested map", groupRatio: `{"new":2}`, groupGroupRatio: `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupGroupRatioOptionsControllerTest(t)
			seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

			recorder, response := performGroupRatioOptionsRequest(t, http.MethodPut, map[string]string{
				"group_ratio":       tt.groupRatio,
				"group_group_ratio": tt.groupGroupRatio,
			}, UpdateGroupRatioOptions)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.False(t, response.Success)
			stored := readStoredGroupRatioOptions(t)
			require.Equal(t, `{"old":1}`, stored.GroupRatio)
			require.Equal(t, `{"old":{"old":1}}`, stored.GroupGroupRatio)
			require.Equal(t, `{"old":1}`, ratio_setting.GroupRatio2JSONString())
			require.Equal(t, `{"old":{"old":1}}`, ratio_setting.GroupGroupRatio2JSONString())
			optionGroupRatio, optionGroupGroupRatio := readGroupRatioOptionMapForControllerTest()
			require.Equal(t, `{"old":1}`, optionGroupRatio)
			require.Equal(t, `{"old":{"old":1}}`, optionGroupGroupRatio)
		})
	}
}

func TestUpdateGroupRatioOptionsReloadsCommittedDBAfterRuntimeApplyFailure(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

	request := map[string]string{
		"group_ratio":       `{" new ":2}`,
		"group_group_ratio": `{" package ":{" child ":1.25}}`,
	}
	requestBody, err := common.Marshal(request)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/option/group-ratios", bytes.NewReader(requestBody))
	forcedErr := errors.New("forced second runtime apply failure")

	updateGroupRatioOptionsWithRuntime(c, func(groupRatio, _ string) error {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(groupRatio))
		return forcedErr
	})

	var response groupRatioOptionsTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.False(t, response.Success)
	require.Contains(t, response.Message, forcedErr.Error())
	stored := readStoredGroupRatioOptions(t)
	require.Equal(t, `{"new":2}`, stored.GroupRatio)
	require.Equal(t, `{"package":{"child":1.25}}`, stored.GroupGroupRatio)
	require.Equal(t, stored.GroupRatio, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, stored.GroupGroupRatio, ratio_setting.GroupGroupRatio2JSONString())
	optionGroupRatio, optionGroupGroupRatio := readGroupRatioOptionMapForControllerTest()
	require.Equal(t, stored.GroupRatio, optionGroupRatio)
	require.Equal(t, stored.GroupGroupRatio, optionGroupGroupRatio)
}

func TestUpdateGroupRatioOptionsReadsCommittedDBAfterRuntimeApply(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)
	var applierFinished atomic.Bool
	var postApplyOptionQueries atomic.Int32
	callbackName := "test:count_post_apply_group_ratio_readback"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if applierFinished.Load() && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Option" {
			postApplyOptionQueries.Add(1)
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(callbackName)) })

	requestBody, err := common.Marshal(map[string]string{
		"group_ratio":       `{"new":2}`,
		"group_group_ratio": `{"package":{"child":1.25}}`,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/option/group-ratios", bytes.NewReader(requestBody))

	updateGroupRatioOptionsWithRuntime(c, func(groupRatio, groupGroupRatio string) error {
		if err := applyGroupRatioRuntime(groupRatio, groupGroupRatio); err != nil {
			return err
		}
		applierFinished.Store(true)
		return nil
	})

	var response groupRatioOptionsTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.GreaterOrEqual(t, postApplyOptionQueries.Load(), int32(1))
}

func TestUpdateGroupRatioOptionsRejectsRuntimeReadbackMismatchAndRestoresDBSnapshot(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)
	requestBody, err := common.Marshal(map[string]string{
		"group_ratio":       `{"committed":2}`,
		"group_group_ratio": `{"package":{"child":1.25}}`,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/option/group-ratios", bytes.NewReader(requestBody))

	updateGroupRatioOptionsWithRuntime(c, func(_, groupGroupRatio string) error {
		if err := ratio_setting.UpdateGroupRatioByJSONString(`{"runtime-only":9}`); err != nil {
			return err
		}
		return ratio_setting.UpdateGroupGroupRatioByJSONString(groupGroupRatio)
	})

	var response groupRatioOptionsTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.False(t, response.Success)
	stored := readStoredGroupRatioOptions(t)
	require.Equal(t, `{"committed":2}`, stored.GroupRatio)
	require.Equal(t, `{"package":{"child":1.25}}`, stored.GroupGroupRatio)
	require.Equal(t, stored.GroupRatio, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, stored.GroupGroupRatio, ratio_setting.GroupGroupRatio2JSONString())
	optionGroupRatio, optionGroupGroupRatio := readGroupRatioOptionMapForControllerTest()
	require.Equal(t, stored.GroupRatio, optionGroupRatio)
	require.Equal(t, stored.GroupGroupRatio, optionGroupGroupRatio)
}

func TestUpdateGroupRatioOptionsBuildsPackageGroupsAfterRuntimeApply(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)
	requestBody, err := common.Marshal(map[string]string{
		"group_ratio":       `{"new":2}`,
		"group_group_ratio": `{"late-card":{"child":1.25}}`,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/option/group-ratios", bytes.NewReader(requestBody))

	updateGroupRatioOptionsWithRuntime(c, func(groupRatio, groupGroupRatio string) error {
		if err := applyGroupRatioRuntime(groupRatio, groupGroupRatio); err != nil {
			return err
		}
		addEnabledValuePackagePlanForGroupRatioTest(t, db, "Late", " late-card ")
		return nil
	})

	var response groupRatioOptionsTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, []string{"late-card"}, response.Data.PackageGroups)
}

func TestGenericUpdateOptionValidatesAndNormalizesGroupGroupRatio(t *testing.T) {
	t.Run("invalid and null preserve state", func(t *testing.T) {
		for _, value := range []string{`{"bad":{"bad":0}}`, `null`} {
			t.Run(value, func(t *testing.T) {
				db := setupGroupRatioOptionsControllerTest(t)
				seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

				recorder, response := performGroupRatioOptionsRequest(t, http.MethodPut, map[string]any{
					"key":   "GroupGroupRatio",
					"value": value,
				}, UpdateOption)

				require.Equal(t, http.StatusOK, recorder.Code)
				require.False(t, response.Success)
				stored := readStoredGroupRatioOptions(t)
				require.Equal(t, `{"old":{"old":1}}`, stored.GroupGroupRatio)
				require.Equal(t, `{"old":{"old":1}}`, ratio_setting.GroupGroupRatio2JSONString())
				_, optionGroupGroupRatio := readGroupRatioOptionMapForControllerTest()
				require.Equal(t, `{"old":{"old":1}}`, optionGroupGroupRatio)
			})
		}
	})

	t.Run("valid raw value is normalized before storage", func(t *testing.T) {
		db := setupGroupRatioOptionsControllerTest(t)
		seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

		recorder, response := performGroupRatioOptionsRequest(t, http.MethodPut, map[string]any{
			"key":   "GroupGroupRatio",
			"value": `{" empty ":{}," package ":{" child ":1.25}}`,
		}, UpdateOption)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.True(t, response.Success)
		stored := readStoredGroupRatioOptions(t)
		require.Equal(t, `{"package":{"child":1.25}}`, stored.GroupGroupRatio)
		require.Equal(t, stored.GroupGroupRatio, ratio_setting.GroupGroupRatio2JSONString())
		_, optionGroupGroupRatio := readGroupRatioOptionMapForControllerTest()
		require.Equal(t, stored.GroupGroupRatio, optionGroupGroupRatio)
	})
}

func TestGenericUpdateOptionNormalizesGroupRatioBeforeStorage(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

	recorder, response := performGroupRatioOptionsRequest(t, http.MethodPut, map[string]any{
		"key":   "GroupRatio",
		"value": `{" zero ":0," paid ":1.5}`,
	}, UpdateOption)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	stored := readStoredGroupRatioOptions(t)
	require.Equal(t, `{"paid":1.5,"zero":0}`, stored.GroupRatio)
	require.Equal(t, stored.GroupRatio, ratio_setting.GroupRatio2JSONString())
	optionGroupRatio, _ := readGroupRatioOptionMapForControllerTest()
	require.Equal(t, stored.GroupRatio, optionGroupRatio)
}

func TestGenericUpdateOptionRejectsNullGroupRatioEntryWithoutChangingState(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

	recorder, response := performGroupRatioOptionsRequest(t, http.MethodPut, map[string]any{
		"key":   "GroupRatio",
		"value": `{"paid":null}`,
	}, UpdateOption)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, response.Success)
	stored := readStoredGroupRatioOptions(t)
	require.Equal(t, `{"old":1}`, stored.GroupRatio)
	require.Equal(t, `{"old":1}`, ratio_setting.GroupRatio2JSONString())
	optionGroupRatio, _ := readGroupRatioOptionMapForControllerTest()
	require.Equal(t, `{"old":1}`, optionGroupRatio)
}

func TestGroupRatioOptionsSerializeGenericRatioUpdatesWithPairRuntimeApply(t *testing.T) {
	db := setupGroupRatioOptionsControllerTest(t)
	seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"old":1}}`)

	pairBody, err := common.Marshal(map[string]string{
		"group_ratio":       `{"pair":2}`,
		"group_group_ratio": `{"pair":{"child":1.25}}`,
	})
	require.NoError(t, err)
	pairRecorder := httptest.NewRecorder()
	pairContext, _ := gin.CreateTestContext(pairRecorder)
	pairContext.Request = httptest.NewRequest(http.MethodPut, "/api/option/group-ratios", bytes.NewReader(pairBody))
	applierStarted := make(chan struct{})
	releaseApplier := make(chan struct{})
	pairDone := make(chan struct{})
	go func() {
		defer close(pairDone)
		updateGroupRatioOptionsWithRuntime(pairContext, func(groupRatio, groupGroupRatio string) error {
			close(applierStarted)
			<-releaseApplier
			return applyGroupRatioRuntime(groupRatio, groupGroupRatio)
		})
	}()
	<-applierStarted

	genericBody, err := common.Marshal(map[string]any{
		"key":   "GroupRatio",
		"value": `{"generic":3}`,
	})
	require.NoError(t, err)
	genericRecorder := httptest.NewRecorder()
	genericContext, _ := gin.CreateTestContext(genericRecorder)
	genericContext.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(genericBody))
	genericDone := make(chan struct{})
	go func() {
		defer close(genericDone)
		UpdateOption(genericContext)
	}()

	select {
	case <-genericDone:
		t.Fatal("generic ratio update completed while pair runtime apply was still in progress")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseApplier)
	<-pairDone
	<-genericDone

	stored := readStoredGroupRatioOptions(t)
	require.Equal(t, `{"generic":3}`, stored.GroupRatio)
	require.Equal(t, `{"pair":{"child":1.25}}`, stored.GroupGroupRatio)
	require.Equal(t, stored.GroupRatio, ratio_setting.GroupRatio2JSONString())
	require.Equal(t, stored.GroupGroupRatio, ratio_setting.GroupGroupRatio2JSONString())
}

func TestGroupRatioOptionsHandlersUseSharedModelOperationLock(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   map[string]string
		handle gin.HandlerFunc
	}{
		{name: "get", method: http.MethodGet, handle: GetGroupRatioOptions},
		{name: "put", method: http.MethodPut, body: map[string]string{
			"group_ratio":       `{"new":2}`,
			"group_group_ratio": `{"new":{"child":1.25}}`,
		}, handle: UpdateGroupRatioOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupGroupRatioOptionsControllerTest(t)
			seedGroupRatioOptionsState(t, db, `{"old":1}`, `{"old":{"child":1}}`)

			lockEntered := make(chan struct{})
			releaseLock := make(chan struct{})
			lockDone := make(chan struct{})
			go func() {
				defer close(lockDone)
				_ = model.WithGroupRatioOptionsLock(func() error {
					close(lockEntered)
					<-releaseLock
					return nil
				})
			}()
			<-lockEntered

			requestBody, err := common.Marshal(tt.body)
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(tt.method, "/api/option/group-ratios", bytes.NewReader(requestBody))
			handlerDone := make(chan struct{})
			go func() {
				defer close(handlerDone)
				tt.handle(c)
			}()
			completedEarly := false
			select {
			case <-handlerDone:
				completedEarly = true
			case <-time.After(100 * time.Millisecond):
			}
			close(releaseLock)
			<-lockDone
			if !completedEarly {
				<-handlerDone
			}
			require.False(t, completedEarly, "handler completed while the shared model operation lock was held")
			require.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}
