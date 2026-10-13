package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

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

	keyID, err := r.insertLegacyProfile(ctx, tx, legacyProfileCreation{params: params, blindIndex: blindIndex})
	if err != nil {
		return 0, err
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

	sortOrder, err := legacyUpdateSortOrder(ctx, tx, params)
	if err != nil {
		return err
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

type legacyProfileCreation struct {
	params     keymanagement.CreateLegacyKeyParams
	blindIndex string
}

func (r *Repository) insertLegacyProfile(ctx context.Context, tx *sql.Tx, creation legacyProfileCreation) (int64, error) {
	categoryIDVal, err := r.categories.upsertTx(ctx, tx, creation.params.Category, "#4B5563")
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
	`, creation.params.Label, creation.blindIndex, categoryIDVal, creation.params.Category, creation.params.Status, creation.params.Kind, nullStringValue(creation.params.TemplateText), nextSortOrder)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrKeyCreateConflict, err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := r.credentials.encrypt(creation.params.KeyURL, keyID)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keymanagement.ErrCredentialEncryption, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		return 0, fmt.Errorf("failed to save key secret: %w", err)
	}

	return keyID, nil
}

func legacyUpdateSortOrder(ctx context.Context, tx *sql.Tx, params keymanagement.UpdateLegacyKeyParams) (int64, error) {
	var existingKind sql.NullString
	var existingSort sql.NullInt64
	var extSourceID sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT key_kind, sort_order, external_source_id FROM vless_keys WHERE id = ?`, params.ID).Scan(&existingKind, &existingSort, &extSourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query existing key: %w", err)
	}

	existingKindNormalized, _ := model.NormalizeKeyKind(existingKind.String)
	if existingKindNormalized == "" {
		existingKindNormalized = model.KeyKindReal
	}

	sortOrder := existingSort.Int64
	if !existingSort.Valid || sortOrder <= 0 || existingKindNormalized != params.Kind {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&sortOrder); err != nil {
			return 0, fmt.Errorf("%w: %v", keymanagement.ErrKeyOrderPersistence, err)
		}
	}

	return sortOrder, nil
}
