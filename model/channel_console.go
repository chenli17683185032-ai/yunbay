package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	ChannelConsoleProviderOpenRouter = "openrouter"
	ChannelConsoleProviderOpenAI     = "openai"
	ChannelConsoleProviderAnthropic  = "anthropic"
	ChannelConsoleProviderGemini     = "gemini"
	ChannelConsoleProviderCustom     = "custom_openai_compatible"

	ChannelConsoleKindThirdPartyAPI = "third_party_api"
	ChannelConsoleKindOAuthCLI      = "oauth_cli"

	ChannelConsoleStatusHealthy   = "healthy"
	ChannelConsoleStatusWarning   = "warning"
	ChannelConsoleStatusFailed    = "failed"
	ChannelConsoleStatusDisabled  = "disabled"
	ChannelConsoleStatusUnchecked = "unchecked"

	ChannelConsolePriceStatusSynced  = "synced"
	ChannelConsolePriceStatusUnknown = "price_unknown"
	ChannelConsolePriceStatusStale   = "stale"
	ChannelConsolePriceStatusManual  = "manual"
)

var (
	errNilChannelConsoleChannel     = errors.New("channel console channel is nil")
	errNilChannelConsoleHealthCheck = errors.New("channel console health check is nil")
)

type ChannelConsoleChannel struct {
	Id                int            `json:"id" gorm:"primaryKey"`
	ChannelId         int            `json:"channel_id" gorm:"uniqueIndex;not null"`
	Provider          string         `json:"provider" gorm:"size:64;index;not null"`
	ProviderKind      string         `json:"provider_kind" gorm:"size:64;index;not null"`
	ImportKind        string         `json:"import_kind" gorm:"size:64;not null"`
	PriceSource       string         `json:"price_source" gorm:"size:64;index;not null"`
	HealthStatus      string         `json:"health_status" gorm:"size:32;index;not null;default:'unchecked'"`
	ModelSyncStatus   string         `json:"model_sync_status" gorm:"size:32;not null;default:'unchecked'"`
	PriceSyncStatus   string         `json:"price_sync_status" gorm:"size:32;not null;default:'unchecked'"`
	LastHealthCheckAt int64          `json:"last_health_check_at" gorm:"bigint;default:0"`
	LastModelSyncAt   int64          `json:"last_model_sync_at" gorm:"bigint;default:0"`
	LastPriceSyncAt   int64          `json:"last_price_sync_at" gorm:"bigint;default:0"`
	LastErrorCode     string         `json:"last_error_code" gorm:"size:128"`
	LastErrorMessage  string         `json:"last_error_message" gorm:"type:text"`
	Markup            float64        `json:"markup" gorm:"default:1.2"`
	AutoDisablePolicy string         `json:"auto_disable_policy" gorm:"size:64;default:'mark_only'"`
	CreatedAt         int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt         int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

type ChannelConsoleModelPrice struct {
	Id                         int      `json:"id" gorm:"primaryKey"`
	ChannelId                  int      `json:"channel_id" gorm:"uniqueIndex:idx_channel_console_model_price,priority:1;not null"`
	ModelName                  string   `json:"model_name" gorm:"size:255;uniqueIndex:idx_channel_console_model_price,priority:2;not null"`
	ProviderModelName          string   `json:"provider_model_name" gorm:"size:255;index"`
	Source                     string   `json:"source" gorm:"size:64;index;not null"`
	InputUSDPer1MTokens        *float64 `json:"input_usd_per_1m_tokens" gorm:"column:input_usd_per_1m_tokens"`
	OutputUSDPer1MTokens       *float64 `json:"output_usd_per_1m_tokens" gorm:"column:output_usd_per_1m_tokens"`
	CachedInputUSDPer1MTokens  *float64 `json:"cached_input_usd_per_1m_tokens" gorm:"column:cached_input_usd_per_1m_tokens"`
	CacheWrite5mUSDPer1MTokens *float64 `json:"cache_write_5m_usd_per_1m_tokens" gorm:"column:cache_write_5m_usd_per_1m_tokens"`
	CacheWrite1hUSDPer1MTokens *float64 `json:"cache_write_1h_usd_per_1m_tokens" gorm:"column:cache_write_1h_usd_per_1m_tokens"`
	RequestUSDPerCall          *float64 `json:"request_usd_per_call"`
	ImageUSDPerUnit            *float64 `json:"image_usd_per_unit"`
	CompiledModelRatio         *float64 `json:"compiled_model_ratio"`
	CompiledCompletionRatio    *float64 `json:"compiled_completion_ratio"`
	CompiledCacheRatio         *float64 `json:"compiled_cache_ratio"`
	CompiledCreateCacheRatio   *float64 `json:"compiled_create_cache_ratio"`
	CompiledModelPrice         *float64 `json:"compiled_model_price"`
	ManualOverride             bool     `json:"manual_override" gorm:"default:false"`
	Enabled                    bool     `json:"enabled" gorm:"default:false"`
	PriceStatus                string   `json:"price_status" gorm:"size:32;index;not null;default:'price_unknown'"`
	SourceUpdatedAt            int64    `json:"source_updated_at" gorm:"bigint;default:0"`
	SyncedAt                   int64    `json:"synced_at" gorm:"bigint;default:0"`
	CreatedAt                  int64    `json:"created_at" gorm:"bigint"`
	UpdatedAt                  int64    `json:"updated_at" gorm:"bigint"`
}

type ChannelConsoleHealthCheck struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ChannelId      int    `json:"channel_id" gorm:"index;not null"`
	KeyIndex       *int   `json:"key_index" gorm:"index"`
	ModelName      string `json:"model_name" gorm:"size:255;index"`
	CheckType      string `json:"check_type" gorm:"size:64;index;not null"`
	Status         string `json:"status" gorm:"size:32;index;not null"`
	ResponseTimeMs int    `json:"response_time_ms" gorm:"default:0"`
	ErrorCode      string `json:"error_code" gorm:"size:128"`
	ErrorMessage   string `json:"error_message" gorm:"type:text"`
	CheckedAt      int64  `json:"checked_at" gorm:"bigint;index;not null"`
}

func (c *ChannelConsoleChannel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	c.CreatedAt = now
	c.UpdatedAt = now
	return nil
}

func (c *ChannelConsoleChannel) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = time.Now().Unix()
	return nil
}

func (p *ChannelConsoleModelPrice) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *ChannelConsoleModelPrice) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now().Unix()
	return nil
}

func (c *ChannelConsoleHealthCheck) BeforeCreate(tx *gorm.DB) error {
	if c.CheckedAt == 0 {
		c.CheckedAt = time.Now().Unix()
	}
	return nil
}

func UpsertChannelConsoleChannel(meta *ChannelConsoleChannel) error {
	if meta == nil {
		return errNilChannelConsoleChannel
	}

	var existing ChannelConsoleChannel
	err := DB.Unscoped().Where("channel_id = ?", meta.ChannelId).First(&existing).Error
	if err == nil {
		merged := mergeChannelConsoleChannel(existing, *meta)
		if err := DB.Unscoped().Model(&ChannelConsoleChannel{}).
			Where("id = ?", existing.Id).
			Updates(channelConsoleChannelUpdateMap(merged)).Error; err != nil {
			return err
		}
		meta.Id = existing.Id
		meta.CreatedAt = existing.CreatedAt
		meta.UpdatedAt = merged.UpdatedAt
		meta.DeletedAt = gorm.DeletedAt{}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	normalizeNewChannelConsoleChannel(meta)
	return DB.Create(meta).Error
}

func GetChannelConsoleChannelByChannelID(channelID int) (*ChannelConsoleChannel, error) {
	var meta ChannelConsoleChannel
	if err := DB.Where("channel_id = ?", channelID).First(&meta).Error; err != nil {
		return nil, err
	}
	return &meta, nil
}

func SaveChannelConsoleModelPrices(channelID int, prices []ChannelConsoleModelPrice) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		for i := range prices {
			price := prices[i]

			var existing ChannelConsoleModelPrice
			err := tx.Where("channel_id = ? AND model_name = ?", channelID, price.ModelName).First(&existing).Error
			if err == nil {
				if existing.ManualOverride {
					continue
				}
				merged := mergeChannelConsoleModelPrice(existing, price, now)
				if err := tx.Model(&ChannelConsoleModelPrice{}).
					Where("id = ?", existing.Id).
					Select(channelConsoleModelPriceUpdateColumns()).
					Updates(&merged).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			normalizeNewChannelConsoleModelPrice(&price, channelID, now)
			if err := tx.Create(&price).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListChannelConsoleModelPrices(channelID int) ([]ChannelConsoleModelPrice, error) {
	var prices []ChannelConsoleModelPrice
	err := DB.Where("channel_id = ?", channelID).Order("model_name ASC").Find(&prices).Error
	return prices, err
}

func CreateChannelConsoleHealthCheck(check *ChannelConsoleHealthCheck) error {
	if check == nil {
		return errNilChannelConsoleHealthCheck
	}
	return DB.Create(check).Error
}

func ListChannelConsoleHealthChecks(channelID int, limit int) ([]ChannelConsoleHealthCheck, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	var checks []ChannelConsoleHealthCheck
	err := DB.Where("channel_id = ?", channelID).Order("checked_at DESC, id DESC").Limit(limit).Find(&checks).Error
	return checks, err
}

func normalizeNewChannelConsoleChannel(meta *ChannelConsoleChannel) {
	if meta.HealthStatus == "" {
		meta.HealthStatus = ChannelConsoleStatusUnchecked
	}
	if meta.ModelSyncStatus == "" {
		meta.ModelSyncStatus = ChannelConsoleStatusUnchecked
	}
	if meta.PriceSyncStatus == "" {
		meta.PriceSyncStatus = ChannelConsoleStatusUnchecked
	}
	if meta.Markup == 0 {
		meta.Markup = 1.2
	}
	if meta.AutoDisablePolicy == "" {
		meta.AutoDisablePolicy = "mark_only"
	}
}

func mergeChannelConsoleChannel(existing ChannelConsoleChannel, incoming ChannelConsoleChannel) ChannelConsoleChannel {
	merged := existing
	if incoming.Provider != "" {
		merged.Provider = incoming.Provider
	}
	if incoming.ProviderKind != "" {
		merged.ProviderKind = incoming.ProviderKind
	}
	if incoming.ImportKind != "" {
		merged.ImportKind = incoming.ImportKind
	}
	if incoming.PriceSource != "" {
		merged.PriceSource = incoming.PriceSource
	}
	if incoming.HealthStatus != "" {
		merged.HealthStatus = incoming.HealthStatus
	}
	if incoming.ModelSyncStatus != "" {
		merged.ModelSyncStatus = incoming.ModelSyncStatus
	}
	if incoming.PriceSyncStatus != "" {
		merged.PriceSyncStatus = incoming.PriceSyncStatus
	}
	if incoming.LastHealthCheckAt != 0 {
		merged.LastHealthCheckAt = incoming.LastHealthCheckAt
	}
	if incoming.LastModelSyncAt != 0 {
		merged.LastModelSyncAt = incoming.LastModelSyncAt
	}
	if incoming.LastPriceSyncAt != 0 {
		merged.LastPriceSyncAt = incoming.LastPriceSyncAt
	}
	if incoming.LastErrorCode != "" {
		merged.LastErrorCode = incoming.LastErrorCode
	}
	if incoming.LastErrorMessage != "" {
		merged.LastErrorMessage = incoming.LastErrorMessage
	}
	if incoming.Markup != 0 {
		merged.Markup = incoming.Markup
	}
	if incoming.AutoDisablePolicy != "" {
		merged.AutoDisablePolicy = incoming.AutoDisablePolicy
	}
	normalizeNewChannelConsoleChannel(&merged)
	merged.DeletedAt = gorm.DeletedAt{}
	merged.UpdatedAt = time.Now().Unix()
	return merged
}

func channelConsoleChannelUpdateMap(meta ChannelConsoleChannel) map[string]interface{} {
	return map[string]interface{}{
		"provider":             meta.Provider,
		"provider_kind":        meta.ProviderKind,
		"import_kind":          meta.ImportKind,
		"price_source":         meta.PriceSource,
		"health_status":        meta.HealthStatus,
		"model_sync_status":    meta.ModelSyncStatus,
		"price_sync_status":    meta.PriceSyncStatus,
		"last_health_check_at": meta.LastHealthCheckAt,
		"last_model_sync_at":   meta.LastModelSyncAt,
		"last_price_sync_at":   meta.LastPriceSyncAt,
		"last_error_code":      meta.LastErrorCode,
		"last_error_message":   meta.LastErrorMessage,
		"markup":               meta.Markup,
		"auto_disable_policy":  meta.AutoDisablePolicy,
		"updated_at":           meta.UpdatedAt,
		"deleted_at":           nil,
	}
}

func normalizeNewChannelConsoleModelPrice(price *ChannelConsoleModelPrice, channelID int, now int64) {
	price.ChannelId = channelID
	if price.PriceStatus == "" {
		price.PriceStatus = ChannelConsolePriceStatusUnknown
	}
	if price.SyncedAt == 0 {
		price.SyncedAt = now
	}
}

func mergeChannelConsoleModelPrice(existing ChannelConsoleModelPrice, incoming ChannelConsoleModelPrice, now int64) ChannelConsoleModelPrice {
	merged := existing
	if incoming.ProviderModelName != "" {
		merged.ProviderModelName = incoming.ProviderModelName
	}
	if incoming.Source != "" {
		merged.Source = incoming.Source
	}
	if incoming.InputUSDPer1MTokens != nil {
		merged.InputUSDPer1MTokens = incoming.InputUSDPer1MTokens
	}
	if incoming.OutputUSDPer1MTokens != nil {
		merged.OutputUSDPer1MTokens = incoming.OutputUSDPer1MTokens
	}
	if incoming.CachedInputUSDPer1MTokens != nil {
		merged.CachedInputUSDPer1MTokens = incoming.CachedInputUSDPer1MTokens
	}
	if incoming.CacheWrite5mUSDPer1MTokens != nil {
		merged.CacheWrite5mUSDPer1MTokens = incoming.CacheWrite5mUSDPer1MTokens
	}
	if incoming.CacheWrite1hUSDPer1MTokens != nil {
		merged.CacheWrite1hUSDPer1MTokens = incoming.CacheWrite1hUSDPer1MTokens
	}
	if incoming.RequestUSDPerCall != nil {
		merged.RequestUSDPerCall = incoming.RequestUSDPerCall
	}
	if incoming.ImageUSDPerUnit != nil {
		merged.ImageUSDPerUnit = incoming.ImageUSDPerUnit
	}
	if incoming.CompiledModelRatio != nil {
		merged.CompiledModelRatio = incoming.CompiledModelRatio
	}
	if incoming.CompiledCompletionRatio != nil {
		merged.CompiledCompletionRatio = incoming.CompiledCompletionRatio
	}
	if incoming.CompiledCacheRatio != nil {
		merged.CompiledCacheRatio = incoming.CompiledCacheRatio
	}
	if incoming.CompiledCreateCacheRatio != nil {
		merged.CompiledCreateCacheRatio = incoming.CompiledCreateCacheRatio
	}
	if incoming.CompiledModelPrice != nil {
		merged.CompiledModelPrice = incoming.CompiledModelPrice
	}
	if incoming.PriceStatus != "" {
		merged.PriceStatus = incoming.PriceStatus
	} else if merged.PriceStatus == "" {
		merged.PriceStatus = ChannelConsolePriceStatusUnknown
	}
	if incoming.SourceUpdatedAt != 0 {
		merged.SourceUpdatedAt = incoming.SourceUpdatedAt
	}
	if incoming.SyncedAt != 0 {
		merged.SyncedAt = incoming.SyncedAt
	} else {
		merged.SyncedAt = now
	}
	merged.UpdatedAt = now
	return merged
}

func channelConsoleModelPriceUpdateColumns() []string {
	return []string{
		"ProviderModelName",
		"Source",
		"InputUSDPer1MTokens",
		"OutputUSDPer1MTokens",
		"CachedInputUSDPer1MTokens",
		"CacheWrite5mUSDPer1MTokens",
		"CacheWrite1hUSDPer1MTokens",
		"RequestUSDPerCall",
		"ImageUSDPerUnit",
		"CompiledModelRatio",
		"CompiledCompletionRatio",
		"CompiledCacheRatio",
		"CompiledCreateCacheRatio",
		"CompiledModelPrice",
		"ManualOverride",
		"Enabled",
		"PriceStatus",
		"SourceUpdatedAt",
		"SyncedAt",
		"UpdatedAt",
	}
}
