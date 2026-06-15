package channelconsole

import (
	"errors"
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
