package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"

	"github.com/romanpodg/SubShare-Go/internal/model"
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

	keyID, err := r.insertLocalProfile(ctx, tx, localProfileCreation{params: params, blindIndex: blindIndex})
	if err != nil {
		return nil, "", err
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

	if err := r.applyLocalProfileUpdate(ctx, tx, localProfileUpdate{params: params, blindIndex: blindIndex}); err != nil {
		return nil, "", err
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

	if err := r.validateSourceMetadataUpdate(ctx, tx, params); err != nil {
		return nil, "", err
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
