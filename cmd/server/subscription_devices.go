package main

import (
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func requestHWID(r *http.Request) string {
	return firstNonEmpty(r.URL.Query().Get("hwid"), r.Header.Get("X-HWID"), r.Header.Get("X-Device-ID"))
}

func (a *App) subscriptionDeviceAllowed(r *http.Request, userID int64, settings model.SubscriptionSettings) (bool, string, error) {
	hwid := requestHWID(r)
	if reason := validateSubscriptionHWID(hwid, settings); reason != "" {
		return false, reason, nil
	}
	if hwid == "" {
		return true, "", nil
	}
	meta := extractDeviceMeta(r)
	parsed := ParseDeviceInfo(hwid, r.UserAgent(), r.Header, r.URL.Query())
	meta = mergeDeviceMeta(meta, parsed)
	allowed, err := a.registerHWID(userID, hwid, meta)
	if err != nil {
		return false, "", err
	}
	if !allowed {
		message := firstNonEmpty(a.deviceLimitMessage, model.DefaultDeviceLimitMessage)
		return false, a.subscriptionRemarkForStatus("limited", message), nil
	}
	return true, "", nil
}
