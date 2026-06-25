package service

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
)

func SignJeepayParams(params map[string]string, secret string) string {
	if len(params) == 0 {
		return strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte("key="+secret))))
	}

	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	parts = append(parts, "key="+secret)
	return strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(parts, "&")))))
}

func VerifyJeepayParams(params map[string]string, secret string) bool {
	sign := strings.TrimSpace(params["sign"])
	if sign == "" {
		return false
	}
	return strings.EqualFold(sign, SignJeepayParams(params, secret))
}
