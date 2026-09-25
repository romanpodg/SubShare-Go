package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

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
