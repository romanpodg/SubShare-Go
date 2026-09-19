package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ErrDuplicateSourceKey is returned by InsertSourceKey when the (source, ref)
// pair already exists.
var ErrDuplicateSourceKey = errors.New("duplicate source key")

// SourceKey is a source-owned key as the sync algorithm sees it. URL is the
// decrypted profile; it is empty when the secret is missing or unreadable.
type SourceKey struct {
	ID                   int64
	Ref                  string
	Fingerprint          string
	Label                string
	ClientDisplayName    string
	URL                  string
	Protocol             string
	ProfileSchemaVersion int
	Compatibility        string
	WarningsJSON         string
}

// SourceKeyWrite carries every column a sync writes for one source-owned key.
type SourceKeyWrite struct {
	SourceID             int64
	Ref                  string
	Label                string
	CategoryID           any
	Category             string
	Status               string
	SortOrder            int64
	Protocol             string
	Fingerprint          string
	ProfileSchemaVersion int
	Compatibility        string
	WarningsJSON         string
	URL                  string
}

// SourceSyncMetadata is what a successful sync records on the source row.
type SourceSyncMetadata struct {
	Title        string
	RefreshHours int
	SupportURL   string
	WebPageURL   string
	Announce     string
}

// SourceSync is one source synchronisation transaction. Every key statement a
// sync needs lives here so the algorithm never touches SQL or encryption.
type SourceSync struct {
	ctx         context.Context
	tx          *sql.Tx
	credentials *credentialStore
	categories  *categoryStore
}

func (r *Repository) BeginSourceSync(ctx context.Context) (*SourceSync, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &SourceSync{ctx: ctx, tx: tx, credentials: r.credentials, categories: r.categories}, nil
}

// SourceSyncOn wraps a transaction the caller already owns; Commit and
// Rollback remain the caller's responsibility.
func (r *Repository) SourceSyncOn(ctx context.Context, tx *sql.Tx) *SourceSync {
	return &SourceSync{ctx: ctx, tx: tx, credentials: r.credentials, categories: r.categories}
}

func (s *SourceSync) Commit() error   { return s.tx.Commit() }
func (s *SourceSync) Rollback() error { return s.tx.Rollback() }

// EnsureCategory upserts the target key category and returns its id, or nil
// when the name is empty.
func (s *SourceSync) EnsureCategory(name string) (any, error) {
	return s.categories.upsertTx(s.ctx, s.tx, name, "#d8b33d")
}

// ListSourceKeys returns the source's keys in id order with secrets decrypted.
func (s *SourceSync) ListSourceKeys(sourceID int64) ([]SourceKey, error) {
	rows, err := s.tx.QueryContext(s.ctx, `
		SELECT k.id, k.external_key_ref, COALESCE(k.profile_fingerprint, ''), k.label,
		       COALESCE(k.client_display_name, ''), s.encrypted_url,
		       k.protocol, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.external_source_id = ?
		ORDER BY k.id
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]SourceKey, 0)
	for rows.Next() {
		var key SourceKey
		var encURL sql.NullString
		if err := rows.Scan(&key.ID, &key.Ref, &key.Fingerprint, &key.Label, &key.ClientDisplayName, &encURL, &key.Protocol, &key.ProfileSchemaVersion, &key.Compatibility, &key.WarningsJSON); err != nil {
			return nil, err
		}
		if encURL.Valid && encURL.String != "" {
			if raw, err := s.credentials.decrypt(encURL.String, key.ID); err == nil {
				key.URL = raw
			}
		}
		key.Ref = strings.TrimSpace(key.Ref)
		key.Fingerprint = strings.TrimSpace(key.Fingerprint)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *SourceSync) SetClientDisplayName(sourceID, id int64, name string) error {
	_, err := s.tx.ExecContext(s.ctx, `UPDATE vless_keys SET client_display_name = ? WHERE id = ? AND external_source_id = ?`, name, id, sourceID)
	return err
}

// MergeUserAssignments gives the survivor every user assignment the duplicate had.
func (s *SourceSync) MergeUserAssignments(survivorID, duplicateID int64) error {
	_, err := s.tx.ExecContext(s.ctx, `INSERT OR IGNORE INTO user_keys(user_id, key_id) SELECT user_id, ? FROM user_keys WHERE key_id = ?`, survivorID, duplicateID)
	return err
}

func (s *SourceSync) DeleteSourceKey(sourceID, id int64) error {
	_, err := s.tx.ExecContext(s.ctx, `DELETE FROM vless_keys WHERE id = ? AND external_source_id = ?`, id, sourceID)
	return err
}

func (s *SourceSync) SetFingerprint(sourceID, id int64, fingerprint string) error {
	_, err := s.tx.ExecContext(s.ctx, `UPDATE vless_keys SET profile_fingerprint = ? WHERE id = ? AND external_source_id = ?`, fingerprint, id, sourceID)
	return err
}

// NextSortOrder returns the first sort_order for keys imported in this sync:
// above every other key in the category when top is set, otherwise below.
func (s *SourceSync) NextSortOrder(categoryID any, sourceID int64, top bool, incoming int) (int64, error) {
	var next int64
	var err error
	if top {
		err = s.tx.QueryRowContext(s.ctx,
			`SELECT COALESCE(MIN(sort_order), 1) - ? FROM vless_keys WHERE category_id IS ? AND external_source_id != ?`,
			incoming+8, categoryID, sourceID,
		).Scan(&next)
	} else {
		err = s.tx.QueryRowContext(s.ctx,
			`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys WHERE category_id IS ? AND external_source_id != ?`,
			categoryID, sourceID,
		).Scan(&next)
	}
	return next, err
}

// UpdateSourceKey rewrites a matched key's metadata and re-encrypts its secret.
func (s *SourceSync) UpdateSourceKey(id int64, w SourceKeyWrite) error {
	if _, err := s.tx.ExecContext(s.ctx,
		`UPDATE vless_keys
		 SET label = ?, category_id = ?, category = ?, key_kind = 'real', template_text = NULL,
		     external_source_id = ?, external_key_ref = ?, sort_order = ?, protocol = ?, profile_fingerprint = ?,
		     profile_schema_version = ?, profile_compatibility = ?, profile_warnings_json = ?
		 WHERE id = ? AND external_source_id = ?`,
		w.Label, w.CategoryID, w.Category, w.SourceID, w.Ref, w.SortOrder,
		w.Protocol, nullStringValue(w.Fingerprint), w.ProfileSchemaVersion, w.Compatibility, w.WarningsJSON, id, w.SourceID,
	); err != nil {
		return err
	}
	env, err := s.credentials.encrypt(w.URL, id)
	if err != nil {
		return err
	}
	_, err = s.tx.ExecContext(s.ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?) ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url`, id, env)
	return err
}

// InsertSourceKey creates a new source-owned key with its secret and assigns
// it to every user in "all" assignment mode.
func (s *SourceSync) InsertSourceKey(w SourceKeyWrite) (int64, error) {
	result, err := s.tx.ExecContext(s.ctx,
		`INSERT INTO vless_keys(
			label, category_id, category, status, check_status, key_kind, template_text, sort_order,
			external_source_id, external_key_ref, protocol, profile_fingerprint, profile_schema_version,
			profile_compatibility, profile_warnings_json
		) VALUES(?, ?, ?, ?, 'unknown', 'real', NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.Label, w.CategoryID, w.Category, w.Status, w.SortOrder, w.SourceID, w.Ref,
		w.Protocol, nullStringValue(w.Fingerprint), w.ProfileSchemaVersion, w.Compatibility, w.WarningsJSON,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return 0, ErrDuplicateSourceKey
		}
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	env, err := s.credentials.encrypt(w.URL, id)
	if err != nil {
		return 0, err
	}
	if _, err := s.tx.ExecContext(s.ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id, env); err != nil {
		return 0, err
	}
	if _, err := s.tx.ExecContext(s.ctx, `INSERT OR IGNORE INTO user_keys(user_id, key_id) SELECT id, ? FROM users WHERE key_assignment_mode = 'all'`, id); err != nil {
		return 0, err
	}
	return id, nil
}

// MarkSourceSynced records a successful sync on the source row.
func (s *SourceSync) MarkSourceSynced(sourceID int64, imported int, meta SourceSyncMetadata) error {
	_, err := s.tx.ExecContext(s.ctx,
		`UPDATE external_subscription_sources
		 SET import_status = 'ok',
		     last_error = NULL,
		     last_synced_at = CURRENT_TIMESTAMP,
		     last_import_count = ?,
		     meta_title = ?,
		     meta_refresh_hours = ?,
		     meta_support_url = ?,
		     meta_web_page_url = ?,
		     meta_announce = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		imported,
		nullStringValue(meta.Title),
		nullInt64Value(int64(meta.RefreshHours)),
		nullStringValue(meta.SupportURL),
		nullStringValue(meta.WebPageURL),
		nullStringValue(meta.Announce),
		sourceID,
	)
	return err
}
