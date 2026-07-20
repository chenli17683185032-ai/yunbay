package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type valuePackageConcurrencyCounter struct {
	mu    sync.Mutex
	count int
}

const valuePackageConcurrencySlotTTL = 2 * time.Minute
const valuePackageConcurrencySlotRefreshInterval = 30 * time.Second

const valuePackageConcurrencyRedisAcquireScript = `
local key = KEYS[1]
local token = ARGV[1]
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - ttl)
local count = redis.call('ZCARD', key)
if count >= limit then
  redis.call('EXPIRE', key, ttl)
  return 0
end
redis.call('ZADD', key, now, token)
redis.call('EXPIRE', key, ttl)
return 1
`

const valuePackageConcurrencyRedisReleaseScript = `
local key = KEYS[1]
local token = ARGV[1]
local ttl = tonumber(ARGV[2])
redis.call('ZREM', key, token)
local count = redis.call('ZCARD', key)
if count == 0 then
  redis.call('DEL', key)
else
  redis.call('EXPIRE', key, ttl)
end
return count
`

const valuePackageConcurrencyRedisRefreshScript = `
local key = KEYS[1]
local token = ARGV[1]
local now = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
if redis.call('ZSCORE', key, token) then
  redis.call('ZADD', key, now, token)
  redis.call('EXPIRE', key, ttl)
  return 1
end
local count = redis.call('ZCARD', key)
if count == 0 then
  redis.call('DEL', key)
end
return 0
`

// valuePackageConcurrencyCounters is the fallback limiter used when Redis is
// disabled. Production deployments with Redis enabled use a shared Redis slot
// set so the configured concurrency limit is enforced across app instances.
var valuePackageConcurrencyCounters sync.Map

func normalizeValuePackageConcurrencyLimit(limit int) int {
	if limit <= 0 {
		return 1
	}
	return limit
}

func acquireValuePackageSlot(userSubscriptionId int, limit int) (func(), bool, error) {
	if userSubscriptionId <= 0 {
		return nil, false, nil
	}
	if common.RedisEnabled && common.RDB != nil {
		return acquireValuePackageRedisSlot(userSubscriptionId, limit)
	}
	release, ok := acquireValuePackageMemorySlot(userSubscriptionId, limit)
	return release, ok, nil
}

func acquireValuePackageMemorySlot(userSubscriptionId int, limit int) (func(), bool) {
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

func acquireValuePackageRedisSlot(userSubscriptionId int, limit int) (func(), bool, error) {
	limit = normalizeValuePackageConcurrencyLimit(limit)
	token, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return nil, false, err
	}
	key := valuePackageConcurrencyRedisKey(userSubscriptionId)
	ttlSeconds := int64(valuePackageConcurrencySlotTTL / time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	acquired, err := common.RDB.Eval(ctx, valuePackageConcurrencyRedisAcquireScript, []string{key}, token, limit, time.Now().Unix(), ttlSeconds).Int()
	if err != nil {
		common.SysLog(fmt.Sprintf("value package concurrency redis acquire error: subscription=%d error=%s", userSubscriptionId, err.Error()))
		return nil, false, err
	}
	if acquired != 1 {
		return nil, false, nil
	}

	stopRefresh := make(chan struct{})
	go keepValuePackageRedisSlotFresh(key, token, ttlSeconds, stopRefresh)
	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			close(stopRefresh)
			if err := releaseValuePackageRedisSlot(key, token, ttlSeconds); err != nil {
				common.SysLog(fmt.Sprintf("value package concurrency redis release error: key=%s error=%s", key, err.Error()))
			}
		})
	}, true, nil
}

func keepValuePackageRedisSlotFresh(key string, token string, ttlSeconds int64, stop <-chan struct{}) {
	ticker := time.NewTicker(valuePackageConcurrencySlotRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			refreshed, err := refreshValuePackageRedisSlot(key, token, ttlSeconds)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to refresh value package concurrency slot: %v", err))
				continue
			}
			if !refreshed {
				common.SysLog(fmt.Sprintf("value package concurrency redis refresh stopped: key=%s token=%s", key, common.LocalLogPreview(token)))
				return
			}
		case <-stop:
			return
		}
	}
}

func refreshValuePackageRedisSlot(key string, token string, ttlSeconds int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	refreshed, err := common.RDB.Eval(ctx, valuePackageConcurrencyRedisRefreshScript, []string{key}, token, time.Now().Unix(), ttlSeconds).Int()
	if err != nil {
		return false, err
	}
	return refreshed == 1, nil
}

func releaseValuePackageRedisSlot(key string, token string, ttlSeconds int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return common.RDB.Eval(ctx, valuePackageConcurrencyRedisReleaseScript, []string{key}, token, ttlSeconds).Err()
}

func valuePackageConcurrencyRedisKey(userSubscriptionId int) string {
	return fmt.Sprintf("value_package_concurrency:%d", userSubscriptionId)
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
		applyValuePackageWalletFallbackPreference(c, state)
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

		walletFallbackEnabled := applyValuePackageWalletFallbackPreference(c, state)
		if isValuePackageReadOnlyRequest(c) {
			applyValuePackageGroupScope(c, state)
			c.Next()
			return
		}
		if state.Subscription.AmountTotal > 0 && state.Subscription.AmountUsed >= state.Subscription.AmountTotal {
			if walletFallbackEnabled {
				markValuePackageWalletFallback(c)
				c.Next()
				return
			}
			abortWithOpenAiMessage(c, http.StatusForbidden, model.ValuePackageQuotaExhaustedUserMessage)
			return
		}

		now := model.GetDBTimestamp()
		usageDetails, err := model.GetValuePackageWindowUsageDetails(userId, state.Subscription.Id, now)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "查询超值套餐用量失败")
			return
		}
		if usageDetails == nil {
			usageDetails = &model.ValuePackageWindowUsageDetails{}
		}
		if state.Plan.Limit5hAmount > 0 && usageDetails.Used5h >= state.Plan.Limit5hAmount {
			if walletFallbackEnabled {
				markValuePackageWalletFallback(c)
				c.Next()
				return
			}
			abortWithOpenAiMessage(c, http.StatusForbidden, formatValuePackageLimitMessage("5 小时", usageDetails.Used5h, state.Plan.Limit5hAmount, usageDetails.ResetSeconds5h))
			return
		}
		if state.Plan.Limit7dAmount > 0 && usageDetails.Used7d >= state.Plan.Limit7dAmount {
			if walletFallbackEnabled {
				markValuePackageWalletFallback(c)
				c.Next()
				return
			}
			abortWithOpenAiMessage(c, http.StatusForbidden, formatValuePackageLimitMessage("7 天", usageDetails.Used7d, state.Plan.Limit7dAmount, usageDetails.ResetSeconds7d))
			return
		}

		applyValuePackageGroupScope(c, state)
		release, ok, err := acquireValuePackageSlot(state.Subscription.Id, state.Plan.ConcurrencyLimit)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "申请超值套餐并发额度失败")
			return
		}
		if !ok {
			common.SysLog(fmt.Sprintf("value package concurrency denied: subscription=%d limit=%d", state.Subscription.Id, normalizeValuePackageConcurrencyLimit(state.Plan.ConcurrencyLimit)))
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "超值套餐并发请求数已达上限")
			return
		}
		defer release()

		c.Next()
	}
}

func applyValuePackageWalletFallbackPreference(c *gin.Context, state *model.ValuePackageState) bool {
	enabled := state != nil && state.Preference.AllowsWalletFallback()
	common.SetContextKey(c, constant.ContextKeyValuePackageWalletFallback, enabled)
	return enabled
}

func markValuePackageWalletFallback(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyValuePackageUseWallet, true)
}

func formatValuePackageLimitMessage(windowLabel string, used int64, limit int64, resetSeconds int64) string {
	if resetSeconds > 0 {
		return fmt.Sprintf("%s（%s：已用 %d / 限额 %d，将在 %s后完全恢复）", model.ValuePackageQuotaExhaustedUserMessage, windowLabel, used, limit, formatValuePackageResetDuration(resetSeconds))
	}
	return fmt.Sprintf("%s（%s：已用 %d / 限额 %d）", model.ValuePackageQuotaExhaustedUserMessage, windowLabel, used, limit)
}

func formatValuePackageResetDuration(seconds int64) string {
	if seconds <= 0 {
		return "不到 1 分钟"
	}
	if seconds < 60 {
		return "不到 1 分钟"
	}
	minutes := (seconds + 59) / 60
	if minutes < 60 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours < 24 {
		if remainingMinutes > 0 {
			return fmt.Sprintf("%d 小时 %d 分钟", hours, remainingMinutes)
		}
		return fmt.Sprintf("%d 小时", hours)
	}
	days := hours / 24
	remainingHours := hours % 24
	if remainingHours > 0 {
		return fmt.Sprintf("%d 天 %d 小时", days, remainingHours)
	}
	return fmt.Sprintf("%d 天", days)
}

func applyValuePackageGroupScope(c *gin.Context, state *model.ValuePackageState) {
	modelGroup := strings.TrimSpace(state.Plan.ModelGroup)
	if modelGroup == "" {
		return
	}
	common.SetContextKey(c, constant.ContextKeyValuePackageSubscriptionId, state.Subscription.Id)
	common.SetContextKey(c, constant.ContextKeyValuePackagePlanId, state.Plan.Id)
	common.SetContextKey(c, constant.ContextKeyValuePackageModelGroup, modelGroup)
	common.SetContextKey(c, constant.ContextKeyValuePackagePackageType, state.Plan.PackageType)
}

func isValuePackageReadOnlyRequest(c *gin.Context) bool {
	if c.Request == nil || c.Request.URL == nil {
		return false
	}
	path := c.Request.URL.Path
	if c.Request.Method == http.MethodGet {
		return isValuePackageReadOnlyGetPath(path)
	}
	if c.Request.Method != http.MethodPost {
		return false
	}
	return path == "/suno/fetch" ||
		strings.HasSuffix(path, "/suno/fetch") ||
		strings.Contains(path, "/mj/task/list-by-condition")
}

func isValuePackageReadOnlyGetPath(path string) bool {
	if path == "/v1/models" ||
		strings.HasPrefix(path, "/v1/models/") ||
		path == "/v1beta/models" ||
		strings.HasPrefix(path, "/v1beta/models/") ||
		path == "/v1beta/openai/models" ||
		strings.HasPrefix(path, "/v1beta/openai/models/") {
		return true
	}
	return strings.HasPrefix(path, "/suno/fetch/") ||
		(strings.Contains(path, "/mj/task/") && (strings.HasSuffix(path, "/fetch") || strings.HasSuffix(path, "/image-seed"))) ||
		strings.HasPrefix(path, "/v1/video/generations/") ||
		strings.HasPrefix(path, "/v1/videos/") ||
		strings.HasPrefix(path, "/kling/v1/videos/text2video/") ||
		strings.HasPrefix(path, "/kling/v1/videos/image2video/")
}
