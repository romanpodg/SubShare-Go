package main

import (
	"database/sql"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

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
