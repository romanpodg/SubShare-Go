package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

// Repository is the SQLite adapter for keymanagement.Repository.
type Repository struct {
	db          *sql.DB
	credentials *credentialStore
	categories  *categoryStore
}

func NewRepository(db *sql.DB, keyring *profilestorage.Keyring) *Repository {
	return &Repository{
		db:          db,
		credentials: newCredentialStore(keyring),
		categories:  newCategoryStore(db),
	}
}

func nullStringValue(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func optionalNullStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return nullStringValue(*value)
}

func nullInt64Value(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func (r *Repository) CreateLocal(ctx context.Context, params keymanagement.CreateProfileParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.BuiltURI)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.categories.resolveTx(ctx, tx, params.Category, params.CategoryID, "#4B5563")
	if err != nil {
		return nil, "", err
	}

	var nextSort int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSort); err != nil {
		return nil, "", fmt.Errorf("failed to calculate sort order: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, client_display_name, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, protocol, profile_schema_version,
			profile_compatibility, profile_warnings_json, profile_revision,
			created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, 'unknown', ?, ?, ?, ?, 1, 'full', '[]', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, params.Label, nullStringValue(params.ClientDisplayName), blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), nextSort, params.Protocol)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrProfileCreateConflict, err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := r.credentials.encrypt(params.BuiltURI, keyID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt secret: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		return nil, "", fmt.Errorf("failed to save secret: %w", err)
	}

	if err := commitLocalProfileCreation(ctx, tx, keyID); err != nil {
		return nil, "", err
	}

	return r.GetByID(ctx, keyID)
}

// commitLocalProfileCreation keeps required all-mode assignments in the same
// transaction as the profile and secret. A failed assignment cannot commit.
func commitLocalProfileCreation(ctx context.Context, tx *sql.Tx, keyID int64) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_keys(user_id, key_id)
		SELECT id, ? FROM users WHERE key_assignment_mode = 'all'
	`, keyID); err != nil {
		return fmt.Errorf("failed to assign key to all-mode users: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit key creation: %w", err)
	}
	return nil
}

func (r *Repository) UpdateLocal(ctx context.Context, params keymanagement.UpdateProfileParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.NewURI)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.categories.resolveTx(ctx, tx, params.Category, params.CategoryID, "#4B5563")
	if err != nil {
		return nil, "", err
	}

	storedURI, storedBlindIndex, err := r.loadLocalKeyForUpdate(ctx, tx, params.ID, params.ExpectedRevision)
	if err != nil {
		return nil, "", err
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
		return nil, "", fmt.Errorf("failed to update key: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, "", keymanagement.ErrProfileRevisionConflict
	}

	if uriChanged {
		env, encryptErr := r.credentials.encrypt(params.NewURI, params.ID)
		if encryptErr != nil {
			return nil, "", fmt.Errorf("failed to encrypt secret: %w", encryptErr)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)
			ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url
		`, params.ID, env); err != nil {
			return nil, "", fmt.Errorf("failed to update secret: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit key update: %w", err)
	}

	return r.GetByID(ctx, params.ID)
}

// loadLocalKeyForUpdate checks that the key exists, is locally owned, sits at
// the expected revision and has a decryptable secret; it returns the current
// plaintext URI and blind index.
func (r *Repository) loadLocalKeyForUpdate(ctx context.Context, tx *sql.Tx, id, expectedRevision int64) (string, string, error) {
	var extSourceID sql.NullInt64
	var storedRev sql.NullInt64
	var storedBlindIndex string
	var storedEncryptedURL sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT k.external_source_id, COALESCE(k.profile_revision, 1), k.url_blind_index, s.encrypted_url
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON s.vless_key_id = k.id
		WHERE k.id = ?
	`, id).Scan(&extSourceID, &storedRev, &storedBlindIndex, &storedEncryptedURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to load key ownership: %w", err)
	}
	if extSourceID.Valid && extSourceID.Int64 > 0 {
		return "", "", keymanagement.ErrSourceOwnedReadOnly
	}
	if storedRev.Int64 != expectedRevision {
		return "", "", keymanagement.ErrProfileRevisionConflict
	}
	if !storedEncryptedURL.Valid || storedEncryptedURL.String == "" {
		return "", "", keymanagement.ErrStorageIntegrity
	}
	storedURI, decryptErr := r.credentials.decrypt(storedEncryptedURL.String, id)
	if decryptErr != nil {
		return "", "", mapKeyCredentialError(decryptErr)
	}
	return storedURI, storedBlindIndex, nil
}

// UpdateSourceOwnedMetadata changes only locally administered delivery metadata.
// Source-controlled fields and the encrypted configuration are deliberately
// excluded from the UPDATE.
func (r *Repository) UpdateSourceOwnedMetadata(ctx context.Context, params keymanagement.UpdateSourceOwnedMetadataParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var storedRevision int64
	var storedEncryptedURL sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(k.profile_revision, 1), s.encrypted_url
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON s.vless_key_id = k.id
		WHERE k.id = ?
	`, params.ID).Scan(&storedRevision, &storedEncryptedURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to load key metadata: %w", err)
	}
	if storedRevision != params.ExpectedRevision {
		return nil, "", keymanagement.ErrProfileRevisionConflict
	}
	if !storedEncryptedURL.Valid || storedEncryptedURL.String == "" {
		return nil, "", keymanagement.ErrStorageIntegrity
	}
	if _, decryptErr := r.credentials.decrypt(storedEncryptedURL.String, params.ID); decryptErr != nil {
		if credentialKeyUnavailable(decryptErr) {
			return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, decryptErr)
		}
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrStorageIntegrity, decryptErr)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE vless_keys
		SET status = ?,
		    client_display_name = CASE WHEN ? THEN ? ELSE client_display_name END,
		    profile_revision = profile_revision + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND profile_revision = ? AND external_source_id IS NOT NULL
	`, params.Status, params.ClientDisplayName != nil, optionalNullStringValue(params.ClientDisplayName), params.ID, params.ExpectedRevision)
	if err != nil {
		return nil, "", fmt.Errorf("failed to update source-owned metadata: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, "", keymanagement.ErrProfileRevisionConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit source-owned metadata: %w", err)
	}
	return r.GetByID(ctx, params.ID)
}

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

	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return nil, "", fmt.Errorf("failed to prepare key order: %w", err)
	}

	blindIndex, err := r.credentials.cloneBlindIndex(decryptedURI)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, client_display_name, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, external_source_id, external_key_ref,
			protocol, profile_schema_version, profile_compatibility, profile_warnings_json,
			profile_revision, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, 'unknown', ?, ?, ?, NULL, NULL, ?, ?, 'full', ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, newLabel, nullStringValue(cloneClientDisplayName), blindIndex, nullInt64Value(src.categoryID.Int64), src.category, src.status, src.kind, nullStringValue(src.templateText.String), nextSortOrder, src.protocol, src.schemaVersion, nullStringValue(src.warnings.String))
	if err != nil {
		return nil, "", fmt.Errorf("failed to insert cloned key: %w", err)
	}

	newID, err := res.LastInsertId()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get new key ID: %w", err)
	}

	env, err := r.credentials.encrypt(decryptedURI, newID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt cloned key: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, newID, env); err != nil {
		return nil, "", fmt.Errorf("failed to save secret: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_keys(user_id, key_id)
		SELECT id, ? FROM users WHERE key_assignment_mode = 'all'
	`, newID); err != nil {
		return nil, "", fmt.Errorf("failed to add cloned key to users: %w", err)
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
	if name != "" || !src.externalSourceID.Valid || src.externalSourceID.Int64 <= 0 {
		return name
	}
	sourceEffectiveName := profileconfig.EffectiveClientDisplayName("", decryptedURI, src.label, true)
	localFallbackName := profileconfig.EffectiveClientDisplayName("", decryptedURI, newLabel, false)
	if sourceEffectiveName != localFallbackName {
		return sourceEffectiveName
	}
	return name
}

func (r *Repository) CreateLegacy(ctx context.Context, params keymanagement.CreateLegacyKeyParams) (int64, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.KeyURL)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.categories.upsertTx(ctx, tx, params.Category, "#4B5563")
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrCategoryPersistence, err)
	}

	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrKeyOrderPersistence, err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(label, url_blind_index, category_id, category, status, check_status, key_kind, template_text, sort_order)
		VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?)
	`, params.Label, blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), nextSortOrder)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrKeyCreateConflict, err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := r.credentials.encrypt(params.KeyURL, keyID)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrCredentialEncryption, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		return 0, fmt.Errorf("failed to save key secret: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_keys(user_id, key_id)
		SELECT id, ? FROM users WHERE key_assignment_mode = 'all'
	`, keyID); err != nil {
		return 0, fmt.Errorf("failed to add key to users: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit key creation: %w", err)
	}

	return keyID, nil
}

func (r *Repository) UpdateLegacy(ctx context.Context, params keymanagement.UpdateLegacyKeyParams) error {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.BuiltURL)
	if err != nil {
		return fmt.Errorf("%w: %v", keymanagement.ErrBlindIndexUnavailable, err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var existingKind sql.NullString
	var existingSort sql.NullInt64
	var extSourceID sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT key_kind, sort_order, external_source_id FROM vless_keys WHERE id = ?`, params.ID).Scan(&existingKind, &existingSort, &extSourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to query existing key: %w", err)
	}

	existingKindNormalized, _ := model.NormalizeKeyKind(existingKind.String)
	if existingKindNormalized == "" {
		existingKindNormalized = model.KeyKindReal
	}

	sortOrder := existingSort.Int64
	if !existingSort.Valid || sortOrder <= 0 || existingKindNormalized != params.Kind {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&sortOrder); err != nil {
			return fmt.Errorf("%w: %v", keymanagement.ErrKeyOrderPersistence, err)
		}
	}

	categoryIDVal, err := r.categories.upsertTx(ctx, tx, params.Category, "#4B5563")
	if err != nil {
		return fmt.Errorf("%w: %v", keymanagement.ErrCategoryPersistence, err)
	}

	env, err := r.credentials.encrypt(params.BuiltURL, params.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", keymanagement.ErrCredentialEncryption, err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE vless_keys
		SET label = ?, url_blind_index = ?, category_id = ?, category = ?, status = ?, key_kind = ?, template_text = ?, sort_order = ?
		WHERE id = ?
	`, params.Label, blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), sortOrder, params.ID); err != nil {
		return fmt.Errorf("failed to update key: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)
		ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url
	`, params.ID, env); err != nil {
		return fmt.Errorf("failed to update key secret: %w", err)
	}

	return tx.Commit()
}

func (r *Repository) DeleteLegacy(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM vless_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return keymanagement.ErrKeyNotFound
	}

	_, _ = tx.ExecContext(ctx, `DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, id)
	_, _ = tx.ExecContext(ctx, `DELETE FROM user_keys WHERE key_id = ?`, id)

	return tx.Commit()
}
