package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type valuePackageConcurrencyCounter struct {
	mu    sync.Mutex
	count int
}

var valuePackageConcurrencyCounters sync.Map

func normalizeValuePackageConcurrencyLimit(limit int) int {
	if limit <= 0 {
		return 1
	}
	if limit > 2 {
		return 2
	}
	return limit
}

func acquireValuePackageSlot(userSubscriptionId int, limit int) (func(), bool) {
	if userSubscriptionId <= 0 {
		return nil, false
	}
	limit = normalizeValuePackageConcurrencyLimit(limit)
	var counter *valuePackageConcurrencyCounter
	for {
		loaded, _ := valuePackageConcurrencyCounters.LoadOrStore(userSubscriptionId, &valuePackageConcurrencyCounter{})
		counter = loaded.(*valuePackageConcurrencyCounter)
		counter.mu.Lock()
		current, ok := valuePackageConcurrencyCounters.Load(userSubscriptionId)
		if ok && current == counter {
			break
		}
		counter.mu.Unlock()
	}
	defer counter.mu.Unlock()
	if counter.count >= limit {
		return nil, false
	}
	counter.count++
	released := false
	return func() {
		counter.mu.Lock()
		defer counter.mu.Unlock()
		if released {
			return
		}
		released = true
		if counter.count > 0 {
			counter.count--
		}
		if counter.count == 0 {
			valuePackageConcurrencyCounters.CompareAndDelete(userSubscriptionId, counter)
		}
	}, true
}

func ValuePackageGroupScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := common.GetContextKeyInt(c, constant.ContextKeyUserId)
		if userId <= 0 {
			c.Next()
			return
		}
		state, err := model.GetActiveValuePackageForRelay(userId)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "查询超值套餐权益失败")
			return
		}
		if state == nil || state.Subscription == nil || state.Plan == nil {
			c.Next()
			return
		}
		applyValuePackageGroupScope(c, state)
		c.Next()
	}
}

func ValuePackageEntitlement() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := common.GetContextKeyInt(c, constant.ContextKeyUserId)
		if userId <= 0 {
			c.Next()
			return
		}

		state, err := model.GetActiveValuePackageForRelay(userId)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "查询超值套餐权益失败")
			return
		}
		if state == nil || state.Subscription == nil || state.Plan == nil {
			c.Next()
			return
		}

		applyValuePackageGroupScope(c, state)
		if isValuePackageReadOnlyRequest(c) {
			c.Next()
			return
		}

		now := model.GetDBTimestamp()
		used5h, used7d, err := model.GetValuePackageWindowUsage(userId, state.Subscription.Id, now)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "查询超值套餐用量失败")
			return
		}
		if state.Plan.Limit5hAmount > 0 && used5h >= state.Plan.Limit5hAmount {
			abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("超值套餐 5 小时滚动额度已用尽，已用 %d / 限额 %d", used5h, state.Plan.Limit5hAmount))
			return
		}
		if state.Plan.Limit7dAmount > 0 && used7d >= state.Plan.Limit7dAmount {
			abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("超值套餐 7 天滚动额度已用尽，已用 %d / 限额 %d", used7d, state.Plan.Limit7dAmount))
			return
		}

		release, ok := acquireValuePackageSlot(state.Subscription.Id, state.Plan.ConcurrencyLimit)
		if !ok {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "超值套餐并发请求数已达上限")
			return
		}
		defer release()

		c.Next()
	}
}

func applyValuePackageGroupScope(c *gin.Context, state *model.ValuePackageState) {
	common.SetContextKey(c, constant.ContextKeyUsingGroup, state.Plan.ModelGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, state.Plan.ModelGroup)
	common.SetContextKey(c, constant.ContextKeyValuePackageSubscriptionId, state.Subscription.Id)
	common.SetContextKey(c, constant.ContextKeyValuePackagePlanId, state.Plan.Id)
	common.SetContextKey(c, constant.ContextKeyValuePackageModelGroup, state.Plan.ModelGroup)
	common.SetContextKey(c, constant.ContextKeyValuePackagePackageType, state.Plan.PackageType)
}

func isValuePackageReadOnlyRequest(c *gin.Context) bool {
	if c.Request == nil || c.Request.URL == nil {
		return false
	}
	if c.Request.Method == http.MethodGet {
		return true
	}
	if c.Request.Method != http.MethodPost {
		return false
	}
	path := c.Request.URL.Path
	return path == "/suno/fetch" ||
		strings.HasSuffix(path, "/suno/fetch") ||
		strings.Contains(path, "/mj/task/list-by-condition")
}
