package channelconsole

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const cliproxyRequestTimeout = 15 * time.Second

type CliProxyAuthFile struct {
	ID            string         `json:"id"`
	AuthIndex     string         `json:"auth_index"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Provider      string         `json:"provider"`
	Status        string         `json:"status"`
	StatusMessage string         `json:"status_message"`
	Disabled      bool           `json:"disabled"`
	Unavailable   bool           `json:"unavailable"`
	RuntimeOnly   bool           `json:"runtime_only"`
	Source        string         `json:"source"`
	Size          int64          `json:"size"`
	ModTime       string         `json:"modtime"`
	Email         string         `json:"email"`
	AccountType   string         `json:"account_type"`
	Account       string         `json:"account"`
	LastRefresh   string         `json:"last_refresh"`
	Success       int64          `json:"success"`
	Failed        int64          `json:"failed"`
	Raw           map[string]any `json:"raw,omitempty"`
}

type CliProxyAuthFilesResult struct {
	Files []CliProxyAuthFile `json:"files"`
}

type CliProxyUploadAuthFileRequest struct {
	Name          string `json:"name"`
	RawCredential string `json:"raw_credential"`
}

type CliProxyDeleteAuthFilesRequest struct {
	Names []string `json:"names"`
}

type CliProxyDeleteAuthFilesResult struct {
	Deleted int      `json:"deleted"`
	Failed  []string `json:"failed"`
}

type CliProxyAuthURLResult struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

type CliProxyStatusResult struct {
	Configured bool   `json:"configured"`
	Reachable  bool   `json:"reachable"`
	BaseURL    string `json:"base_url"`
	FilesCount int    `json:"files_count"`
	Message    string `json:"message"`
}

func GetCliProxyStatus(ctx context.Context) (*CliProxyStatusResult, error) {
	baseURL := cliproxyManagementBaseURL()
	if baseURL == "" || strings.TrimSpace(os.Getenv("CLIPROXY_MANAGEMENT_KEY")) == "" {
		return &CliProxyStatusResult{Configured: false, Message: "CliProxy 未配置"}, nil
	}
	files, err := ListCliProxyAuthFiles(ctx)
	if err != nil {
		return &CliProxyStatusResult{Configured: true, Reachable: false, BaseURL: baseURL, Message: err.Error()}, nil
	}
	return &CliProxyStatusResult{Configured: true, Reachable: true, BaseURL: baseURL, FilesCount: len(files.Files)}, nil
}

func ListCliProxyAuthFiles(ctx context.Context) (*CliProxyAuthFilesResult, error) {
	body, err := cliproxyManagementRequest(ctx, http.MethodGet, "/auth-files", nil, nil, "")
	if err != nil {
		return nil, err
	}
	var result CliProxyAuthFilesResult
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	sort.SliceStable(result.Files, func(i, j int) bool {
		return strings.ToLower(result.Files[i].Name) < strings.ToLower(result.Files[j].Name)
	})
	return &result, nil
}

func UploadCliProxyAuthFile(ctx context.Context, req CliProxyUploadAuthFileRequest) error {
	name := sanitizeCliProxyAuthFileName(req.Name)
	if name == "" {
		return errors.New("请填写 auth 文件名")
	}
	raw := strings.TrimSpace(req.RawCredential)
	if raw == "" {
		return errors.New("请粘贴 OAuth/auth JSON")
	}
	if !strings.HasPrefix(raw, "{") {
		return errors.New("CliProxy auth 文件必须是 JSON")
	}
	var tmp map[string]any
	if err := common.Unmarshal([]byte(raw), &tmp); err != nil {
		return errors.New("OAuth/auth JSON 格式不正确")
	}
	q := url.Values{"name": []string{name}}
	_, err := cliproxyManagementRequest(ctx, http.MethodPost, "/auth-files", q, []byte(raw), "application/json")
	return err
}

func DeleteCliProxyAuthFiles(ctx context.Context, names []string) (*CliProxyDeleteAuthFilesResult, error) {
	result := &CliProxyDeleteAuthFilesResult{}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = sanitizeCliProxyAuthFileName(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		q := url.Values{"name": []string{name}}
		if _, err := cliproxyManagementRequest(ctx, http.MethodDelete, "/auth-files", q, nil, ""); err != nil {
			result.Failed = append(result.Failed, name)
			continue
		}
		result.Deleted++
	}
	return result, nil
}

func GetCliProxyAuthURL(ctx context.Context, provider string) (*CliProxyAuthURLResult, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	endpoint := map[string]string{
		"codex":       "/codex-auth-url",
		"anthropic":   "/anthropic-auth-url",
		"claude":      "/anthropic-auth-url",
		"gemini":      "/gemini-cli-auth-url",
		"gemini-cli":  "/gemini-cli-auth-url",
		"antigravity": "/antigravity-auth-url",
	}[provider]
	if endpoint == "" {
		return nil, errors.New("不支持的 CliProxy OAuth provider")
	}
	q := url.Values{"is_webui": []string{"true"}}
	body, err := cliproxyManagementRequest(ctx, http.MethodGet, endpoint, q, nil, "")
	if err != nil {
		return nil, err
	}
	var result CliProxyAuthURLResult
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func cliproxyManagementRequest(ctx context.Context, method string, endpoint string, query url.Values, body []byte, contentType string) ([]byte, error) {
	baseURL := cliproxyManagementBaseURL()
	key := strings.TrimSpace(os.Getenv("CLIPROXY_MANAGEMENT_KEY"))
	if baseURL == "" || key == "" {
		return nil, errors.New("CliProxy 未配置")
	}
	requestURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	requestURL.Path = path.Join(requestURL.Path, endpoint)
	if query != nil {
		requestURL.RawQuery = query.Encode()
	}

	ctx, cancel := context.WithTimeout(ctx, cliproxyRequestTimeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CliProxy 请求失败: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func cliproxyManagementBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("CLIPROXY_MANAGEMENT_BASE_URL")), "/")
}

func sanitizeCliProxyAuthFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "-")
	name = path.Base(strings.ReplaceAll(name, "/", "-"))
	if name == "." || name == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		name += ".json"
	}
	return name
}
