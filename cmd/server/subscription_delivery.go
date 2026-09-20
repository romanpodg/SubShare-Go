package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

// subscriptionDeliveryContext is everything one delivery request needs from the
// user row and settings, loaded exactly once per request.
type subscriptionDeliveryContext struct {
	SubscriptionID string
	UserID         int64
	MaxDevices     int
	UserName       string
	Telegram       string
	ExpiresAt      sql.NullTime
	Settings       model.SubscriptionSettings
	Rule           *responseRule
}

// subscriptionDenial is a non-success delivery outcome. The zero value means
// "allowed".
type subscriptionDenial struct {
	Code   int
	Status string
	Reason string
}

func (d subscriptionDenial) denied() bool { return d.Code != 0 }

func deny(code int, status, reason string) subscriptionDenial {
	return subscriptionDenial{Code: code, Status: status, Reason: reason}
}

// loadSubscriptionContext reads global settings and the user's row once and
// merges the per-user overrides.
func (a *App) loadSubscriptionContext(subscriptionID string) (subscriptionDeliveryContext, error) {
	out := subscriptionDeliveryContext{SubscriptionID: strings.TrimSpace(subscriptionID)}
	settings, err := a.getSubscriptionSettings()
	if err != nil {
		return out, err
	}
	var name, email, infoURL, extraURL, extraStatus, timeZone, language sql.NullString
	var refreshHours sql.NullInt64
	var subscriptionName sql.NullString
	err = a.db.QueryRow(`
		SELECT id, max_devices, name, email, expires_at, subscription_name, subscription_refresh_hours,
		       subscription_info_url, subscription_extra_url, subscription_extra_status,
		       time_zone, language
		FROM users WHERE subscription_id = ?
	`, subscriptionID).Scan(
		&out.UserID, &out.MaxDevices, &name, &email, &out.ExpiresAt, &subscriptionName, &refreshHours, &infoURL, &extraURL, &extraStatus,
		&timeZone, &language,
	)
	if err != nil {
		return out, err
	}
	out.UserName = strings.TrimSpace(name.String)
	out.Telegram = strings.TrimPrefix(strings.TrimSpace(email.String), "@")
	if value := strings.TrimSpace(subscriptionName.String); value != "" {
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
	out.Settings = settings
	return out, nil
}

// templateData projects the request context onto informational-key templates.
// realKeys is the count of deliverable real keys for the chosen format.
func (ctx subscriptionDeliveryContext) templateData(realKeys int) delivery.TemplateData {
	out := delivery.TemplateData{SubscriptionID: ctx.SubscriptionID, UserName: ctx.UserName, Telegram: ctx.Telegram, RealKeysCount: realKeys}
	if ctx.ExpiresAt.Valid {
		local := ctx.ExpiresAt.Time.Local()
		out.ExpiryDate = local.Format("02/01/2006")
		out.ExpiryDateTime = local.Format("02/01/2006 15:04")
	}
	return out
}

func writeSubscriptionDenial(w http.ResponseWriter, d subscriptionDenial) {
	code, status, message := d.Code, d.Status, d.Reason
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

// prepareSubscriptionDelivery resolves access, settings, the response rule and
// the device policy for one request. A non-zero denial means stop and reply
// with it.
func (a *App) prepareSubscriptionDelivery(
	r *http.Request,
	subscriptionID string,
	matchRule bool,
) (subscriptionDeliveryContext, subscriptionDenial, error) {
	var out subscriptionDeliveryContext
	out.SubscriptionID = strings.TrimSpace(subscriptionID)
	if out.SubscriptionID == "" || strings.Contains(out.SubscriptionID, "/") {
		return out, deny(http.StatusNotFound, "not-found", "subscription not found"), nil
	}
	allowed, userID, code, reason, err := a.subscriptionAccessAllowed(out.SubscriptionID)
	if err != nil {
		return out, subscriptionDenial{}, err
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
		return out, deny(code, status, a.subscriptionRemarkForStatus(status, reason)), nil
	}
	loaded, err := a.loadSubscriptionContext(out.SubscriptionID)
	if errors.Is(err, sql.ErrNoRows) {
		return out, deny(http.StatusNotFound, "not-found", "subscription not found"), nil
	}
	if err != nil {
		return out, subscriptionDenial{}, err
	}
	if loaded.UserID != userID {
		return out, subscriptionDenial{}, fmt.Errorf("subscription user mismatch")
	}
	out = loaded
	if matchRule {
		out.Rule, err = a.matchSubscriptionResponseRule(r)
		if err != nil {
			return out, subscriptionDenial{}, err
		}
		if out.Rule != nil {
			switch out.Rule.ResponseType {
			case "block":
				return out, deny(http.StatusForbidden, "blocked", "subscription request blocked by response rule"), nil
			case "not-found":
				return out, deny(http.StatusNotFound, "not-found", "subscription not found"), nil
			}
		}
	}
	deviceAllowed, deviceReason, err := a.subscriptionDeviceAllowed(r, userID, out.Settings)
	if err != nil {
		return out, subscriptionDenial{}, err
	}
	if !deviceAllowed {
		return out, deny(http.StatusForbidden, "limited", deviceReason), nil
	}
	return out, subscriptionDenial{}, nil
}
