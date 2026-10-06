package main

import "database/sql"

func updateRegisteredDevice(tx *sql.Tx, userID int64, meta deviceMeta) (bool, error) {
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
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// loadDeviceCapacity reads both values inside the registration transaction,
// after its initial write. Unlimited users retain the existing count-query skip.
func loadDeviceCapacity(tx *sql.Tx, userID int64) (deviceCapacity, error) {
	var capacity deviceCapacity
	if err := tx.QueryRow(`SELECT max_devices FROM users WHERE id = ?`, userID).Scan(&capacity.limit); err != nil {
		return capacity, err
	}

	if capacity.limit <= 0 {
		return capacity, nil
	}

	err := tx.QueryRow(
		`SELECT COUNT(DISTINCT COALESCE(NULLIF(normalized_hwid, ''), LOWER(TRIM(hwid)))) FROM user_devices WHERE user_id = ?`,
		userID,
	).Scan(&capacity.count)
	return capacity, err
}

func insertRegisteredDevice(tx *sql.Tx, userID int64, hwid string, meta deviceMeta) error {
	_, err := tx.Exec(
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
	return err
}
