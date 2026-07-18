package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

type relayErrorClassification struct {
	code      types.ErrorCode
	skipRetry bool
	message   string
}

func classifyRelayError(resp *http.Response, body []byte, err *types.NewAPIError) *types.NewAPIError {
	classification := classifyRelayErrorResponse(resp, body)
	if classification.code == "" || err == nil {
		return err
	}

	message := err.Error()
	if classification.message != "" {
		message = classification.message
	}
	if message == "" {
		message = fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode)
	}

	options := make([]types.NewAPIErrorOptions, 0, 1)
	if classification.skipRetry {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.NewOpenAIError(errors.New(message), classification.code, resp.StatusCode, options...)
}

func classifyRelayErrorResponse(resp *http.Response, body []byte) relayErrorClassification {
	if resp == nil {
		return relayErrorClassification{}
	}

	status := resp.StatusCode
	text := strings.ToLower(string(body))
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	server := strings.ToLower(resp.Header.Get("Server"))

	if status == http.StatusForbidden && isCloudflareEdgeResponse(resp, text, contentType, server) {
		message := "request blocked at the edge"
		if ray := strings.TrimSpace(resp.Header.Get("CF-Ray")); ray != "" {
			message = fmt.Sprintf("request blocked at the edge (cf-ray: %s)", ray)
		}
		return relayErrorClassification{
			code:      types.ErrorCodeEdgeBlocked,
			skipRetry: true,
			message:   message,
		}
	}

	switch status {
	case http.StatusRequestEntityTooLarge:
		return relayErrorClassification{code: types.ErrorCodePayloadTooLarge, skipRetry: true}
	case http.StatusTooManyRequests:
		return relayErrorClassification{code: types.ErrorCodeRateLimited}
	case http.StatusUnauthorized:
		return relayErrorClassification{code: types.ErrorCodeAuthRejected}
	case http.StatusMethodNotAllowed:
		return relayErrorClassification{code: types.ErrorCodeRouteMismatch}
	case http.StatusNotFound:
		if containsAny(text, "unknown request url", "unknown endpoint", "route not found", "endpoint not found") {
			return relayErrorClassification{code: types.ErrorCodeRouteMismatch}
		}
	case http.StatusBadRequest, http.StatusForbidden:
		if containsAny(text, "invalid api key", "invalid_api_key", "authentication", "unauthorized") {
			return relayErrorClassification{code: types.ErrorCodeAuthRejected}
		}
		if containsAny(text, "not available", "not allowed", "unsupported", "policy", "organization", "permission") {
			return relayErrorClassification{code: types.ErrorCodeUpstreamPolicy}
		}
	}

	return relayErrorClassification{}
}

func isCloudflareEdgeResponse(resp *http.Response, body, contentType, server string) bool {
	cloudflareMarkers := containsAny(body, "cloudflare", "attention required", "error 1010", "error 1020", "error 1027", "access denied")
	if strings.Contains(contentType, "application/json") {
		return cloudflareMarkers
	}
	if resp.Header.Get("CF-Ray") != "" || strings.Contains(server, "cloudflare") {
		return true
	}
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "text/plain") {
		return false
	}
	return cloudflareMarkers
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
