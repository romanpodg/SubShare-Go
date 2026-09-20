package main

import (
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"net/http"
	"strings"
)

type subscriptionDeliverySettings struct {
	ResponseHeaders []responseHeader    `json:"response_headers"`
	Remarks         map[string][]string `json:"remarks"`
}

type subscriptionDeliverySettingsUpdate struct {
	subscriptionDeliverySettings
	// LegacyAnnouncement is accepted only as an upgrade shim for old clients.
	// It is promoted to subscription_settings.extra_status when that canonical
	// value is empty and is never persisted as a delivery setting.
	LegacyAnnouncement *string `json:"announcement,omitempty"`
}

type subscriptionDeliverySettingsResponse struct {
	subscriptionDeliverySettings
	// Capabilities and GenerationExclusionReasonCodes are runtime response
	// metadata. They are absent from the update DTO and cannot be persisted.
	Capabilities                   []delivery.ProtocolCapability `json:"capabilities"`
	GenerationExclusionReasonCodes []string                      `json:"generation_exclusion_reason_codes,omitempty"`
}

var deliveryRemarkStatuses = []string{"expired", "paused", "blocked", "limited", "empty"}

func defaultSubscriptionDeliverySettings() subscriptionDeliverySettings {
	remarks := make(map[string][]string, len(deliveryRemarkStatuses))
	for _, status := range deliveryRemarkStatuses {
		remarks[status] = []string{}
	}
	return subscriptionDeliverySettings{ResponseHeaders: []responseHeader{}, Remarks: remarks}
}

func validateSubscriptionDeliverySettings(input subscriptionDeliverySettings) (subscriptionDeliverySettings, error) {
	if len(input.ResponseHeaders) > 30 {
		return input, fmt.Errorf("too many response headers")
	}
	for index := range input.ResponseHeaders {
		header := &input.ResponseHeaders[index]
		header.Key = http.CanonicalHeaderKey(strings.TrimSpace(header.Key))
		header.Value = strings.TrimSpace(header.Value)
		if !safeCustomResponseHeader(header.Key, header.Value) {
			return input, fmt.Errorf("unsafe response header %q", header.Key)
		}
	}
	normalized := make(map[string][]string, len(deliveryRemarkStatuses))
	for _, status := range deliveryRemarkStatuses {
		values := input.Remarks[status]
		if len(values) > 10 {
			return input, fmt.Errorf("too many remarks for %s", status)
		}
		normalized[status] = make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if len([]rune(value)) > 200 {
				return input, fmt.Errorf("remark for %s is too long", status)
			}
			normalized[status] = append(normalized[status], value)
		}
	}
	input.Remarks = normalized
	return input, nil
}

func (a *App) getSubscriptionDeliverySettings() (subscriptionDeliverySettings, error) {
	settings := defaultSubscriptionDeliverySettings()
	var headersJSON, remarksJSON string
	err := a.db.QueryRow(`
		SELECT response_headers_json, remarks_json
		FROM subscription_delivery_settings WHERE id = 1
	`).Scan(&headersJSON, &remarksJSON)
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal([]byte(headersJSON), &settings.ResponseHeaders); err != nil {
		settings.ResponseHeaders = []responseHeader{}
	}
	var storedRemarks map[string][]string
	if json.Unmarshal([]byte(remarksJSON), &storedRemarks) == nil {
		for _, status := range deliveryRemarkStatuses {
			if values, ok := storedRemarks[status]; ok {
				settings.Remarks[status] = values
			}
		}
	}
	return settings, nil
}

func (a *App) apiV1GetSubscriptionDeliverySettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getSubscriptionDeliverySettings()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "delivery_settings_load_failed", "failed to load subscription delivery settings")
		return
	}
	writeJSON(w, http.StatusOK, subscriptionDeliverySettingsResponse{
		subscriptionDeliverySettings:   settings,
		Capabilities:                   delivery.CapabilityMatrix(),
		GenerationExclusionReasonCodes: delivery.ExclusionReasonCodes(),
	})
}

func (a *App) apiV1UpdateSubscriptionDeliverySettings(w http.ResponseWriter, r *http.Request) {
	var input subscriptionDeliverySettingsUpdate
	if err := readJSON(r, &input); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	settings, err := validateSubscriptionDeliverySettings(input.subscriptionDeliverySettings)
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "delivery_settings_invalid", err.Error())
		return
	}
	legacyAnnouncement := ""
	if input.LegacyAnnouncement != nil {
		legacyAnnouncement = strings.TrimSpace(*input.LegacyAnnouncement)
		if len([]rune(legacyAnnouncement)) > 255 {
			writeV1Error(w, r, http.StatusBadRequest, "delivery_settings_invalid", "deprecated announcement is too long")
			return
		}
	}

	headersJSON, _ := json.Marshal(settings.ResponseHeaders)
	remarksJSON, _ := json.Marshal(settings.Remarks)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "delivery_settings_update_failed", "failed to update subscription delivery settings")
		return
	}
	defer tx.Rollback()
	promoted, err := storage.PromoteLegacyDeliveryAnnouncement(r.Context(), tx, legacyAnnouncement)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `
		INSERT INTO subscription_delivery_settings(id, response_headers_json, announcement, remarks_json, updated_at)
		VALUES(1, ?, '', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
		  response_headers_json = excluded.response_headers_json,
		  announcement = '',
		  remarks_json = excluded.remarks_json,
		  updated_at = CURRENT_TIMESTAMP
		`, string(headersJSON), string(remarksJSON))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "delivery_settings_update_failed", "failed to update subscription delivery settings")
		return
	}
	a.recordAuditEvent(r, "subscription_delivery_settings.update", "settings", "subscription", map[string]any{
		"response_header_count":        len(settings.ResponseHeaders),
		"legacy_announcement_promoted": promoted,
	})
	writeMessage(w, "subscription delivery settings updated")
}

func (a *App) applyGlobalDeliveryHeaders(w http.ResponseWriter) {
	settings, err := a.getSubscriptionDeliverySettings()
	if err == nil {
		applyRuleHeaders(w, settings.ResponseHeaders)
	}
	a.applyHappRoutingHeader(w.Header())
}

func remarkStatusFromReason(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(reason, "expired"):
		return "expired"
	case strings.Contains(reason, "not active"):
		return "future"
	case strings.Contains(reason, "paused"):
		return "paused"
	case strings.Contains(reason, "blocked"):
		return "blocked"
	case strings.Contains(reason, "device"), strings.Contains(reason, "limit"):
		return "limited"
	case strings.Contains(reason, "no available"), strings.Contains(reason, "empty"):
		return "empty"
	default:
		return ""
	}
}

func (a *App) subscriptionRemark(reason string) string {
	status := remarkStatusFromReason(reason)
	return a.subscriptionRemarkForStatus(status, reason)
}

func (a *App) subscriptionRemarkForStatus(status, fallback string) string {
	if status == "" {
		return fallback
	}
	settings, err := a.getSubscriptionDeliverySettings()
	if err != nil || len(settings.Remarks[status]) == 0 {
		return fallback
	}
	return strings.Join(settings.Remarks[status], "\n")
}
