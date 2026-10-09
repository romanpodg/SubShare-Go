package main

import (
	"fmt"
	"net/http"
	"strings"
)

func safeCustomResponseHeader(key, value string) bool {
	if key == "" {
		return false
	}
	if len(key) > 80 || len(value) > 1024 {
		return false
	}
	if strings.ContainsAny(key+value, "\r\n") {
		return false
	}
	switch strings.ToLower(key) {
	case "set-cookie", "content-length", "transfer-encoding", "connection", "content-type",
		"content-disposition", "strict-transport-security", "access-control-allow-origin",
		"announce", "profile-title", "profile-update-interval", "profile-web-page-url",
		"support-url", "subscription-userinfo", "routing":
		return false
	default:
		return true
	}
}

func normalizeResponseRuleHeaders(headers []responseHeader) error {
	if len(headers) > 30 {
		return fmt.Errorf("too many response headers")
	}
	for index := range headers {
		header := &headers[index]
		header.Key = http.CanonicalHeaderKey(strings.TrimSpace(header.Key))
		header.Value = strings.TrimSpace(header.Value)
		if !safeCustomResponseHeader(header.Key, header.Value) {
			return fmt.Errorf("unsafe response header %q", header.Key)
		}
	}
	return nil
}
