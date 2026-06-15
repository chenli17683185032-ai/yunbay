package channelconsole

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	CredentialKindAPIKey       = "api_key"
	CredentialKindCliProxyAuth = "cliproxy_auth"

	defaultCredentialPoolPageSize = 100
	modelDiscoveryTimeout         = 20 * time.Second
)

type CreateCredentialPoolRequest struct {
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	ProviderKind string  `json:"provider_kind"`
	BaseURL      string  `json:"base_url"`
	Markup       float64 `json:"markup"`
}

type AddThirdPartyCredentialRequest struct {
	APIKey      string   `json:"api_key"`
	DisplayName string   `json:"display_name"`
	Models      []string `json:"models"`
}

type AddCliProxyCredentialRequest struct {
	Name          string `json:"name"`
	RawCredential string `json:"raw_credential"`
}

type CredentialBatchDeleteRequest struct {
	IDs []int `json:"ids"`
}

type CredentialBatchDeleteResult struct {
	Deleted int   `json:"deleted"`
	Failed  []int `json:"failed"`
}

type CredentialPoolListResult struct {
	Items []model.ChannelConsolePool `json:"items"`
	Total int64                      `json:"total"`
}

type CredentialPoolDetail struct {
	Pool        model.ChannelConsolePool         `json:"pool"`
	Credentials []model.ChannelConsoleCredential `json:"credentials"`
}

func CreateCredentialPool(req CreateCredentialPoolRequest) (*model.ChannelConsolePool, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("请填写渠道名称")
	}

	providerKind := strings.TrimSpace(req.ProviderKind)
	if providerKind == "" {
		providerKind = model.ChannelConsoleKindThirdPartyAPI
	}
	if providerKind != model.ChannelConsoleKindThirdPartyAPI && providerKind != model.ChannelConsoleKindOAuthCLI {
		return nil, errors.New("不支持的渠道类型")
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	if providerKind == model.ChannelConsoleKindThirdPartyAPI && baseURL == "" {
		return nil, errors.New("请填写 API URL")
	}
	if baseURL != "" {
		baseURL = normalizeBaseURL(baseURL)
	}

	defaults := detectProvider(baseURL, baseURL, nil)
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = defaults.provider
	}
	priceSource := defaults.priceSource
	if providerKind == model.ChannelConsoleKindOAuthCLI {
		priceSource = PriceSourceManual
	}
	markup := req.Markup
	if markup <= 0 {
		markup = defaultCommitMarkup
	}

	pool := &model.ChannelConsolePool{
		Name:            name,
		Provider:        provider,
		ProviderKind:    providerKind,
		BaseURL:         baseURL,
		PriceSource:     priceSource,
		HealthStatus:    model.ChannelConsoleStatusUnchecked,
		ModelSyncStatus: model.ChannelConsoleStatusUnchecked,
		PriceSyncStatus: model.ChannelConsolePriceStatusUnknown,
		Markup:          markup,
	}
	if err := model.DB.Create(pool).Error; err != nil {
		return nil, err
	}
	return pool, nil
}

func ListCredentialPools() (*CredentialPoolListResult, error) {
	var total int64
	if err := model.DB.Model(&model.ChannelConsolePool{}).Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]model.ChannelConsolePool, 0, defaultCredentialPoolPageSize)
	if err := model.DB.Order("updated_at DESC, id DESC").Limit(defaultCredentialPoolPageSize).Find(&items).Error; err != nil {
		return nil, err
	}

	return &CredentialPoolListResult{Items: items, Total: total}, nil
}

func GetCredentialPoolDetail(poolID int) (*CredentialPoolDetail, error) {
	pool, err := getCredentialPool(poolID)
	if err != nil {
		return nil, err
	}

	credentials := make([]model.ChannelConsoleCredential, 0)
	if err := model.DB.
		Where("pool_id = ?", poolID).
		Order("updated_at DESC, id DESC").
		Find(&credentials).Error; err != nil {
		return nil, err
	}

	return &CredentialPoolDetail{
		Pool:        *pool,
		Credentials: credentials,
	}, nil
}

func AddThirdPartyCredentialToPool(ctx context.Context, poolID int, req AddThirdPartyCredentialRequest) (*model.ChannelConsoleCredential, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return nil, errors.New("请填写 API Key")
	}

	pool, err := getCredentialPool(poolID)
	if err != nil {
		return nil, err
	}
	if pool.ProviderKind != model.ChannelConsoleKindThirdPartyAPI {
		return nil, errors.New("当前渠道不是第三方 API 渠道")
	}
	if strings.TrimSpace(pool.BaseURL) == "" {
		return nil, errors.New("当前渠道缺少 API URL")
	}

	models := normalizeCommitModels(req.Models, "")
	status := model.ChannelConsoleStatusHealthy
	statusMessage := ""
	if len(models) == 0 {
		discovered, discoveryErr := discoverOpenAICompatibleModels(ctx, pool.BaseURL, apiKey)
		if discoveryErr != nil {
			status = model.ChannelConsoleStatusFailed
			statusMessage = discoveryErr.Error()
		} else {
			models = discovered
		}
	}
	if len(models) == 0 && strings.TrimSpace(pool.DefaultTestModel) != "" {
		models = []string{pool.DefaultTestModel}
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = MaskCredential(apiKey)
	}
	now := common.GetTimestamp()
	credential := &model.ChannelConsoleCredential{
		PoolID:              poolID,
		CredentialKind:      CredentialKindAPIKey,
		DisplayName:         displayName,
		Credential:          apiKey,
		Status:              status,
		StatusMessage:       statusMessage,
		LastModelSyncAt:     now,
		LastHealthCheckAt:   now,
		LastSuccessfulModel: firstString(models),
	}
	if status == model.ChannelConsoleStatusHealthy {
		credential.SuccessCount = 1
	} else {
		credential.FailureCount = 1
		credential.LastErrorCode = "model_discovery_failed"
		credential.LastErrorMessage = statusMessage
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if len(models) > 0 {
			pool.Models = strings.Join(models, ",")
			pool.DefaultTestModel = firstString(models)
			pool.ModelSyncStatus = model.ChannelConsoleStatusHealthy
			pool.LastModelSyncAt = now
			pool.HealthStatus = status
			pool.LastErrorCode = credential.LastErrorCode
			pool.LastErrorMessage = credential.LastErrorMessage
			if err := tx.Model(&model.ChannelConsolePool{}).
				Where("id = ?", pool.Id).
				Select("models", "default_test_model", "model_sync_status", "last_model_sync_at", "health_status", "last_error_code", "last_error_message").
				Updates(pool).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&model.ChannelConsolePool{}).
				Where("id = ?", pool.Id).
				Updates(map[string]interface{}{
					"health_status":      status,
					"last_error_code":    credential.LastErrorCode,
					"last_error_message": credential.LastErrorMessage,
				}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(credential).Error; err != nil {
			return err
		}
		return syncThirdPartyPoolToNewAPIChannel(tx, pool.Id)
	}); err != nil {
		return nil, err
	}
	model.InitChannelCache()

	return credential, nil
}

func AddCliProxyCredentialToPool(ctx context.Context, poolID int, req AddCliProxyCredentialRequest) (*model.ChannelConsoleCredential, error) {
	pool, err := getCredentialPool(poolID)
	if err != nil {
		return nil, err
	}
	if pool.ProviderKind != model.ChannelConsoleKindOAuthCLI {
		return nil, errors.New("当前渠道不是 OAuth/CliProxy 渠道")
	}

	name := sanitizeCliProxyAuthFileName(req.Name)
	if err := UploadCliProxyAuthFile(ctx, CliProxyUploadAuthFileRequest{Name: name, RawCredential: req.RawCredential}); err != nil {
		return nil, err
	}

	credential := &model.ChannelConsoleCredential{
		PoolID:           poolID,
		CredentialKind:   CredentialKindCliProxyAuth,
		DisplayName:      name,
		CliProxyAuthFile: name,
		Status:           model.ChannelConsoleStatusUnchecked,
	}
	if err := model.DB.Create(credential).Error; err != nil {
		return nil, err
	}
	return credential, nil
}

func BatchDeleteCredentials(ids []int) (*CredentialBatchDeleteResult, error) {
	ids = uniquePositiveIDs(ids)
	result := &CredentialBatchDeleteResult{}
	if len(ids) == 0 {
		return result, nil
	}

	var credentials []model.ChannelConsoleCredential
	if err := model.DB.Where("id IN ?", ids).Find(&credentials).Error; err != nil {
		return nil, err
	}

	foundIDs := make(map[int]struct{}, len(credentials))
	affectedPoolIDs := make(map[int]struct{}, len(credentials))
	cliproxyNames := make([]string, 0)
	for _, credential := range credentials {
		foundIDs[credential.Id] = struct{}{}
		affectedPoolIDs[credential.PoolID] = struct{}{}
		if credential.CredentialKind == CredentialKindCliProxyAuth && credential.CliProxyAuthFile != "" {
			cliproxyNames = append(cliproxyNames, credential.CliProxyAuthFile)
		}
	}
	for _, id := range ids {
		if _, ok := foundIDs[id]; !ok {
			result.Failed = append(result.Failed, id)
		}
	}

	if len(cliproxyNames) > 0 {
		deleteResult, _ := DeleteCliProxyAuthFiles(context.Background(), cliproxyNames)
		for _, failedName := range deleteResult.Failed {
			for _, credential := range credentials {
				if credential.CliProxyAuthFile == failedName {
					result.Failed = append(result.Failed, credential.Id)
				}
			}
		}
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id IN ?", ids).Delete(&model.ChannelConsoleCredential{}).Error; err != nil {
			return err
		}
		for poolID := range affectedPoolIDs {
			if err := syncThirdPartyPoolToNewAPIChannel(tx, poolID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	model.InitChannelCache()
	result.Deleted = len(credentials)
	return result, nil
}

func getCredentialPool(poolID int) (*model.ChannelConsolePool, error) {
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	pool := &model.ChannelConsolePool{}
	if err := model.DB.First(pool, "id = ?", poolID).Error; err != nil {
		return nil, err
	}
	return pool, nil
}

func syncThirdPartyPoolToNewAPIChannel(tx *gorm.DB, poolID int) error {
	pool := &model.ChannelConsolePool{}
	if err := tx.First(pool, "id = ?", poolID).Error; err != nil {
		return err
	}
	if pool.ProviderKind != model.ChannelConsoleKindThirdPartyAPI {
		return nil
	}

	var credentials []model.ChannelConsoleCredential
	if err := tx.
		Where("pool_id = ? AND credential_kind = ? AND status <> ?", poolID, CredentialKindAPIKey, model.ChannelConsoleStatusFailed).
		Order("id ASC").
		Find(&credentials).Error; err != nil {
		return err
	}

	keys := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		key := strings.TrimSpace(credential.Credential)
		if key != "" {
			keys = append(keys, key)
		}
	}

	models := normalizeCommitModels(strings.Split(pool.Models, ","), pool.DefaultTestModel)
	modelsValue := strings.Join(models, ",")
	testModel := firstString(models)
	group := defaultCommitGroup
	tag := channelConsoleTag
	baseURL := pool.BaseURL
	defaults := detectProvider(baseURL, baseURL, keys)
	channelType := defaults.channelType
	status := common.ChannelStatusEnabled
	if len(keys) == 0 {
		status = common.ChannelStatusManuallyDisabled
	}

	channelInfo := model.ChannelInfo{
		IsMultiKey:   len(keys) > 1,
		MultiKeySize: len(keys),
		MultiKeyMode: constant.MultiKeyModePolling,
	}

	if pool.NewAPIChannelID == 0 {
		channel := &model.Channel{
			Type:        channelType,
			Key:         strings.Join(keys, "\n"),
			Name:        pool.Name,
			Status:      status,
			CreatedTime: common.GetTimestamp(),
			BaseURL:     &baseURL,
			Models:      modelsValue,
			Group:       group,
			TestModel:   &testModel,
			Tag:         &tag,
			ChannelInfo: channelInfo,
		}
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if len(keys) > 0 && modelsValue != "" {
			if err := channel.AddAbilities(tx); err != nil {
				return err
			}
		}
		pool.NewAPIChannelID = channel.Id
		if err := tx.Model(&model.ChannelConsolePool{}).Where("id = ?", pool.Id).Update("new_api_channel_id", channel.Id).Error; err != nil {
			return err
		}
		return tx.Create(&model.ChannelConsoleChannel{
			ChannelId:       channel.Id,
			Provider:        pool.Provider,
			ProviderKind:    model.ChannelConsoleKindThirdPartyAPI,
			ImportKind:      ImportKindStructured,
			PriceSource:     pool.PriceSource,
			HealthStatus:    pool.HealthStatus,
			ModelSyncStatus: pool.ModelSyncStatus,
			PriceSyncStatus: pool.PriceSyncStatus,
			Markup:          pool.Markup,
		}).Error
	}

	channel := &model.Channel{
		Id:          pool.NewAPIChannelID,
		Type:        channelType,
		Key:         strings.Join(keys, "\n"),
		Name:        pool.Name,
		Status:      status,
		BaseURL:     &baseURL,
		Models:      modelsValue,
		Group:       group,
		TestModel:   &testModel,
		Tag:         &tag,
		ChannelInfo: channelInfo,
	}
	if err := tx.Model(&model.Channel{}).
		Where("id = ?", pool.NewAPIChannelID).
		Select("type", "key", "name", "status", "base_url", "models", "group", "test_model", "tag", "channel_info").
		Updates(channel).Error; err != nil {
		return err
	}
	if err := tx.Where("channel_id = ?", pool.NewAPIChannelID).Delete(&model.Ability{}).Error; err != nil {
		return err
	}
	if len(keys) > 0 && modelsValue != "" {
		if err := channel.AddAbilities(tx); err != nil {
			return err
		}
	}
	meta := &model.ChannelConsoleChannel{
		ChannelId:       pool.NewAPIChannelID,
		Provider:        pool.Provider,
		ProviderKind:    model.ChannelConsoleKindThirdPartyAPI,
		ImportKind:      ImportKindStructured,
		PriceSource:     pool.PriceSource,
		HealthStatus:    pool.HealthStatus,
		ModelSyncStatus: pool.ModelSyncStatus,
		PriceSyncStatus: pool.PriceSyncStatus,
		Markup:          pool.Markup,
	}
	var existing model.ChannelConsoleChannel
	err := tx.Unscoped().Where("channel_id = ?", pool.NewAPIChannelID).First(&existing).Error
	if err == nil {
		return tx.Unscoped().Model(&model.ChannelConsoleChannel{}).
			Where("id = ?", existing.Id).
			Updates(map[string]interface{}{
				"provider":          meta.Provider,
				"provider_kind":     meta.ProviderKind,
				"import_kind":       meta.ImportKind,
				"price_source":      meta.PriceSource,
				"health_status":     meta.HealthStatus,
				"model_sync_status": meta.ModelSyncStatus,
				"price_sync_status": meta.PriceSyncStatus,
				"markup":            meta.Markup,
				"deleted_at":        nil,
			}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(meta).Error
}

func discoverOpenAICompatibleModels(parent context.Context, baseURL string, apiKey string) ([]string, error) {
	candidates := modelDiscoveryURLs(baseURL)
	if len(candidates) == 0 {
		return nil, errors.New("API URL 不正确")
	}

	ctx, cancel := context.WithTimeout(parent, modelDiscoveryTimeout)
	defer cancel()

	var lastErr error
	for _, candidate := range candidates {
		models, err := fetchModelIDs(ctx, candidate, apiKey)
		if err == nil && len(models) > 0 {
			return models, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("未从 API 返回中识别到模型")
}

func modelDiscoveryURLs(rawBaseURL string) []string {
	trimmed := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if trimmed == "" {
		return nil
	}

	seen := map[string]struct{}{}
	add := func(value string, out *[]string) {
		value = strings.TrimRight(value, "/")
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		*out = append(*out, value)
	}

	urls := make([]string, 0, 3)
	add(trimmed+"/models", &urls)
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		path := strings.TrimRight(parsed.Path, "/")
		if !strings.HasSuffix(path, "/v1") {
			parsed.Path = strings.TrimRight(path, "/") + "/v1/models"
			add(parsed.String(), &urls)
		}
	}
	return urls
}

func fetchModelIDs(ctx context.Context, endpoint string, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("拉取模型失败: status=%d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []string `json:"models"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	modelIDs := make([]string, 0, len(payload.Data)+len(payload.Models))
	seen := make(map[string]struct{})
	for _, item := range payload.Data {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		modelIDs = append(modelIDs, modelID)
	}
	for _, item := range payload.Models {
		modelID := strings.TrimSpace(item)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	return modelIDs, nil
}

func firstString(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
