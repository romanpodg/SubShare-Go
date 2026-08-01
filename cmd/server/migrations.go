package main

import (
	"context"
	"database/sql"
	"fmt"
)

type schemaMigration struct {
	version            int
	name               string
	statements         []string
	disableForeignKeys bool
	verifyForeignKeys  bool
}

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		name:    "subscription_hub_foundation",
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
				status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed')),
				target_type TEXT NOT NULL DEFAULT '',
				target_id TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
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
		version: 2,
		name:    "seed_default_templates_and_rules",
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
		version: 3,
		name:    "owner_operator_viewer_roles",
		statements: []string{
			`UPDATE admins SET role = 'owner' WHERE role = 'super_admin'`,
			`UPDATE admins SET role = 'operator' WHERE role = 'support_admin'`,
		},
	},
	{
		version: 4,
		name:    "category_foreign_keys",
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
		version: 5,
		name:    "client_format_system_rules",
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
		version: 6,
		name:    "subscription_delivery_settings",
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
		version: 7,
		name:    "subscription_entitlements_and_health_policy",
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
		version: 8,
		name:    "category_ids_as_canonical_source",
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
		version: 9,
		name:    "external_protocol_profile_persistence",
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
		version:            10,
		name:               "source_owned_profile_urls",
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
}

func runVersionedMigrations(db *sql.DB) error {
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

	for _, migration := range schemaMigrations {
		var applied int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, migration.version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.version, err)
		}
		if applied > 0 {
			continue
		}

		if err := applySchemaMigration(ctx, conn, migration); err != nil {
			return err
		}
	}
	return nil
}

func applySchemaMigration(ctx context.Context, conn *sql.Conn, migration schemaMigration) (resultErr error) {
	foreignKeysEnabled := false
	if migration.disableForeignKeys {
		var enabled int
		if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
			return fmt.Errorf("read foreign key state for migration %d: %w", migration.version, err)
		}
		foreignKeysEnabled = enabled != 0
		if foreignKeysEnabled {
			if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
				return fmt.Errorf("disable foreign keys for migration %d: %w", migration.version, err)
			}
			defer func() {
				if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil && resultErr == nil {
					resultErr = fmt.Errorf("restore foreign keys after migration %d: %w", migration.version, err)
				}
			}()
		}
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.version, err)
	}
	defer tx.Rollback()
	for _, statement := range migration.statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.version, migration.name, err)
		}
	}
	if migration.verifyForeignKeys {
		var violations int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
			return fmt.Errorf("verify foreign keys for migration %d: %w", migration.version, err)
		}
		if violations != 0 {
			return fmt.Errorf("verify foreign keys for migration %d: %d violation(s)", migration.version, violations)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name) VALUES(?, ?)`,
		migration.version,
		migration.name,
	); err != nil {
		return fmt.Errorf("record migration %d: %w", migration.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.version, err)
	}
	return nil
}
