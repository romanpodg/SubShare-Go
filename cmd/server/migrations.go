package main

import (
	"database/sql"
	"fmt"
)

type schemaMigration struct {
	version    int
	name       string
	statements []string
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
}

func runVersionedMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, migration := range schemaMigrations {
		var applied int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, migration.version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.version, err)
		}
		if applied > 0 {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.version, err)
		}
		for _, statement := range migration.statements {
			if _, err := tx.Exec(statement); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %d (%s): %w", migration.version, migration.name, err)
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations(version, name) VALUES(?, ?)`,
			migration.version,
			migration.name,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}
	}
	return nil
}
