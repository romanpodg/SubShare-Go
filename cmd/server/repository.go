package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

func (a *App) listUsers() ([]User, error) {
	rows, err := a.db.Query(`
		SELECT
			u.id,
			u.name,
			u.email,
			u.activation_code,
			u.subscription_id,
			u.activation_used_at,
			u.status,
			u.starts_at,
			u.expires_at,
			u.blocked_reason,
			COALESCE(NULLIF(u.max_devices, 0), 1) AS max_devices,
			(SELECT COUNT(1) FROM user_devices ud WHERE ud.user_id = u.id) AS connected_devices,
			(SELECT COALESCE(GROUP_CONCAT(ud.hwid, '||'), '') FROM user_devices ud WHERE ud.user_id = u.id ORDER BY ud.last_seen_at DESC) AS connected_hwids,
			u.created_at,
			(SELECT COALESCE(GROUP_CONCAT(uk.key_id), '') FROM user_keys uk WHERE uk.user_id = u.id) AS assigned_key_ids
		FROM users u
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
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
		u.Status = normalizeStoredStatus(status.String)
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
	return out, rows.Err()
}

func (a *App) listAssignableKeys() ([]VLESSKey, error) {
	rows, err := a.db.Query(`
		SELECT id, label
		FROM vless_keys
		WHERE LOWER(COALESCE(NULLIF(TRIM(status), ''), 'active')) = 'active'
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []VLESSKey
	for rows.Next() {
		var key VLESSKey
		if err := rows.Scan(&key.ID, &key.Label); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (a *App) listKeys() ([]VLESSKey, error) {
	rows, err := a.db.Query(`
		SELECT id, label, url, status, check_status, check_error, last_checked_at, last_latency_ms, created_at
		FROM vless_keys
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []VLESSKey
	for rows.Next() {
		var key VLESSKey
		var status sql.NullString
		var checkStatus sql.NullString
		var checkError sql.NullString
		var lastCheckedAt sql.NullTime
		var latency sql.NullInt64
		if err := rows.Scan(&key.ID, &key.Label, &key.URL, &status, &checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt); err != nil {
			return nil, err
		}
		key.Status, _ = normalizeKeyStatus(status.String)
		if key.Status == "" {
			key.Status = keyStatusActive
		}
		key.StatusLabel = keyStatusLabel(key.Status)
		key.URLShort = truncateMiddle(key.URL, 88)
		key.CheckStatus = normalizeCheckStatus(checkStatus.String)
		key.CheckStatusLabel = checkStatusLabel(key.CheckStatus)
		key.CheckError = strings.TrimSpace(checkError.String)
		key.EditUUID, key.EditHost, key.EditPort, key.EditQuery, key.EditFragment, _ = parseVLESSParts(key.URL)
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

	normalizedStatus := normalizeStoredStatus(status.String)
	now := time.Now().UTC()

	if normalizedStatus == userStatusBlocked {
		reason := strings.TrimSpace(blockedReason.String)
		if reason == "" {
			reason = "subscription blocked"
		}
		return false, userID, http.StatusForbidden, reason, nil
	}
	if normalizedStatus == userStatusPaused {
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

func (a *App) registerHWID(userID int64, hwid string) (bool, error) {
	hwid = strings.TrimSpace(hwid)
	if hwid == "" {
		return true, nil
	}

	var maxDevices int64
	if err := a.db.QueryRow(`SELECT COALESCE(NULLIF(max_devices, 0), 1) FROM users WHERE id = ?`, userID).Scan(&maxDevices); err != nil {
		return false, err
	}
	if maxDevices < 1 {
		maxDevices = 1
	}

	if _, err := a.db.Exec(
		`UPDATE user_devices SET last_seen_at = CURRENT_TIMESTAMP WHERE user_id = ? AND hwid = ?`,
		userID,
		hwid,
	); err != nil {
		return false, err
	}

	var existing int64
	if err := a.db.QueryRow(`SELECT COUNT(1) FROM user_devices WHERE user_id = ? AND hwid = ?`, userID, hwid).Scan(&existing); err != nil {
		return false, err
	}
	if existing > 0 {
		return true, nil
	}

	var devicesCount int64
	if err := a.db.QueryRow(`SELECT COUNT(1) FROM user_devices WHERE user_id = ?`, userID).Scan(&devicesCount); err != nil {
		return false, err
	}
	if devicesCount >= maxDevices {
		return false, nil
	}

	if _, err := a.db.Exec(
		`INSERT INTO user_devices(user_id, hwid) VALUES(?, ?)`,
		userID,
		hwid,
	); err != nil {
		return false, err
	}
	return true, nil
}
