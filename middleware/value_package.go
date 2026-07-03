package middleware

import (
	"fmt"
	"net/http"
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
	loaded, _ := valuePackageConcurrencyCounters.LoadOrStore(userSubscriptionId, &valuePackageConcurrencyCounter{})
	counter := loaded.(*valuePackageConcurrencyCounter)
	counter.mu.Lock()
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
	}, true
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

		common.SetContextKey(c, constant.ContextKeyUsingGroup, state.Plan.ModelGroup)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, state.Plan.ModelGroup)
		common.SetContextKey(c, constant.ContextKeyValuePackageSubscriptionId, state.Subscription.Id)
		common.SetContextKey(c, constant.ContextKeyValuePackagePlanId, state.Plan.Id)
		common.SetContextKey(c, constant.ContextKeyValuePackageModelGroup, state.Plan.ModelGroup)
		common.SetContextKey(c, constant.ContextKeyValuePackagePackageType, state.Plan.PackageType)

		c.Next()
	}
}
