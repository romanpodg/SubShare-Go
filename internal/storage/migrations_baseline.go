package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

var validSQLIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// statement or an ensureColumn spec.
type baselineStep struct {
	sql                      string
	table, column, columnDef string
}

// applyBaselineSchema is the pre-versioned bootstrap, kept verbatim as migration 0.
// baselineSchemaQueries creates the tables of the pre-versioned schema.
var baselineSchemaQueries = []string{
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
		client_display_name TEXT,
		url TEXT NOT NULL,
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
		show_subscription_expiration INTEGER NOT NULL DEFAULT 0,
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
			delivery_mode TEXT NOT NULL DEFAULT 'disabled' CHECK (delivery_mode IN ('disabled', 'add', 'onadd')),
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

// baselineSchemaSteps are the ordered column additions and data fix-ups; a
// statement runs verbatim, a column spec goes through ensureColumn.
var baselineSchemaSteps = []baselineStep{
	{table: "admins", column: "role", columnDef: "TEXT NOT NULL DEFAULT 'super_admin'"},
	{table: "admin_sessions", column: "admin_id", columnDef: "INTEGER NOT NULL DEFAULT 1"},
	{table: "users", column: "status", columnDef: "TEXT NOT NULL DEFAULT 'active'"},
	{table: "users", column: "starts_at", columnDef: "DATETIME"},
	{table: "users", column: "expires_at", columnDef: "DATETIME"},
	{table: "users", column: "blocked_reason", columnDef: "TEXT"},
	{table: "users", column: "activation_code", columnDef: "TEXT"},
	{table: "users", column: "activation_used_at", columnDef: "DATETIME"},
	{table: "users", column: "subscription_id", columnDef: "TEXT"},
	{table: "users", column: "max_devices", columnDef: "INTEGER NOT NULL DEFAULT 1"},
	{table: "users", column: "subscription_name", columnDef: "TEXT"},
	{table: "users", column: "subscription_refresh_hours", columnDef: "INTEGER NOT NULL DEFAULT 12"},
	{table: "users", column: "subscription_info_url", columnDef: "TEXT"},
	{table: "users", column: "subscription_extra_url", columnDef: "TEXT"},
	{table: "users", column: "subscription_extra_status", columnDef: "TEXT"},
	{table: "users", column: "time_zone", columnDef: "TEXT NOT NULL DEFAULT 'Europe/Moscow'"},
	{table: "users", column: "language", columnDef: "TEXT NOT NULL DEFAULT 'ru'"},
	{table: "vless_keys", column: "check_status", columnDef: "TEXT NOT NULL DEFAULT 'unknown'"},
	{table: "vless_keys", column: "check_error", columnDef: "TEXT"},
	{table: "vless_keys", column: "last_checked_at", columnDef: "DATETIME"},
	{table: "vless_keys", column: "last_latency_ms", columnDef: "INTEGER"},
	{table: "vless_keys", column: "status", columnDef: "TEXT NOT NULL DEFAULT 'active'"},
	{table: "vless_keys", column: "key_kind", columnDef: "TEXT NOT NULL DEFAULT 'real'"},
	{table: "vless_keys", column: "template_text", columnDef: "TEXT"},
	{table: "vless_keys", column: "sort_order", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "vless_keys", column: "starts_at", columnDef: "DATETIME"},
	{table: "vless_keys", column: "expires_at", columnDef: "DATETIME"},
	{table: "vless_keys", column: "blocked_reason", columnDef: "TEXT"},
	{table: "vless_keys", column: "external_source_id", columnDef: "INTEGER"},
	{table: "vless_keys", column: "external_key_ref", columnDef: "TEXT"},
	{table: "vless_keys", column: "category", columnDef: "TEXT NOT NULL DEFAULT ''"},
	{table: "vless_keys", column: "profile_revision", columnDef: "INTEGER NOT NULL DEFAULT 1"},
	{table: "vless_keys", column: "updated_at", columnDef: "DATETIME"},
	{table: "vless_keys", column: "client_display_name", columnDef: "TEXT"},
	{table: "key_categories", column: "color", columnDef: "TEXT NOT NULL DEFAULT '#d8b33d'"},
	{table: "key_categories", column: "sort_order", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "external_subscription_sources", column: "key_category", columnDef: "TEXT NOT NULL DEFAULT ''"},
	{table: "external_subscription_sources", column: "key_insert_mode", columnDef: "TEXT NOT NULL DEFAULT 'bottom'"},
	{table: "external_subscription_sources", column: "pass_hwid", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "external_subscription_sources", column: "hwid_version", columnDef: "TEXT"},
	{table: "external_subscription_sources", column: "hwid_model_name", columnDef: "TEXT"},
	{table: "external_subscription_sources", column: "hwid_value", columnDef: "TEXT"},
	{table: "user_devices", column: "device_name", columnDef: "TEXT"},
	{table: "user_devices", column: "device_model", columnDef: "TEXT"},
	{table: "user_devices", column: "platform", columnDef: "TEXT"},
	{table: "user_devices", column: "os_version", columnDef: "TEXT"},
	{table: "user_devices", column: "app_name", columnDef: "TEXT"},
	{table: "user_devices", column: "app_version", columnDef: "TEXT"},
	{table: "user_devices", column: "user_agent", columnDef: "TEXT"},
	{table: "user_devices", column: "normalized_hwid", columnDef: "TEXT"},
	{sql: `UPDATE users SET status = 'active' WHERE status IS NULL OR TRIM(status) = ''`},
	{sql: `UPDATE users SET activation_code = token WHERE activation_code IS NULL OR TRIM(activation_code) = ''`},
	{sql: `UPDATE users SET subscription_id = token WHERE subscription_id IS NULL OR TRIM(subscription_id) = ''`},
	{sql: `UPDATE users SET max_devices = 1 WHERE max_devices IS NULL OR max_devices < 0`},
	{sql: `UPDATE users SET subscription_refresh_hours = 12 WHERE subscription_refresh_hours IS NULL OR subscription_refresh_hours < 1`},
	{sql: `UPDATE users SET time_zone = 'Europe/Moscow' WHERE time_zone IS NULL OR TRIM(time_zone) = ''`},
	{sql: `UPDATE users SET language = 'ru' WHERE language IS NULL OR TRIM(language) = ''`},
	{sql: `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_activation_code ON users(activation_code)`},
	{sql: `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_subscription_id ON users(subscription_id)`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_user_devices_user_id ON user_devices(user_id)`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_user_devices_user_norm ON user_devices(user_id, normalized_hwid)`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_user_keys_key_id ON user_keys(key_id)`},
	{sql: `UPDATE user_devices SET normalized_hwid = LOWER(TRIM(hwid)) WHERE normalized_hwid IS NULL OR TRIM(normalized_hwid) = ''`},
	{sql: `UPDATE vless_keys SET check_status = 'unknown' WHERE check_status IS NULL OR TRIM(check_status) = ''`},
	{sql: `UPDATE vless_keys SET status = 'non-active' WHERE LOWER(TRIM(status)) = 'blocked'`},
	{sql: `UPDATE vless_keys SET status = 'active' WHERE status IS NULL OR TRIM(status) = ''`},
	{sql: `UPDATE vless_keys SET key_kind = 'real' WHERE key_kind IS NULL OR TRIM(key_kind) = ''`},
	{sql: `UPDATE vless_keys SET sort_order = id WHERE sort_order IS NULL OR sort_order <= 0`},
	{sql: `UPDATE vless_keys SET category = TRIM(COALESCE(category, ''))`},
	{sql: `UPDATE vless_keys SET starts_at = created_at WHERE starts_at IS NULL`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_vless_keys_kind_sort ON vless_keys(key_kind, sort_order, id)`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_vless_keys_category_sort ON vless_keys(category, sort_order, id)`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_vless_keys_external_source ON vless_keys(external_source_id)`},
	{sql: `CREATE UNIQUE INDEX IF NOT EXISTS idx_vless_keys_external_source_ref ON vless_keys(external_source_id, external_key_ref)`},
	{sql: `CREATE INDEX IF NOT EXISTS idx_external_subscription_sources_category ON external_subscription_sources(category, id)`},
	{sql: `CREATE UNIQUE INDEX IF NOT EXISTS idx_external_source_categories_name ON external_source_categories(name)`},
	{sql: `CREATE UNIQUE INDEX IF NOT EXISTS idx_key_categories_name ON key_categories(name)`},
	{sql: `INSERT OR IGNORE INTO subscription_settings(id, title, refresh_hours) VALUES(1, 'AllKeys', 12)`},
	{sql: `INSERT OR IGNORE INTO panel_settings(id) VALUES(1)`},
	{table: "panel_settings", column: "subscription_page_config", columnDef: "TEXT NOT NULL DEFAULT ''"},
	{sql: `INSERT OR IGNORE INTO routing_settings(id, config_json) VALUES(1, '')`},
	{table: "subscription_settings", column: "time_zone", columnDef: "TEXT NOT NULL DEFAULT 'Europe/Moscow'"},
	{table: "subscription_settings", column: "language", columnDef: "TEXT NOT NULL DEFAULT 'ru'"},
	{table: "subscription_settings", column: "subscription_format", columnDef: "TEXT NOT NULL DEFAULT 'links'"},
	{table: "subscription_settings", column: "show_subscription_expiration", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "subscription_settings", column: "provider_id", columnDef: "TEXT"},
	{table: "subscription_settings", column: "happ_no_limit_mode", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "subscription_settings", column: "happ_no_limit_mode_xhttp_only", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "subscription_settings", column: "happ_mandatory_hwid", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "subscription_settings", column: "happ_notify_expiration", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "subscription_settings", column: "happ_hide_server_settings", columnDef: "INTEGER NOT NULL DEFAULT 0"},
	{table: "subscription_settings", column: "happ_subscription_body", columnDef: "TEXT"},
	{sql: `UPDATE subscription_settings SET refresh_hours = 12 WHERE refresh_hours < 1`},
	{sql: `UPDATE subscription_settings SET time_zone = 'Europe/Moscow' WHERE time_zone IS NULL OR TRIM(time_zone) = ''`},
	{sql: `UPDATE subscription_settings SET language = 'ru' WHERE language IS NULL OR TRIM(language) = ''`},
	{sql: `UPDATE external_subscription_sources SET category = 'Общее' WHERE category IS NULL OR TRIM(category) = ''`},
	{sql: `UPDATE external_subscription_sources SET category = 'Общее' WHERE LOWER(TRIM(category)) IN ('general', 'default')`},
	{sql: `INSERT OR IGNORE INTO external_source_categories(name) VALUES ('Общее')`},
	{sql: `UPDATE key_categories SET color = '#d8b33d' WHERE color IS NULL OR TRIM(color) = ''`},
	{sql: `UPDATE key_categories SET sort_order = id WHERE sort_order IS NULL OR sort_order <= 0`},
	{sql: `
	INSERT OR IGNORE INTO external_source_categories(name)
	SELECT DISTINCT TRIM(category)
	FROM external_subscription_sources
	WHERE TRIM(category) <> ''
`},
	{sql: `
	INSERT OR IGNORE INTO key_categories(name)
	SELECT DISTINCT TRIM(category)
	FROM vless_keys
	WHERE TRIM(category) <> ''
`},
	{sql: `UPDATE external_subscription_sources SET key_category = category WHERE key_category IS NULL OR TRIM(key_category) = ''`},
	{sql: `UPDATE external_subscription_sources SET key_category = TRIM(COALESCE(key_category, '')) WHERE key_category != TRIM(COALESCE(key_category, ''))`},
	{sql: `UPDATE external_subscription_sources SET key_insert_mode = 'bottom' WHERE LOWER(TRIM(COALESCE(key_insert_mode, ''))) NOT IN ('top','bottom')`},
	{sql: `UPDATE subscription_settings SET subscription_format = 'links' WHERE subscription_format IS NULL OR TRIM(subscription_format) = ''`},
	{sql: `UPDATE subscription_settings SET subscription_format = 'links' WHERE LOWER(TRIM(subscription_format)) NOT IN ('links','xray-json')`},
}

func applyBaselineSchema(_ context.Context, _ *sql.Conn, db *sql.DB, _ *profilestorage.Keyring) error {
	for _, q := range baselineSchemaQueries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	for _, step := range baselineSchemaSteps {
		var err error
		if step.sql != "" {
			_, err = db.Exec(step.sql)
		} else {
			err = ensureColumn(db, step.table, step.column, step.columnDef)
		}
		if err != nil {
			return err
		}
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
