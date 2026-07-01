package constant

import "strings"

const TokenGroupGPTPlus = "gpt-plus"

func NormalizeTokenGroup(group string) string {
	trimmed := strings.TrimSpace(group)
	if trimmed != "" {
		return trimmed
	}
	return TokenGroupGPTPlus
}

func NormalizeTokenCrossGroupRetry(group string, crossGroupRetry bool) bool {
	return group == "auto" && crossGroupRetry
}
