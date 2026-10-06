package main

import (
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type deviceCapacity struct {
	limit, count int
}

func (capacity deviceCapacity) available() bool {
	return capacity.limit <= 0 || capacity.count < capacity.limit
}

func normalizeDeviceRegistration(hwid string, meta deviceMeta) (string, deviceMeta) {
	hwid = strings.TrimSpace(hwid)
	meta.NormalizedHWID = normalizeHWID(firstNonEmpty(meta.NormalizedHWID, hwid))
	return hwid, meta
}

func validateSubscriptionHWID(hwid string, settings model.SubscriptionSettings) string {
	if len(hwid) > 128 {
		return "invalid HWID"
	}
	mandatory := settings.ProviderID != "" && settings.HappMandatoryHWID
	if hwid == "" && mandatory {
		return "HWID is required for this subscription"
	}
	return ""
}
