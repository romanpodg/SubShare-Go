package main

import (
	"database/sql"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) getSubscriptionSettings() (model.SubscriptionSettings, error) {
	return scanSubscriptionSettings(a.db.QueryRow(
		`SELECT title, refresh_hours, info_url, extra_url, extra_status, subscription_format, show_subscription_expiration, time_zone, language,
		        provider_id, happ_no_limit_mode, happ_no_limit_mode_xhttp_only, happ_mandatory_hwid,
		        happ_notify_expiration, happ_hide_server_settings, happ_subscription_body
		   FROM subscription_settings WHERE id = 1`,
	))
}

type subscriptionSettingsRow struct {
	title, infoURL, extraURL, extraStatus, subscriptionFormat    sql.NullString
	timeZone, language, providerID, happSubscriptionBody         sql.NullString
	refreshHours, showSubscriptionExpiration                     sql.NullInt64
	happNoLimitMode, happNoLimitModeXHTTPOnly, happMandatoryHWID sql.NullInt64
	happNotifyExpiration, happHideServerSettings                 sql.NullInt64
}

func scanSubscriptionSettings(row *sql.Row) (model.SubscriptionSettings, error) {
	var stored subscriptionSettingsRow
	if err := row.Scan(
		&stored.title,
		&stored.refreshHours,
		&stored.infoURL,
		&stored.extraURL,
		&stored.extraStatus,
		&stored.subscriptionFormat,
		&stored.showSubscriptionExpiration,
		&stored.timeZone,
		&stored.language,
		&stored.providerID,
		&stored.happNoLimitMode,
		&stored.happNoLimitModeXHTTPOnly,
		&stored.happMandatoryHWID,
		&stored.happNotifyExpiration,
		&stored.happHideServerSettings,
		&stored.happSubscriptionBody,
	); err != nil {
		return model.SubscriptionSettings{}, err
	}
	return stored.toSettings(), nil
}

func (row subscriptionSettingsRow) toSettings() model.SubscriptionSettings {
	return model.SubscriptionSettings{
		Title:                      firstNonEmpty(row.title.String, "AllKeys"),
		RefreshHours:               positiveIntOrDefault(row.refreshHours, 12),
		InfoURL:                    strings.TrimSpace(row.infoURL.String),
		ExtraURL:                   strings.TrimSpace(row.extraURL.String),
		ExtraStatus:                strings.TrimSpace(row.extraStatus.String),
		SubscriptionFormat:         storedSubscriptionFormat(row.subscriptionFormat.String),
		ShowSubscriptionExpiration: nullIntBool(row.showSubscriptionExpiration),
		TimeZone:                   firstNonEmpty(row.timeZone.String, "Europe/Moscow"),
		Language:                   firstNonEmpty(row.language.String, "ru"),
		ProviderID:                 strings.TrimSpace(row.providerID.String),
		HappNoLimitMode:            nullIntBool(row.happNoLimitMode),
		HappNoLimitModeXHTTPOnly:   nullIntBool(row.happNoLimitModeXHTTPOnly),
		HappMandatoryHWID:          nullIntBool(row.happMandatoryHWID),
		HappNotifyExpiration:       nullIntBool(row.happNotifyExpiration),
		HappHideServerSettings:     nullIntBool(row.happHideServerSettings),
		HappSubscriptionBody:       row.happSubscriptionBody.String,
	}
}

func storedSubscriptionFormat(raw string) string {
	if normalized, ok := model.NormalizeSubscriptionFormat(strings.TrimSpace(raw)); ok {
		return normalized
	}
	return model.SubscriptionFormatLinks
}
