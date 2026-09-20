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

type schemaMigration struct {
	Version            int
	Name               string
	baseline           bool
	statements         []string
	disableForeignKeys bool
	verifyForeignKeys  bool
	runGo              func(ctx context.Context, conn *sql.Conn, db *sql.DB, keyring *profilestorage.Keyring) error
}

// SchemaMigrations is the single ordered definition of the database schema.
// Version 0 is the pre-versioned baseline: it runs only for databases that have
// no schema_migrations table yet (fresh databases and legacy installs).
var SchemaMigrations = []schemaMigration{
	{
		Version:  0,
		Name:     "baseline_schema",
		baseline: true,
		runGo:    applyBaselineSchema,
	},
	{
		Version: 1,
		Name:    "subscription_hub_foundation",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS audit_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				actor_admin_id INTEGER,
				action TEXT NOT NULL,
				target_type TEXT NOT NULL,
				target_id TEXT NOT NULL DEFAULT '',
				metadata_json TEXT NOT NULL DEFAULT '{}',
				request_id TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (actor_admin_id) REFERENCES admins(id) ON DELETE SET NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events(created_at DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_events_target ON audit_events(target_type, target_id)`,
			`CREATE TABLE IF NOT EXISTS subscription_templates (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				format TEXT NOT NULL CHECK (format IN ('base64','plain','xray-json','mihomo','sing-box')),
				content TEXT NOT NULL DEFAULT '',
				enabled INTEGER NOT NULL DEFAULT 1,
				is_system INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS response_rules (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				enabled INTEGER NOT NULL DEFAULT 1,
				priority INTEGER NOT NULL DEFAULT 100,
				operator TEXT NOT NULL DEFAULT 'AND' CHECK (operator IN ('AND','OR')),
				conditions_json TEXT NOT NULL DEFAULT '[]',
				response_type TEXT NOT NULL CHECK (response_type IN ('browser','base64','plain','xray-json','mihomo','sing-box','block','not-found')),
				template_id INTEGER,
				headers_json TEXT NOT NULL DEFAULT '[]',
				is_system INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (template_id) REFERENCES subscription_templates(id) ON DELETE SET NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_response_rules_order ON response_rules(enabled, priority, id)`,
			`CREATE TABLE IF NOT EXISTS background_jobs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				kind TEXT NOT NULL,
				status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','succeeded_with_warnings','failed')),
				target_type TEXT NOT NULL DEFAULT '',
				target_id TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				result_counts_json TEXT NOT NULL DEFAULT '{}',
				run_after DATETIME,
				started_at DATETIME,
				finished_at DATETIME,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_background_jobs_status ON background_jobs(status, run_after, id)`,
			`CREATE TABLE IF NOT EXISTS source_sync_runs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source_id INTEGER NOT NULL,
				status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
				imported_count INTEGER NOT NULL DEFAULT 0,
				skipped_count INTEGER NOT NULL DEFAULT 0,
				error_message TEXT NOT NULL DEFAULT '',
				started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				finished_at DATETIME,
				FOREIGN KEY (source_id) REFERENCES external_subscription_sources(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_source_sync_runs_source ON source_sync_runs(source_id, started_at DESC)`,
			`CREATE TABLE IF NOT EXISTS api_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				token_prefix TEXT NOT NULL,
				token_hash TEXT NOT NULL UNIQUE,
				scopes_json TEXT NOT NULL DEFAULT '[]',
				expires_at DATETIME,
				last_used_at DATETIME,
				created_by_admin_id INTEGER,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				revoked_at DATETIME,
				FOREIGN KEY (created_by_admin_id) REFERENCES admins(id) ON DELETE SET NULL
			)`,
			`CREATE TABLE IF NOT EXISTS subscription_metrics (
				day TEXT NOT NULL,
				response_type TEXT NOT NULL,
				result_status TEXT NOT NULL,
				requests INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (day, response_type, result_status)
			)`,
		},
	},
	{
		Version: 2,
		Name:    "seed_default_templates_and_rules",
		statements: []string{
			`INSERT OR IGNORE INTO subscription_templates(slug, name, format, content, enabled, is_system)
			 VALUES ('default-base64', 'Base64 fallback', 'base64', '', 1, 1)`,
			`INSERT OR IGNORE INTO subscription_templates(slug, name, format, content, enabled, is_system)
			 VALUES ('default-xray', 'Xray JSON', 'xray-json', '', 1, 1)`,
			`INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json, is_system)
			 SELECT 'Browser subscription page', 'Render the public page for browser requests', 1, 10, 'AND',
			        '[{"headerName":"user-agent","operator":"CONTAINS","value":"mozilla","caseSensitive":false}]',
			        'browser', NULL, '[]', 1
			 WHERE NOT EXISTS (SELECT 1 FROM response_rules WHERE is_system = 1 AND response_type = 'browser')`,
			`INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json, is_system)
			 SELECT 'Happ clients', 'Serve Xray JSON to Happ clients', 1, 20, 'AND',
			        '[{"headerName":"user-agent","operator":"CONTAINS","value":"happ","caseSensitive":false}]',
			        'xray-json', id, '[]', 1
			 FROM subscription_templates
			 WHERE slug = 'default-xray'
			   AND NOT EXISTS (SELECT 1 FROM response_rules WHERE is_system = 1 AND name = 'Happ clients')`,
			`INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json, is_system)
			 SELECT 'Fallback', 'Default Base64 subscription response', 1, 1000, 'AND', '[]',
			        'base64', id, '[]', 1
			 FROM subscription_templates
			 WHERE slug = 'default-base64'
			   AND NOT EXISTS (SELECT 1 FROM response_rules WHERE is_system = 1 AND name = 'Fallback')`,
		},
	},
	{
		Version: 3,
		Name:    "owner_operator_viewer_roles",
		statements: []string{
			`UPDATE admins SET role = 'owner' WHERE role = 'super_admin'`,
			`UPDATE admins SET role = 'operator' WHERE role = 'support_admin'`,
		},
	},
	{
		Version: 4,
		Name:    "category_foreign_keys",
		statements: []string{
			`ALTER TABLE vless_keys ADD COLUMN category_id INTEGER REFERENCES key_categories(id) ON DELETE SET NULL`,
			`UPDATE vless_keys
			 SET category_id = (SELECT id FROM key_categories WHERE name = vless_keys.category)
			 WHERE TRIM(COALESCE(category, '')) <> ''`,
			`CREATE INDEX IF NOT EXISTS idx_vless_keys_category_id ON vless_keys(category_id, sort_order, id)`,
			`CREATE TRIGGER IF NOT EXISTS trg_vless_keys_category_insert
			 AFTER INSERT ON vless_keys
			 WHEN NEW.category_id IS NULL AND TRIM(COALESCE(NEW.category, '')) <> ''
			 BEGIN
			   UPDATE vless_keys
			      SET category_id = (SELECT id FROM key_categories WHERE name = NEW.category)
			    WHERE id = NEW.id;
			 END`,
			`CREATE TRIGGER IF NOT EXISTS trg_vless_keys_category_update
			 AFTER UPDATE OF category ON vless_keys
			 BEGIN
			   UPDATE vless_keys
			      SET category_id = (SELECT id FROM key_categories WHERE name = NEW.category)
			    WHERE id = NEW.id;
			 END`,
			`ALTER TABLE external_subscription_sources ADD COLUMN source_category_id INTEGER REFERENCES external_source_categories(id) ON DELETE SET NULL`,
			`ALTER TABLE external_subscription_sources ADD COLUMN key_category_id INTEGER REFERENCES key_categories(id) ON DELETE SET NULL`,
			`UPDATE external_subscription_sources
			 SET source_category_id = (SELECT id FROM external_source_categories WHERE name = external_subscription_sources.category),
			     key_category_id = (SELECT id FROM key_categories WHERE name = external_subscription_sources.key_category)`,
			`CREATE INDEX IF NOT EXISTS idx_external_sources_category_ids
			 ON external_subscription_sources(source_category_id, key_category_id, id)`,
			`CREATE TRIGGER IF NOT EXISTS trg_external_sources_categories_insert
			 AFTER INSERT ON external_subscription_sources
			 BEGIN
			   UPDATE external_subscription_sources
			      SET source_category_id = (SELECT id FROM external_source_categories WHERE name = NEW.category),
			          key_category_id = (SELECT id FROM key_categories WHERE name = NEW.key_category)
			    WHERE id = NEW.id;
			 END`,
			`CREATE TRIGGER IF NOT EXISTS trg_external_sources_categories_update
			 AFTER UPDATE OF category, key_category ON external_subscription_sources
			 BEGIN
			   UPDATE external_subscription_sources
			      SET source_category_id = (SELECT id FROM external_source_categories WHERE name = NEW.category),
			          key_category_id = (SELECT id FROM key_categories WHERE name = NEW.key_category)
			    WHERE id = NEW.id;
			 END`,
		},
	},
	{
		Version: 5,
		Name:    "client_format_system_rules",
		statements: []string{
			`INSERT OR IGNORE INTO subscription_templates(slug, name, format, content, enabled, is_system)
			 VALUES ('default-mihomo', 'Mihomo', 'mihomo', '', 1, 1)`,
			`INSERT OR IGNORE INTO subscription_templates(slug, name, format, content, enabled, is_system)
			 VALUES ('default-sing-box', 'Sing-box', 'sing-box', '', 1, 1)`,
			`INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json, is_system)
			 SELECT 'Mihomo and Clash clients', 'Serve a generated Mihomo configuration', 1, 30, 'AND',
			        '[{"headerName":"user-agent","operator":"REGEX","value":"(mihomo|clash|stash|meta)","caseSensitive":false}]',
			        'mihomo', id, '[]', 1
			 FROM subscription_templates
			 WHERE slug = 'default-mihomo'
			   AND NOT EXISTS (SELECT 1 FROM response_rules WHERE is_system = 1 AND name = 'Mihomo and Clash clients')`,
			`INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json, is_system)
			 SELECT 'Sing-box clients', 'Serve a generated Sing-box configuration', 1, 40, 'AND',
			        '[{"headerName":"user-agent","operator":"REGEX","value":"(sing-box|singbox)","caseSensitive":false}]',
			        'sing-box', id, '[]', 1
			 FROM subscription_templates
			 WHERE slug = 'default-sing-box'
			   AND NOT EXISTS (SELECT 1 FROM response_rules WHERE is_system = 1 AND name = 'Sing-box clients')`,
			`INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json, is_system)
			 SELECT 'Xray and v2ray clients', 'Serve Xray JSON', 1, 50, 'AND',
			        '[{"headerName":"user-agent","operator":"REGEX","value":"(xray|v2ray|v2rayng)","caseSensitive":false}]',
			        'xray-json', id, '[]', 1
			 FROM subscription_templates
			 WHERE slug = 'default-xray'
			   AND NOT EXISTS (SELECT 1 FROM response_rules WHERE is_system = 1 AND name = 'Xray and v2ray clients')`,
		},
	},
	{
		Version: 6,
		Name:    "subscription_delivery_settings",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS subscription_delivery_settings (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				response_headers_json TEXT NOT NULL DEFAULT '[]',
				announcement TEXT NOT NULL DEFAULT '',
				remarks_json TEXT NOT NULL DEFAULT '{}',
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`INSERT OR IGNORE INTO subscription_delivery_settings(id, response_headers_json, announcement, remarks_json)
			 VALUES(1, '[]', '', '{"expired":[],"paused":[],"blocked":[],"limited":[],"empty":[]}')`,
		},
	},
	{
		Version: 7,
		Name:    "subscription_entitlements_and_health_policy",
		statements: []string{
			`ALTER TABLE users ADD COLUMN key_assignment_mode TEXT NOT NULL DEFAULT 'all'
			 CHECK (key_assignment_mode IN ('all','selected'))`,
			`ALTER TABLE vless_keys ADD COLUMN health_failure_count INTEGER NOT NULL DEFAULT 0`,
			`UPDATE users
			 SET key_assignment_mode = CASE
			   WHEN (SELECT COUNT(*) FROM user_keys uk WHERE uk.user_id = users.id) <
			        (SELECT COUNT(*) FROM vless_keys)
			   THEN 'selected'
			   ELSE 'all'
			 END`,
			`UPDATE users
			    SET blocked_reason = NULLIF(TRIM(COALESCE(blocked_reason, '')), ''),
			        subscription_name = NULLIF(TRIM(COALESCE(subscription_name, '')), ''),
			        subscription_info_url = NULLIF(TRIM(COALESCE(subscription_info_url, '')), ''),
			        subscription_extra_url = NULLIF(TRIM(COALESCE(subscription_extra_url, '')), ''),
			        subscription_extra_status = NULLIF(TRIM(COALESCE(subscription_extra_status, '')), '')`,
			`UPDATE users
			 SET subscription_refresh_hours = 0
			 WHERE subscription_refresh_hours = 12
			   AND TRIM(COALESCE(subscription_name, '')) = ''
			   AND TRIM(COALESCE(subscription_info_url, '')) = ''
			   AND TRIM(COALESCE(subscription_extra_url, '')) = ''
			   AND TRIM(COALESCE(subscription_extra_status, '')) = ''`,
			`CREATE INDEX IF NOT EXISTS idx_users_key_assignment_mode
			 ON users(key_assignment_mode, id)`,
			`CREATE INDEX IF NOT EXISTS idx_vless_keys_delivery_health
			 ON vless_keys(status, key_kind, health_failure_count, sort_order, id)`,
		},
	},
	{
		Version: 8,
		Name:    "category_ids_as_canonical_source",
		statements: []string{
			`UPDATE vless_keys
			    SET category_id = (SELECT id FROM key_categories WHERE name = vless_keys.category)
			  WHERE category_id IS NULL AND TRIM(COALESCE(category, '')) <> ''`,
			`UPDATE external_subscription_sources
			    SET source_category_id = (SELECT id FROM external_source_categories WHERE name = external_subscription_sources.category),
			        key_category_id = (SELECT id FROM key_categories WHERE name = external_subscription_sources.key_category)
			  WHERE source_category_id IS NULL OR key_category_id IS NULL`,
			`DROP TRIGGER IF EXISTS trg_vless_keys_category_update`,
			`CREATE TRIGGER IF NOT EXISTS trg_vless_keys_category_id_insert
			 AFTER INSERT ON vless_keys
			 WHEN NEW.category_id IS NOT NULL
			 BEGIN
			   UPDATE vless_keys
			      SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '')
			    WHERE id = NEW.id;
			 END`,
			`CREATE TRIGGER IF NOT EXISTS trg_vless_keys_category_id_update
			 AFTER UPDATE OF category_id ON vless_keys
			 BEGIN
			   UPDATE vless_keys
			      SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '')
			    WHERE id = NEW.id;
			 END`,
			`CREATE TRIGGER IF NOT EXISTS trg_key_categories_name_compat
			 AFTER UPDATE OF name ON key_categories
			 BEGIN
			   UPDATE vless_keys SET category = NEW.name WHERE category_id = NEW.id;
			   UPDATE external_subscription_sources SET key_category = NEW.name WHERE key_category_id = NEW.id;
			 END`,
			`CREATE TRIGGER IF NOT EXISTS trg_external_source_category_name_compat
			 AFTER UPDATE OF name ON external_source_categories
			 BEGIN
			   UPDATE external_subscription_sources SET category = NEW.name WHERE source_category_id = NEW.id;
			 END`,
		},
	},
	{
		Version: 9,
		Name:    "external_protocol_profile_persistence",
		statements: []string{
			`ALTER TABLE vless_keys ADD COLUMN protocol TEXT NOT NULL DEFAULT 'legacy'`,
			`ALTER TABLE vless_keys ADD COLUMN profile_fingerprint TEXT`,
			`ALTER TABLE vless_keys ADD COLUMN profile_schema_version INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE vless_keys ADD COLUMN profile_compatibility TEXT NOT NULL DEFAULT 'legacy'`,
			`ALTER TABLE vless_keys ADD COLUMN profile_warnings_json TEXT NOT NULL DEFAULT '[]'`,
			`CREATE INDEX IF NOT EXISTS idx_vless_keys_external_source_fingerprint
			 ON vless_keys(external_source_id, profile_fingerprint)`,
			`ALTER TABLE source_sync_runs ADD COLUMN result_counts_json TEXT NOT NULL DEFAULT '{}'`,
		},
	},
	{
		Version:            10,
		Name:               "source_owned_profile_urls",
		disableForeignKeys: true,
		verifyForeignKeys:  true,
		statements: []string{
			`CREATE TABLE vless_keys_source_owned (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				label TEXT NOT NULL,
				url TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				check_status TEXT NOT NULL DEFAULT 'unknown',
				check_error TEXT,
				last_checked_at DATETIME,
				last_latency_ms INTEGER,
				status TEXT NOT NULL DEFAULT 'active',
				key_kind TEXT NOT NULL DEFAULT 'real',
				template_text TEXT,
				sort_order INTEGER NOT NULL DEFAULT 0,
				starts_at DATETIME,
				expires_at DATETIME,
				blocked_reason TEXT,
				external_source_id INTEGER,
				external_key_ref TEXT,
				category TEXT NOT NULL DEFAULT '',
				category_id INTEGER REFERENCES key_categories(id) ON DELETE SET NULL,
				health_failure_count INTEGER NOT NULL DEFAULT 0,
				protocol TEXT NOT NULL DEFAULT 'legacy',
				profile_fingerprint TEXT,
				profile_schema_version INTEGER NOT NULL DEFAULT 0,
				profile_compatibility TEXT NOT NULL DEFAULT 'legacy',
				profile_warnings_json TEXT NOT NULL DEFAULT '[]'
			)`,
			`INSERT INTO vless_keys_source_owned(
				id, label, url, created_at, check_status, check_error, last_checked_at, last_latency_ms,
				status, key_kind, template_text, sort_order, starts_at, expires_at, blocked_reason,
				external_source_id, external_key_ref, category, category_id, health_failure_count,
				protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json
			)
			SELECT id, label, url, created_at, check_status, check_error, last_checked_at, last_latency_ms,
			       status, key_kind, template_text, sort_order, starts_at, expires_at, blocked_reason,
			       external_source_id, external_key_ref, category, category_id, health_failure_count,
			       protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json
			  FROM vless_keys`,
			`DROP TRIGGER IF EXISTS trg_key_categories_name_compat`,
			`DROP TABLE vless_keys`,
			`ALTER TABLE vless_keys_source_owned RENAME TO vless_keys`,
			`CREATE INDEX idx_vless_keys_url ON vless_keys(url)`,
			`CREATE UNIQUE INDEX idx_vless_keys_local_url
				 ON vless_keys(url) WHERE external_source_id IS NULL`,
			`CREATE INDEX idx_vless_keys_kind_sort ON vless_keys(key_kind, sort_order, id)`,
			`CREATE INDEX idx_vless_keys_category_sort ON vless_keys(category, sort_order, id)`,
			`CREATE INDEX idx_vless_keys_external_source ON vless_keys(external_source_id)`,
			`CREATE UNIQUE INDEX idx_vless_keys_external_source_ref
				 ON vless_keys(external_source_id, external_key_ref)`,
			`CREATE INDEX idx_vless_keys_category_id ON vless_keys(category_id, sort_order, id)`,
			`CREATE INDEX idx_vless_keys_delivery_health
				 ON vless_keys(status, key_kind, health_failure_count, sort_order, id)`,
			`CREATE UNIQUE INDEX idx_vless_keys_external_source_fingerprint
				 ON vless_keys(external_source_id, profile_fingerprint)
				 WHERE external_source_id IS NOT NULL
				   AND profile_fingerprint IS NOT NULL
				   AND TRIM(profile_fingerprint) <> ''`,
			`CREATE TRIGGER trg_vless_keys_category_insert
				 AFTER INSERT ON vless_keys
				 WHEN NEW.category_id IS NULL AND TRIM(COALESCE(NEW.category, '')) <> ''
				 BEGIN
				   UPDATE vless_keys
				      SET category_id = (SELECT id FROM key_categories WHERE name = NEW.category)
				    WHERE id = NEW.id;
				 END`,
			`CREATE TRIGGER trg_vless_keys_category_id_insert
				 AFTER INSERT ON vless_keys
				 WHEN NEW.category_id IS NOT NULL
				 BEGIN
				   UPDATE vless_keys
				      SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '')
				    WHERE id = NEW.id;
				 END`,
			`CREATE TRIGGER trg_vless_keys_category_id_update
				 AFTER UPDATE OF category_id ON vless_keys
				 BEGIN
				   UPDATE vless_keys
				      SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '')
				    WHERE id = NEW.id;
				 END`,
			`CREATE TRIGGER trg_key_categories_name_compat
				 AFTER UPDATE OF name ON key_categories
				 BEGIN
				   UPDATE vless_keys SET category = NEW.name WHERE category_id = NEW.id;
				   UPDATE external_subscription_sources SET key_category = NEW.name WHERE key_category_id = NEW.id;
				 END`,
		},
	},
	{
		Version: 11,
		Name:    "vless_key_secrets_and_blind_index",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS vless_key_secrets (
				vless_key_id INTEGER PRIMARY KEY REFERENCES vless_keys(id) ON DELETE CASCADE,
				encrypted_url TEXT NOT NULL
			)`,
			`ALTER TABLE vless_keys ADD COLUMN url_blind_index TEXT`,
			`CREATE TABLE IF NOT EXISTS encryption_metadata (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL
			)`,
		},
	},
	{
		Version: 12,
		Name:    "encrypt_vless_keys_data",
		runGo:   migrateVlessKeysData,
	},
	{
		Version: 13,
		Name:    "remove_plaintext_url_column",
		runGo:   applyMigration13Rebuild,
	},
	{
		Version: 14,
		Name:    "add_profile_revision_and_updated_at_to_vless_keys",
		statements: []string{
			`ALTER TABLE vless_keys ADD COLUMN profile_revision INTEGER NOT NULL DEFAULT 1`,
			`ALTER TABLE vless_keys ADD COLUMN updated_at DATETIME`,
			`UPDATE vless_keys SET updated_at = COALESCE(created_at, CURRENT_TIMESTAMP) WHERE updated_at IS NULL`,
		},
	},
	{
		Version: 15,
		Name:    "add_general_subscription_expiration_setting",
		runGo:   migrateShowSubscriptionExpiration,
	},
	{
		Version: 16,
		Name:    "add_client_display_name_to_vless_keys",
		runGo:   migrateClientDisplayName,
	},
	{
		Version: 17,
		Name:    "add_background_job_warning_results",
		runGo:   migrateBackgroundJobWarningResults,
	},
	{
		Version: 18,
		Name:    "canonical_subscription_announcement",
		runGo:   migrateCanonicalSubscriptionAnnouncement,
	},
	{
		Version: 19,
		Name:    "add_happ_routing_delivery_mode",
		runGo:   migrateHappRoutingDeliveryMode,
	},
}

func migrateHappRoutingDeliveryMode(ctx context.Context, conn *sql.Conn, _ *sql.DB, _ *profilestorage.Keyring) error {
	var tableExists, columnExists int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'routing_settings'`).Scan(&tableExists); err != nil {
		return fmt.Errorf("inspect routing settings table: %w", err)
	}
	if tableExists == 0 {
		return nil
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('routing_settings') WHERE name = 'delivery_mode'`).Scan(&columnExists); err != nil {
		return fmt.Errorf("inspect routing delivery mode column: %w", err)
	}
	if columnExists > 0 {
		return nil
	}
	if _, err := conn.ExecContext(ctx, `ALTER TABLE routing_settings ADD COLUMN delivery_mode TEXT NOT NULL DEFAULT 'disabled' CHECK (delivery_mode IN ('disabled', 'add', 'onadd'))`); err != nil {
		return fmt.Errorf("add routing delivery mode: %w", err)
	}
	return nil
}

func migrateCanonicalSubscriptionAnnouncement(ctx context.Context, conn *sql.Conn, _ *sql.DB, _ *profilestorage.Keyring) error {
	var deliveryAnnouncementColumn, canonicalAnnouncementColumn int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('subscription_delivery_settings') WHERE name = 'announcement'`).Scan(&deliveryAnnouncementColumn); err != nil {
		return fmt.Errorf("inspect legacy delivery announcement column: %w", err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('subscription_settings') WHERE name = 'extra_status'`).Scan(&canonicalAnnouncementColumn); err != nil {
		return fmt.Errorf("inspect canonical announcement column: %w", err)
	}
	// Some very old or partially reconstructed schemas recorded historical
	// migration versions without all corresponding tables. There is no legacy
	// value to preserve when either side of this compatibility migration is absent.
	if deliveryAnnouncementColumn == 0 || canonicalAnnouncementColumn == 0 {
		return nil
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin announcement migration: %w", err)
	}
	defer tx.Rollback()

	var legacyAnnouncement string
	err = tx.QueryRowContext(ctx, `
		SELECT announcement FROM subscription_delivery_settings WHERE id = 1
	`).Scan(&legacyAnnouncement)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read legacy delivery announcement: %w", err)
	}
	if _, err := PromoteLegacyDeliveryAnnouncement(ctx, tx, legacyAnnouncement); err != nil {
		return fmt.Errorf("promote legacy delivery announcement: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_delivery_settings SET announcement = '' WHERE id = 1`); err != nil {
		return fmt.Errorf("retire legacy delivery announcement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit announcement migration: %w", err)
	}
	return nil
}

func migrateShowSubscriptionExpiration(_ context.Context, _ *sql.Conn, db *sql.DB, _ *profilestorage.Keyring) error {
	return ensureColumn(db, "subscription_settings", "show_subscription_expiration", "INTEGER NOT NULL DEFAULT 0")
}

func migrateClientDisplayName(_ context.Context, _ *sql.Conn, db *sql.DB, _ *profilestorage.Keyring) error {
	return ensureColumn(db, "vless_keys", "client_display_name", "TEXT")
}

func migrateBackgroundJobWarningResults(ctx context.Context, conn *sql.Conn, _ *sql.DB, _ *profilestorage.Keyring) error {
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(background_jobs)`)
	if err != nil {
		return fmt.Errorf("inspect background_jobs schema: %w", err)
	}
	hasResultCounts := false
	hasBackgroundJobs := false
	for rows.Next() {
		hasBackgroundJobs = true
		var columnID int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan background_jobs schema: %w", err)
		}
		if name == "result_counts_json" {
			hasResultCounts = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate background_jobs schema: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close background_jobs schema rows: %w", err)
	}
	if !hasBackgroundJobs {
		if _, err := conn.ExecContext(ctx, `CREATE TABLE background_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','succeeded_with_warnings','failed')),
			target_type TEXT NOT NULL DEFAULT '', target_id TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '', result_counts_json TEXT NOT NULL DEFAULT '{}',
			run_after DATETIME, started_at DATETIME, finished_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
			return fmt.Errorf("create background_jobs table: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `CREATE INDEX idx_background_jobs_status ON background_jobs(status, run_after, id)`); err != nil {
			return fmt.Errorf("create background_jobs status index: %w", err)
		}
		return nil
	}
	if hasResultCounts {
		return nil
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin background_jobs rebuild: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE background_jobs_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','succeeded_with_warnings','failed')),
			target_type TEXT NOT NULL DEFAULT '',
			target_id TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			result_counts_json TEXT NOT NULL DEFAULT '{}',
			run_after DATETIME,
			started_at DATETIME,
			finished_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO background_jobs_new(
			id, kind, status, target_type, target_id, error_message,
			result_counts_json, run_after, started_at, finished_at, created_at
		)
		SELECT id, kind, status, target_type, target_id, error_message,
		       '{}', run_after, started_at, finished_at, created_at
		FROM background_jobs`,
		`DROP TABLE background_jobs`,
		`ALTER TABLE background_jobs_new RENAME TO background_jobs`,
		`CREATE INDEX idx_background_jobs_status ON background_jobs(status, run_after, id)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("rebuild background_jobs: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit background_jobs rebuild: %w", err)
	}
	return nil
}

// Migrate applies every pending schema migration without a keyring.
func Migrate(db *sql.DB) error {
	return MigrateWithKeyring(db, nil)
}

// MigrateWithKeyring applies every pending schema migration, including the
// baseline for databases that predate versioned migrations.
func MigrateWithKeyring(db *sql.DB, keyring *profilestorage.Keyring) error {
	var versionTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&versionTables); err != nil {
		return err
	}
	hadVersionTable := versionTables > 0
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	for _, migration := range SchemaMigrations {
		var applied int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, migration.Version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.Version, err)
		}
		if applied > 0 {
			continue
		}

		if migration.baseline && hadVersionTable {
			// Already-versioned databases were bootstrapped before version 0 existed.
			if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version, name) VALUES(?, ?)`, migration.Version, migration.Name); err != nil {
				return fmt.Errorf("record migration %d: %w", migration.Version, err)
			}
			continue
		}

		if migration.runGo != nil {
			if err := migration.runGo(ctx, conn, db, keyring); err != nil {
				return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version, name) VALUES(?, ?)`, migration.Version, migration.Name); err != nil {
				return fmt.Errorf("record migration %d: %w", migration.Version, err)
			}
		} else {
			if err := applySchemaMigration(ctx, conn, migration); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateVlessKeysData(ctx context.Context, conn *sql.Conn, db *sql.DB, keyring *profilestorage.Keyring) error {
	if keyring == nil {
		return profilestorage.ErrMissingKeyring
	}
	activeID, activeKey, err := keyring.GetActiveEncryptionKey()
	if err != nil {
		return err
	}
	bikID, bikKey, err := keyring.GetActiveBlindIndexKey()
	if err != nil {
		return err
	}
	if err := recordBlindIndexKeyID(ctx, db, bikID); err != nil {
		return err
	}

	lastID := int64(0)
	for {
		batch, err := loadVlessKeyMigrationBatch(ctx, db, lastID)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		lastID = batch[len(batch)-1].id
		if err := migrateVlessKeyBatch(ctx, db, keyring, batch, activeID, activeKey, bikKey); err != nil {
			return err
		}
	}

	return verifyVlessKeysMigration(ctx, db, keyring)
}

// recordBlindIndexKeyID pins the configured BIK id in encryption_metadata, or
// refuses to run against a database that was indexed with a different one.
func recordBlindIndexKeyID(ctx context.Context, db *sql.DB, bikID string) error {
	var existingBIKID string
	err := db.QueryRowContext(ctx, `SELECT value FROM encryption_metadata WHERE key = 'active_blind_index_key_id'`).Scan(&existingBIKID)
	if err == sql.ErrNoRows {
		if _, err := db.ExecContext(ctx, `INSERT INTO encryption_metadata(key, value) VALUES('active_blind_index_key_id', ?)`, bikID); err != nil {
			return fmt.Errorf("initialize active_blind_index_key_id: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read active_blind_index_key_id: %w", err)
	}
	if existingBIKID != bikID {
		return fmt.Errorf("%w: db recorded BIK %q, configured %q", profilestorage.ErrBlindIndexKeyMismatch, existingBIKID, bikID)
	}
	return nil
}

type vlessKeyMigrationItem struct {
	id               int64
	rawURL           string
	hasSecret        bool
	hasBlindIndex    bool
	externalSourceID sql.NullInt64
}

// loadVlessKeyMigrationBatch returns the next 100 rows after lastID that still
// lack a secret or a local blind index.
func loadVlessKeyMigrationBatch(ctx context.Context, db *sql.DB, lastID int64) ([]vlessKeyMigrationItem, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT k.id, k.url, s.encrypted_url, k.url_blind_index, k.external_source_id
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE (s.vless_key_id IS NULL OR (k.external_source_id IS NULL AND k.url_blind_index IS NULL))
		  AND k.id > ?
		ORDER BY k.id ASC LIMIT 100`, lastID)
	if err != nil {
		return nil, fmt.Errorf("query migration batch: %w", err)
	}
	defer rows.Close()

	var batch []vlessKeyMigrationItem
	for rows.Next() {
		var item vlessKeyMigrationItem
		var encURL sql.NullString
		var blindIdx sql.NullString
		if err := rows.Scan(&item.id, &item.rawURL, &encURL, &blindIdx, &item.externalSourceID); err != nil {
			return nil, fmt.Errorf("scan migration item: %w", err)
		}
		item.hasSecret = encURL.Valid
		item.hasBlindIndex = blindIdx.Valid
		batch = append(batch, item)
	}
	return batch, nil
}

// migrateVlessKeyBatch encrypts the missing secrets and fills the missing local
// blind indexes of one batch inside a single transaction.
func migrateVlessKeyBatch(ctx context.Context, db *sql.DB, keyring *profilestorage.Keyring, batch []vlessKeyMigrationItem, activeID string, activeKey, bikKey []byte) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration batch tx: %w", err)
	}

	for _, item := range batch {
		if !item.hasSecret {
			if err := insertVlessKeySecret(ctx, tx, keyring, item, activeID, activeKey); err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		if !item.externalSourceID.Valid && !item.hasBlindIndex {
			bIdx := profilestorage.ComputeBlindIndex(bikKey, item.rawURL)
			if _, err := tx.ExecContext(ctx, `UPDATE vless_keys SET url_blind_index = ? WHERE id = ?`, bIdx, item.id); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("update blind index for row %d: %w", item.id, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration batch tx: %w", err)
	}
	return nil
}

// insertVlessKeySecret encrypts the plaintext URL, proves it round-trips, and
// stores the envelope.
func insertVlessKeySecret(ctx context.Context, tx *sql.Tx, keyring *profilestorage.Keyring, item vlessKeyMigrationItem, activeID string, activeKey []byte) error {
	env, err := profilestorage.Encrypt([]byte(item.rawURL), activeID, activeKey, item.id)
	if err != nil {
		return fmt.Errorf("encrypt row %d: %w", item.id, err)
	}
	dec, err := profilestorage.Decrypt(env, keyring, item.id)
	if err != nil || dec.Reveal() != item.rawURL {
		return fmt.Errorf("decrypt verification failed for row %d", item.id)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, item.id, env); err != nil {
		return fmt.Errorf("insert secret for row %d: %w", item.id, err)
	}
	return nil
}

// verifyVlessKeysMigration checks the post-migration invariants: every key has
// a decryptable secret, no orphan secrets, unique local blind indexes.
func verifyVlessKeysMigration(ctx context.Context, db *sql.DB, keyring *profilestorage.Keyring) error {
	invariants := []struct {
		query  string
		format string
	}{
		{`SELECT COUNT(*) FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE s.vless_key_id IS NULL`, "%w: %d rows missing secrets"},
		{`SELECT COUNT(*) FROM vless_key_secrets s LEFT JOIN vless_keys k ON s.vless_key_id = k.id WHERE k.id IS NULL`, "%w: %d orphan secrets found"},
		{`SELECT COUNT(*) FROM vless_keys WHERE external_source_id IS NULL AND url_blind_index IS NULL`, "%w: %d local rows missing blind index"},
		{`SELECT COUNT(*) FROM (SELECT url_blind_index FROM vless_keys WHERE external_source_id IS NULL GROUP BY url_blind_index HAVING COUNT(*) > 1)`, "%w: %d duplicate local blind indexes found"},
	}
	for _, inv := range invariants {
		var count int
		if err := db.QueryRowContext(ctx, inv.query).Scan(&count); err != nil || count > 0 {
			return fmt.Errorf(inv.format, profilestorage.ErrMigrationVerificationFailed, count)
		}
	}

	secRows, err := db.QueryContext(ctx, `SELECT vless_key_id, encrypted_url FROM vless_key_secrets`)
	if err != nil {
		return fmt.Errorf("verify encrypted secrets: %w", err)
	}
	defer secRows.Close()
	for secRows.Next() {
		var rowID int64
		var env string
		if err := secRows.Scan(&rowID, &env); err != nil {
			return fmt.Errorf("scan verification secret: %w", err)
		}
		if _, err := profilestorage.Decrypt(env, keyring, rowID); err != nil {
			return fmt.Errorf("%w: row %d secret decryption check failed", profilestorage.ErrMigrationVerificationFailed, rowID)
		}
	}
	if err := secRows.Err(); err != nil {
		return fmt.Errorf("verify encrypted secrets iteration: %w", err)
	}

	return nil
}

// migration13RebuildStatements rebuilds vless_keys without the legacy url
// column, preserving ids, sequence, indexes and triggers.
var migration13RebuildStatements = []string{
	`DROP TRIGGER IF EXISTS trg_key_categories_name_compat`,
	`DROP TRIGGER IF EXISTS trg_vless_keys_category_insert`,
	`DROP TRIGGER IF EXISTS trg_vless_keys_category_id_insert`,
	`DROP TRIGGER IF EXISTS trg_vless_keys_category_id_update`,
	`CREATE TABLE vless_keys_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		label TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		check_status TEXT NOT NULL DEFAULT 'unknown',
		check_error TEXT,
		last_checked_at DATETIME,
		last_latency_ms INTEGER,
		status TEXT NOT NULL DEFAULT 'active',
		key_kind TEXT NOT NULL DEFAULT 'real',
		template_text TEXT,
		sort_order INTEGER NOT NULL DEFAULT 0,
		starts_at DATETIME,
		expires_at DATETIME,
		blocked_reason TEXT,
		external_source_id INTEGER,
		external_key_ref TEXT,
		category TEXT NOT NULL DEFAULT '',
		category_id INTEGER REFERENCES key_categories(id) ON DELETE SET NULL,
		health_failure_count INTEGER NOT NULL DEFAULT 0,
		protocol TEXT NOT NULL DEFAULT 'legacy',
		profile_fingerprint TEXT,
		profile_schema_version INTEGER NOT NULL DEFAULT 0,
		profile_compatibility TEXT NOT NULL DEFAULT 'legacy',
		profile_warnings_json TEXT NOT NULL DEFAULT '[]',
		url_blind_index TEXT
	)`,
	`INSERT INTO vless_keys_new(
		id, label, created_at, check_status, check_error, last_checked_at, last_latency_ms,
		status, key_kind, template_text, sort_order, starts_at, expires_at, blocked_reason,
		external_source_id, external_key_ref, category, category_id, health_failure_count,
		protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json,
		url_blind_index
	)
	SELECT id, label, created_at, check_status, check_error, last_checked_at, last_latency_ms,
	       status, key_kind, template_text, sort_order, starts_at, expires_at, blocked_reason,
	       external_source_id, external_key_ref, category, category_id, health_failure_count,
	       protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json,
	       url_blind_index
	  FROM vless_keys`,
	`INSERT OR REPLACE INTO sqlite_sequence(name, seq) SELECT 'vless_keys_new', COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'vless_keys'), (SELECT MAX(id) FROM vless_keys), 0)`,
	`DROP TABLE vless_keys`,
	`ALTER TABLE vless_keys_new RENAME TO vless_keys`,
	`UPDATE sqlite_sequence SET name = 'vless_keys' WHERE name = 'vless_keys_new'`,
	`CREATE UNIQUE INDEX idx_vless_keys_local_blind_index ON vless_keys(url_blind_index) WHERE external_source_id IS NULL`,
	`CREATE INDEX idx_vless_keys_kind_sort ON vless_keys(key_kind, sort_order, id)`,
	`CREATE INDEX idx_vless_keys_category_sort ON vless_keys(category, sort_order, id)`,
	`CREATE INDEX idx_vless_keys_external_source ON vless_keys(external_source_id)`,
	`CREATE UNIQUE INDEX idx_vless_keys_external_source_ref ON vless_keys(external_source_id, external_key_ref)`,
	`CREATE INDEX idx_vless_keys_category_id ON vless_keys(category_id, sort_order, id)`,
	`CREATE INDEX idx_vless_keys_delivery_health ON vless_keys(status, key_kind, health_failure_count, sort_order, id)`,
	`CREATE UNIQUE INDEX idx_vless_keys_external_source_fingerprint ON vless_keys(external_source_id, profile_fingerprint) WHERE external_source_id IS NOT NULL AND profile_fingerprint IS NOT NULL AND TRIM(profile_fingerprint) <> ''`,
	`CREATE TRIGGER trg_vless_keys_category_insert AFTER INSERT ON vless_keys WHEN NEW.category_id IS NULL AND TRIM(COALESCE(NEW.category, '')) <> '' BEGIN UPDATE vless_keys SET category_id = (SELECT id FROM key_categories WHERE name = NEW.category) WHERE id = NEW.id; END`,
	`CREATE TRIGGER trg_vless_keys_category_id_insert AFTER INSERT ON vless_keys WHEN NEW.category_id IS NOT NULL BEGIN UPDATE vless_keys SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '') WHERE id = NEW.id; END`,
	`CREATE TRIGGER trg_vless_keys_category_id_update AFTER UPDATE OF category_id ON vless_keys BEGIN UPDATE vless_keys SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '') WHERE id = NEW.id; END`,
	`CREATE TRIGGER trg_key_categories_name_compat AFTER UPDATE OF name ON key_categories BEGIN UPDATE vless_keys SET category = NEW.name WHERE category_id = NEW.id; UPDATE external_subscription_sources SET key_category = NEW.name WHERE key_category_id = NEW.id; END`,
}

func applyMigration13Rebuild(ctx context.Context, conn *sql.Conn, db *sql.DB, keyring *profilestorage.Keyring) (resultErr error) {
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}

	connectionDirty := false
	defer func() {
		if connectionDirty {
			return
		}
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("restore foreign_keys: %w", err)
			connectionDirty = true
		}
		var fkStatus int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkStatus); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("verify foreign_keys status: %w", err)
			connectionDirty = true
		} else if fkStatus != 1 && resultErr == nil {
			resultErr = fmt.Errorf("foreign_keys failed to re-enable (status: %d)", fkStatus)
			connectionDirty = true
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		connectionDirty = true
		return fmt.Errorf("begin rebuild tx: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range migration13RebuildStatements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			connectionDirty = true
			return fmt.Errorf("rebuild statement failed: %w", err)
		}
	}

	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		connectionDirty = true
		return fmt.Errorf("execute foreign_key_check: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var table, rowid, parent, fkid string
		_ = rows.Scan(&table, &rowid, &parent, &fkid)
		connectionDirty = true
		return fmt.Errorf("foreign key violation detected on table %s (rowid %s)", table, rowid)
	}

	var integrityResult string
	if err := tx.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrityResult); err != nil {
		connectionDirty = true
		return fmt.Errorf("execute integrity_check: %w", err)
	}
	if integrityResult != "ok" {
		connectionDirty = true
		return fmt.Errorf("integrity_check failed: %s", integrityResult)
	}

	if err := tx.Commit(); err != nil {
		connectionDirty = true
		return fmt.Errorf("commit rebuild tx: %w", err)
	}
	return nil
}

func applySchemaMigration(ctx context.Context, conn *sql.Conn, migration schemaMigration) (resultErr error) {
	if migration.disableForeignKeys {
		restore, err := suspendForeignKeys(ctx, conn, migration)
		if err != nil {
			return err
		}
		defer func() {
			if err := restore(); err != nil && resultErr == nil {
				resultErr = err
			}
		}()
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.Version, err)
	}
	defer tx.Rollback()
	for _, statement := range migration.statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}
	if migration.verifyForeignKeys {
		var violations int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
			return fmt.Errorf("verify foreign keys for migration %d: %w", migration.Version, err)
		}
		if violations != 0 {
			return fmt.Errorf("verify foreign keys for migration %d: %d violation(s)", migration.Version, violations)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name) VALUES(?, ?)`,
		migration.Version,
		migration.Name,
	); err != nil {
		return fmt.Errorf("record migration %d: %w", migration.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.Version, err)
	}
	return nil
}

// suspendForeignKeys turns PRAGMA foreign_keys off for the migration when it is
// currently on and returns the function that switches it back (a no-op when
// nothing was changed).
func suspendForeignKeys(ctx context.Context, conn *sql.Conn, migration schemaMigration) (func() error, error) {
	var enabled int
	if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		return nil, fmt.Errorf("read foreign key state for migration %d: %w", migration.Version, err)
	}
	if enabled == 0 {
		return func() error { return nil }, nil
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return nil, fmt.Errorf("disable foreign keys for migration %d: %w", migration.Version, err)
	}
	return func() error {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return fmt.Errorf("restore foreign keys after migration %d: %w", migration.Version, err)
		}
		return nil
	}, nil
}

// baselineStep is one ordered step of the baseline bootstrap: either a raw
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

type contextSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// PromoteLegacyDeliveryAnnouncement copies a legacy announcement into
// subscription_settings.extra_status when nothing is set there yet.
func PromoteLegacyDeliveryAnnouncement(ctx context.Context, executor contextSQLExecutor, announcement string) (bool, error) {
	announcement = strings.TrimSpace(announcement)
	if announcement == "" {
		return false, nil
	}
	result, err := executor.ExecContext(ctx, `
		UPDATE subscription_settings
		SET extra_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1 AND TRIM(COALESCE(extra_status, '')) = ''
	`, announcement)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	return rowsAffected > 0, err
}
