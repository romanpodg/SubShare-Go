package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) listUsers() ([]model.User, error) {
	rows, err := a.db.Query(`
		WITH device_agg AS (
			SELECT user_id,
			       COUNT(1) AS connected_devices,
			       GROUP_CONCAT(hwid, '||') AS connected_hwids
			FROM user_devices
			GROUP BY user_id
		),
		key_agg AS (
			SELECT user_id,
			       GROUP_CONCAT(key_id) AS assigned_key_ids
			FROM user_keys
			GROUP BY user_id
		)
		SELECT
			u.id, u.name, u.email, u.token,
			COALESCE(NULLIF(TRIM(u.time_zone), ''), 'Europe/Moscow') AS time_zone,
			COALESCE(NULLIF(TRIM(u.language), ''), 'ru') AS language,
			u.activation_code, u.subscription_id,
			u.subscription_name, COALESCE(NULLIF(u.subscription_refresh_hours, 0), 12) AS subscription_refresh_hours,
			u.subscription_info_url, u.subscription_extra_url, u.subscription_extra_status,
			u.activation_used_at, u.status, u.starts_at, u.expires_at,
			u.blocked_reason,
			COALESCE(u.max_devices, 0) AS max_devices,
			COALESCE(d.connected_devices, 0) AS connected_devices,
			COALESCE(d.connected_hwids, '') AS connected_hwids,
			COALESCE(NULLIF(TRIM(u.key_assignment_mode), ''), 'all') AS key_assignment_mode,
			u.created_at,
			COALESCE(k.assigned_key_ids, '') AS assigned_key_ids
		FROM users u
		LEFT JOIN device_agg d ON d.user_id = u.id
		LEFT JOIN key_agg k ON k.user_id = u.id
		ORDER BY u.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return out, nil
	}

	devicesByUser, err := a.loadConnectedDevicesByUser()
	if err != nil {
		return nil, err
	}

	for index := range out {
		devices := devicesByUser[out[index].ID]
		if devices == nil {
			devices = make([]model.ConnectedDevice, 0)
		}
		out[index].ConnectedDevices = devices
		if out[index].ConnectedHWIDs == nil {
			out[index].ConnectedHWIDs = make([]string, 0)
		}
	}

	return out, nil
}

// scanUserRow reads one row of the listUsers query and derives the display
// fields (localized inputs, effective status, split HWIDs).
func scanUserRow(rows *sql.Rows) (model.User, error) {
	var u model.User
	var status sql.NullString
	var timeZone sql.NullString
	var language sql.NullString
	var activationCode sql.NullString
	var subscriptionID sql.NullString
	var subscriptionName sql.NullString
	var subscriptionRefreshHours sql.NullInt64
	var subscriptionInfoURL sql.NullString
	var subscriptionExtraURL sql.NullString
	var subscriptionExtraStatus sql.NullString
	var activationUsedAt sql.NullTime
	var startsAt sql.NullTime
	var expiresAt sql.NullTime
	var blockedReason sql.NullString
	var maxDevices sql.NullInt64
	var connectedDevices sql.NullInt64
	var connectedHWIDs sql.NullString
	var assignedKeyIDs sql.NullString
	if err := rows.Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.Token,
		&timeZone,
		&language,
		&activationCode,
		&subscriptionID,
		&subscriptionName,
		&subscriptionRefreshHours,
		&subscriptionInfoURL,
		&subscriptionExtraURL,
		&subscriptionExtraStatus,
		&activationUsedAt,
		&status,
		&startsAt,
		&expiresAt,
		&blockedReason,
		&maxDevices,
		&connectedDevices,
		&connectedHWIDs,
		&u.KeyAssignmentMode,
		&u.CreatedAt,
		&assignedKeyIDs,
	); err != nil {
		return u, err
	}
	u.ActivationCode = strings.TrimSpace(activationCode.String)
	u.TimeZone = strings.TrimSpace(timeZone.String)
	if u.TimeZone == "" {
		u.TimeZone = "Europe/Moscow"
	}
	u.Language = strings.TrimSpace(language.String)
	if u.Language == "" {
		u.Language = "ru"
	}
	u.SubscriptionID = strings.TrimSpace(subscriptionID.String)
	u.SubscriptionName = strings.TrimSpace(subscriptionName.String)
	u.SubscriptionRefreshHours = 12
	if subscriptionRefreshHours.Valid && subscriptionRefreshHours.Int64 > 0 {
		u.SubscriptionRefreshHours = int(subscriptionRefreshHours.Int64)
	}
	u.SubscriptionInfoURL = strings.TrimSpace(subscriptionInfoURL.String)
	u.SubscriptionExtraURL = strings.TrimSpace(subscriptionExtraURL.String)
	u.SubscriptionExtraStatus = strings.TrimSpace(subscriptionExtraStatus.String)
	location, locationErr := time.LoadLocation(u.TimeZone)
	if locationErr != nil {
		location = time.UTC
	}
	if activationUsedAt.Valid {
		u.ActivationUsedAt = activationUsedAt.Time.In(location).Format("02/01/2006 15:04")
	}
	u.Status = model.NormalizeStoredStatus(status.String)
	u.StartsAtInput = formatDateTimeInputInLocation(startsAt, location)
	u.ExpiresAtInput = formatDateTimeInputInLocation(expiresAt, location)
	u.BlockedReason = strings.TrimSpace(blockedReason.String)
	u.MaxDevices = 0
	if maxDevices.Valid && maxDevices.Int64 > 0 {
		u.MaxDevices = int(maxDevices.Int64)
	}
	if connectedDevices.Valid && connectedDevices.Int64 > 0 {
		u.ConnectedDeviceCount = int(connectedDevices.Int64)
	}
	u.EffectiveStatus = model.EffectiveUserStatus(
		u.Status,
		expiresAt.Time,
		expiresAt.Valid,
		u.ConnectedDeviceCount,
		u.MaxDevices,
		time.Now(),
	)
	rawHWIDs := strings.TrimSpace(connectedHWIDs.String)
	if rawHWIDs != "" {
		u.ConnectedHWIDs = strings.Split(rawHWIDs, "||")
	}
	u.AssignedKeyIDs = strings.TrimSpace(assignedKeyIDs.String)
	return u, nil
}

// loadConnectedDevicesByUser returns every user device grouped by user id,
// most recently seen first.
func (a *App) loadConnectedDevicesByUser() (map[int64][]model.ConnectedDevice, error) {
	deviceRows, err := a.db.Query(`
		SELECT user_id, hwid, normalized_hwid, device_name, device_model, platform, os_version, app_name, app_version, user_agent, created_at, last_seen_at
		FROM user_devices
		ORDER BY last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer deviceRows.Close()

	devicesByUser := make(map[int64][]model.ConnectedDevice)
	for deviceRows.Next() {
		userID, device, err := scanConnectedDevice(deviceRows)
		if err != nil {
			return nil, err
		}
		devicesByUser[userID] = append(devicesByUser[userID], device)
	}
	if err := deviceRows.Err(); err != nil {
		return nil, err
	}
	return devicesByUser, nil
}

// scanConnectedDevice reads one user_devices row, filling gaps in the stored
// metadata from the user agent.
func scanConnectedDevice(deviceRows *sql.Rows) (int64, model.ConnectedDevice, error) {
	var userID int64
	var hwid sql.NullString
	var normalizedHWID sql.NullString
	var deviceName sql.NullString
	var deviceModel sql.NullString
	var platform sql.NullString
	var osVersion sql.NullString
	var appName sql.NullString
	var appVersion sql.NullString
	var userAgent sql.NullString
	var createdAt sql.NullTime
	var lastSeenAt sql.NullTime

	if err := deviceRows.Scan(&userID, &hwid, &normalizedHWID, &deviceName, &deviceModel, &platform, &osVersion, &appName, &appVersion, &userAgent, &createdAt, &lastSeenAt); err != nil {
		return 0, model.ConnectedDevice{}, err
	}

	parsed := ParseDeviceInfo(hwid.String, userAgent.String, nil, nil)
	normalizedID := strings.TrimSpace(normalizedHWID.String)
	if normalizedID == "" {
		normalizedID = parsed.NormalizedID
	}
	app := firstNonEmpty(strings.TrimSpace(appName.String), parsed.ClientApp)
	appVersionText := firstNonEmpty(strings.TrimSpace(appVersion.String), parsed.ClientVersion)
	platformText := firstNonEmpty(strings.TrimSpace(platform.String), parsed.Platform)
	osVersionText := firstNonEmpty(strings.TrimSpace(osVersion.String), parsed.OSVersion)
	deviceModelText := firstNonEmpty(strings.TrimSpace(deviceModel.String), parsed.DeviceModel)
	deviceBrandText := firstNonEmpty(parsed.DeviceBrand)

	device := model.ConnectedDevice{
		HWID:           strings.TrimSpace(hwid.String),
		NormalizedHWID: normalizedID,
		DeviceName:     strings.TrimSpace(deviceName.String),
		DeviceModel:    deviceModelText,
		DeviceBrand:    deviceBrandText,
		Platform:       platformText,
		OSVersion:      osVersionText,
		AppName:        app,
		AppVersion:     appVersionText,
		ClientApp:      parsed.ClientApp,
		ClientVersion:  parsed.ClientVersion,
		UserAgent:      strings.TrimSpace(userAgent.String),
	}
	if createdAt.Valid {
		device.CreatedAt = createdAt.Time.Local().Format("2006-01-02 15:04:05")
	}
	if lastSeenAt.Valid {
		device.LastSeenAt = lastSeenAt.Time.Local().Format("2006-01-02 15:04:05")
	}
	return userID, device, nil
}

func (a *App) getSubscriptionSettings() (model.SubscriptionSettings, error) {
	var title sql.NullString
	var refreshHours sql.NullInt64
	var infoURL sql.NullString
	var extraURL sql.NullString
	var extraStatus sql.NullString
	var subscriptionFormat sql.NullString
	var showSubscriptionExpiration sql.NullInt64
	var timeZone sql.NullString
	var language sql.NullString
	var providerID sql.NullString
	var happNoLimitMode sql.NullInt64
	var happNoLimitModeXHTTPOnly sql.NullInt64
	var happMandatoryHWID sql.NullInt64
	var happNotifyExpiration sql.NullInt64
	var happHideServerSettings sql.NullInt64
	var happSubscriptionBody sql.NullString

	err := a.db.QueryRow(
		`SELECT title, refresh_hours, info_url, extra_url, extra_status, subscription_format, show_subscription_expiration, time_zone, language,
		        provider_id, happ_no_limit_mode, happ_no_limit_mode_xhttp_only, happ_mandatory_hwid,
		        happ_notify_expiration, happ_hide_server_settings, happ_subscription_body
		   FROM subscription_settings WHERE id = 1`,
	).Scan(
		&title,
		&refreshHours,
		&infoURL,
		&extraURL,
		&extraStatus,
		&subscriptionFormat,
		&showSubscriptionExpiration,
		&timeZone,
		&language,
		&providerID,
		&happNoLimitMode,
		&happNoLimitModeXHTTPOnly,
		&happMandatoryHWID,
		&happNotifyExpiration,
		&happHideServerSettings,
		&happSubscriptionBody,
	)
	if err != nil {
		return model.SubscriptionSettings{}, err
	}

	settings := model.SubscriptionSettings{
		Title:                      strings.TrimSpace(title.String),
		RefreshHours:               12,
		InfoURL:                    strings.TrimSpace(infoURL.String),
		ExtraURL:                   strings.TrimSpace(extraURL.String),
		ExtraStatus:                strings.TrimSpace(extraStatus.String),
		SubscriptionFormat:         strings.TrimSpace(subscriptionFormat.String),
		ShowSubscriptionExpiration: showSubscriptionExpiration.Valid && showSubscriptionExpiration.Int64 != 0,
		TimeZone:                   strings.TrimSpace(timeZone.String),
		Language:                   strings.TrimSpace(language.String),
		ProviderID:                 strings.TrimSpace(providerID.String),
		HappNoLimitMode:            happNoLimitMode.Valid && happNoLimitMode.Int64 != 0,
		HappNoLimitModeXHTTPOnly:   happNoLimitModeXHTTPOnly.Valid && happNoLimitModeXHTTPOnly.Int64 != 0,
		HappMandatoryHWID:          happMandatoryHWID.Valid && happMandatoryHWID.Int64 != 0,
		HappNotifyExpiration:       happNotifyExpiration.Valid && happNotifyExpiration.Int64 != 0,
		HappHideServerSettings:     happHideServerSettings.Valid && happHideServerSettings.Int64 != 0,
		HappSubscriptionBody:       happSubscriptionBody.String,
	}
	if refreshHours.Valid && refreshHours.Int64 > 0 {
		settings.RefreshHours = int(refreshHours.Int64)
	}
	if settings.Title == "" {
		settings.Title = "AllKeys"
	}
	if settings.TimeZone == "" {
		settings.TimeZone = "Europe/Moscow"
	}
	if settings.Language == "" {
		settings.Language = "ru"
	}
	if normalizedFormat, ok := model.NormalizeSubscriptionFormat(settings.SubscriptionFormat); ok {
		settings.SubscriptionFormat = normalizedFormat
	} else {
		settings.SubscriptionFormat = model.SubscriptionFormatLinks
	}
	return settings, nil
}

func (a *App) getPanelSettings() (model.PanelSettings, error) {
	var panelTitle, logoData, faviconData sql.NullString
	var pageTitleAdmin, pageTitleAdminLogin, pageTitleSubscription sql.NullString
	var subscriptionPageConfig sql.NullString

	err := a.db.QueryRow(
		`SELECT panel_title, logo_data, favicon_data, page_title_admin, page_title_admin_login, page_title_subscription, subscription_page_config
		 FROM panel_settings WHERE id = 1`,
	).Scan(&panelTitle, &logoData, &faviconData, &pageTitleAdmin, &pageTitleAdminLogin, &pageTitleSubscription, &subscriptionPageConfig)
	if err != nil {
		return model.PanelSettings{}, err
	}

	s := model.PanelSettings{
		PanelTitle:             strings.TrimSpace(panelTitle.String),
		LogoDataURL:            logoData.String,
		FaviconDataURL:         faviconData.String,
		PageTitleAdmin:         strings.TrimSpace(pageTitleAdmin.String),
		PageTitleAdminLogin:    strings.TrimSpace(pageTitleAdminLogin.String),
		PageTitleSubscription:  strings.TrimSpace(pageTitleSubscription.String),
		SubscriptionPageConfig: subscriptionPageConfig.String,
	}
	if s.PanelTitle == "" {
		s.PanelTitle = "SubShare"
	}
	if s.PageTitleAdmin == "" {
		s.PageTitleAdmin = "Панель управления — SubShare"
	}
	if s.PageTitleAdminLogin == "" {
		s.PageTitleAdminLogin = "Вход — SubShare"
	}
	if s.PageTitleSubscription == "" {
		s.PageTitleSubscription = "VPN-подписка — SubShare"
	}
	return s, nil
}

func (a *App) getRoutingSettings() (model.RoutingSettings, error) {
	var configJSON, deliveryMode sql.NullString
	if err := a.db.QueryRow(`SELECT config_json, delivery_mode FROM routing_settings WHERE id = 1`).Scan(&configJSON, &deliveryMode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RoutingSettings{ConfigJSON: "", DeliveryMode: routingDeliveryModeDisabled}, nil
		}
		return model.RoutingSettings{}, err
	}

	return model.RoutingSettings{
		ConfigJSON:   strings.TrimSpace(configJSON.String),
		DeliveryMode: strings.TrimSpace(deliveryMode.String),
	}, nil
}

func (a *App) updatePanelSettings(s model.PanelSettings) error {
	_, err := a.db.Exec(
		`UPDATE panel_settings SET
			panel_title = ?,
			logo_data = ?,
			favicon_data = ?,
			page_title_admin = ?,
			page_title_admin_login = ?,
			page_title_subscription = ?,
			updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		s.PanelTitle, s.LogoDataURL, s.FaviconDataURL,
		s.PageTitleAdmin, s.PageTitleAdminLogin, s.PageTitleSubscription,
	)
	return err
}

func (a *App) updateSubscriptionPageConfig(configJSON string) error {
	_, err := a.db.Exec(
		`UPDATE panel_settings SET
			subscription_page_config = ?,
			updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		strings.TrimSpace(configJSON),
	)
	return err
}

func (a *App) updateRoutingSettings(s model.RoutingSettings) error {
	_, err := a.db.Exec(
		`INSERT INTO routing_settings(id, config_json, delivery_mode, updated_at)
		 VALUES(1, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		   config_json = excluded.config_json,
		   delivery_mode = excluded.delivery_mode,
		   updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(s.ConfigJSON), strings.TrimSpace(s.DeliveryMode),
	)
	return err
}

func (a *App) subscriptionAccessAllowed(subscriptionID string) (bool, int64, int, string, error) {
	var status sql.NullString
	var userID int64
	var startsAt sql.NullTime
	var expiresAt sql.NullTime
	var blockedReason sql.NullString
	err := a.db.QueryRow(
		`SELECT id, status, starts_at, expires_at, blocked_reason FROM users WHERE subscription_id = ?`,
		subscriptionID,
	).Scan(&userID, &status, &startsAt, &expiresAt, &blockedReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, http.StatusNotFound, "subscription not found", nil
		}
		return false, 0, 0, "", err
	}

	normalizedStatus := model.NormalizeStoredStatus(status.String)
	now := time.Now().UTC()

	if normalizedStatus == model.UserStatusBlocked {
		reason := strings.TrimSpace(blockedReason.String)
		if reason == "" {
			reason = "subscription blocked"
		}
		return false, userID, http.StatusForbidden, reason, nil
	}
	if normalizedStatus == model.UserStatusPaused {
		return false, userID, http.StatusForbidden, "subscription paused", nil
	}
	if startsAt.Valid && now.Before(startsAt.Time.UTC()) {
		return false, userID, http.StatusForbidden, "subscription is not active yet", nil
	}
	if expiresAt.Valid && now.After(expiresAt.Time.UTC()) {
		return false, userID, http.StatusGone, "subscription expired", nil
	}

	return true, userID, http.StatusOK, "", nil
}

func (a *App) redeemActivationCode(code string) (string, int, string, error) {
	var userID int64
	var usedAt sql.NullTime
	var subscriptionID sql.NullString
	err := a.db.QueryRow(
		`SELECT id, activation_used_at, subscription_id FROM users WHERE activation_code = ?`,
		code,
	).Scan(&userID, &usedAt, &subscriptionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", http.StatusNotFound, "Подписка не найдена", nil
		}
		return "", 0, "", err
	}

	if usedAt.Valid {
		return "", http.StatusForbidden, "Ключ уже активирован", nil
	}

	generatedSubscriptionID := strings.TrimSpace(subscriptionID.String)
	if generatedSubscriptionID == "" {
		generatedSubscriptionID, err = generateToken(24)
		if err != nil {
			return "", 0, "", err
		}
	}

	res, err := a.db.Exec(
		`UPDATE users
		 SET subscription_id = ?, activation_used_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND activation_used_at IS NULL`,
		generatedSubscriptionID,
		userID,
	)
	if err != nil {
		return "", 0, "", err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return "", 0, "", err
	}
	if rows == 0 {
		return "", http.StatusForbidden, "Ключ уже активирован", nil
	}

	return generatedSubscriptionID, http.StatusOK, "", nil
}

func (a *App) registerHWID(userID int64, hwid string, meta deviceMeta) (bool, error) {
	hwid = strings.TrimSpace(hwid)
	meta.NormalizedHWID = normalizeHWID(firstNonEmpty(meta.NormalizedHWID, hwid))
	if meta.NormalizedHWID == "" {
		return true, nil
	}

	tx, err := a.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Try to update existing device
	res, err := tx.Exec(
		`UPDATE user_devices
		 SET last_seen_at = CURRENT_TIMESTAMP,
		     normalized_hwid = ?,
		     device_name = COALESCE(NULLIF(?, ''), device_name),
		     device_model = COALESCE(NULLIF(?, ''), device_model),
		     platform = COALESCE(NULLIF(?, ''), platform),
		     os_version = COALESCE(NULLIF(?, ''), os_version),
		     app_name = COALESCE(NULLIF(?, ''), app_name),
		     app_version = COALESCE(NULLIF(?, ''), app_version),
		     user_agent = COALESCE(NULLIF(?, ''), user_agent)
		 WHERE user_id = ? AND normalized_hwid = ?`,
		meta.NormalizedHWID,
		meta.DeviceName,
		meta.DeviceModel,
		meta.Platform,
		meta.OSVersion,
		meta.AppName,
		meta.AppVersion,
		meta.UserAgent,
		userID,
		meta.NormalizedHWID,
	)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	if rows > 0 {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit transaction: %w", err)
		}
		return true, nil
	}

	// Check device limit and get max_devices atomically within transaction
	var maxDevices int
	err = tx.QueryRow(`SELECT max_devices FROM users WHERE id = ?`, userID).Scan(&maxDevices)
	if err != nil {
		return false, err
	}

	if maxDevices <= 0 {
		// No limit set, allow
		_, err = tx.Exec(
			`INSERT INTO user_devices (user_id, hwid, normalized_hwid, device_name, device_model, platform, os_version, app_name, app_version, user_agent)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			userID,
			hwid,
			meta.NormalizedHWID,
			meta.DeviceName,
			meta.DeviceModel,
			meta.Platform,
			meta.OSVersion,
			meta.AppName,
			meta.AppVersion,
			meta.UserAgent,
		)
		if err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit transaction: %w", err)
		}
		return true, nil
	}

	var count int
	err = tx.QueryRow(
		`SELECT COUNT(DISTINCT COALESCE(NULLIF(normalized_hwid, ''), LOWER(TRIM(hwid)))) FROM user_devices WHERE user_id = ?`,
		userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}

	if count >= maxDevices {
		return false, nil // limit exceeded
	}

	_, err = tx.Exec(
		`INSERT INTO user_devices (user_id, hwid, normalized_hwid, device_name, device_model, platform, os_version, app_name, app_version, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID,
		hwid,
		meta.NormalizedHWID,
		meta.DeviceName,
		meta.DeviceModel,
		meta.Platform,
		meta.OSVersion,
		meta.AppName,
		meta.AppVersion,
		meta.UserAgent,
	)
	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}
	return true, nil
}
