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
	ChannelId                  int      `json:"channel_id" gorm:"index;not null"`
	ModelName                  string   `json:"model_name" gorm:"size:255;index;not null"`
	ProviderModelName          string   `json:"provider_model_name" gorm:"size:255;index"`
	Source                     string   `json:"source" gorm:"size:64;index;not null"`
	InputUSDPer1MTokens        *float64 `json:"input_usd_per_1m_tokens"`
	OutputUSDPer1MTokens       *float64 `json:"output_usd_per_1m_tokens"`
	CachedInputUSDPer1MTokens  *float64 `json:"cached_input_usd_per_1m_tokens"`
	CacheWrite5mUSDPer1MTokens *float64 `json:"cache_write_5m_usd_per_1m_tokens"`
	CacheWrite1hUSDPer1MTokens *float64 `json:"cache_write_1h_usd_per_1m_tokens"`
	RequestUSDPerCall          *float64 `json:"request_usd_per_call"`
	ImageUSDPerUnit            *float64 `json:"image_usd_per_unit"`
	CompiledModelRatio         *float64 `json:"compiled_model_ratio"`
	CompiledCompletionRatio    *float64 `json:"compiled_completion_ratio"`
	CompiledCacheRatio         *float64 `json:"compiled_cache_ratio"`
	CompiledCreateCacheRatio   *float64 `json:"compiled_create_cache_ratio"`
	CompiledModelPrice         *float64 `json:"compiled_model_price"`
	ManualOverride             bool     `json:"manual_override" gorm:"default:false"`
	Enabled                    bool     `json:"enabled" gorm:"default:false"`
	PriceStatus                string   `json:"price_status" gorm:"size:32;index;not null"`
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

func UpsertChannelConsoleChannel(meta *ChannelConsoleChannel) error {
	if meta == nil {
		return nil
	}

	var existing ChannelConsoleChannel
	err := DB.Where("channel_id = ?", meta.ChannelId).First(&existing).Error
	if err == nil {
		meta.Id = existing.Id
		meta.CreatedAt = existing.CreatedAt
		return DB.Save(meta).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
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
		for i := range prices {
			price := prices[i]
			price.ChannelId = channelID

			var existing ChannelConsoleModelPrice
			err := tx.Where("channel_id = ? AND model_name = ?", channelID, price.ModelName).First(&existing).Error
			if err == nil {
				if existing.ManualOverride {
					continue
				}
				price.Id = existing.Id
				price.CreatedAt = existing.CreatedAt
				if err := tx.Save(&price).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
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
		return nil
	}
	if check.CheckedAt == 0 {
		check.CheckedAt = time.Now().Unix()
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
