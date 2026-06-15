package channelconsole

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	defaultCommitGroup       = "default"
	defaultCommitMarkup      = 1.2
	channelConsoleTag        = "yunbay-console"
	defaultAutoDisablePolicy = "mark_only"
)

var errImportCredentialMissing = errors.New(noKeyWarning)

var (
	ErrChannelConsoleMetadataNotFound = errors.New("渠道控制台元数据不存在")
	ErrChannelConsoleChannelNotFound  = errors.New("渠道不存在")
)

type ManagedChannelSummary struct {
	Id                 int               `json:"id"`
	Type               int               `json:"type"`
	TestModel          *string           `json:"test_model,omitempty"`
	Status             int               `json:"status"`
	Name               string            `json:"name"`
	Weight             *uint             `json:"weight,omitempty"`
	CreatedTime        int64             `json:"created_time"`
	TestTime           int64             `json:"test_time"`
	ResponseTime       int               `json:"response_time"`
	BaseURL            *string           `json:"base_url,omitempty"`
	Balance            float64           `json:"balance"`
	BalanceUpdatedTime int64             `json:"balance_updated_time"`
	Models             string            `json:"models"`
	Group              string            `json:"group"`
	UsedQuota          int64             `json:"used_quota"`
	Priority           *int64            `json:"priority,omitempty"`
	AutoBan            *int              `json:"auto_ban,omitempty"`
	Tag                *string           `json:"tag,omitempty"`
	Remark             *string           `json:"remark,omitempty"`
	ChannelInfo        model.ChannelInfo `json:"channel_info"`
}

type ManagedChannelListItem struct {
	Channel ManagedChannelSummary        `json:"channel"`
	Console *model.ChannelConsoleChannel `json:"console"`
}

type ManagedChannelListResult struct {
	Items    []ManagedChannelListItem `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type ManagedChannelDetail struct {
	Channel      ManagedChannelSummary             `json:"channel"`
	Console      *model.ChannelConsoleChannel      `json:"console"`
	Prices       []model.ChannelConsoleModelPrice  `json:"prices"`
	HealthChecks []model.ChannelConsoleHealthCheck `json:"health_checks"`
}

type ManagedChannelBatchDeleteRequest struct {
	IDs []int `json:"ids"`
}

type ManagedChannelBatchDeleteResult struct {
	Requested  int   `json:"requested"`
	Deleted    int   `json:"deleted"`
	SkippedIDs []int `json:"skipped_ids"`
}

func CommitImport(req ImportCommitRequest) (*ImportCommitResult, error) {
	preview := PreviewImport(req.RawInput)
	if len(preview.Keys) == 0 {
		return nil, errImportCredentialMissing
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = preview.SuggestedName
	}

	group := strings.TrimSpace(req.Group)
	if group == "" {
		group = defaultCommitGroup
	}

	markup := req.Markup
	if markup <= 0 {
		markup = defaultCommitMarkup
	}

	multiKeyMode := normalizeMultiKeyMode(req.MultiKeyMode)
	models := normalizeCommitModels(req.Models, preview.DefaultTestModel)
	modelsValue := strings.Join(models, ",")
	keysValue := strings.Join(preview.Keys, "\n")
	baseURL := preview.BaseURL
	testModel := preview.DefaultTestModel
	tag := channelConsoleTag

	channel := &model.Channel{
		Type:        preview.ChannelType,
		Key:         keysValue,
		Name:        name,
		Status:      common.ChannelStatusEnabled,
		CreatedTime: common.GetTimestamp(),
		BaseURL:     &baseURL,
		Models:      modelsValue,
		Group:       group,
		TestModel:   &testModel,
		Tag:         &tag,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   len(preview.Keys) > 1,
			MultiKeySize: len(preview.Keys),
			MultiKeyMode: multiKeyMode,
		},
	}

	meta := &model.ChannelConsoleChannel{
		Provider:          preview.Provider,
		ProviderKind:      model.ChannelConsoleKindThirdPartyAPI,
		ImportKind:        preview.ImportKind,
		PriceSource:       preview.PriceSource,
		HealthStatus:      model.ChannelConsoleStatusUnchecked,
		ModelSyncStatus:   model.ChannelConsoleStatusUnchecked,
		PriceSyncStatus:   model.ChannelConsoleStatusUnchecked,
		Markup:            markup,
		AutoDisablePolicy: defaultAutoDisablePolicy,
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if err := channel.AddAbilities(tx); err != nil {
			return err
		}
		meta.ChannelId = channel.Id
		if err := tx.Create(meta).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &ImportCommitResult{
		ChannelID:    channel.Id,
		Name:         name,
		Provider:     preview.Provider,
		KeyCount:     len(preview.Keys),
		ModelCount:   len(models),
		HealthStatus: meta.HealthStatus,
		PriceStatus:  meta.PriceSyncStatus,
	}, nil
}

func ListManagedChannels(startIdx int, pageSize int, page int) (*ManagedChannelListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var total int64
	if err := model.DB.Model(&model.ChannelConsoleChannel{}).Count(&total).Error; err != nil {
		return nil, err
	}

	metas := make([]model.ChannelConsoleChannel, 0, pageSize)
	if err := model.DB.
		Order("updated_at DESC, id DESC").
		Limit(pageSize).
		Offset(startIdx).
		Find(&metas).Error; err != nil {
		return nil, err
	}

	channelIDs := make([]int, 0, len(metas))
	for _, meta := range metas {
		channelIDs = append(channelIDs, meta.ChannelId)
	}

	summaries, err := loadManagedChannelSummaries(channelIDs)
	if err != nil {
		return nil, err
	}

	items := make([]ManagedChannelListItem, 0, len(metas))
	for i := range metas {
		meta := &metas[i]
		summary, ok := summaries[meta.ChannelId]
		if !ok {
			summary = ManagedChannelSummary{Id: meta.ChannelId}
		}
		items = append(items, ManagedChannelListItem{
			Channel: summary,
			Console: meta,
		})
	}

	return &ManagedChannelListResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func GetManagedChannelDetail(channelID int) (*ManagedChannelDetail, error) {
	meta, err := model.GetChannelConsoleChannelByChannelID(channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelConsoleMetadataNotFound
		}
		return nil, err
	}

	channel, err := loadManagedChannel(channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelConsoleChannelNotFound
		}
		return nil, err
	}

	prices, err := model.ListChannelConsoleModelPrices(channelID)
	if err != nil {
		return nil, err
	}

	healthChecks, err := model.ListChannelConsoleHealthChecks(channelID, 50)
	if err != nil {
		return nil, err
	}

	return &ManagedChannelDetail{
		Channel:      managedChannelSummary(channel),
		Console:      meta,
		Prices:       prices,
		HealthChecks: healthChecks,
	}, nil
}

func BatchDeleteManagedChannels(ids []int) (*ManagedChannelBatchDeleteResult, error) {
	ids = uniquePositiveIDs(ids)
	result := &ManagedChannelBatchDeleteResult{Requested: len(ids)}
	if len(ids) == 0 {
		return result, nil
	}

	var consoleIDs []int
	if err := model.DB.
		Model(&model.ChannelConsoleChannel{}).
		Where("channel_id IN ?", ids).
		Pluck("channel_id", &consoleIDs).Error; err != nil {
		return nil, err
	}
	sort.Ints(consoleIDs)

	consoleIDSet := make(map[int]struct{}, len(consoleIDs))
	for _, id := range consoleIDs {
		consoleIDSet[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := consoleIDSet[id]; !ok {
			result.SkippedIDs = append(result.SkippedIDs, id)
		}
	}
	if len(consoleIDs) == 0 {
		return result, nil
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id IN ?", consoleIDs).Delete(&model.ChannelConsoleHealthCheck{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id IN ?", consoleIDs).Delete(&model.ChannelConsoleModelPrice{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id IN ?", consoleIDs).Delete(&model.ChannelConsoleChannel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id IN ?", consoleIDs).Delete(&model.Ability{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", consoleIDs).Delete(&model.Channel{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	result.Deleted = len(consoleIDs)
	return result, nil
}

func normalizeMultiKeyMode(mode string) constant.MultiKeyMode {
	switch constant.MultiKeyMode(strings.TrimSpace(mode)) {
	case constant.MultiKeyModeRandom:
		return constant.MultiKeyModeRandom
	default:
		return constant.MultiKeyModePolling
	}
}

func normalizeCommitModels(models []string, defaultModel string) []string {
	normalized := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		normalized = append(normalized, modelName)
	}
	if len(normalized) == 0 && strings.TrimSpace(defaultModel) != "" {
		normalized = append(normalized, strings.TrimSpace(defaultModel))
	}
	return normalized
}

func uniquePositiveIDs(ids []int) []int {
	unique := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func loadManagedChannel(channelID int) (*model.Channel, error) {
	channel := &model.Channel{}
	if err := model.DB.
		Omit(managedChannelSensitiveColumns()...).
		First(channel, "id = ?", channelID).Error; err != nil {
		return nil, err
	}
	return channel, nil
}

func loadManagedChannelSummaries(channelIDs []int) (map[int]ManagedChannelSummary, error) {
	summaries := make(map[int]ManagedChannelSummary, len(channelIDs))
	if len(channelIDs) == 0 {
		return summaries, nil
	}

	channels := make([]model.Channel, 0, len(channelIDs))
	if err := model.DB.
		Omit(managedChannelSensitiveColumns()...).
		Where("id IN ?", channelIDs).
		Find(&channels).Error; err != nil {
		return nil, err
	}

	for i := range channels {
		summary := managedChannelSummary(&channels[i])
		summaries[summary.Id] = summary
	}
	return summaries, nil
}

func managedChannelSensitiveColumns() []string {
	return []string{
		"key",
		"openai_organization",
		"other",
		"other_info",
		"model_mapping",
		"status_code_mapping",
		"setting",
		"param_override",
		"header_override",
		"settings",
	}
}

func managedChannelSummary(channel *model.Channel) ManagedChannelSummary {
	if channel == nil {
		return ManagedChannelSummary{}
	}
	return ManagedChannelSummary{
		Id:                 channel.Id,
		Type:               channel.Type,
		TestModel:          channel.TestModel,
		Status:             channel.Status,
		Name:               channel.Name,
		Weight:             channel.Weight,
		CreatedTime:        channel.CreatedTime,
		TestTime:           channel.TestTime,
		ResponseTime:       channel.ResponseTime,
		BaseURL:            channel.BaseURL,
		Balance:            channel.Balance,
		BalanceUpdatedTime: channel.BalanceUpdatedTime,
		Models:             channel.Models,
		Group:              channel.Group,
		UsedQuota:          channel.UsedQuota,
		Priority:           channel.Priority,
		AutoBan:            channel.AutoBan,
		Tag:                channel.Tag,
		Remark:             channel.Remark,
		ChannelInfo:        channel.ChannelInfo,
	}
}
