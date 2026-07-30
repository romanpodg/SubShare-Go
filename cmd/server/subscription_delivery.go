package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"subshare/internal/model"
)

type subscriptionDeliveryContext struct {
	SubscriptionID string
	UserID         int64
	MaxDevices     int
	Settings       model.SubscriptionSettings
	Rule           *responseRule
}

func (a *App) effectiveSubscriptionSettings(subscriptionID string) (model.SubscriptionSettings, int64, int, error) {
	settings, err := a.getSubscriptionSettings()
	if err != nil {
		return model.SubscriptionSettings{}, 0, 0, err
	}
	var userID int64
	var maxDevices int
	var name, infoURL, extraURL, extraStatus, timeZone, language sql.NullString
	var refreshHours sql.NullInt64
	err = a.db.QueryRow(`
		SELECT id, max_devices, subscription_name, subscription_refresh_hours,
		       subscription_info_url, subscription_extra_url, subscription_extra_status,
		       time_zone, language
		FROM users WHERE subscription_id = ?
	`, subscriptionID).Scan(
		&userID, &maxDevices, &name, &refreshHours, &infoURL, &extraURL, &extraStatus,
		&timeZone, &language,
	)
	if err != nil {
		return model.SubscriptionSettings{}, 0, 0, err
	}
	if value := strings.TrimSpace(name.String); value != "" {
		settings.Title = value
	}
	if refreshHours.Valid && refreshHours.Int64 > 0 {
		settings.RefreshHours = int(refreshHours.Int64)
	}
	if value := strings.TrimSpace(infoURL.String); value != "" {
		settings.InfoURL = value
	}
	if value := strings.TrimSpace(extraURL.String); value != "" {
		settings.ExtraURL = value
	}
	if value := strings.TrimSpace(extraStatus.String); value != "" {
		settings.ExtraStatus = value
	}
	if value := strings.TrimSpace(timeZone.String); value != "" {
		settings.TimeZone = value
	}
	if value := strings.TrimSpace(language.String); value != "" {
		settings.Language = value
	}
	return settings, userID, maxDevices, nil
}

func writeSubscriptionDenial(w http.ResponseWriter, code int, status, message string) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "denied"
	}
	w.Header().Set("Subscription-Status", status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, message, code)
}

func (a *App) subscriptionDeviceAllowed(
	r *http.Request,
	userID int64,
	settings model.SubscriptionSettings,
) (bool, string, error) {
	hwid := strings.TrimSpace(r.URL.Query().Get("hwid"))
	if hwid == "" {
		hwid = strings.TrimSpace(r.Header.Get("X-HWID"))
	}
	if hwid == "" {
		hwid = strings.TrimSpace(r.Header.Get("X-Device-ID"))
	}
	if len(hwid) > 128 {
		return false, "invalid HWID", nil
	}
	mandatory := settings.ProviderID != "" && settings.HappMandatoryHWID
	if hwid == "" {
		if mandatory {
			return false, "HWID is required for this subscription", nil
		}
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
		message := strings.TrimSpace(a.deviceLimitMessage)
		if message == "" {
			message = model.DefaultDeviceLimitMessage
		}
		return false, a.subscriptionRemarkForStatus("limited", message), nil
	}
	return true, "", nil
}

func (a *App) prepareSubscriptionDelivery(
	r *http.Request,
	subscriptionID string,
	matchRule bool,
) (subscriptionDeliveryContext, int, string, string, error) {
	var out subscriptionDeliveryContext
	out.SubscriptionID = strings.TrimSpace(subscriptionID)
	if out.SubscriptionID == "" || strings.Contains(out.SubscriptionID, "/") {
		return out, http.StatusNotFound, "not-found", "subscription not found", nil
	}
	allowed, userID, code, reason, err := a.subscriptionAccessAllowed(out.SubscriptionID)
	if err != nil {
		return out, 0, "", "", err
	}
	if !allowed {
		status := remarkStatusFromReason(reason)
		if status == "" {
			switch code {
			case http.StatusGone:
				status = "expired"
			case http.StatusNotFound:
				status = "not-found"
			case http.StatusForbidden:
				status = "blocked"
			default:
				status = "denied"
			}
		}
		return out, code, status, a.subscriptionRemarkForStatus(status, reason), nil
	}
	settings, effectiveUserID, maxDevices, err := a.effectiveSubscriptionSettings(out.SubscriptionID)
	if errors.Is(err, sql.ErrNoRows) {
		return out, http.StatusNotFound, "not-found", "subscription not found", nil
	}
	if err != nil {
		return out, 0, "", "", err
	}
	if effectiveUserID != userID {
		return out, 0, "", "", fmt.Errorf("subscription user mismatch")
	}
	out.UserID = userID
	out.MaxDevices = maxDevices
	out.Settings = settings
	if matchRule {
		out.Rule, err = a.matchSubscriptionResponseRule(r)
		if err != nil {
			return out, 0, "", "", err
		}
		if out.Rule != nil {
			switch out.Rule.ResponseType {
			case "block":
				return out, http.StatusForbidden, "blocked", "subscription request blocked by response rule", nil
			case "not-found":
				return out, http.StatusNotFound, "not-found", "subscription not found", nil
			}
		}
	}
	deviceAllowed, deviceReason, err := a.subscriptionDeviceAllowed(r, userID, settings)
	if err != nil {
		return out, 0, "", "", err
	}
	if !deviceAllowed {
		return out, http.StatusForbidden, "limited", deviceReason, nil
	}
	return out, 0, "", "", nil
}
