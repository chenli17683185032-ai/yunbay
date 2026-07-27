package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

const (
	SubscriptionPlanKindSubscription = "subscription"
	SubscriptionPlanKindValuePackage = "value_package"
)

const (
	ValuePackageTypeDay   = "day"
	ValuePackageTypeWeek  = "week"
	ValuePackageTypeMonth = "month"
)

const (
	ValuePackageLevelDay   = 1
	ValuePackageLevelWeek  = 2
	ValuePackageLevelMonth = 3
)

const (
	valuePackage5hWindowSeconds = int64(5 * 3600)
	valuePackage7dWindowSeconds = int64(7 * 24 * 3600)
	valuePackageDaySeconds      = int64(24 * 3600)
	valuePackageWeekSeconds     = int64(7 * 24 * 3600)
	valuePackageMonthSeconds    = int64(30 * 24 * 3600)
)

const (
	ValuePackageExhaustedReasonTotal = "total_quota_exhausted"
	ValuePackageExhaustedReason5h    = "limit_5h_exhausted"
	ValuePackageExhaustedReason7d    = "limit_7d_exhausted"
)

const valuePackageRenewalExhaustedReasonTotalDust = "total_quota_dust"

const (
	ValuePackageQuotaResetSourceUserConsumeCount = "user_consume_count"
	ValuePackageQuotaResetSourceAdminManualReset = "admin_manual_reset"
	ValuePackageQuotaResetSourceCycleRenewal     = "cycle_renewal"
	ValuePackageQuotaResetSourcePackageRenewal   = "package_renewal"

	ValuePackageResetCountLedgerSourceAdminSet      = "admin_set"
	ValuePackageResetCountLedgerSourceAdminAdd      = "admin_add"
	ValuePackageResetCountLedgerSourceAdminSubtract = "admin_subtract"
	ValuePackageResetCountLedgerSourceUserConsume   = "user_consume"
	// 重置卡兑换码兑换发放
	ValuePackageResetCountLedgerSourceRedemption = "redemption"
	// 开通/续费套餐按套餐配置赠送
	ValuePackageResetCountLedgerSourcePlanGift = "plan_gift"
)

// 每个套餐开通赠送重置卡张数上限
const MaxSubscriptionPlanGiftResetCount = 100

func ClampSubscriptionPlanGiftResetCount(count int) int {
	if count <= 0 {
		return 0
	}
	if count > MaxSubscriptionPlanGiftResetCount {
		return MaxSubscriptionPlanGiftResetCount
	}
	return count
}

const ValuePackageQuotaExhaustedUserMessage = "当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用"

const ValuePackageEffectiveBillingRatio = 1.0

const (
	UserSubscriptionStatusActive    = "active"
	UserSubscriptionStatusExpired   = "expired"
	UserSubscriptionStatusCancelled = "cancelled"
	UserSubscriptionStatusCovered   = "covered"
)

var (
	ErrSubscriptionOrderNotFound      = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid = errors.New("subscription order status invalid")
	ErrCompletedSubscriptionNotFound  = errors.New("completed subscription not found")
)

const (
	subscriptionPlanCacheNamespace     = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace = "new-api:subscription_plan_info:v1"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func withUpdateLock(tx *gorm.DB) *gorm.DB {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	PlanKind     string `json:"plan_kind" gorm:"type:varchar(32);not null;default:'subscription'"`
	PackageType  string `json:"package_type" gorm:"type:varchar(16);default:''"`
	PackageLevel int    `json:"package_level" gorm:"type:int;default:0"`

	ModelGroup       string `json:"model_group" gorm:"type:varchar(64);default:''"`
	ConcurrencyLimit int    `json:"concurrency_limit" gorm:"type:int;default:1"`
	Limit5hAmount    int64  `json:"limit_5h_amount" gorm:"column:limit_5h_amount;type:bigint;not null;default:0"`
	Limit7dAmount    int64  `json:"limit_7d_amount" gorm:"column:limit_7d_amount;type:bigint;not null;default:0"`
	Benefits         string `json:"benefits" gorm:"type:text"`

	LdxpProductUrl        string  `json:"ldxp_product_url" gorm:"type:text"`
	LdxpProductName       string  `json:"ldxp_product_name" gorm:"type:text"`
	LdxpProductAmount     float64 `json:"ldxp_product_amount" gorm:"type:decimal(10,6);not null;default:0"`
	LdxpProductRef        string  `json:"ldxp_product_ref" gorm:"type:varchar(128);default:''"`
	LdxpSessionTTLSeconds int64   `json:"ldxp_session_ttl_seconds" gorm:"type:bigint;not null;default:0"`

	AllowBalancePay *bool `json:"allow_balance_pay" gorm:"default:true"`

	StripePriceId         string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId        string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`
	WaffoPancakeProductId string `json:"waffo_pancake_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	// 开通/续费超值套餐时赠送的重置卡张数（0 = 不赠送，仅超值套餐生效）
	GiftResetCount int `json:"gift_reset_count" gorm:"type:int;not null;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	if err := ensureSubscriptionPlanValuePackageColumnsTx(tx); err != nil {
		return err
	}
	InvalidateSubscriptionPlanCache(p.Id)
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) AfterCreate(tx *gorm.DB) error {
	InvalidateSubscriptionPlanCache(p.Id)
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	if err := ensureSubscriptionPlanValuePackageColumnsTx(tx); err != nil {
		return err
	}
	InvalidateSubscriptionPlanCache(p.Id)
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

func ensureSubscriptionPlanValuePackageColumnsTx(tx *gorm.DB) error {
	if tx == nil || !common.UsingSQLite {
		return nil
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := tx.Raw("PRAGMA table_info(`subscription_plans`)").Scan(&cols).Error; err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	for _, col := range []sqliteColumnDef{
		{Name: "limit_5h_amount", DDL: "`limit_5h_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "limit_7d_amount", DDL: "`limit_7d_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "gift_reset_count", DDL: "`gift_reset_count` int NOT NULL DEFAULT 0"},
	} {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := tx.Exec("ALTER TABLE `subscription_plans` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

func (p *SubscriptionPlan) NormalizeDefaults() {
	if p.AllowBalancePay == nil {
		p.AllowBalancePay = common.GetPointer(true)
	}
	p.PlanKind = strings.TrimSpace(p.PlanKind)
	if p.PlanKind == "" {
		p.PlanKind = SubscriptionPlanKindSubscription
	}
	if p.PlanKind == SubscriptionPlanKindValuePackage {
		p.Currency = "CNY"
		normalizeValuePackageFixedDurationFields(p)
	} else {
		p.Currency = "USD"
	}
	if p.ConcurrencyLimit <= 0 {
		p.ConcurrencyLimit = 1
	}
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id     int     `json:"id"`
	UserId int     `json:"user_id" gorm:"index"`
	PlanId int     `json:"plan_id" gorm:"index"`
	Money  float64 `json:"money"`

	TradeNo            string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod      string `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider    string `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index;default:0"`
	GiftResetCount     int    `json:"gift_reset_count" gorm:"type:int;not null;default:0"`
	Status             string `json:"status"`
	CreateTime         int64  `json:"create_time"`
	CompleteTime       int64  `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	return DB.Create(o).Error
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`
	QuotaEpoch  int64 `json:"quota_epoch" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled/covered

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	CoveredBySubscriptionId int   `json:"covered_by_subscription_id" gorm:"type:int;default:0"`
	CoveredTime             int64 `json:"covered_time" gorm:"type:bigint;default:0"`

	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

type UserValuePackagePreference struct {
	Id                       int   `json:"id"`
	UserId                   int   `json:"user_id" gorm:"uniqueIndex"`
	Enabled                  bool  `json:"enabled" gorm:"default:false"`
	WalletFallbackEnabled    *bool `json:"wallet_fallback_enabled" gorm:"column:wallet_fallback_enabled"`
	ActiveUserSubscriptionId int   `json:"active_user_subscription_id" gorm:"index;default:0"`
	ResetCount               int   `json:"reset_count" gorm:"default:0"`
	CreatedAt                int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64 `json:"updated_at" gorm:"bigint"`
}

func (p UserValuePackagePreference) AllowsWalletFallback() bool {
	return p.WalletFallbackEnabled == nil || *p.WalletFallbackEnabled
}

func (p *UserValuePackagePreference) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *UserValuePackagePreference) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

type ValuePackageUsageRecord struct {
	Id                 int    `json:"id"`
	UserId             int    `json:"user_id" gorm:"index:idx_vp_usage_user_time,priority:1"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index;uniqueIndex:idx_vp_usage_sub_request,priority:1"`
	PlanId             int    `json:"plan_id" gorm:"index"`
	PackageType        string `json:"package_type" gorm:"type:varchar(16);index"`
	ModelGroup         string `json:"model_group" gorm:"type:varchar(64);index"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);index;uniqueIndex:idx_vp_usage_sub_request,priority:2"`
	Quota              int64  `json:"quota" gorm:"type:bigint;not null;default:0"`
	QuotaEpoch         int64  `json:"quota_epoch" gorm:"type:bigint;not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index:idx_vp_usage_user_time,priority:2"`
}

func (r *ValuePackageUsageRecord) BeforeCreate(tx *gorm.DB) error {
	if r.CreatedAt == 0 {
		r.CreatedAt = common.GetTimestamp()
	}
	return nil
}

type ValuePackageQuotaReset struct {
	Id                 int    `json:"id"`
	UserId             int    `json:"user_id" gorm:"index:idx_vp_reset_user_time,priority:1"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PlanId             int    `json:"plan_id" gorm:"index"`
	PackageType        string `json:"package_type" gorm:"type:varchar(16);index"`
	ResetAt            int64  `json:"reset_at" gorm:"bigint;index:idx_vp_reset_user_time,priority:2"`
	FromEpoch          int64  `json:"from_epoch" gorm:"type:bigint;not null;default:0"`
	ToEpoch            int64  `json:"to_epoch" gorm:"type:bigint;not null;default:0"`
	AmountUsedBefore   int64  `json:"amount_used_before" gorm:"type:bigint;not null;default:0"`
	Source             string `json:"source" gorm:"type:varchar(32);index"`
	CreatedByUserId    int    `json:"created_by_user_id" gorm:"index"`
	Note               string `json:"note" gorm:"type:text"`
}

func (r *ValuePackageQuotaReset) BeforeCreate(tx *gorm.DB) error {
	if r.ResetAt == 0 {
		r.ResetAt = common.GetTimestamp()
	}
	return nil
}

type ValuePackageResetCountLedger struct {
	Id              int    `json:"id"`
	UserId          int    `json:"user_id" gorm:"index:idx_vp_reset_count_ledger_user_time,priority:1"`
	Delta           int    `json:"delta"`
	BeforeCount     int    `json:"before_count"`
	AfterCount      int    `json:"after_count"`
	Source          string `json:"source" gorm:"type:varchar(32);index"`
	CreatedByUserId int    `json:"created_by_user_id" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index:idx_vp_reset_count_ledger_user_time,priority:2"`
	Note            string `json:"note" gorm:"type:text"`
}

func (l *ValuePackageResetCountLedger) BeforeCreate(tx *gorm.DB) error {
	if l.CreatedAt == 0 {
		l.CreatedAt = common.GetTimestamp()
	}
	return nil
}

type SubscriptionSummary struct {
	Subscription *UserSubscription `json:"subscription"`
}

type SubscriptionPlanStats struct {
	ActiveUserCount         int64 `json:"active_user_count"`
	ActiveSubscriptionCount int64 `json:"active_subscription_count"`
	RemainingAmount         int64 `json:"remaining_amount"`
	UnlimitedCount          int64 `json:"unlimited_count"`
}

func GetSubscriptionPlanStatsMap(now int64) (map[int]SubscriptionPlanStats, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > ?", "active", now).Find(&subs).Error; err != nil {
		return nil, err
	}
	stats := make(map[int]SubscriptionPlanStats)
	usersByPlan := make(map[int]map[int]struct{})
	for _, sub := range subs {
		planId := sub.PlanId
		stat := stats[planId]
		stat.ActiveSubscriptionCount++
		if _, ok := usersByPlan[planId]; !ok {
			usersByPlan[planId] = make(map[int]struct{})
		}
		usersByPlan[planId][sub.UserId] = struct{}{}
		if sub.AmountTotal == 0 {
			stat.UnlimitedCount++
		} else {
			remaining := sub.AmountTotal - sub.AmountUsed
			if remaining > 0 {
				stat.RemainingAmount += remaining
			}
		}
		stats[planId] = stat
	}
	for planId, users := range usersByPlan {
		stat := stats[planId]
		stat.ActiveUserCount = int64(len(users))
		stats[planId] = stat
	}
	return stats, nil
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// Align to next Monday 00:00
		weekday := int(base.Weekday()) // Sunday=0
		// Convert to Monday=1..Sunday=7
		if weekday == 0 {
			weekday = 7
		}
		daysUntil := 8 - weekday
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, daysUntil)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

func getSubscriptionPlanByIdFreshTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	query := DB
	if tx != nil {
		query = tx
	}
	var plan SubscriptionPlan
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	plan.NormalizeDefaults()
	return &plan, nil
}

func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if tx != nil {
		return getSubscriptionPlanByIdFreshTx(tx, id)
	}
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	if tx == nil && key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			cached.NormalizeDefaults()
			return &cached, nil
		}
	}
	plan, err := getSubscriptionPlanByIdFreshTx(nil, id)
	if err != nil {
		return nil, err
	}
	plan.NormalizeDefaults()
	if tx == nil {
		_ = getSubscriptionPlanCache().SetWithTTL(key, *plan, subscriptionPlanCacheTTL())
	}
	return plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var group string
	if err := tx.Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}
	var activeSub UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}

func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	normalizeValuePackagePlan(plan)
	if plan.IsValuePackage() {
		return nil, errors.New("超值套餐不能通过普通订阅创建，请使用超值套餐专用流程")
	}
	if plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := getDBTimestampTx(tx)
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" {
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}
	sub := &UserSubscription{
		UserId:        userId,
		PlanId:        plan.Id,
		AmountTotal:   plan.TotalAmount,
		AmountUsed:    0,
		StartTime:     now.Unix(),
		EndTime:       endUnix,
		Status:        "active",
		Source:        source,
		LastResetTime: lastReset,
		NextResetTime: nextReset,
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: prevGroup,
		CreatedAt:     common.GetTimestamp(),
		UpdatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func CreateValuePackageSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	normalizeValuePackagePlan(plan)
	if !plan.IsValuePackage() {
		return nil, errors.New("目标套餐不是超值套餐")
	}
	if !plan.Enabled {
		return nil, errors.New("套餐未启用")
	}
	if err := ensureExistingUserForUpdateTx(tx, userId); err != nil {
		return nil, err
	}
	intent, err := checkValuePackagePurchaseIntentTx(tx, userId, plan, true)
	if err != nil {
		return nil, err
	}
	nowUnix := getDBTimestampTx(tx)
	start := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(start, plan)
	if err != nil {
		return nil, err
	}
	var completed *UserSubscription
	switch intent.Action {
	case ValuePackagePurchaseActionExtend:
		existing, err := extendValuePackageSubscriptionTx(tx, intent.CurrentSubscription.Id, plan, nowUnix, endUnix)
		if err != nil {
			return nil, err
		}
		completed = existing
	case ValuePackagePurchaseActionUpgrade:
		if intent.CurrentSubscription != nil {
			if err := tx.Model(&UserSubscription{}).Where("id = ?", intent.CurrentSubscription.Id).Updates(map[string]interface{}{
				"status":       UserSubscriptionStatusCovered,
				"covered_time": nowUnix,
				"updated_at":   common.GetTimestamp(),
			}).Error; err != nil {
				return nil, err
			}
		}
		fallthrough
	case ValuePackagePurchaseActionCreate:
		sub := &UserSubscription{
			UserId:      userId,
			PlanId:      plan.Id,
			AmountTotal: plan.TotalAmount,
			AmountUsed:  0,
			StartTime:   nowUnix,
			EndTime:     endUnix,
			Status:      UserSubscriptionStatusActive,
			Source:      source,
			CreatedAt:   common.GetTimestamp(),
			UpdatedAt:   common.GetTimestamp(),
		}
		syncValuePackageCycleSchedule(sub, plan)
		if err := tx.Create(sub).Error; err != nil {
			return nil, err
		}
		completed = sub
		if intent.Action == ValuePackagePurchaseActionUpgrade && intent.CurrentSubscription != nil {
			if err := tx.Model(&UserSubscription{}).Where("id = ?", intent.CurrentSubscription.Id).Update("covered_by_subscription_id", sub.Id).Error; err != nil {
				return nil, err
			}
		}
	default:
		return nil, errors.New("unknown value package purchase action")
	}
	if completed == nil || completed.Id <= 0 {
		return nil, errors.New("completed subscription missing")
	}
	if err := ensureValuePackagePreferenceAfterPurchaseTx(
		tx,
		userId,
		completed.Id,
		plan,
		ClampSubscriptionPlanGiftResetCount(plan.GiftResetCount),
		fmt.Sprintf("开通套餐赠送：%s，来源 %s", plan.Title, source),
	); err != nil {
		return nil, err
	}
	return completed, nil
}

func extendValuePackageSubscriptionTx(tx *gorm.DB, subscriptionID int, plan *SubscriptionPlan, nowUnix int64, purchasedEndUnix int64) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if subscriptionID <= 0 {
		return nil, errors.New("invalid subscription id")
	}
	if plan == nil || !plan.IsValuePackage() || plan.TotalAmount <= 0 {
		return nil, errors.New("invalid value package plan total")
	}
	duration := purchasedEndUnix - nowUnix
	if nowUnix <= 0 || duration <= 0 {
		return nil, errors.New("invalid value package purchase duration")
	}

	var existing UserSubscription
	if err := withUpdateLock(tx).Where("id = ?", subscriptionID).First(&existing).Error; err != nil {
		return nil, err
	}
	if existing.Status != UserSubscriptionStatusActive {
		return nil, errors.New("value package subscription is not active")
	}
	if err := maybeAdvanceValuePackageCycleTx(tx, &existing, plan, nowUnix); err != nil {
		return nil, err
	}
	usage, err := buildValuePackageUsageSummaryTx(tx, existing.UserId, &existing, plan, nowUnix)
	if err != nil {
		return nil, err
	}
	renewalExhaustedReason := valuePackageRenewalExhaustedReason(usage)
	if renewalExhaustedReason != "" {
		if existing.QuotaEpoch == math.MaxInt64 {
			return nil, errors.New("value package quota epoch overflow")
		}
		fromEpoch := existing.QuotaEpoch
		amountUsedBefore := existing.AmountUsed
		existing.AmountTotal = plan.TotalAmount
		existing.AmountUsed = 0
		existing.StartTime = nowUnix
		existing.EndTime = purchasedEndUnix
		existing.QuotaEpoch++
		existing.LastResetTime = nowUnix
		existing.NextResetTime = 0
		existing.UpdatedAt = common.GetTimestamp()
		if err := tx.Create(&ValuePackageQuotaReset{
			UserId:             existing.UserId,
			UserSubscriptionId: existing.Id,
			PlanId:             existing.PlanId,
			PackageType:        plan.PackageType,
			ResetAt:            nowUnix,
			FromEpoch:          fromEpoch,
			ToEpoch:            existing.QuotaEpoch,
			AmountUsedBefore:   amountUsedBefore,
			Source:             ValuePackageQuotaResetSourcePackageRenewal,
			Note:               "exhausted package renewal: " + renewalExhaustedReason,
		}).Error; err != nil {
			return nil, err
		}
		if err := tx.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	base := existing.EndTime
	if base < nowUnix {
		base = nowUnix
	}
	if base > math.MaxInt64-duration {
		return nil, errors.New("value package subscription end time overflow")
	}
	existing.EndTime = base + duration
	syncValuePackageCycleSchedule(&existing, plan)
	existing.UpdatedAt = common.GetTimestamp()
	if err := tx.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func valuePackageRenewalExhaustedReason(usage *ValuePackageUsageSummary) string {
	if usage == nil {
		return ""
	}
	if usage.Exhausted {
		return usage.ExhaustedReason
	}
	if usage.TotalLimit <= 0 || usage.TotalRemaining <= 0 {
		return ""
	}

	// A rejected final reserve can leave a positive tail. Only treat it as
	// exhausted when it is both at most one basis point and at most one cent.
	relativeDustLimit := usage.TotalLimit / 10_000
	absoluteDustLimit := int64(common.QuotaPerUnit / 100)
	if relativeDustLimit <= 0 || absoluteDustLimit <= 0 {
		return ""
	}
	if relativeDustLimit > absoluteDustLimit {
		relativeDustLimit = absoluteDustLimit
	}
	if usage.TotalRemaining <= relativeDustLimit {
		return valuePackageRenewalExhaustedReasonTotalDust
	}
	return ""
}

// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
// expectedPaymentProvider guards against cross-gateway callback attacks (empty skips the check).
// actualPaymentMethod updates the order's PaymentMethod to reflect the real payment type used (empty skips update).
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	var vipUpgraded bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if plan.IsValuePackage() {
			return errors.New("超值套餐仅支持联动小铺购买")
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		createdSub, err := CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order")
		if err != nil {
			return err
		}
		order.UserSubscriptionId = createdSub.Id
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if _, err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		vipUpgraded, err = MaybeUpgradeUserToVIPTx(tx, order.UserId)
		if err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if vipUpgraded && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, UserGroupVIP)
	} else if upgradeGroup != "" && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, upgradeGroup)
	}
	if logUserId > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) (*TopUp, error) {
	if tx == nil || order == nil {
		return nil, errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	var topup TopUp
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId:          order.UserId,
				Amount:          0,
				Money:           order.Money,
				TradeNo:         order.TradeNo,
				PaymentMethod:   order.PaymentMethod,
				PaymentProvider: order.PaymentProvider,
				CreateTime:      order.CreateTime,
				CompleteTime:    now,
				Status:          common.TopUpStatusSuccess,
			}
			if err := tx.Create(&topup).Error; err != nil {
				return nil, err
			}
			return &topup, nil
		}
		return nil, err
	}
	if topup.UserId != order.UserId {
		return nil, errors.New("topup user mismatch")
	}
	if topup.Amount > 0 {
		return nil, errors.New("existing topup is a balance recharge")
	}
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return nil, ErrPaymentMethodMismatch
	}
	if topup.PaymentProvider == "" {
		topup.PaymentProvider = order.PaymentProvider
	} else if topup.PaymentProvider != order.PaymentProvider {
		return nil, ErrPaymentMethodMismatch
	}
	topup.Amount = 0
	topup.Money = order.Money
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = common.TopUpStatusSuccess
	if err := tx.Save(&topup).Error; err != nil {
		return nil, err
	}
	return &topup, nil
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

// Admin bind (no payment). Creates a UserSubscription from a plan.
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	normalizeValuePackagePlan(plan)
	if plan.IsValuePackage() {
		return "", errors.New("超值套餐不能通过普通订阅绑定，请使用超值套餐专用流程")
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plan.UpgradeGroup) != "" {
		_ = UpdateUserGroupCache(userId, plan.UpgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
	}
	return "", nil
}

func calcSubscriptionBalanceQuota(priceAmount float64) (int, error) {
	if priceAmount <= 0 {
		return 0, nil
	}
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("额度单位配置错误")
	}
	quota := decimal.NewFromFloat(priceAmount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Ceil().
		IntPart()
	return int(quota), nil
}

// PurchaseSubscriptionWithBalance creates a subscription by deducting the user's wallet quota.
func PurchaseSubscriptionWithBalance(userId int, planId int) error {
	if userId <= 0 || planId <= 0 {
		return errors.New("invalid userId or planId")
	}

	var logPlanTitle string
	var logMoney float64
	var chargedQuota int
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		plan, err := getSubscriptionPlanByIdTx(tx, planId)
		if err != nil {
			return err
		}
		if plan.IsValuePackage() {
			return errors.New("超值套餐仅支持联动小铺购买")
		}
		if !plan.Enabled {
			return errors.New("套餐未启用")
		}
		if plan.PriceAmount < 0 {
			return errors.New("套餐价格不能为负数")
		}
		if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
			return errors.New("该套餐不允许使用余额兑换")
		}

		requiredQuota, err := calcSubscriptionBalanceQuota(plan.PriceAmount)
		if err != nil {
			return err
		}

		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if requiredQuota > 0 && user.Quota < requiredQuota {
			return errors.New("余额不足")
		}
		if requiredQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota - ?", requiredQuota)).Error; err != nil {
				return err
			}
		}

		if _, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, PaymentMethodBalance); err != nil {
			return err
		}

		now := common.GetTimestamp()
		tradeNo := fmt.Sprintf("SUBBALUSR%dNO%s%d", userId, common.GetRandomString(6), time.Now().UnixNano())
		order := &SubscriptionOrder{
			UserId:          userId,
			PlanId:          plan.Id,
			Money:           plan.PriceAmount,
			TradeNo:         tradeNo,
			PaymentMethod:   PaymentMethodBalance,
			PaymentProvider: PaymentProviderBalance,
			Status:          common.TopUpStatusSuccess,
			CreateTime:      now,
			CompleteTime:    now,
			ProviderPayload: fmt.Sprintf("charged_quota=%d", requiredQuota),
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}

		logPlanTitle = plan.Title
		logMoney = plan.PriceAmount
		chargedQuota = requiredQuota
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		return nil
	})
	if err != nil {
		return err
	}

	if chargedQuota > 0 {
		if err := cacheDecrUserQuota(userId, int64(chargedQuota)); err != nil {
			common.SysLog("failed to decrease user quota cache after subscription balance purchase: " + err.Error())
		}
	}
	if upgradeGroup != "" {
		_ = UpdateUserGroupCache(userId, upgradeGroup)
	}
	msg := fmt.Sprintf("使用余额购买订阅成功，套餐: %s，支付金额: %.2f，扣除额度: %d", logPlanTitle, logMoney, chargedQuota)
	RecordLog(userId, LogTypeTopup, msg)
	return nil
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	subs, err = filterRegularUserSubscriptionsTx(nil, subs)
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active regular subscription.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	err := DB.Table("user_subscriptions").
		Joins("JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id").
		Where("user_subscriptions.user_id = ? AND user_subscriptions.status = ? AND user_subscriptions.end_time > ?", userId, UserSubscriptionStatusActive, now).
		Where("(subscription_plans.plan_kind = ? OR subscription_plans.plan_kind = '' OR subscription_plans.plan_kind IS NULL)", SubscriptionPlanKindSubscription).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	subs, err = filterRegularUserSubscriptionsTx(nil, subs)
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
		})
	}
	return result
}

func filterRegularUserSubscriptionsTx(tx *gorm.DB, subs []UserSubscription) ([]UserSubscription, error) {
	if len(subs) == 0 {
		return []UserSubscription{}, nil
	}
	out := make([]UserSubscription, 0, len(subs))
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return nil, err
		}
		normalizeValuePackagePlan(plan)
		if plan.IsValuePackage() {
			continue
		}
		out = append(out, sub)
	}
	return out, nil
}

const (
	ValuePackagePurchaseActionCreate  = "create"
	ValuePackagePurchaseActionExtend  = "extend"
	ValuePackagePurchaseActionUpgrade = "upgrade"
)

type ValuePackagePurchaseIntent struct {
	Action               string            `json:"action"`
	RequiresConfirmation bool              `json:"requires_confirmation"`
	CurrentSubscription  *UserSubscription `json:"current_subscription,omitempty"`
	CurrentPlan          *SubscriptionPlan `json:"current_plan,omitempty"`
	TargetPlan           *SubscriptionPlan `json:"target_plan,omitempty"`
	Message              string            `json:"message,omitempty"`
}

func (p *SubscriptionPlan) IsValuePackage() bool {
	return strings.TrimSpace(p.PlanKind) == SubscriptionPlanKindValuePackage
}

func normalizeValuePackagePlan(plan *SubscriptionPlan) {
	if plan == nil {
		return
	}
	plan.PlanKind = strings.TrimSpace(plan.PlanKind)
	if plan.PlanKind == "" {
		plan.PlanKind = SubscriptionPlanKindSubscription
	}
	plan.PackageType = strings.TrimSpace(plan.PackageType)
	if plan.PackageLevel <= 0 {
		switch plan.PackageType {
		case ValuePackageTypeDay:
			plan.PackageLevel = ValuePackageLevelDay
		case ValuePackageTypeWeek:
			plan.PackageLevel = ValuePackageLevelWeek
		case ValuePackageTypeMonth:
			plan.PackageLevel = ValuePackageLevelMonth
		}
	}
	normalizeValuePackageFixedDurationFields(plan)
	if plan.ConcurrencyLimit <= 0 {
		plan.ConcurrencyLimit = 1
	}
}

func normalizeValuePackageFixedDurationFields(plan *SubscriptionPlan) {
	if plan == nil || !plan.IsValuePackage() {
		return
	}
	switch plan.PackageType {
	case ValuePackageTypeDay:
		plan.DurationUnit = SubscriptionDurationDay
		plan.DurationValue = 1
		plan.CustomSeconds = 0
		plan.Limit7dAmount = 0
	case ValuePackageTypeWeek:
		plan.DurationUnit = SubscriptionDurationDay
		plan.DurationValue = 7
		plan.CustomSeconds = 0
		plan.Limit7dAmount = 0
	case ValuePackageTypeMonth:
		plan.DurationUnit = SubscriptionDurationDay
		plan.DurationValue = 30
		plan.CustomSeconds = 0
	}
}

func valuePackageCycleSeconds(plan *SubscriptionPlan) int64 {
	if plan == nil || !plan.IsValuePackage() {
		return 0
	}
	switch plan.PackageType {
	case ValuePackageTypeDay:
		return 24 * 60 * 60
	case ValuePackageTypeWeek:
		return 7 * 24 * 60 * 60
	case ValuePackageTypeMonth:
		return 30 * 24 * 60 * 60
	default:
		return 0
	}
}

func syncValuePackageCycleSchedule(sub *UserSubscription, plan *SubscriptionPlan) {
	if sub == nil {
		return
	}
	cycleSeconds := valuePackageCycleSeconds(plan)
	if cycleSeconds <= 0 || sub.StartTime <= 0 {
		sub.LastResetTime = 0
		sub.NextResetTime = 0
		return
	}
	if sub.LastResetTime < sub.StartTime {
		sub.LastResetTime = sub.StartTime
	}
	nextResetTime := sub.LastResetTime + cycleSeconds
	if sub.EndTime > 0 && nextResetTime < sub.EndTime {
		sub.NextResetTime = nextResetTime
	} else {
		sub.NextResetTime = 0
	}
}

func maybeAdvanceValuePackageCycleTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid value package cycle args")
	}
	cycleSeconds := valuePackageCycleSeconds(plan)
	if cycleSeconds <= 0 || sub.StartTime <= 0 || now < sub.StartTime+cycleSeconds {
		return nil
	}
	completedCycles := (now - sub.StartTime) / cycleSeconds
	cycleStart := sub.StartTime + completedCycles*cycleSeconds
	lastCycleStart := sub.LastResetTime
	if lastCycleStart < sub.StartTime {
		lastCycleStart = sub.StartTime
	}
	if cycleStart <= lastCycleStart {
		return nil
	}
	if sub.QuotaEpoch == math.MaxInt64 {
		return errors.New("value package quota epoch overflow")
	}
	fromEpoch := sub.QuotaEpoch
	amountUsedBefore := sub.AmountUsed
	sub.AmountUsed = 0
	sub.QuotaEpoch++
	sub.LastResetTime = cycleStart
	nextCycleStart := cycleStart + cycleSeconds
	if sub.EndTime > 0 && nextCycleStart >= sub.EndTime {
		sub.NextResetTime = 0
	} else {
		sub.NextResetTime = nextCycleStart
	}
	if err := tx.Create(&ValuePackageQuotaReset{
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PlanId:             sub.PlanId,
		PackageType:        plan.PackageType,
		ResetAt:            cycleStart,
		FromEpoch:          fromEpoch,
		ToEpoch:            sub.QuotaEpoch,
		AmountUsedBefore:   amountUsedBefore,
		Source:             ValuePackageQuotaResetSourceCycleRenewal,
		Note:               "fixed package cycle renewal",
	}).Error; err != nil {
		return err
	}
	return tx.Save(sub).Error
}

func getActiveValuePackageSubscriptionsTx(tx *gorm.DB, userId int, now int64) ([]UserSubscription, error) {
	if tx == nil {
		tx = DB
	}
	var subs []UserSubscription
	err := tx.Where("user_id = ? AND status = ? AND end_time > ?", userId, UserSubscriptionStatusActive, now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	out := make([]UserSubscription, 0, len(subs))
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return nil, err
		}
		normalizeValuePackagePlan(plan)
		if plan.IsValuePackage() {
			out = append(out, sub)
		}
	}
	return out, nil
}

func getHighestActiveValuePackageTx(tx *gorm.DB, userId int, now int64) (*UserSubscription, *SubscriptionPlan, error) {
	subs, err := getActiveValuePackageSubscriptionsTx(tx, userId, now)
	if err != nil {
		return nil, nil, err
	}
	var bestSub *UserSubscription
	var bestPlan *SubscriptionPlan
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return nil, nil, err
		}
		normalizeValuePackagePlan(plan)
		if bestPlan == nil || plan.PackageLevel > bestPlan.PackageLevel || (plan.PackageLevel == bestPlan.PackageLevel && sub.EndTime > bestSub.EndTime) {
			subCopy := sub
			bestSub = &subCopy
			bestPlan = plan
		}
	}
	return bestSub, bestPlan, nil
}

func CheckValuePackagePurchaseIntent(userId int, planId int, confirmedCover bool) (*ValuePackagePurchaseIntent, error) {
	if userId <= 0 || planId <= 0 {
		return nil, errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return nil, err
	}
	normalizeValuePackagePlan(plan)
	if !plan.IsValuePackage() {
		return nil, errors.New("目标套餐不是超值套餐")
	}
	if !plan.Enabled {
		return nil, errors.New("套餐未启用")
	}
	return checkValuePackagePurchaseIntentTx(nil, userId, plan, confirmedCover)
}

func CheckValuePackagePurchaseIntentTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, confirmedCover bool) (*ValuePackagePurchaseIntent, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if userId <= 0 || plan == nil || plan.Id <= 0 {
		return nil, errors.New("invalid userId or plan")
	}
	normalizeValuePackagePlan(plan)
	if !plan.IsValuePackage() {
		return nil, errors.New("目标套餐不是超值套餐")
	}
	if !plan.Enabled {
		return nil, errors.New("套餐未启用")
	}
	return checkValuePackagePurchaseIntentTx(tx, userId, plan, confirmedCover)
}

type ValuePackageUsageSummary struct {
	TotalUsed        int64                     `json:"total_used"`
	TotalLimit       int64                     `json:"total_limit"`
	TotalRemaining   int64                     `json:"total_remaining"`
	TotalPercent     float64                   `json:"total_percent"`
	Used5h           int64                     `json:"used_5h"`
	Limit5h          int64                     `json:"limit_5h"`
	Percent5h        float64                   `json:"percent_5h"`
	ResetAt5h        int64                     `json:"reset_at_5h"`
	ResetSeconds5h   int64                     `json:"reset_seconds_5h"`
	Limited5h        bool                      `json:"limited_5h"`
	Used7d           int64                     `json:"used_7d"`
	Limit7d          int64                     `json:"limit_7d"`
	Percent7d        float64                   `json:"percent_7d"`
	ResetAt7d        int64                     `json:"reset_at_7d"`
	ResetSeconds7d   int64                     `json:"reset_seconds_7d"`
	Limited7d        bool                      `json:"limited_7d"`
	Exhausted        bool                      `json:"exhausted"`
	ExhaustedReason  string                    `json:"exhausted_reason"`
	ExhaustedMessage string                    `json:"exhausted_message"`
	PeriodLimits     []ValuePackagePeriodLimit `json:"period_limits"`
}

type ValuePackageWindowUsageDetails struct {
	Used5h              int64
	Earliest5hCreatedAt int64
	ResetAt5h           int64
	ResetSeconds5h      int64
	Used7d              int64
	Earliest7dCreatedAt int64
	ResetAt7d           int64
	ResetSeconds7d      int64
}

type ValuePackageBillingState struct {
	Active             bool    `json:"active"`
	RoutingGroup       string  `json:"routing_group"`
	PackageGroup       string  `json:"package_group"`
	EffectiveRatio     float64 `json:"effective_ratio"`
	OriginalGroupRatio float64 `json:"original_group_ratio"`
	PlanTitle          string  `json:"plan_title"`
	PlanId             int     `json:"plan_id"`
}

type ValuePackageState struct {
	Preference   UserValuePackagePreference `json:"preference"`
	Subscription *UserSubscription          `json:"subscription,omitempty"`
	Plan         *SubscriptionPlan          `json:"plan,omitempty"`
	Usage        *ValuePackageUsageSummary  `json:"usage,omitempty"`
	Billing      *ValuePackageBillingState  `json:"billing"`
}

type ValuePackageResetCountAdjustMode string

const (
	ValuePackageResetCountAdjustModeSet      ValuePackageResetCountAdjustMode = "set"
	ValuePackageResetCountAdjustModeAdd      ValuePackageResetCountAdjustMode = "add"
	ValuePackageResetCountAdjustModeSubtract ValuePackageResetCountAdjustMode = "subtract"
)

type ValuePackageResetCountAdjustment struct {
	UserId      int    `json:"user_id"`
	OldCount    int    `json:"old_count"`
	NewCount    int    `json:"new_count"`
	Delta       int    `json:"delta"`
	Mode        string `json:"mode"`
	Reason      string `json:"reason"`
	AdminUserId int    `json:"admin_user_id"`
}

type ValuePackageUsageRow struct {
	UserId       int                       `json:"user_id"`
	Username     string                    `json:"username"`
	Subscription UserSubscription          `json:"subscription"`
	Plan         SubscriptionPlan          `json:"plan"`
	Usage        *ValuePackageUsageSummary `json:"usage"`
}

type ValuePackageManagementFilter struct {
	Keyword     string
	PackageType string
	Active      string
	Page        int
	PageSize    int
}

type ValuePackageManagementResult struct {
	Items []ValuePackageManagementRow `json:"items"`
	Total int64                       `json:"total"`
}

type ValuePackageManagementRow struct {
	UserId             int                       `json:"user_id"`
	Username           string                    `json:"username"`
	DisplayName        string                    `json:"display_name"`
	PackageType        string                    `json:"package_type"`
	PlanTitle          string                    `json:"plan_title"`
	SubscriptionId     int                       `json:"subscription_id"`
	SubscriptionStatus string                    `json:"subscription_status"`
	StartTime          int64                     `json:"start_time"`
	EndTime            int64                     `json:"end_time"`
	Enabled            bool                      `json:"enabled"`
	ResetCount         int                       `json:"reset_count"`
	Usage              *ValuePackageUsageSummary `json:"usage"`
	LastResetAt        int64                     `json:"last_reset_at"`
}

func GetValuePackagePlansForUser(userId int) ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan
	if err := DB.Where("enabled = ? AND plan_kind = ?", true, SubscriptionPlanKindValuePackage).
		Order("package_level asc, sort_order desc, id desc").
		Find(&plans).Error; err != nil {
		return nil, err
	}
	for i := range plans {
		plans[i].NormalizeDefaults()
		normalizeValuePackagePlan(&plans[i])
	}
	return plans, nil
}

func ListEnabledValuePackageBillingGroups() ([]string, error) {
	var candidates []string
	if err := DB.Model(&SubscriptionPlan{}).
		Where("enabled = ? AND plan_kind = ?", true, SubscriptionPlanKindValuePackage).
		Pluck("model_group", &candidates).Error; err != nil {
		return nil, err
	}

	unique := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		group := strings.TrimSpace(candidate)
		if group != "" {
			unique[group] = struct{}{}
		}
	}
	groups := make([]string, 0, len(unique))
	for group := range unique {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, nil
}

func GetValuePackageState(userId int) (*ValuePackageState, error) {
	return getValuePackageStateTx(DB, userId)
}

func ListActiveValuePackageUsageRows(now int64) ([]ValuePackageUsageRow, error) {
	return listActiveValuePackageUsageRowsTx(DB, now)
}

func ListValuePackageManagementRows(filter ValuePackageManagementFilter, now int64) (*ValuePackageManagementResult, error) {
	return listValuePackageManagementRowsTx(DB, filter, now)
}

func valuePackageFixedWindowUsageDetails(records []ValuePackageUsageRecord, windowSeconds int64, now int64) (int64, int64) {
	if windowSeconds <= 0 || now <= 0 || len(records) == 0 {
		return 0, 0
	}
	var windowStart int64
	var windowEnd int64
	var expiredUntil int64
	var used int64
	for _, record := range records {
		if record.Quota <= 0 || record.CreatedAt <= 0 || record.CreatedAt > now {
			continue
		}
		if record.CreatedAt < expiredUntil {
			continue
		}
		if windowStart == 0 || record.CreatedAt >= windowEnd {
			windowStart = record.CreatedAt
			windowEnd = windowStart + windowSeconds
			used = 0
		}
		if now >= windowEnd {
			expiredUntil = windowEnd
			windowStart = 0
			windowEnd = 0
			used = 0
			continue
		}
		used += record.Quota
	}
	if windowStart == 0 {
		return 0, 0
	}
	return used, windowStart
}

func valuePackageRollingUsageDetails(records []ValuePackageUsageRecord) (int64, int64) {
	var used int64
	var earliestCreatedAt int64
	for _, record := range records {
		if record.Quota <= 0 || record.CreatedAt <= 0 {
			continue
		}
		used += record.Quota
		if earliestCreatedAt == 0 || record.CreatedAt < earliestCreatedAt {
			earliestCreatedAt = record.CreatedAt
		}
	}
	return used, earliestCreatedAt
}

func buildValuePackageUsageSummaryFromDetails(sub *UserSubscription, plan *SubscriptionPlan, usageDetails *ValuePackageWindowUsageDetails, now int64) *ValuePackageUsageSummary {
	if sub == nil || plan == nil || usageDetails == nil {
		return nil
	}
	limited5h := plan.Limit5hAmount > 0 && usageDetails.Used5h >= plan.Limit5hAmount
	has7dWindow := valuePackageHas7dWindow(plan)
	used7d := int64(0)
	limit7d := int64(0)
	percent7d := float64(0)
	limited7d := false
	if has7dWindow {
		used7d = usageDetails.Used7d
		limit7d = plan.Limit7dAmount
		percent7d = valuePackagePercent(used7d, limit7d)
		limited7d = limit7d > 0 && used7d >= limit7d
	}
	totalRemaining := int64(0)
	if sub.AmountTotal > 0 && sub.AmountTotal > sub.AmountUsed {
		totalRemaining = sub.AmountTotal - sub.AmountUsed
	}
	summary := &ValuePackageUsageSummary{
		TotalUsed:      sub.AmountUsed,
		TotalLimit:     sub.AmountTotal,
		TotalRemaining: totalRemaining,
		TotalPercent:   valuePackagePercent(sub.AmountUsed, sub.AmountTotal),
		Used5h:         usageDetails.Used5h,
		Limit5h:        plan.Limit5hAmount,
		Percent5h:      valuePackagePercent(usageDetails.Used5h, plan.Limit5hAmount),
		Limited5h:      limited5h,
		Used7d:         used7d,
		Limit7d:        limit7d,
		Percent7d:      percent7d,
		Limited7d:      limited7d,
	}
	summary.PeriodLimits = buildValuePackagePeriodLimits(sub, plan, usageDetails)
	if plan.Limit5hAmount > 0 {
		summary.ResetAt5h = usageDetails.ResetAt5h
		summary.ResetSeconds5h = usageDetails.ResetSeconds5h
	}
	if has7dWindow {
		summary.ResetAt7d = usageDetails.ResetAt7d
		summary.ResetSeconds7d = usageDetails.ResetSeconds7d
	}
	switch {
	case sub.AmountTotal > 0 && sub.AmountUsed >= sub.AmountTotal:
		summary.Exhausted = true
		summary.ExhaustedReason = ValuePackageExhaustedReasonTotal
	case limited5h:
		summary.Exhausted = true
		summary.ExhaustedReason = ValuePackageExhaustedReason5h
	case limited7d:
		summary.Exhausted = true
		summary.ExhaustedReason = ValuePackageExhaustedReason7d
	}
	if summary.Exhausted {
		summary.ExhaustedMessage = ValuePackageQuotaExhaustedUserMessage
	}
	return summary
}

func buildValuePackageWindowUsageDetailsFromRecords(sub *UserSubscription, plan *SubscriptionPlan, records []ValuePackageUsageRecord, lastResetAt int64, now int64) *ValuePackageWindowUsageDetails {
	details := &ValuePackageWindowUsageDetails{}
	if sub == nil || plan == nil || now <= 0 {
		return details
	}
	effectiveLastResetAt := int64(0)
	if lastResetAt > 0 && lastResetAt <= now {
		effectiveLastResetAt = lastResetAt
	}
	fiveHourRecords := make([]ValuePackageUsageRecord, 0, len(records))
	for _, record := range records {
		if record.QuotaEpoch != sub.QuotaEpoch {
			continue
		}
		if record.Quota <= 0 || record.CreatedAt <= 0 || record.CreatedAt > now {
			continue
		}
		if effectiveLastResetAt > 0 && record.CreatedAt < effectiveLastResetAt {
			continue
		}
		fiveHourRecords = append(fiveHourRecords, record)
	}
	details.Used5h, details.Earliest5hCreatedAt = valuePackageFixedWindowUsageDetails(fiveHourRecords, valuePackage5hWindowSeconds, now)
	if details.Used5h > 0 && details.Earliest5hCreatedAt > 0 {
		details.ResetAt5h = details.Earliest5hCreatedAt + valuePackage5hWindowSeconds
		details.ResetSeconds5h = details.ResetAt5h - now
		if details.ResetSeconds5h < 0 {
			details.ResetSeconds5h = 0
		}
	}

	if !valuePackageHas7dWindow(plan) {
		return details
	}
	anchorStart := valuePackageSubscriptionAnchorStart(sub, now)
	if anchorStart <= 0 {
		return details
	}
	window := calcValuePackageAnchoredWindow(anchorStart, sub.EndTime, valuePackage7dWindowSeconds, now)
	if window.Start <= 0 || window.End <= 0 {
		return details
	}
	effective7dStart := window.Start
	if valuePackageResetClears7d(plan) && effectiveLastResetAt > window.Start {
		effective7dStart = effectiveLastResetAt
	}
	for _, record := range records {
		if record.QuotaEpoch != sub.QuotaEpoch {
			continue
		}
		if record.Quota <= 0 || record.CreatedAt <= 0 || record.CreatedAt > now {
			continue
		}
		if record.CreatedAt < effective7dStart || record.CreatedAt >= window.End {
			continue
		}
		details.Used7d += record.Quota
		if details.Earliest7dCreatedAt == 0 || record.CreatedAt < details.Earliest7dCreatedAt {
			details.Earliest7dCreatedAt = record.CreatedAt
		}
	}
	details.ResetAt7d = window.End
	details.ResetSeconds7d = details.ResetAt7d - now
	if details.ResetSeconds7d < 0 {
		details.ResetSeconds7d = 0
	}
	return details
}

func listValuePackageManagementRowsTx(tx *gorm.DB, filter ValuePackageManagementFilter, now int64) (*ValuePackageManagementResult, error) {
	if tx == nil {
		tx = DB
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	} else if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	active := strings.TrimSpace(filter.Active)
	if active == "" {
		active = "active"
	}
	packageType := strings.TrimSpace(filter.PackageType)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))

	buildQuery := func() *gorm.DB {
		query := tx.Model(&UserSubscription{}).
			Joins("JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id").
			Joins("JOIN users ON users.id = user_subscriptions.user_id").
			Where("subscription_plans.plan_kind = ?", SubscriptionPlanKindValuePackage)
		if packageType != "" && packageType != "all" {
			query = query.Where("subscription_plans.package_type = ?", packageType)
		}
		switch active {
		case "expired":
			query = query.Where("(user_subscriptions.status = ? OR (user_subscriptions.status = ? AND user_subscriptions.end_time <= ?))", UserSubscriptionStatusExpired, UserSubscriptionStatusActive, now)
		case "all":
			query = query.Where("(user_subscriptions.status = ? OR user_subscriptions.status = ?)", UserSubscriptionStatusExpired, UserSubscriptionStatusActive)
		default:
			query = query.Where("user_subscriptions.status = ? AND user_subscriptions.end_time > ?", UserSubscriptionStatusActive, now)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			if keywordUserId, err := strconv.Atoi(keyword); err == nil {
				query = query.Where("(LOWER(users.username) LIKE ? OR LOWER(users.display_name) LIKE ? OR users.id = ?)", like, like, keywordUserId)
			} else {
				query = query.Where("(LOWER(users.username) LIKE ? OR LOWER(users.display_name) LIKE ?)", like, like)
			}
		}
		return query
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, err
	}
	if total == 0 {
		return &ValuePackageManagementResult{Items: []ValuePackageManagementRow{}, Total: 0}, nil
	}

	var pageSubs []UserSubscription
	if err := buildQuery().
		Select("user_subscriptions.*").
		Order("user_subscriptions.end_time desc, user_subscriptions.id desc").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Find(&pageSubs).Error; err != nil {
		return nil, err
	}
	if len(pageSubs) == 0 {
		return &ValuePackageManagementResult{Items: []ValuePackageManagementRow{}, Total: total}, nil
	}

	subIDs := make([]int, 0, len(pageSubs))
	userIDs := make([]int, 0, len(pageSubs))
	planIDs := make([]int, 0, len(pageSubs))
	for _, sub := range pageSubs {
		subIDs = append(subIDs, sub.Id)
		userIDs = append(userIDs, sub.UserId)
		planIDs = append(planIDs, sub.PlanId)
	}

	var users []User
	if err := tx.Select("id", "username", "display_name").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	usersByID := make(map[int]User, len(users))
	for _, user := range users {
		usersByID[user.Id] = user
	}

	var plans []SubscriptionPlan
	if err := tx.Where("id IN ?", planIDs).Find(&plans).Error; err != nil {
		return nil, err
	}
	plansByID := make(map[int]SubscriptionPlan, len(plans))
	for _, plan := range plans {
		normalizeValuePackagePlan(&plan)
		if plan.IsValuePackage() {
			plansByID[plan.Id] = plan
		}
	}

	var prefs []UserValuePackagePreference
	if err := tx.Where("user_id IN ?", userIDs).Find(&prefs).Error; err != nil {
		return nil, err
	}
	prefsByUserID := make(map[int]UserValuePackagePreference, len(prefs))
	for _, pref := range prefs {
		prefsByUserID[pref.UserId] = pref
	}

	var resetRows []struct {
		UserSubscriptionId int
		ResetAt            int64
	}
	if err := tx.Model(&ValuePackageQuotaReset{}).
		Where("user_subscription_id IN ? AND reset_at <= ?", subIDs, now).
		Select("user_subscription_id, COALESCE(MAX(reset_at), 0) AS reset_at").
		Group("user_subscription_id").
		Scan(&resetRows).Error; err != nil {
		return nil, err
	}
	lastResetBySubID := make(map[int]int64, len(resetRows))
	for _, row := range resetRows {
		lastResetBySubID[row.UserSubscriptionId] = row.ResetAt
	}

	usageLowerBound := int64(0)
	hasMissingAnchorStart := false
	for _, sub := range pageSubs {
		anchorStart := valuePackageSubscriptionAnchorStart(&sub, now)
		if anchorStart > 0 {
			if usageLowerBound == 0 || anchorStart < usageLowerBound {
				usageLowerBound = anchorStart
			}
		} else {
			hasMissingAnchorStart = true
		}
	}
	fallbackUsageLowerBound := now - valuePackage7dWindowSeconds
	if hasMissingAnchorStart && (usageLowerBound == 0 || fallbackUsageLowerBound < usageLowerBound) {
		usageLowerBound = fallbackUsageLowerBound
	}
	if usageLowerBound == 0 {
		usageLowerBound = fallbackUsageLowerBound
	}

	var usageRecords []ValuePackageUsageRecord
	if err := tx.Where("user_subscription_id IN ? AND created_at >= ? AND created_at <= ? AND quota > ?", subIDs, usageLowerBound, now, 0).
		Order("user_subscription_id asc, created_at asc, id asc").
		Find(&usageRecords).Error; err != nil {
		return nil, err
	}
	usageRecordsBySubID := make(map[int][]ValuePackageUsageRecord, len(pageSubs))
	for _, record := range usageRecords {
		usageRecordsBySubID[record.UserSubscriptionId] = append(usageRecordsBySubID[record.UserSubscriptionId], record)
	}

	items := make([]ValuePackageManagementRow, 0, len(pageSubs))
	for _, sub := range pageSubs {
		user, ok := usersByID[sub.UserId]
		if !ok {
			continue
		}
		plan, ok := plansByID[sub.PlanId]
		if !ok {
			continue
		}
		pref := prefsByUserID[user.Id]
		details := buildValuePackageWindowUsageDetailsFromRecords(&sub, &plan, usageRecordsBySubID[sub.Id], lastResetBySubID[sub.Id], now)
		usage := buildValuePackageUsageSummaryFromDetails(&sub, &plan, details, now)
		items = append(items, ValuePackageManagementRow{
			UserId:             user.Id,
			Username:           user.Username,
			DisplayName:        user.DisplayName,
			PackageType:        plan.PackageType,
			PlanTitle:          plan.Title,
			SubscriptionId:     sub.Id,
			SubscriptionStatus: sub.Status,
			StartTime:          sub.StartTime,
			EndTime:            sub.EndTime,
			Enabled:            pref.Enabled && pref.ActiveUserSubscriptionId == sub.Id,
			ResetCount:         pref.ResetCount,
			Usage:              usage,
			LastResetAt:        lastResetBySubID[sub.Id],
		})
	}

	return &ValuePackageManagementResult{Items: items, Total: total}, nil
}

func listActiveValuePackageUsageRowsTx(tx *gorm.DB, now int64) ([]ValuePackageUsageRow, error) {
	if tx == nil {
		tx = DB
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	if err := backfillDefaultEnabledValuePackagePreferencesTx(tx, now); err != nil {
		return nil, err
	}

	var prefs []UserValuePackagePreference
	if err := tx.Where("enabled = ? AND active_user_subscription_id > ?", true, 0).
		Order("active_user_subscription_id asc").Find(&prefs).Error; err != nil {
		return nil, err
	}

	rows := make([]ValuePackageUsageRow, 0, len(prefs))
	for _, pref := range prefs {
		var sub UserSubscription
		subResult := tx.Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", pref.ActiveUserSubscriptionId, pref.UserId, UserSubscriptionStatusActive, now).Limit(1).Find(&sub)
		if subResult.Error != nil {
			return nil, subResult.Error
		}
		if subResult.RowsAffected == 0 {
			continue
		}

		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			continue
		}

		var user User
		userResult := tx.Select("id", "username", "display_name").Where("id = ?", pref.UserId).Limit(1).Find(&user)
		if userResult.Error != nil {
			return nil, userResult.Error
		}
		if userResult.RowsAffected == 0 {
			continue
		}

		usage, err := buildValuePackageUsageSummaryTx(tx, pref.UserId, &sub, plan, now)
		if err != nil {
			return nil, err
		}
		rows = append(rows, ValuePackageUsageRow{
			UserId:       pref.UserId,
			Username:     user.Username,
			Subscription: sub,
			Plan:         *plan,
			Usage:        usage,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Plan.PackageLevel != rows[j].Plan.PackageLevel {
			return rows[i].Plan.PackageLevel < rows[j].Plan.PackageLevel
		}
		if rows[i].Username != rows[j].Username {
			return rows[i].Username < rows[j].Username
		}
		return rows[i].UserId < rows[j].UserId
	})

	return rows, nil
}

func backfillDefaultEnabledValuePackagePreferencesTx(tx *gorm.DB, now int64) error {
	if tx == nil {
		tx = DB
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	var subs []UserSubscription
	if err := tx.Where("status = ? AND end_time > ?", UserSubscriptionStatusActive, now).
		Order("user_id asc, end_time desc, id desc").
		Find(&subs).Error; err != nil {
		return err
	}
	candidateUsers := make(map[int]struct{})
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			continue
		}
		candidateUsers[sub.UserId] = struct{}{}
	}
	for userId := range candidateUsers {
		var pref UserValuePackagePreference
		if err := tx.Where("user_id = ?", userId).First(&pref).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				bestSub, _, err := getHighestActiveValuePackageTx(tx, userId, now)
				if err != nil {
					return err
				}
				if bestSub == nil {
					continue
				}
				if _, err := upsertValuePackagePreferenceTx(tx, userId, true, bestSub.Id); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if pref.Enabled || pref.ActiveUserSubscriptionId <= 0 || pref.CreatedAt <= 0 || pref.UpdatedAt <= 0 || pref.CreatedAt != pref.UpdatedAt {
			continue
		}
		var sub UserSubscription
		subResult := tx.Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", pref.ActiveUserSubscriptionId, userId, UserSubscriptionStatusActive, now).Limit(1).Find(&sub)
		if subResult.Error != nil {
			return subResult.Error
		}
		if subResult.RowsAffected == 0 {
			continue
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		normalizeValuePackagePlan(plan)
		if plan.IsValuePackage() {
			if _, err := upsertValuePackagePreferenceTx(tx, userId, true, sub.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

func inactiveValuePackageBillingState() *ValuePackageBillingState {
	return &ValuePackageBillingState{Active: false}
}

func newValuePackageState(pref UserValuePackagePreference) *ValuePackageState {
	return &ValuePackageState{Preference: pref, Billing: inactiveValuePackageBillingState()}
}

func buildValuePackageBillingState(pref *UserValuePackagePreference, sub *UserSubscription, plan *SubscriptionPlan) *ValuePackageBillingState {
	// Active subscription billing intentionally does not depend on plan.Enabled:
	// disabling a plan should not revoke already purchased active packages.
	if pref == nil || sub == nil || plan == nil || !pref.Enabled || !plan.IsValuePackage() {
		return inactiveValuePackageBillingState()
	}
	packageGroup := strings.TrimSpace(plan.ModelGroup)
	if packageGroup == "" {
		return inactiveValuePackageBillingState()
	}
	return &ValuePackageBillingState{
		Active:         true,
		RoutingGroup:   "",
		PackageGroup:   packageGroup,
		EffectiveRatio: ValuePackageEffectiveBillingRatio,
		PlanTitle:      plan.Title,
		PlanId:         plan.Id,
	}
}

func GetActiveValuePackageForRelay(userId int) (*ValuePackageState, error) {
	state, err := loadValuePackageStateTx(DB, userId, false)
	if err != nil {
		return nil, err
	}
	if state == nil || !state.Preference.Enabled || state.Subscription == nil || state.Plan == nil {
		return nil, nil
	}
	if !state.Plan.IsValuePackage() || strings.TrimSpace(state.Plan.ModelGroup) == "" {
		return nil, nil
	}
	return state, nil
}

func getValuePackageStateTx(tx *gorm.DB, userId int) (*ValuePackageState, error) {
	return loadValuePackageStateTx(tx, userId, true)
}

func loadValuePackageStateTx(tx *gorm.DB, userId int, includeUsage bool) (*ValuePackageState, error) {
	if userId <= 0 {
		return newValuePackageState(UserValuePackagePreference{}), nil
	}
	if tx == nil {
		tx = DB
	}
	var pref UserValuePackagePreference
	if err := tx.Where("user_id = ?", userId).First(&pref).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			now := getDBTimestampTx(tx)
			sub, plan, err := getHighestActiveValuePackageTx(tx, userId, now)
			if err != nil {
				return nil, err
			}
			if sub == nil || plan == nil {
				return newValuePackageState(UserValuePackagePreference{UserId: userId}), nil
			}
			prefPtr, err := upsertValuePackagePreferenceTx(tx, userId, true, sub.Id)
			if err != nil {
				return nil, err
			}
			state := newValuePackageState(*prefPtr)
			state.Subscription = sub
			state.Plan = plan
			state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
			if includeUsage {
				usage, err := buildValuePackageUsageSummaryTx(tx, userId, sub, plan, now)
				if err != nil {
					return nil, err
				}
				state.Usage = usage
			}
			return state, nil
		}
		return nil, err
	}
	state := newValuePackageState(pref)
	if pref.ActiveUserSubscriptionId <= 0 {
		return state, nil
	}
	now := getDBTimestampTx(tx)
	var sub UserSubscription
	if err := tx.Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", pref.ActiveUserSubscriptionId, userId, UserSubscriptionStatusActive, now).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return state, nil
		}
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return state, nil
		}
		return nil, err
	}
	normalizeValuePackagePlan(plan)
	if !plan.IsValuePackage() {
		return state, nil
	}
	if !pref.Enabled && pref.CreatedAt > 0 && pref.UpdatedAt > 0 && pref.CreatedAt == pref.UpdatedAt {
		prefPtr, err := upsertValuePackagePreferenceTx(tx, userId, true, sub.Id)
		if err != nil {
			return nil, err
		}
		pref = *prefPtr
		state.Preference = pref
	}
	state.Subscription = &sub
	state.Plan = plan
	state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
	if !includeUsage {
		return state, nil
	}
	usage, err := buildValuePackageUsageSummaryTx(tx, userId, &sub, plan, now)
	if err != nil {
		return nil, err
	}
	state.Usage = usage
	return state, nil
}

func valuePackagePercent(used int64, limit int64) float64 {
	if limit <= 0 || used <= 0 {
		return 0
	}
	percent := float64(used) * 100 / float64(limit)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}

func buildValuePackageUsageSummaryTx(tx *gorm.DB, userId int, sub *UserSubscription, plan *SubscriptionPlan, now int64) (*ValuePackageUsageSummary, error) {
	if tx == nil {
		tx = DB
	}
	if sub == nil || plan == nil || sub.Id <= 0 {
		return nil, nil
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	usageDetails, err := getValuePackageWindowUsageDetailsTx(tx, userId, sub.Id, now)
	if err != nil {
		return nil, err
	}
	return buildValuePackageUsageSummaryFromDetails(sub, plan, usageDetails, now), nil
}

func CompleteValuePackageOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string, confirmedCover bool) (*UserSubscription, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("tradeNo is empty")
	}
	var completed *UserSubscription
	var vipUpgraded bool
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := withUpdateLock(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubscriptionOrderNotFound
			}
			return err
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			if order.UserSubscriptionId <= 0 {
				return errors.New("completed order missing user subscription id")
			}
			var sub UserSubscription
			if err := tx.Where("id = ? AND user_id = ?", order.UserSubscriptionId, order.UserId).First(&sub).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("%w: %w", ErrCompletedSubscriptionNotFound, err)
				}
				return err
			}
			completed = &sub
			userId = order.UserId
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		var user User
		if err := withUpdateLock(tx).Where("id = ?", order.UserId).First(&user).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("order plan is not value package")
		}
		intent, err := checkValuePackagePurchaseIntentTx(tx, order.UserId, plan, confirmedCover)
		if err != nil {
			return err
		}
		if intent.RequiresConfirmation {
			return errors.New("购买高级套餐需要确认覆盖当前低级套餐")
		}
		nowUnix := getDBTimestampTx(tx)
		start := time.Unix(nowUnix, 0)
		endUnix, err := calcPlanEndTime(start, plan)
		if err != nil {
			return err
		}
		switch intent.Action {
		case ValuePackagePurchaseActionExtend:
			existing, err := extendValuePackageSubscriptionTx(tx, intent.CurrentSubscription.Id, plan, nowUnix, endUnix)
			if err != nil {
				return err
			}
			completed = existing
		case ValuePackagePurchaseActionUpgrade:
			if intent.CurrentSubscription != nil {
				if err := tx.Model(&UserSubscription{}).Where("id = ?", intent.CurrentSubscription.Id).Updates(map[string]interface{}{
					"status":       UserSubscriptionStatusCovered,
					"covered_time": nowUnix,
					"updated_at":   common.GetTimestamp(),
				}).Error; err != nil {
					return err
				}
			}
			fallthrough
		case ValuePackagePurchaseActionCreate:
			sub := &UserSubscription{UserId: order.UserId, PlanId: plan.Id, AmountTotal: plan.TotalAmount, AmountUsed: 0, StartTime: nowUnix, EndTime: endUnix, Status: UserSubscriptionStatusActive, Source: "ldxp", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp()}
			syncValuePackageCycleSchedule(sub, plan)
			if err := tx.Create(sub).Error; err != nil {
				return err
			}
			completed = sub
			if intent.Action == ValuePackagePurchaseActionUpgrade && intent.CurrentSubscription != nil {
				if err := tx.Model(&UserSubscription{}).Where("id = ?", intent.CurrentSubscription.Id).Update("covered_by_subscription_id", sub.Id).Error; err != nil {
					return err
				}
			}
		default:
			return errors.New("unknown value package purchase action")
		}
		if completed == nil || completed.Id <= 0 {
			return errors.New("completed subscription missing")
		}
		if err := ensureValuePackagePreferenceAfterPurchaseTx(
			tx,
			order.UserId,
			completed.Id,
			plan,
			order.GiftResetCount,
			fmt.Sprintf("开通套餐赠送：%s，订单 %s", plan.Title, order.TradeNo),
		); err != nil {
			return err
		}
		order.UserSubscriptionId = completed.Id
		if actualPaymentMethod != "" {
			order.PaymentMethod = actualPaymentMethod
		}
		topUp, err := upsertSubscriptionTopUpTx(tx, &order)
		if err != nil {
			return err
		}
		if err := AddUserValidTopupCentsTx(tx, order.UserId, MoneyToValidTopupCents(order.Money)); err != nil {
			return err
		}
		if err := MaybeCreateAffiliateCommissionForTopUpTx(tx, topUp); err != nil {
			return err
		}
		vipUpgraded, err = MaybeUpgradeUserToVIPTx(tx, order.UserId)
		if err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		userId = order.UserId
		return nil
	})
	if err != nil {
		return nil, err
	}
	if vipUpgraded && userId > 0 {
		_ = UpdateUserGroupCache(userId, UserGroupVIP)
	}
	return completed, nil
}

func ensureValuePackagePreferenceAfterPurchaseTx(tx *gorm.DB, userId int, completedSubId int, plan *SubscriptionPlan, giftResetCount int, giftNote string) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	if userId <= 0 || completedSubId <= 0 {
		return errors.New("invalid value package preference args")
	}
	if _, err := upsertValuePackagePreferenceTx(tx, userId, true, completedSubId); err != nil {
		return err
	}
	// 每次成交（新开/续费/升级）按套餐配置赠送重置卡
	if plan != nil && plan.IsValuePackage() {
		giftCount := ClampSubscriptionPlanGiftResetCount(giftResetCount)
		if giftCount == 0 {
			return nil
		}
		if _, err := grantValuePackageResetCountTx(tx, userId, giftCount,
			ValuePackageResetCountLedgerSourcePlanGift, userId,
			giftNote); err != nil {
			return err
		}
	}
	return nil
}

// grantValuePackageResetCountTx 在同一事务内为用户增加重置卡并记录台账。
func grantValuePackageResetCountTx(tx *gorm.DB, userId int, count int, source string, actorUserId int, note string) (*UserValuePackagePreference, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if count <= 0 {
		return nil, errors.New("invalid reset card count")
	}
	pref, err := ensureValuePackagePreferenceForUpdateTx(tx, userId)
	if err != nil {
		return nil, err
	}
	oldCount := pref.ResetCount
	newCount := oldCount + count
	if err := tx.Model(&UserValuePackagePreference{}).Where("user_id = ?", userId).Update("reset_count", newCount).Error; err != nil {
		return nil, err
	}
	if err := tx.Create(&ValuePackageResetCountLedger{
		UserId:          userId,
		Delta:           count,
		BeforeCount:     oldCount,
		AfterCount:      newCount,
		Source:          source,
		CreatedByUserId: actorUserId,
		Note:            note,
	}).Error; err != nil {
		return nil, err
	}
	pref.ResetCount = newCount
	return pref, nil
}

func validateValuePackagePlanLifecycleLimits(plan *SubscriptionPlan) error {
	if plan == nil || plan.TotalAmount <= 0 {
		return errors.New("value package total_amount must be greater than zero")
	}
	if plan.PackageType == ValuePackageTypeMonth && plan.Limit7dAmount > plan.TotalAmount {
		return errors.New("value package limit_7d_amount must not exceed total_amount")
	}
	return nil
}

func checkValuePackagePurchaseIntentTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, confirmedCover bool) (*ValuePackagePurchaseIntent, error) {
	if tx == nil {
		tx = DB
	}
	if err := validateValuePackagePlanLifecycleLimits(plan); err != nil {
		return nil, err
	}
	now := getDBTimestampTx(tx)
	currentSub, currentPlan, err := getHighestActiveValuePackageTx(tx, userId, now)
	if err != nil {
		return nil, err
	}
	intent := &ValuePackagePurchaseIntent{Action: ValuePackagePurchaseActionCreate, TargetPlan: plan}
	if currentSub == nil || currentPlan == nil {
		return intent, nil
	}
	intent.CurrentSubscription = currentSub
	intent.CurrentPlan = currentPlan
	if plan.Id == currentPlan.Id {
		intent.Action = ValuePackagePurchaseActionExtend
		return intent, nil
	}
	if plan.PackageLevel < currentPlan.PackageLevel {
		return nil, errors.New("当前已有更高等级套餐未过期，暂不能购买低等级套餐")
	}
	intent.Action = ValuePackagePurchaseActionUpgrade
	if !confirmedCover {
		intent.RequiresConfirmation = true
		intent.Message = fmt.Sprintf("购买 %s 将直接覆盖当前 %s，剩余时间不会折算或顺延", plan.Title, currentPlan.Title)
	}
	return intent, nil
}

func getDBTimestampTx(tx *gorm.DB) int64 {
	if tx == nil {
		return GetDBTimestamp()
	}
	var ts int64
	var err error
	switch {
	case common.UsingPostgreSQL:
		err = tx.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case common.UsingSQLite:
		err = tx.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	default:
		err = tx.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}

func ActivateValuePackage(userId int, userSubscriptionId int) (*ValuePackageState, error) {
	if userId <= 0 || userSubscriptionId <= 0 {
		return nil, errors.New("invalid activation args")
	}
	now := GetDBTimestamp()
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := withUpdateLock(tx).Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", userSubscriptionId, userId, UserSubscriptionStatusActive, now).First(&sub).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("订阅不是超值套餐")
		}
		pref, err := upsertValuePackagePreferenceTx(tx, userId, true, sub.Id)
		if err != nil {
			return err
		}
		usage, err := buildValuePackageUsageSummaryTx(tx, userId, &sub, plan, now)
		if err != nil {
			return err
		}
		state = &ValuePackageState{Preference: *pref, Subscription: &sub, Plan: plan, Usage: usage}
		state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
		return nil
	})
	return state, err
}

func DeactivateValuePackage(userId int) (*ValuePackageState, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		activeSubId := 0
		var current UserValuePackagePreference
		if err := tx.Where("user_id = ?", userId).First(&current).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else {
			activeSubId = current.ActiveUserSubscriptionId
		}
		pref, err := upsertValuePackagePreferenceTx(tx, userId, false, activeSubId)
		if err != nil {
			return err
		}
		state, err = getValuePackageStateTx(tx, userId)
		if err != nil {
			return err
		}
		state.Preference = *pref
		return nil
	})
	return state, err
}

func UpdateValuePackageWalletFallback(userId int, enabled bool) (*ValuePackageState, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := ensureExistingUserForUpdateTx(tx, userId); err != nil {
			return err
		}
		var err error
		state, err = getValuePackageStateTx(tx, userId)
		if err != nil {
			return err
		}
		pref, err := ensureValuePackagePreferenceForUpdateTx(tx, userId)
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		if err := tx.Model(&UserValuePackagePreference{}).
			Where("user_id = ?", userId).
			Updates(map[string]interface{}{
				"wallet_fallback_enabled": enabled,
				"updated_at":              now,
			}).Error; err != nil {
			return err
		}
		pref.WalletFallbackEnabled = common.GetPointer(enabled)
		pref.UpdatedAt = now
		state.Preference = *pref
		return nil
	})
	return state, err
}

func ensureExistingUserForUpdateTx(tx *gorm.DB, userId int) error {
	if tx == nil {
		tx = DB
	}
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	var user User
	err := withUpdateLock(tx).Select("id").Where("id = ?", userId).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("用户不存在")
	}
	return err
}

func ensureValuePackagePreferenceForUpdateTx(tx *gorm.DB, userId int) (*UserValuePackagePreference, error) {
	if tx == nil {
		tx = DB
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var pref UserValuePackagePreference
	err := withUpdateLock(tx).Where("user_id = ?", userId).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pref = UserValuePackagePreference{UserId: userId, Enabled: false, ActiveUserSubscriptionId: 0, ResetCount: 0}
		if err := tx.Create(&pref).Error; err != nil {
			return nil, err
		}
		return &pref, nil
	}
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

func AdjustValuePackageResetCount(userId int, mode ValuePackageResetCountAdjustMode, value int, reason string, adminUserId int) (*ValuePackageResetCountAdjustment, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if value < 0 {
		return nil, errors.New("重置次数不能为负数")
	}
	reason = strings.TrimSpace(reason)
	var adjustment *ValuePackageResetCountAdjustment
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := ensureExistingUserForUpdateTx(tx, userId); err != nil {
			return err
		}
		pref, err := ensureValuePackagePreferenceForUpdateTx(tx, userId)
		if err != nil {
			return err
		}
		oldCount := pref.ResetCount
		newCount := oldCount
		source := ""
		switch mode {
		case ValuePackageResetCountAdjustModeSet:
			newCount = value
			source = ValuePackageResetCountLedgerSourceAdminSet
		case ValuePackageResetCountAdjustModeAdd:
			if value <= 0 {
				return errors.New("调整次数必须大于 0")
			}
			newCount = oldCount + value
			source = ValuePackageResetCountLedgerSourceAdminAdd
		case ValuePackageResetCountAdjustModeSubtract:
			if value <= 0 {
				return errors.New("调整次数必须大于 0")
			}
			newCount = oldCount - value
			if newCount < 0 {
				newCount = 0
			}
			source = ValuePackageResetCountLedgerSourceAdminSubtract
		default:
			return errors.New("无效的调整模式")
		}
		delta := newCount - oldCount
		if delta == 0 {
			return errors.New("重置次数没有变化")
		}
		if err := tx.Model(&UserValuePackagePreference{}).Where("user_id = ?", userId).Update("reset_count", newCount).Error; err != nil {
			return err
		}
		if err := tx.Create(&ValuePackageResetCountLedger{
			UserId:          userId,
			Delta:           delta,
			BeforeCount:     oldCount,
			AfterCount:      newCount,
			Source:          source,
			CreatedByUserId: adminUserId,
			Note:            reason,
		}).Error; err != nil {
			return err
		}
		adjustment = &ValuePackageResetCountAdjustment{
			UserId:      userId,
			OldCount:    oldCount,
			NewCount:    newCount,
			Delta:       delta,
			Mode:        string(mode),
			Reason:      reason,
			AdminUserId: adminUserId,
		}
		return nil
	})
	return adjustment, err
}

func ConsumeValuePackageResetCount(userId int, userSubscriptionId int, resetAt int64, actorUserId int, note string) (*ValuePackageState, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	note = strings.TrimSpace(note)
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		dbNow := getDBTimestampTx(tx)
		if resetAt <= 0 || resetAt > dbNow {
			resetAt = dbNow
		}
		pref, err := ensureValuePackagePreferenceForUpdateTx(tx, userId)
		if err != nil {
			return err
		}
		if pref.ResetCount <= 0 {
			return errors.New("重置次数不足")
		}
		if !pref.Enabled || pref.ActiveUserSubscriptionId <= 0 {
			return errors.New("请先启用超值套餐后再重置额度")
		}
		if userSubscriptionId > 0 && userSubscriptionId != pref.ActiveUserSubscriptionId {
			return errors.New("当前套餐不匹配，请刷新后重试")
		}

		var sub UserSubscription
		if err := withUpdateLock(tx).
			Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", pref.ActiveUserSubscriptionId, userId, UserSubscriptionStatusActive, resetAt).
			First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("当前没有可重置的超值套餐")
			}
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("当前没有可重置的超值套餐")
		}
		if sub.QuotaEpoch == math.MaxInt64 {
			return errors.New("value package quota epoch overflow")
		}
		fromEpoch := sub.QuotaEpoch
		toEpoch := fromEpoch + 1
		amountUsedBefore := sub.AmountUsed

		oldCount := pref.ResetCount
		newCount := oldCount - 1
		result := tx.Model(&UserValuePackagePreference{}).
			Where("user_id = ? AND reset_count > ?", userId, 0).
			Update("reset_count", newCount)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("重置次数不足或状态已变化，请刷新后重试")
		}
		if err := tx.Create(&ValuePackageQuotaReset{
			UserId:             userId,
			UserSubscriptionId: sub.Id,
			PlanId:             plan.Id,
			PackageType:        plan.PackageType,
			ResetAt:            resetAt,
			FromEpoch:          fromEpoch,
			ToEpoch:            toEpoch,
			AmountUsedBefore:   amountUsedBefore,
			Source:             ValuePackageQuotaResetSourceUserConsumeCount,
			CreatedByUserId:    actorUserId,
			Note:               note,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&ValuePackageResetCountLedger{
			UserId:          userId,
			Delta:           -1,
			BeforeCount:     oldCount,
			AfterCount:      newCount,
			Source:          ValuePackageResetCountLedgerSourceUserConsume,
			CreatedByUserId: actorUserId,
			Note:            note,
		}).Error; err != nil {
			return err
		}
		sub.AmountUsed = 0
		sub.QuotaEpoch = toEpoch
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}

		pref.ResetCount = newCount
		usage, err := buildValuePackageUsageSummaryTx(tx, userId, &sub, plan, resetAt)
		if err != nil {
			return err
		}
		state = &ValuePackageState{
			Preference:   *pref,
			Subscription: &sub,
			Plan:         plan,
			Usage:        usage,
		}
		state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
		return nil
	})
	return state, err
}

func upsertValuePackagePreferenceTx(tx *gorm.DB, userId int, enabled bool, activeSubId int) (*UserValuePackagePreference, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	now := common.GetTimestamp()
	updateTime := now
	if !enabled {
		updateTime = now + 1
	}
	pref := UserValuePackagePreference{
		UserId:                   userId,
		Enabled:                  enabled,
		ActiveUserSubscriptionId: activeSubId,
		CreatedAt:                now,
		UpdatedAt:                updateTime,
	}
	updates := map[string]interface{}{
		"enabled":    enabled,
		"updated_at": updateTime,
	}
	if activeSubId > 0 || !enabled {
		updates["active_user_subscription_id"] = activeSubId
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&pref).Error; err != nil {
		return nil, err
	}
	var reloaded UserValuePackagePreference
	if err := tx.Where("user_id = ?", userId).First(&reloaded).Error; err != nil {
		return nil, err
	}
	return &reloaded, nil
}

func RecordValuePackageUsage(record *ValuePackageUsageRecord) error {
	return recordValuePackageUsageTx(DB, record)
}

// ReserveValuePackageUsageToTarget reserves a value-package request to an absolute target quota.
// targetQuota is the same request's desired total usage, not a delta. If a
// ValuePackageUsageRecord already exists for (user_subscription_id, request_id),
// the record uses replacement semantics: only the difference between the old
// quota and targetQuota is applied to UserSubscription.AmountUsed, and window
// checks compare the replaced target instead of double-counting the existing
// request quota. This function updates ValuePackageUsageRecord and
// UserSubscription.AmountUsed atomically, but intentionally does not update
// SubscriptionPreConsumeRecord.PreConsumed; realtime mid-stream/final target
// reserves must not interfere with the existing pre-consume refund contract.
func ReserveValuePackageUsageToTarget(requestId string, userId int, userSubscriptionId int, targetQuota int64) (*SubscriptionPreConsumeResult, error) {
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	if targetQuota < 0 {
		return nil, errors.New("targetQuota must be non-negative")
	}
	now := GetDBTimestamp()
	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := withUpdateLock(tx).
			Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", userSubscriptionId, userId, UserSubscriptionStatusActive, now).
			First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("no active subscription")
			}
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("subscription is not value package")
		}
		if err := maybeAdvanceValuePackageCycleTx(tx, &sub, plan, now); err != nil {
			return err
		}

		requestEpoch := sub.QuotaEpoch
		var preConsumeRecord SubscriptionPreConsumeRecord
		preConsumeQuery := tx.Where("request_id = ?", requestId).Limit(1).Find(&preConsumeRecord)
		if preConsumeQuery.Error != nil {
			return preConsumeQuery.Error
		}
		if preConsumeQuery.RowsAffected > 0 {
			if preConsumeRecord.UserId != userId || preConsumeRecord.UserSubscriptionId != userSubscriptionId {
				return errors.New("subscription pre-consume request mismatch")
			}
			requestEpoch = preConsumeRecord.QuotaEpoch
		}

		var existing ValuePackageUsageRecord
		currentQuota := int64(0)
		currentCreatedAt := int64(0)
		query := tx.Where("user_subscription_id = ? AND request_id = ?", userSubscriptionId, requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			currentQuota = existing.Quota
			currentCreatedAt = existing.CreatedAt
			requestEpoch = existing.QuotaEpoch
			if preConsumeQuery.RowsAffected > 0 && preConsumeRecord.QuotaEpoch != existing.QuotaEpoch {
				return errors.New("value package usage epoch mismatch")
			}
		}

		usedBefore := sub.AmountUsed
		delta := targetQuota - currentQuota
		if delta == 0 {
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = targetQuota
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			returnValue.QuotaEpoch = requestEpoch
			return nil
		}

		if sub.QuotaEpoch != requestEpoch {
			if err := recordValuePackageUsageTx(tx, &ValuePackageUsageRecord{
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				PlanId:             plan.Id,
				PackageType:        plan.PackageType,
				ModelGroup:         plan.ModelGroup,
				RequestId:          requestId,
				Quota:              targetQuota,
				QuotaEpoch:         requestEpoch,
				CreatedAt:          now,
			}); err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = targetQuota
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			returnValue.QuotaEpoch = requestEpoch
			return nil
		}

		if delta > 0 {
			if sub.AmountTotal > 0 && sub.AmountTotal-usedBefore < delta {
				return fmt.Errorf("subscription quota insufficient: %s, need=%d", ValuePackageQuotaExhaustedUserMessage, delta)
			}
			lastResetAt, err := getLastValuePackageQuotaResetAtTx(tx, userId, sub.Id, now)
			if err != nil {
				return err
			}
			lowerBound := valuePackageSubscriptionAnchorStart(&sub, now)
			if lowerBound <= 0 {
				lowerBound = now - valuePackage7dWindowSeconds
			}
			var usageRecords []ValuePackageUsageRecord
			if err := tx.Where("user_id = ? AND user_subscription_id = ? AND quota_epoch = ? AND created_at >= ? AND created_at <= ? AND (quota > ? OR request_id = ?)", userId, sub.Id, requestEpoch, lowerBound, now, 0, requestId).
				Order("created_at asc, id asc").
				Find(&usageRecords).Error; err != nil {
				return err
			}
			nextUsageRecords := make([]ValuePackageUsageRecord, 0, len(usageRecords)+1)
			replacedCurrentRecord := false
			for _, record := range usageRecords {
				if record.RequestId == requestId {
					record.Quota = targetQuota
					replacedCurrentRecord = true
				}
				nextUsageRecords = append(nextUsageRecords, record)
			}
			if currentCreatedAt == 0 && !replacedCurrentRecord {
				nextUsageRecords = append(nextUsageRecords, ValuePackageUsageRecord{
					UserId:             userId,
					UserSubscriptionId: sub.Id,
					PlanId:             plan.Id,
					PackageType:        plan.PackageType,
					ModelGroup:         plan.ModelGroup,
					RequestId:          requestId,
					Quota:              targetQuota,
					QuotaEpoch:         requestEpoch,
					CreatedAt:          now,
				})
			}
			nextDetails := buildValuePackageWindowUsageDetailsFromRecords(&sub, plan, nextUsageRecords, lastResetAt, now)
			if plan.Limit5hAmount > 0 && nextDetails.Used5h > plan.Limit5hAmount {
				return fmt.Errorf("subscription quota insufficient: %s, 5h limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, targetQuota)
			}
			if valuePackageHas7dWindow(plan) && nextDetails.Used7d > plan.Limit7dAmount {
				return fmt.Errorf("subscription quota insufficient: %s, 7d period limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, targetQuota)
			}
		}

		if err := recordValuePackageUsageTx(tx, &ValuePackageUsageRecord{
			UserId:             userId,
			UserSubscriptionId: sub.Id,
			PlanId:             plan.Id,
			PackageType:        plan.PackageType,
			ModelGroup:         plan.ModelGroup,
			RequestId:          requestId,
			Quota:              targetQuota,
			QuotaEpoch:         requestEpoch,
			CreatedAt:          now,
		}); err != nil {
			return err
		}
		sub.AmountUsed += delta
		if sub.AmountUsed < 0 {
			sub.AmountUsed = 0
		}
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		returnValue.UserSubscriptionId = sub.Id
		returnValue.PreConsumed = targetQuota
		returnValue.AmountTotal = sub.AmountTotal
		returnValue.AmountUsedBefore = usedBefore
		returnValue.AmountUsedAfter = sub.AmountUsed
		returnValue.QuotaEpoch = requestEpoch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

func recordValuePackageUsageTx(tx *gorm.DB, record *ValuePackageUsageRecord) error {
	if tx == nil {
		return errors.New("db is nil")
	}
	if record == nil || record.UserId <= 0 || record.UserSubscriptionId <= 0 || record.Quota < 0 || strings.TrimSpace(record.RequestId) == "" {
		return errors.New("invalid value package usage record: requestId, userId and userSubscriptionId are required; quota must be non-negative")
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_subscription_id"}, {Name: "request_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id",
			"plan_id",
			"package_type",
			"model_group",
			"quota",
		}),
	}).Create(record).Error
}

func GetValuePackageWindowUsage(userId int, userSubscriptionId int, now int64) (int64, int64, error) {
	return getValuePackageWindowUsageTx(DB, userId, userSubscriptionId, now)
}

func getValuePackageWindowUsageTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (int64, int64, error) {
	details, err := getValuePackageWindowUsageDetailsTx(tx, userId, userSubscriptionId, now)
	if err != nil {
		return 0, 0, err
	}
	return details.Used5h, details.Used7d, nil
}

func GetValuePackageWindowUsageDetails(userId int, userSubscriptionId int, now int64) (*ValuePackageWindowUsageDetails, error) {
	return getValuePackageWindowUsageDetailsTx(DB, userId, userSubscriptionId, now)
}

func getLastValuePackageQuotaResetAtTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (int64, error) {
	if tx == nil {
		tx = DB
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	var resetAt int64
	err := tx.Model(&ValuePackageQuotaReset{}).
		Where("user_id = ? AND user_subscription_id = ? AND reset_at <= ?", userId, userSubscriptionId, now).
		Select("COALESCE(MAX(reset_at), 0)").
		Scan(&resetAt).Error
	return resetAt, err
}

func valuePackageSubscriptionAnchorStart(sub *UserSubscription, now int64) int64 {
	if sub == nil {
		return 0
	}
	if sub.StartTime > 0 {
		return sub.StartTime
	}
	if sub.CreatedAt > 0 && (now <= 0 || sub.CreatedAt <= now) {
		return sub.CreatedAt
	}
	return 0
}

type valuePackageAnchoredWindow struct {
	Start int64
	End   int64
}

func calcValuePackageAnchoredWindow(startTime int64, endTime int64, windowSeconds int64, now int64) valuePackageAnchoredWindow {
	if startTime <= 0 || windowSeconds <= 0 {
		return valuePackageAnchoredWindow{}
	}
	if now <= 0 || now < startTime {
		now = startTime
	}
	index := (now - startTime) / windowSeconds
	windowStart := startTime + index*windowSeconds
	windowEnd := windowStart + windowSeconds
	if endTime > 0 && windowEnd > endTime {
		windowEnd = endTime
	}
	if windowEnd <= windowStart {
		return valuePackageAnchoredWindow{}
	}
	return valuePackageAnchoredWindow{Start: windowStart, End: windowEnd}
}

func valuePackageHas7dWindow(plan *SubscriptionPlan) bool {
	return plan != nil && plan.IsValuePackage() && plan.PackageType == ValuePackageTypeMonth && plan.Limit7dAmount > 0
}

func valuePackageResetClears7d(plan *SubscriptionPlan) bool {
	return plan != nil && plan.IsValuePackage() && plan.PackageType == ValuePackageTypeMonth && plan.Limit7dAmount > 0
}

func getValuePackageWindowUsageDetailsTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (*ValuePackageWindowUsageDetails, error) {
	if tx == nil {
		tx = DB
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	var sub UserSubscription
	if err := tx.Where("id = ? AND user_id = ?", userSubscriptionId, userId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
	if err != nil {
		return nil, err
	}
	normalizeValuePackagePlan(plan)
	lastResetAt, err := getLastValuePackageQuotaResetAtTx(tx, userId, userSubscriptionId, now)
	if err != nil {
		return nil, err
	}
	lowerBound := valuePackageSubscriptionAnchorStart(&sub, now)
	if lowerBound <= 0 {
		lowerBound = now - valuePackage7dWindowSeconds
	}

	var usageRecords []ValuePackageUsageRecord
	if err := tx.Where("user_id = ? AND user_subscription_id = ? AND quota_epoch = ? AND created_at >= ? AND created_at <= ? AND quota > ?", userId, userSubscriptionId, sub.QuotaEpoch, lowerBound, now, 0).
		Order("created_at asc, id asc").
		Find(&usageRecords).Error; err != nil {
		return nil, err
	}
	return buildValuePackageWindowUsageDetailsFromRecords(&sub, plan, usageRecords, lastResetAt, now), nil
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":     "cancelled",
			"end_time":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
	QuotaEpoch         int64
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":     "expired",
					"updated_at": common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's an active upgraded subscription, keep current group.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// No active upgraded subscription, downgrade to previous group if needed.
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> ''",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userId, cacheGroup)
		}
	}
	return expiredCount, nil
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	QuotaEpoch         int64  `json:"quota_epoch" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTime(base, plan, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTime(base, plan, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

// PreConsumeUserSubscription pre-consumes from any active subscription total quota.
func PreConsumeUserSubscription(requestId string, userId int, modelName string, quotaType int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()

	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = existing.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			returnValue.QuotaEpoch = existing.QuotaEpoch
			return nil
		}

		var subs []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}
		for _, candidate := range subs {
			sub := candidate
			plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
			if err != nil {
				return err
			}
			normalizeValuePackagePlan(plan)
			if plan.IsValuePackage() {
				continue
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
				return err
			}
			usedBefore := sub.AmountUsed
			if sub.AmountTotal > 0 {
				remain := sub.AmountTotal - usedBefore
				if remain < amount {
					continue
				}
			}
			record := &SubscriptionPreConsumeRecord{
				RequestId:          requestId,
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				PreConsumed:        amount,
				Status:             "consumed",
			}
			if err := tx.Create(record).Error; err != nil {
				var dup SubscriptionPreConsumeRecord
				if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
					if dup.Status == "refunded" {
						return errors.New("subscription pre-consume already refunded")
					}
					returnValue.UserSubscriptionId = sub.Id
					returnValue.PreConsumed = dup.PreConsumed
					returnValue.AmountTotal = sub.AmountTotal
					returnValue.AmountUsedBefore = sub.AmountUsed
					returnValue.AmountUsedAfter = sub.AmountUsed
					returnValue.QuotaEpoch = existing.QuotaEpoch
					return nil
				}
				return err
			}
			sub.AmountUsed += amount
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = amount
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

func PreConsumeValuePackageSubscription(requestId string, userId int, userSubscriptionId int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()
	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			if existing.UserId != userId || existing.UserSubscriptionId != userSubscriptionId {
				return errors.New("subscription pre-consume request mismatch")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = existing.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			returnValue.QuotaEpoch = existing.QuotaEpoch
			return nil
		}

		var sub UserSubscription
		if err := withUpdateLock(tx).
			Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", userSubscriptionId, userId, UserSubscriptionStatusActive, now).
			First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("no active subscription")
			}
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("subscription is not value package")
		}
		if err := maybeAdvanceValuePackageCycleTx(tx, &sub, plan, now); err != nil {
			return err
		}
		usedBefore := sub.AmountUsed
		if sub.AmountTotal > 0 && sub.AmountTotal-usedBefore < amount {
			return fmt.Errorf("subscription quota insufficient: %s, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
		}
		usageDetails, err := getValuePackageWindowUsageDetailsTx(tx, userId, sub.Id, now)
		if err != nil {
			return err
		}
		if plan.Limit5hAmount > 0 && usageDetails.Used5h+amount > plan.Limit5hAmount {
			return fmt.Errorf("subscription quota insufficient: %s, 5h limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
		}
		if valuePackageHas7dWindow(plan) && usageDetails.Used7d+amount > plan.Limit7dAmount {
			return fmt.Errorf("subscription quota insufficient: %s, 7d period limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
		}

		record := &SubscriptionPreConsumeRecord{
			RequestId:          requestId,
			UserId:             userId,
			UserSubscriptionId: sub.Id,
			PreConsumed:        amount,
			QuotaEpoch:         sub.QuotaEpoch,
			Status:             "consumed",
		}
		createPreConsume := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "request_id"}}, DoNothing: true}).Create(record)
		if createPreConsume.Error != nil {
			return createPreConsume.Error
		}
		if createPreConsume.RowsAffected == 0 {
			var dup SubscriptionPreConsumeRecord
			if err := tx.Where("request_id = ?", requestId).First(&dup).Error; err != nil {
				return err
			}
			if dup.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			if dup.UserId != userId || dup.UserSubscriptionId != sub.Id {
				return errors.New("subscription pre-consume request mismatch")
			}
			returnValue.UserSubscriptionId = dup.UserSubscriptionId
			returnValue.PreConsumed = dup.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			returnValue.QuotaEpoch = dup.QuotaEpoch
			return nil
		}
		if err := recordValuePackageUsageTx(tx, &ValuePackageUsageRecord{
			UserId:             userId,
			UserSubscriptionId: sub.Id,
			PlanId:             plan.Id,
			PackageType:        plan.PackageType,
			ModelGroup:         plan.ModelGroup,
			RequestId:          requestId,
			Quota:              amount,
			QuotaEpoch:         sub.QuotaEpoch,
			CreatedAt:          now,
		}); err != nil {
			return err
		}
		sub.AmountUsed += amount
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		returnValue.UserSubscriptionId = sub.Id
		returnValue.PreConsumed = amount
		returnValue.AmountTotal = sub.AmountTotal
		returnValue.AmountUsedBefore = usedBefore
		returnValue.AmountUsedAfter = sub.AmountUsed
		returnValue.QuotaEpoch = sub.QuotaEpoch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := withUpdateLock(tx).
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return revokeValuePackageUsageReservationTx(tx, record.UserSubscriptionId, record.RequestId)
		}
		var sub UserSubscription
		if err := withUpdateLock(tx).Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if plan.IsValuePackage() {
			refundQuota := record.PreConsumed
			var usage ValuePackageUsageRecord
			usageQuery := tx.Where("user_subscription_id = ? AND request_id = ?", record.UserSubscriptionId, record.RequestId).Limit(1).Find(&usage)
			if usageQuery.Error != nil {
				return usageQuery.Error
			}
			if usageQuery.RowsAffected > 0 {
				if usage.QuotaEpoch != record.QuotaEpoch {
					return errors.New("value package usage epoch mismatch")
				}
				refundQuota = usage.Quota
			}
			if sub.QuotaEpoch == record.QuotaEpoch && refundQuota > 0 {
				sub.AmountUsed -= refundQuota
				if sub.AmountUsed < 0 {
					sub.AmountUsed = 0
				}
				if err := tx.Save(&sub).Error; err != nil {
					return err
				}
			}
		} else if record.PreConsumed > 0 {
			sub.AmountUsed -= record.PreConsumed
			if sub.AmountUsed < 0 {
				sub.AmountUsed = 0
			}
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}
		}
		if err := revokeValuePackageUsageReservationTx(tx, record.UserSubscriptionId, record.RequestId); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

func revokeValuePackageUsageReservationTx(tx *gorm.DB, userSubscriptionId int, requestId string) error {
	if tx == nil {
		return errors.New("db is nil")
	}
	if userSubscriptionId <= 0 || strings.TrimSpace(requestId) == "" {
		return nil
	}
	return tx.Model(&ValuePackageUsageRecord{}).
		Where("user_subscription_id = ? AND request_id = ?", userSubscriptionId, requestId).
		Update("quota", 0).Error
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := withUpdateLock(tx).
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			previousEpoch := locked.QuotaEpoch
			previousResetTime := locked.NextResetTime
			normalizeValuePackagePlan(plan)
			if plan.IsValuePackage() {
				if err := maybeAdvanceValuePackageCycleTx(tx, &locked, plan, now); err != nil {
					return err
				}
			} else {
				if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
					return err
				}
			}
			if locked.QuotaEpoch == previousEpoch && locked.NextResetTime == previousResetTime {
				return nil
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// Update subscription used amount by delta (positive consume more, negative refund).
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return postConsumeUserSubscriptionDeltaTx(tx, userSubscriptionId, delta)
	})
}

func postConsumeUserSubscriptionDeltaTx(tx *gorm.DB, userSubscriptionId int, delta int64) error {
	if tx == nil {
		return errors.New("db is nil")
	}
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	var sub UserSubscription
	if err := withUpdateLock(tx).
		Where("id = ?", userSubscriptionId).
		First(&sub).Error; err != nil {
		return err
	}
	newUsed := sub.AmountUsed + delta
	if newUsed < 0 {
		newUsed = 0
	}
	if sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
		return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
	}
	sub.AmountUsed = newUsed
	return tx.Save(&sub).Error
}
