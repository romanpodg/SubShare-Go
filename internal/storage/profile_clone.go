package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"strings"
)

func (r *Repository) CloneLocal(ctx context.Context, params keymanagement.CloneProfileParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	if _, err := r.credentials.blindIndex(""); err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	src, decryptedURI, err := r.loadCloneSource(ctx, tx, params.ID, params.ExpectedRevision)
	if err != nil {
		return nil, "", err
	}

	newLabel := strings.TrimSpace(params.NewLabel)
	if newLabel == "" {
		newLabel = src.label + " (Копия)"
	}
	cloneClientDisplayName := src.cloneClientDisplayName(newLabel, decryptedURI)

	newID, err := r.insertClonedProfile(ctx, tx, clonedProfile{source: src, uri: decryptedURI, label: newLabel, clientDisplayName: cloneClientDisplayName})
	if err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit clone: %w", err)
	}

	return r.GetByID(ctx, newID)
}

// cloneSource is the row a clone is copied from.
type cloneSource struct {
	label, category, kind, status, protocol string
	templateText, warnings, encryptedURL    sql.NullString
	clientDisplayName                       sql.NullString
	categoryID, externalSourceID            sql.NullInt64
	schemaVersion                           int
	revision                                int64
}

// loadCloneSource reads the source row, checks its revision and returns it
// together with the decrypted configuration.
func (r *Repository) loadCloneSource(ctx context.Context, tx *sql.Tx, id, expectedRevision int64) (*cloneSource, string, error) {
	var src cloneSource
	err := tx.QueryRowContext(ctx, `
		SELECT k.label, k.client_display_name, k.category, k.key_kind, k.template_text, k.status, k.protocol,
		       k.profile_schema_version, k.profile_warnings_json, s.encrypted_url,
		       k.category_id, k.external_source_id, COALESCE(k.profile_revision, 1)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.id = ?
	`, id).Scan(
		&src.label, &src.clientDisplayName, &src.category, &src.kind, &src.templateText, &src.status,
		&src.protocol, &src.schemaVersion, &src.warnings, &src.encryptedURL, &src.categoryID, &src.externalSourceID, &src.revision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to load source key: %w", err)
	}

	if src.revision != expectedRevision {
		return nil, "", keymanagement.ErrProfileRevisionConflict
	}

	if !src.encryptedURL.Valid || src.encryptedURL.String == "" {
		return nil, "", keymanagement.ErrStorageIntegrity
	}

	decryptedURI, decErr := r.credentials.decrypt(src.encryptedURL.String, id)
	if decErr != nil {
		return nil, "", mapKeyCredentialError(decErr)
	}
	return &src, decryptedURI, nil
}

// cloneClientDisplayName keeps an explicit display name; for a source-owned
// key without one it pins the source-derived name only when a local key with
// the new label would display something different.
func (src *cloneSource) cloneClientDisplayName(newLabel, decryptedURI string) string {
	name := strings.TrimSpace(src.clientDisplayName.String)
	if name != "" {
		return name
	}
	if !src.externalSourceID.Valid {
		return name
	}
	if src.externalSourceID.Int64 <= 0 {
		return name
	}
	sourceEffectiveName := profileconfig.EffectiveClientDisplayName("", decryptedURI, src.label, true)
	localFallbackName := profileconfig.EffectiveClientDisplayName("", decryptedURI, newLabel, false)
	if sourceEffectiveName != localFallbackName {
		return sourceEffectiveName
	}
	return name
}

type clonedProfile struct {
	source                        *cloneSource
	uri, label, clientDisplayName string
}

func (r *Repository) insertClonedProfile(ctx context.Context, tx *sql.Tx, clone clonedProfile) (int64, error) {
	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return 0, fmt.Errorf("failed to prepare key order: %w", err)
	}

	blindIndex, err := r.credentials.cloneBlindIndex(clone.uri)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, client_display_name, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, external_source_id, external_key_ref,
			protocol, profile_schema_version, profile_compatibility, profile_warnings_json,
			profile_revision, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, 'unknown', ?, ?, ?, NULL, NULL, ?, ?, 'full', ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, clone.label, nullStringValue(clone.clientDisplayName), blindIndex, nullInt64Value(clone.source.categoryID.Int64), clone.source.category, clone.source.status, clone.source.kind, nullStringValue(clone.source.templateText.String), nextSortOrder, clone.source.protocol, clone.source.schemaVersion, nullStringValue(clone.source.warnings.String))
	if err != nil {
		return 0, fmt.Errorf("failed to insert cloned key: %w", err)
	}

	newID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get new key ID: %w", err)
	}

	env, err := r.credentials.encrypt(clone.uri, newID)
	if err != nil {
		return 0, fmt.Errorf("failed to encrypt cloned key: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, newID, env); err != nil {
		return 0, fmt.Errorf("failed to save secret: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_keys(user_id, key_id)
		SELECT id, ? FROM users WHERE key_assignment_mode = 'all'
	`, newID); err != nil {
		return 0, fmt.Errorf("failed to add cloned key to users: %w", err)
	}

	return newID, nil
}
