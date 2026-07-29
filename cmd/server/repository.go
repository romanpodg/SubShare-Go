package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"subshare/internal/model"
	"subshare/internal/vless"
)

var validSQLIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func clientDisplayNameFromKeyURL(rawURL, fallback string) string {
	rawURL = strings.TrimSpace(rawURL)
	fallback = strings.TrimSpace(fallback)
	if rawURL == "" {
		return fallback
	}

	switch supportedConfigScheme(rawURL) {
	case "vless", "vmess", "trojan":
		draft, err := parseLinkConfiguration(rawURL)
		if err != nil {
			return fallback
		}
		name := strings.TrimSpace(firstNonEmpty(draft.Remark, draft.ServerDescription, fallback))
		if name != "" {
			return name
		}
	case "xray-json":
		var parsed any
		if err := json.Unmarshal([]byte(rawURL), &parsed); err != nil {
			return fallback
		}
		switch typed := parsed.(type) {
		case map[string]any:
			name := strings.TrimSpace(extractJSONSubscriptionLabel(typed, fallback))
			if name != "" {
				return name
			}
		case []any:
			for _, item := range typed {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				name := strings.TrimSpace(extractJSONSubscriptionLabel(obj, fallback))
				if name != "" {
					return name
				}
			}
		}
	}

	return fallback
}

func migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT,
			token TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (
			id TEXT PRIMARY KEY,
			admin_id INTEGER NOT NULL DEFAULT 1,
			csrf_token TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'super_admin',
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
		`CREATE TABLE IF NOT EXISTS subscription_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			title TEXT,
			refresh_hours INTEGER NOT NULL DEFAULT 12,
			info_url TEXT,
			extra_url TEXT,
			extra_status TEXT,
			subscription_format TEXT NOT NULL DEFAULT 'links',
			provider_id TEXT,
			happ_no_limit_mode INTEGER NOT NULL DEFAULT 0,
			happ_no_limit_mode_xhttp_only INTEGER NOT NULL DEFAULT 0,
			happ_mandatory_hwid INTEGER NOT NULL DEFAULT 0,
			happ_notify_expiration INTEGER NOT NULL DEFAULT 0,
			happ_hide_server_settings INTEGER NOT NULL DEFAULT 0,
			happ_subscription_body TEXT,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS panel_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			panel_title TEXT NOT NULL DEFAULT 'SubShare',
			logo_data TEXT NOT NULL DEFAULT '',
			favicon_data TEXT NOT NULL DEFAULT '',
			page_title_admin TEXT NOT NULL DEFAULT 'Панель управления — SubShare',
			page_title_admin_login TEXT NOT NULL DEFAULT 'Вход — SubShare',
			page_title_subscription TEXT NOT NULL DEFAULT 'VPN-подписка — SubShare',
			subscription_page_config TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS routing_settings (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				config_json TEXT NOT NULL DEFAULT '',
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);`,
		`CREATE TABLE IF NOT EXISTS external_subscription_sources (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT NOT NULL,
					category TEXT NOT NULL DEFAULT 'general',
					key_category TEXT NOT NULL DEFAULT '',
					key_insert_mode TEXT NOT NULL DEFAULT 'bottom',
					source_url TEXT NOT NULL UNIQUE,
					enabled INTEGER NOT NULL DEFAULT 1,
					apply_remote_metadata INTEGER NOT NULL DEFAULT 1,
					pass_hwid INTEGER NOT NULL DEFAULT 0,
					hwid_version TEXT,
					hwid_model_name TEXT,
					hwid_value TEXT,
					last_import_count INTEGER NOT NULL DEFAULT 0,
					import_status TEXT NOT NULL DEFAULT 'idle',
					last_error TEXT,
					last_synced_at DATETIME,
				meta_title TEXT,
				meta_refresh_hours INTEGER,
				meta_support_url TEXT,
				meta_web_page_url TEXT,
				meta_announce TEXT,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);`,
		`CREATE TABLE IF NOT EXISTS external_source_categories (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name TEXT NOT NULL UNIQUE,
						created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
						updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
					);`,
		`CREATE TABLE IF NOT EXISTS key_categories (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name TEXT NOT NULL UNIQUE,
						color TEXT NOT NULL DEFAULT '#d8b33d',
						sort_order INTEGER NOT NULL DEFAULT 0,
						created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
						updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
					);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	if err := ensureColumn(db, "admins", "role", "TEXT NOT NULL DEFAULT 'super_admin'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "admin_sessions", "admin_id", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
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
	if err := ensureColumn(db, "users", "subscription_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "subscription_refresh_hours", "INTEGER NOT NULL DEFAULT 12"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "subscription_info_url", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "subscription_extra_url", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "subscription_extra_status", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "time_zone", "TEXT NOT NULL DEFAULT 'Europe/Moscow'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "language", "TEXT NOT NULL DEFAULT 'ru'"); err != nil {
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
	if err := ensureColumn(db, "vless_keys", "key_kind", "TEXT NOT NULL DEFAULT 'real'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "template_text", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "sort_order", "INTEGER NOT NULL DEFAULT 0"); err != nil {
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
	if err := ensureColumn(db, "vless_keys", "external_source_id", "INTEGER"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "external_key_ref", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "vless_keys", "category", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "key_categories", "color", "TEXT NOT NULL DEFAULT '#d8b33d'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "key_categories", "sort_order", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "external_subscription_sources", "key_category", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "external_subscription_sources", "key_insert_mode", "TEXT NOT NULL DEFAULT 'bottom'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "external_subscription_sources", "pass_hwid", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "external_subscription_sources", "hwid_version", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "external_subscription_sources", "hwid_model_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "external_subscription_sources", "hwid_value", "TEXT"); err != nil {
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
	if err := ensureColumn(db, "user_devices", "normalized_hwid", "TEXT"); err != nil {
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
	if _, err := db.Exec(`UPDATE users SET max_devices = 1 WHERE max_devices IS NULL OR max_devices < 1`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET subscription_refresh_hours = 12 WHERE subscription_refresh_hours IS NULL OR subscription_refresh_hours < 1`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET time_zone = 'Europe/Moscow' WHERE time_zone IS NULL OR TRIM(time_zone) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE users SET language = 'ru' WHERE language IS NULL OR TRIM(language) = ''`); err != nil {
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
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_devices_user_norm ON user_devices(user_id, normalized_hwid)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_keys_key_id ON user_keys(key_id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE user_devices SET normalized_hwid = LOWER(TRIM(hwid)) WHERE normalized_hwid IS NULL OR TRIM(normalized_hwid) = ''`); err != nil {
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
	if _, err := db.Exec(`UPDATE vless_keys SET key_kind = 'real' WHERE key_kind IS NULL OR TRIM(key_kind) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET sort_order = id WHERE sort_order IS NULL OR sort_order <= 0`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET category = TRIM(COALESCE(category, ''))`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE vless_keys SET starts_at = created_at WHERE starts_at IS NULL`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_vless_keys_kind_sort ON vless_keys(key_kind, sort_order, id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_vless_keys_category_sort ON vless_keys(category, sort_order, id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_vless_keys_external_source ON vless_keys(external_source_id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_vless_keys_external_source_ref ON vless_keys(external_source_id, external_key_ref)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_external_subscription_sources_category ON external_subscription_sources(category, id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_external_source_categories_name ON external_source_categories(name)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_key_categories_name ON key_categories(name)`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO subscription_settings(id, title, refresh_hours) VALUES(1, 'AllKeys', 12)`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO panel_settings(id) VALUES(1)`); err != nil {
		return err
	}
	if err := ensureColumn(db, "panel_settings", "subscription_page_config", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO routing_settings(id, config_json) VALUES(1, '')`); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "time_zone", "TEXT NOT NULL DEFAULT 'Europe/Moscow'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "language", "TEXT NOT NULL DEFAULT 'ru'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "subscription_format", "TEXT NOT NULL DEFAULT 'links'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "provider_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "happ_no_limit_mode", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "happ_no_limit_mode_xhttp_only", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "happ_mandatory_hwid", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "happ_notify_expiration", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "happ_hide_server_settings", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "subscription_settings", "happ_subscription_body", "TEXT"); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE subscription_settings SET refresh_hours = 12 WHERE refresh_hours < 1`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE subscription_settings SET time_zone = 'Europe/Moscow' WHERE time_zone IS NULL OR TRIM(time_zone) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE subscription_settings SET language = 'ru' WHERE language IS NULL OR TRIM(language) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE external_subscription_sources SET category = 'Общее' WHERE category IS NULL OR TRIM(category) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE external_subscription_sources SET category = 'Общее' WHERE LOWER(TRIM(category)) IN ('general', 'default')`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO external_source_categories(name) VALUES ('Общее')`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE key_categories SET color = '#d8b33d' WHERE color IS NULL OR TRIM(color) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE key_categories SET sort_order = id WHERE sort_order IS NULL OR sort_order <= 0`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO external_source_categories(name)
		SELECT DISTINCT TRIM(category)
		FROM external_subscription_sources
		WHERE TRIM(category) <> ''
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO key_categories(name)
		SELECT DISTINCT TRIM(category)
		FROM vless_keys
		WHERE TRIM(category) <> ''
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE external_subscription_sources SET key_category = category WHERE key_category IS NULL OR TRIM(key_category) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE external_subscription_sources SET key_category = TRIM(COALESCE(key_category, '')) WHERE key_category != TRIM(COALESCE(key_category, ''))`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE external_subscription_sources SET key_insert_mode = 'bottom' WHERE LOWER(TRIM(COALESCE(key_insert_mode, ''))) NOT IN ('top','bottom')`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE subscription_settings SET subscription_format = 'links' WHERE subscription_format IS NULL OR TRIM(subscription_format) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE subscription_settings SET subscription_format = 'links' WHERE LOWER(TRIM(subscription_format)) NOT IN ('links','xray-json')`); err != nil {
		return err
	}
	return runVersionedMigrations(db)
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
			u.id, u.name, u.email, u.token,
			COALESCE(NULLIF(TRIM(u.time_zone), ''), 'Europe/Moscow') AS time_zone,
			COALESCE(NULLIF(TRIM(u.language), ''), 'ru') AS language,
			u.activation_code, u.subscription_id,
			u.subscription_name, COALESCE(NULLIF(u.subscription_refresh_hours, 0), 12) AS subscription_refresh_hours,
			u.subscription_info_url, u.subscription_extra_url, u.subscription_extra_status,
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
			&u.CreatedAt,
			&assignedKeyIDs,
		); err != nil {
			return nil, err
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
		if activationUsedAt.Valid {
			u.ActivationUsedAt = activationUsedAt.Time.Local().Format("02/01/2006 15:04")
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
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return out, nil
	}

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
			return nil, err
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
		SELECT k.id, k.label, k.url, k.category_id, COALESCE(kc.name, k.category), k.key_kind, k.template_text, k.status, k.check_status, k.check_error, k.last_checked_at, k.last_latency_ms, k.created_at, k.external_source_id, COALESCE(es.name, '')
		FROM vless_keys k
		LEFT JOIN key_categories kc ON kc.id = k.category_id
		LEFT JOIN external_subscription_sources es ON es.id = k.external_source_id
		ORDER BY k.sort_order, k.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.VLESSKey
	for rows.Next() {
		var key model.VLESSKey
		var category sql.NullString
		var kind sql.NullString
		var templateText sql.NullString
		var status sql.NullString
		var checkStatus sql.NullString
		var checkError sql.NullString
		var lastCheckedAt sql.NullTime
		var latency sql.NullInt64
		var categoryID sql.NullInt64
		var externalSourceID sql.NullInt64
		var externalSourceName sql.NullString
		if err := rows.Scan(&key.ID, &key.Label, &key.URL, &categoryID, &category, &kind, &templateText, &status, &checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt, &externalSourceID, &externalSourceName); err != nil {
			return nil, err
		}
		if categoryID.Valid {
			key.CategoryID = categoryID.Int64
		}
		key.Category = strings.TrimSpace(category.String)
		key.Kind, _ = model.NormalizeKeyKind(kind.String)
		if key.Kind == "" {
			key.Kind = model.KeyKindReal
		}
		key.TemplateText = strings.TrimSpace(templateText.String)
		key.Status, _ = model.NormalizeKeyStatus(status.String)
		if key.Status == "" {
			key.Status = model.KeyStatusActive
		}
		key.StatusLabel = model.KeyStatusLabel(key.Status)
		if key.Kind == model.KeyKindInformational {
			key.URLShort = "Информационный ключ"
			if key.TemplateText != "" {
				key.URLShort = vless.TruncateMiddle(key.TemplateText, 88)
			}
		} else {
			key.URLShort = vless.TruncateMiddle(key.URL, 88)
		}
		key.CheckStatus = model.NormalizeCheckStatus(checkStatus.String)
		key.CheckStatusLabel = model.CheckStatusLabel(key.CheckStatus)
		key.CheckError = strings.TrimSpace(checkError.String)
		if key.Kind == model.KeyKindReal {
			key.EditUUID, key.EditHost, key.EditPort, key.EditQuery, key.EditFragment, _ = vless.ParseVLESSParts(key.URL)
		}
		if latency.Valid {
			key.LastLatencyMS = latency.Int64
		}
		if lastCheckedAt.Valid {
			key.LastCheckedAtText = lastCheckedAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		if externalSourceID.Valid && externalSourceID.Int64 > 0 {
			key.ExternalSourceID = externalSourceID.Int64
		}
		key.ExternalSourceName = strings.TrimSpace(externalSourceName.String)
		key.ClientDisplayName = clientDisplayNameFromKeyURL(key.URL, key.Label)
		out = append(out, key)
	}
	return out, rows.Err()
}

func (a *App) getSubscriptionSettings() (model.SubscriptionSettings, error) {
	var title sql.NullString
	var refreshHours sql.NullInt64
	var infoURL sql.NullString
	var extraURL sql.NullString
	var extraStatus sql.NullString
	var subscriptionFormat sql.NullString
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
		`SELECT title, refresh_hours, info_url, extra_url, extra_status, subscription_format, time_zone, language,
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
		Title:                    strings.TrimSpace(title.String),
		RefreshHours:             12,
		InfoURL:                  strings.TrimSpace(infoURL.String),
		ExtraURL:                 strings.TrimSpace(extraURL.String),
		ExtraStatus:              strings.TrimSpace(extraStatus.String),
		SubscriptionFormat:       strings.TrimSpace(subscriptionFormat.String),
		TimeZone:                 strings.TrimSpace(timeZone.String),
		Language:                 strings.TrimSpace(language.String),
		ProviderID:               strings.TrimSpace(providerID.String),
		HappNoLimitMode:          happNoLimitMode.Valid && happNoLimitMode.Int64 != 0,
		HappNoLimitModeXHTTPOnly: happNoLimitModeXHTTPOnly.Valid && happNoLimitModeXHTTPOnly.Int64 != 0,
		HappMandatoryHWID:        happMandatoryHWID.Valid && happMandatoryHWID.Int64 != 0,
		HappNotifyExpiration:     happNotifyExpiration.Valid && happNotifyExpiration.Int64 != 0,
		HappHideServerSettings:   happHideServerSettings.Valid && happHideServerSettings.Int64 != 0,
		HappSubscriptionBody:     happSubscriptionBody.String,
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
	var configJSON sql.NullString
	if err := a.db.QueryRow(`SELECT config_json FROM routing_settings WHERE id = 1`).Scan(&configJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RoutingSettings{ConfigJSON: ""}, nil
		}
		return model.RoutingSettings{}, err
	}

	return model.RoutingSettings{
		ConfigJSON: strings.TrimSpace(configJSON.String),
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
		`INSERT INTO routing_settings(id, config_json, updated_at)
		 VALUES(1, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET config_json = excluded.config_json, updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(s.ConfigJSON),
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
