package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"xary-sub/internal/model"
	"xary-sub/internal/vless"
)

var validSQLIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func migrate(db *sql.DB) error {
	queries := []string{
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT,
			token TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS vless_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS user_keys (
			user_id INTEGER NOT NULL,
			key_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, key_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (key_id) REFERENCES vless_keys(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_devices (
			user_id INTEGER NOT NULL,
			hwid TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, hwid),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	if err := ensureColumn(db, "users", "status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "starts_at", "DATETIME"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "expires_at", "DATETIME"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "blocked_reason", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "activation_code", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "activation_used_at", "DATETIME"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "subscription_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "max_devices", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "check_status", "TEXT NOT NULL DEFAULT 'unknown'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "check_error", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "last_checked_at", "DATETIME"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "last_latency_ms", "INTEGER"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "starts_at", "DATETIME"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "expires_at", "DATETIME"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "blocked_reason", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "device_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "device_model", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "platform", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "os_version", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "app_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "app_version", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "user_devices", "user_agent", "TEXT"); err != nil {
		return err
	}

	if _, err := db.Exec(`UPDATE users SET status = 'active' WHERE status IS NULL OR TRIM(status) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET activation_code = token WHERE activation_code IS NULL OR TRIM(activation_code) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET subscription_id = token WHERE subscription_id IS NULL OR TRIM(subscription_id) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET activation_used_at = CURRENT_TIMESTAMP WHERE activation_used_at IS NULL AND subscription_id IS NOT NULL AND TRIM(subscription_id) <> ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET max_devices = 1 WHERE max_devices IS NULL OR max_devices < 1`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_activation_code ON users(activation_code)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_subscription_id ON users(subscription_id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_devices_user_id ON user_devices(user_id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_keys_key_id ON user_keys(key_id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET check_status = 'unknown' WHERE check_status IS NULL OR TRIM(check_status) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET status = 'non-active' WHERE LOWER(TRIM(status)) = 'blocked'`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET status = 'active' WHERE status IS NULL OR TRIM(status) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET starts_at = created_at WHERE starts_at IS NULL`); err != nil {
		return err
	}
	return nil
}

func ensureColumn(db *sql.DB, tableName, columnName, definition string) error {
	if !validSQLIdentifier.MatchString(tableName) {
		return fmt.Errorf("invalid SQL identifier: %q", tableName)
	}
	if !validSQLIdentifier.MatchString(columnName) {
		return fmt.Errorf("invalid SQL identifier: %q", columnName)
	}

	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(name), columnName) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableName, columnName, definition))
	return err
}

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
			u.id, u.name, u.email, u.activation_code, u.subscription_id,
			u.activation_used_at, u.status, u.starts_at, u.expires_at,
			u.blocked_reason,
			COALESCE(NULLIF(u.max_devices, 0), 1) AS max_devices,
			COALESCE(d.connected_devices, 0) AS connected_devices,
			COALESCE(d.connected_hwids, '') AS connected_hwids,
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
		var u model.User
		var status sql.NullString
		var activationCode sql.NullString
		var subscriptionID sql.NullString
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
			&activationCode,
			&subscriptionID,
			&activationUsedAt,
			&status,
			&startsAt,
			&expiresAt,
			&blockedReason,
			&maxDevices,
			&connectedDevices,
			&connectedHWIDs,
			&u.CreatedAt,
			&assignedKeyIDs,
		); err != nil {
			return nil, err
		}
		u.ActivationCode = strings.TrimSpace(activationCode.String)
		u.SubscriptionID = strings.TrimSpace(subscriptionID.String)
		if activationUsedAt.Valid {
			u.ActivationUsedAt = activationUsedAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		u.Status = model.NormalizeStoredStatus(status.String)
		u.StartsAtInput = formatDateTimeInput(startsAt)
		u.ExpiresAtInput = formatDateTimeInput(expiresAt)
		u.BlockedReason = strings.TrimSpace(blockedReason.String)
		u.MaxDevices = 1
		if maxDevices.Valid && maxDevices.Int64 > 0 {
			u.MaxDevices = int(maxDevices.Int64)
		}
		if connectedDevices.Valid && connectedDevices.Int64 > 0 {
			u.ConnectedDeviceCount = int(connectedDevices.Int64)
		}
		rawHWIDs := strings.TrimSpace(connectedHWIDs.String)
		if rawHWIDs != "" {
			u.ConnectedHWIDs = strings.Split(rawHWIDs, "||")
		}
		u.AssignedKeyIDs = strings.TrimSpace(assignedKeyIDs.String)
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return out, nil
	}

	deviceRows, err := a.db.Query(`
		SELECT user_id, hwid, device_name, device_model, platform, os_version, app_name, app_version, user_agent, created_at, last_seen_at
		FROM user_devices
		ORDER BY last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer deviceRows.Close()

	devicesByUser := make(map[int64][]model.ConnectedDevice)
	for deviceRows.Next() {
		var userID int64
		var hwid sql.NullString
		var deviceName sql.NullString
		var deviceModel sql.NullString
		var platform sql.NullString
		var osVersion sql.NullString
		var appName sql.NullString
		var appVersion sql.NullString
		var userAgent sql.NullString
		var createdAt sql.NullTime
		var lastSeenAt sql.NullTime

		if err := deviceRows.Scan(&userID, &hwid, &deviceName, &deviceModel, &platform, &osVersion, &appName, &appVersion, &userAgent, &createdAt, &lastSeenAt); err != nil {
			return nil, err
		}

		device := model.ConnectedDevice{
			HWID:        strings.TrimSpace(hwid.String),
			DeviceName:  strings.TrimSpace(deviceName.String),
			DeviceModel: strings.TrimSpace(deviceModel.String),
			Platform:    strings.TrimSpace(platform.String),
			OSVersion:   strings.TrimSpace(osVersion.String),
			AppName:     strings.TrimSpace(appName.String),
			AppVersion:  strings.TrimSpace(appVersion.String),
			UserAgent:   strings.TrimSpace(userAgent.String),
		}
		if createdAt.Valid {
			device.CreatedAt = createdAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		if lastSeenAt.Valid {
			device.LastSeenAt = lastSeenAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		devicesByUser[userID] = append(devicesByUser[userID], device)
	}
	if err := deviceRows.Err(); err != nil {
		return nil, err
	}

	for index := range out {
		out[index].ConnectedDevices = devicesByUser[out[index].ID]
	}

	return out, nil
}

func (a *App) listKeys() ([]model.VLESSKey, error) {
	rows, err := a.db.Query(`
		SELECT id, label, url, status, check_status, check_error, last_checked_at, last_latency_ms, created_at
		FROM vless_keys
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.VLESSKey
	for rows.Next() {
		var key model.VLESSKey
		var status sql.NullString
		var checkStatus sql.NullString
		var checkError sql.NullString
		var lastCheckedAt sql.NullTime
		var latency sql.NullInt64
		if err := rows.Scan(&key.ID, &key.Label, &key.URL, &status, &checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt); err != nil {
			return nil, err
		}
		key.Status, _ = model.NormalizeKeyStatus(status.String)
		if key.Status == "" {
			key.Status = model.KeyStatusActive
		}
		key.StatusLabel = model.KeyStatusLabel(key.Status)
		key.URLShort = vless.TruncateMiddle(key.URL, 88)
		key.CheckStatus = model.NormalizeCheckStatus(checkStatus.String)
		key.CheckStatusLabel = model.CheckStatusLabel(key.CheckStatus)
		key.CheckError = strings.TrimSpace(checkError.String)
		key.EditUUID, key.EditHost, key.EditPort, key.EditQuery, key.EditFragment, _ = vless.ParseVLESSParts(key.URL)
		if latency.Valid {
			key.LastLatencyMS = latency.Int64
		}
		if lastCheckedAt.Valid {
			key.LastCheckedAtText = lastCheckedAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		out = append(out, key)
	}
	return out, rows.Err()
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
		return false, userID, http.StatusForbidden, "subscription blocked", nil
	}
	if normalizedStatus == model.UserStatusPaused {
		return false, userID, http.StatusForbidden, "subscription paused", nil
	}
	if startsAt.Valid && now.Before(startsAt.Time.UTC()) {
		return false, userID, http.StatusForbidden, "subscription is not active yet", nil
	}
	if expiresAt.Valid && now.After(expiresAt.Time.UTC()) {
		return false, userID, http.StatusForbidden, "subscription expired", nil
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
	if hwid == "" {
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
		     device_name = COALESCE(NULLIF(?, ''), device_name),
		     device_model = COALESCE(NULLIF(?, ''), device_model),
		     platform = COALESCE(NULLIF(?, ''), platform),
		     os_version = COALESCE(NULLIF(?, ''), os_version),
		     app_name = COALESCE(NULLIF(?, ''), app_name),
		     app_version = COALESCE(NULLIF(?, ''), app_version),
		     user_agent = COALESCE(NULLIF(?, ''), user_agent)
		 WHERE user_id = ? AND hwid = ?`,
		meta.DeviceName,
		meta.DeviceModel,
		meta.Platform,
		meta.OSVersion,
		meta.AppName,
		meta.AppVersion,
		meta.UserAgent,
		userID,
		hwid,
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
			`INSERT INTO user_devices (user_id, hwid, device_name, device_model, platform, os_version, app_name, app_version, user_agent)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			userID,
			hwid,
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
	err = tx.QueryRow(`SELECT COUNT(*) FROM user_devices WHERE user_id = ?`, userID).Scan(&count)
	if err != nil {
		return false, err
	}

	if count >= maxDevices {
		return false, nil // limit exceeded
	}

	_, err = tx.Exec(
		`INSERT INTO user_devices (user_id, hwid, device_name, device_model, platform, os_version, app_name, app_version, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID,
		hwid,
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
