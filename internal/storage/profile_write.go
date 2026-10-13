package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

type localProfileCreation struct {
	params     keymanagement.CreateProfileParams
	blindIndex string
}

func (r *Repository) insertLocalProfile(ctx context.Context, tx *sql.Tx, creation localProfileCreation) (int64, error) {
	categoryIDVal, err := r.categories.resolveTx(ctx, tx, creation.params.Category, creation.params.CategoryID, "#4B5563")
	if err != nil {
		return 0, err
	}

	var nextSort int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSort); err != nil {
		return 0, fmt.Errorf("failed to calculate sort order: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, client_display_name, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, protocol, profile_schema_version,
			profile_compatibility, profile_warnings_json, profile_revision,
			created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, 'unknown', ?, ?, ?, ?, 1, 'full', '[]', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, creation.params.Label, nullStringValue(creation.params.ClientDisplayName), creation.blindIndex, categoryIDVal, creation.params.Category, creation.params.Status, creation.params.Kind, nullStringValue(creation.params.TemplateText), nextSort, creation.params.Protocol)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrProfileCreateConflict, err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := r.credentials.encrypt(creation.params.BuiltURI, keyID)
	if err != nil {
		return 0, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		return 0, fmt.Errorf("failed to save secret: %w", err)
	}

	return keyID, nil
}
func (r *Repository) updateLocalSecret(ctx context.Context, tx *sql.Tx, id int64, uri string) error {
	env, encryptErr := r.credentials.encrypt(uri, id)
	if encryptErr != nil {
		return fmt.Errorf("failed to encrypt secret: %w", encryptErr)
	}
	if _, err := tx.ExecContext(ctx, `
			INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)
			ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url
		`, id, env); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}
	return nil
}

type localProfileUpdate struct {
	params     keymanagement.UpdateProfileParams
	blindIndex string
}

func (r *Repository) applyLocalProfileUpdate(ctx context.Context, tx *sql.Tx, update localProfileUpdate) error {
	params, blindIndex := update.params, update.blindIndex
	categoryIDVal, err := r.categories.resolveTx(ctx, tx, params.Category, params.CategoryID, "#4B5563")
	if err != nil {
		return err
	}

	storedURI, storedBlindIndex, err := r.loadLocalKeyForUpdate(ctx, tx, params.ID, params.ExpectedRevision)
	if err != nil {
		return err
	}
	if storedURI == params.NewURI {
		blindIndex = storedBlindIndex
	}
	uriChanged := storedURI != params.NewURI

	res, err := tx.ExecContext(ctx, `
		UPDATE vless_keys
		SET label = ?,
		    client_display_name = CASE WHEN ? THEN ? ELSE client_display_name END,
		    url_blind_index = ?, category_id = ?, category = ?, status = ?,
		    key_kind = ?, template_text = ?, protocol = ?, profile_revision = profile_revision + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND profile_revision = ? AND external_source_id IS NULL
	`, params.Label, params.ClientDisplayName != nil, optionalNullStringValue(params.ClientDisplayName), blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), params.Protocol, params.ID, params.ExpectedRevision)
	if err != nil {
		return fmt.Errorf("failed to update key: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return keymanagement.ErrProfileRevisionConflict
	}

	if uriChanged {
		return r.updateLocalSecret(ctx, tx, params.ID, params.NewURI)
	}
	return nil
}
func (r *Repository) validateSourceMetadataUpdate(ctx context.Context, tx *sql.Tx, params keymanagement.UpdateSourceOwnedMetadataParams) error {
	var storedRevision int64
	var storedEncryptedURL sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(k.profile_revision, 1), s.encrypted_url
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON s.vless_key_id = k.id
		WHERE k.id = ?
	`, params.ID).Scan(&storedRevision, &storedEncryptedURL)
	if errors.Is(err, sql.ErrNoRows) {
		return keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to load key metadata: %w", err)
	}
	if storedRevision != params.ExpectedRevision {
		return keymanagement.ErrProfileRevisionConflict
	}
	if !storedEncryptedURL.Valid || storedEncryptedURL.String == "" {
		return keymanagement.ErrStorageIntegrity
	}
	if _, decryptErr := r.credentials.decrypt(storedEncryptedURL.String, params.ID); decryptErr != nil {
		if credentialKeyUnavailable(decryptErr) {
			return fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, decryptErr)
		}
		return fmt.Errorf("%w: %v", keymanagement.ErrStorageIntegrity, decryptErr)
	}

	return nil
}

// Credentials are checked before opening the transaction, preserving error precedence.
func (r *Repository) beginIndexedProfileWrite(ctx context.Context, uri string) (*sql.Tx, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	index, err := r.credentials.blindIndex(uri)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	return tx, index, nil
}

func profileLookupError(err error, operation string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func (r *Repository) decryptLoadedProfile(envelope sql.NullString, id int64) (string, error) {
	if !envelope.Valid || envelope.String == "" {
		return "", keymanagement.ErrStorageIntegrity
	}
	raw, err := r.credentials.decrypt(envelope.String, id)
	if err != nil {
		return "", mapKeyCredentialError(err)
	}
	return raw, nil
}

func (r *Repository) commitProfileMutation(ctx context.Context, tx *sql.Tx, id int64, operation string) (*model.VLESSKey, string, error) {
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("%s: %w", operation, err)
	}
	return r.GetByID(ctx, id)
}
