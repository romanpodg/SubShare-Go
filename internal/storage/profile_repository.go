package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/vless"
)

type ProfileRepository struct {
	db      *sql.DB
	keyring *profilestorage.Keyring
}

func NewProfileRepository(db *sql.DB, keyring *profilestorage.Keyring) *ProfileRepository {
	return &ProfileRepository{
		db:      db,
		keyring: keyring,
	}
}

func nullStringValue(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt64Value(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func (r *ProfileRepository) GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	var key model.VLESSKey
	var encURL sql.NullString
	var category sql.NullString
	var kind sql.NullString
	var templateText sql.NullString
	var status sql.NullString
	var checkStatus sql.NullString
	var checkError sql.NullString
	var lastCheckedAt sql.NullTime
	var latency sql.NullInt64
	var categoryID sql.NullInt64
	var externalSourceID sql.NullInt64
	var externalSourceName sql.NullString
	var warningsJSON string
	var revision sql.NullInt64
	var updatedAt sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT k.id, k.label, s.encrypted_url, k.category_id, COALESCE(kc.name, k.category),
		       k.key_kind, k.template_text, k.status, k.check_status, k.check_error,
		       k.last_checked_at, k.last_latency_ms, k.created_at, k.external_source_id,
		       COALESCE(es.name, ''), k.protocol, k.profile_schema_version,
		       k.profile_compatibility, k.profile_warnings_json,
		       COALESCE(k.profile_revision, 1), COALESCE(k.updated_at, k.created_at)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		LEFT JOIN key_categories kc ON kc.id = k.category_id
		LEFT JOIN external_subscription_sources es ON es.id = k.external_source_id
		WHERE k.id = ?
	`, id).Scan(
		&key.ID, &key.Label, &encURL, &categoryID, &category, &kind, &templateText, &status,
		&checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt, &externalSourceID,
		&externalSourceName, &key.Protocol, &key.ProfileSchemaVersion, &key.ProfileCompatibility,
		&warningsJSON, &revision, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	if err != nil {
		return nil, "", err
	}

	decryptedURI := ""
	if encURL.Valid && encURL.String != "" && r.keyring != nil {
		if dec, err := profilestorage.Decrypt(encURL.String, r.keyring, key.ID); err == nil {
			decryptedURI = dec.Reveal()
		}
	}
	if err := json.Unmarshal([]byte(warningsJSON), &key.ProfileWarnings); err != nil || key.ProfileWarnings == nil {
		key.ProfileWarnings = []string{}
	}
	if categoryID.Valid {
		key.CategoryID = categoryID.Int64
	}
	key.Category = strings.TrimSpace(category.String)
	key.Kind, _ = model.NormalizeKeyKind(kind.String)
	if key.Kind == "" {
		key.Kind = model.KeyKindReal
	}
	key.TemplateText = strings.TrimSpace(templateText.String)
	key.Status, _ = model.NormalizeKeyStatus(status.String)
	if key.Status == "" {
		key.Status = model.KeyStatusActive
	}
	key.CheckStatus = model.NormalizeCheckStatus(checkStatus.String)
	key.CheckError = checkError.String
	if lastCheckedAt.Valid {
		key.LastCheckedAtText = lastCheckedAt.Time.Format("2006-01-02 15:04:05")
	}
	if latency.Valid {
		key.LastLatencyMS = latency.Int64
	}
	key.ProfileRevision = revision.Int64
	if key.ProfileRevision < 1 {
		key.ProfileRevision = 1
	}
	if updatedAt.Valid && updatedAt.String != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
			key.UpdatedAt = t
		} else if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			key.UpdatedAt = t
		} else {
			key.UpdatedAt = key.CreatedAt
		}
	} else {
		key.UpdatedAt = key.CreatedAt
	}
	if externalSourceID.Valid {
		key.ExternalSourceID = externalSourceID.Int64
	}
	key.ExternalSourceName = strings.TrimSpace(externalSourceName.String)

	return &key, decryptedURI, nil
}

func (r *ProfileRepository) UpsertCategory(ctx context.Context, category string) (int64, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return 0, nil
	}
	var nextSortOrder int64
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return 0, fmt.Errorf("failed to query sort order for category: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, '#4B5563', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
	`, category, nextSortOrder); err != nil {
		return 0, fmt.Errorf("failed to upsert category: %w", err)
	}
	var id int64
	if err := r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, category).Scan(&id); err != nil {
		return 0, nil
	}
	return id, nil
}

func (r *ProfileRepository) upsertCategoryTx(ctx context.Context, tx *sql.Tx, category string) (any, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil, nil
	}
	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return nil, fmt.Errorf("failed to query sort order for category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, '#4B5563', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
	`, category, nextSortOrder); err != nil {
		return nil, fmt.Errorf("failed to upsert category: %w", err)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, category).Scan(&id); err != nil {
		return nil, nil
	}
	return id, nil
}

func (r *ProfileRepository) resolveCategoryIDTx(ctx context.Context, tx *sql.Tx, category string, reqID *int64) (any, error) {
	if reqID != nil && *reqID > 0 {
		return *reqID, nil
	}
	if category == "" {
		return nil, nil
	}
	return r.upsertCategoryTx(ctx, tx, category)
}

func (r *ProfileRepository) CreateLocal(ctx context.Context, params profilepersistence.CreateProfileParams) (*model.VLESSKey, string, error) {
	activeID, activeKey, err := r.keyring.GetActiveEncryptionKey()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	_, bikKey, err := r.keyring.GetActiveBlindIndexKey()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, params.BuiltURI)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.resolveCategoryIDTx(ctx, tx, params.Category, params.CategoryID)
	if err != nil {
		return nil, "", err
	}

	var nextSort int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSort); err != nil {
		return nil, "", fmt.Errorf("failed to calculate sort order: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, protocol, profile_schema_version,
			profile_compatibility, profile_warnings_json, profile_revision,
			created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?, ?, 1, 'full', '[]', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, params.Label, blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), nextSort, params.Protocol)
	if err != nil {
		return nil, "", fmt.Errorf("create_failed: %w", err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := profilestorage.Encrypt([]byte(params.BuiltURI), activeID, activeKey, keyID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt secret: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		return nil, "", fmt.Errorf("failed to save secret: %w", err)
	}

	_, _ = tx.ExecContext(ctx, `
		INSERT INTO user_keys(user_id, key_id)
		SELECT id, ? FROM users WHERE key_assignment_mode = 'all'
	`, keyID)

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit key creation: %w", err)
	}

	return r.GetByID(ctx, keyID)
}

func (r *ProfileRepository) UpdateLocal(ctx context.Context, params profilepersistence.UpdateProfileParams) (*model.VLESSKey, string, error) {
	activeID, activeKey, err := r.keyring.GetActiveEncryptionKey()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	_, bikKey, err := r.keyring.GetActiveBlindIndexKey()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, params.NewURI)
	env, err := profilestorage.Encrypt([]byte(params.NewURI), activeID, activeKey, params.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt secret: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.resolveCategoryIDTx(ctx, tx, params.Category, params.CategoryID)
	if err != nil {
		return nil, "", err
	}

	var extSourceID sql.NullInt64
	var storedRev sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT external_source_id, COALESCE(profile_revision, 1) FROM vless_keys WHERE id = ?`, params.ID).Scan(&extSourceID, &storedRev)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to load key ownership: %w", err)
	}
	if extSourceID.Valid && extSourceID.Int64 > 0 {
		return nil, "", profilepersistence.ErrSourceOwnedProfile
	}
	if storedRev.Int64 != params.ExpectedRevision {
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE vless_keys
		SET label = ?, url_blind_index = ?, category_id = ?, category = ?, status = ?,
		    key_kind = ?, template_text = ?, protocol = ?, profile_revision = profile_revision + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND profile_revision = ? AND external_source_id IS NULL
	`, params.Label, blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), params.Protocol, params.ID, params.ExpectedRevision)
	if err != nil {
		return nil, "", fmt.Errorf("failed to update key: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)
		ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url
	`, params.ID, env); err != nil {
		return nil, "", fmt.Errorf("failed to update secret: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit key update: %w", err)
	}

	return r.GetByID(ctx, params.ID)
}

func (r *ProfileRepository) CloneLocal(ctx context.Context, params profilepersistence.CloneProfileParams) (*model.VLESSKey, string, error) {
	activeID, activeKey, err := r.keyring.GetActiveEncryptionKey()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	_, bikKey, err := r.keyring.GetActiveBlindIndexKey()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var sourceLabel, sourceCategory, sourceKind, sourceStatus, sourceProtocol string
	var sourceTemplateText, sourceWarnings, sourceEncURL sql.NullString
	var sourceCategoryID sql.NullInt64
	var sourceSchemaVer int
	var sourceRevision int64
	err = tx.QueryRowContext(ctx, `
		SELECT k.label, k.category, k.key_kind, k.template_text, k.status, k.protocol,
		       k.profile_schema_version, k.profile_warnings_json, s.encrypted_url,
		       k.category_id, COALESCE(k.profile_revision, 1)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.id = ?
	`, params.ID).Scan(
		&sourceLabel, &sourceCategory, &sourceKind, &sourceTemplateText, &sourceStatus,
		&sourceProtocol, &sourceSchemaVer, &sourceWarnings, &sourceEncURL, &sourceCategoryID, &sourceRevision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to load source key: %w", err)
	}

	if sourceRevision != params.ExpectedRevision {
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}

	if !sourceEncURL.Valid || sourceEncURL.String == "" {
		return nil, "", profilepersistence.ErrStorageIntegrity
	}

	dec, decErr := profilestorage.Decrypt(sourceEncURL.String, r.keyring, params.ID)
	if decErr != nil {
		if errors.Is(decErr, profilestorage.ErrUnknownKeyID) || errors.Is(decErr, profilestorage.ErrMissingKeyring) {
			return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, decErr)
		}
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrStorageIntegrity, decErr)
	}
	decryptedURI := dec.Reveal()

	newLabel := strings.TrimSpace(params.NewLabel)
	if newLabel == "" {
		newLabel = sourceLabel + " (Копия)"
	}

	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return nil, "", fmt.Errorf("failed to prepare key order: %w", err)
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, decryptedURI)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, external_source_id, external_key_ref,
			protocol, profile_schema_version, profile_compatibility, profile_warnings_json,
			profile_revision, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?, NULL, NULL, ?, ?, 'full', ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, newLabel, blindIndex, nullInt64Value(sourceCategoryID.Int64), sourceCategory, sourceStatus, sourceKind, nullStringValue(sourceTemplateText.String), nextSortOrder, sourceProtocol, sourceSchemaVer, nullStringValue(sourceWarnings.String))
	if err != nil {
		return nil, "", fmt.Errorf("failed to insert cloned key: %w", err)
	}

	newID, err := res.LastInsertId()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get new key ID: %w", err)
	}

	env, err := profilestorage.Encrypt([]byte(decryptedURI), activeID, activeKey, newID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt cloned key: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, newID, env); err != nil {
		return nil, "", fmt.Errorf("failed to save secret: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit clone: %w", err)
	}

	return r.GetByID(ctx, newID)
}

func (r *ProfileRepository) ListLegacy(ctx context.Context) ([]model.VLESSKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT k.id, k.label, s.encrypted_url, k.category_id, COALESCE(kc.name, k.category), k.key_kind, k.template_text, k.status, k.check_status, k.check_error, k.last_checked_at, k.last_latency_ms, k.created_at, k.external_source_id, COALESCE(es.name, ''), k.protocol, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json, COALESCE(k.profile_revision, 1), COALESCE(k.updated_at, k.created_at)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		LEFT JOIN key_categories kc ON kc.id = k.category_id
		LEFT JOIN external_subscription_sources es ON es.id = k.external_source_id
		ORDER BY k.sort_order, k.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.VLESSKey
	for rows.Next() {
		var key model.VLESSKey
		var encURL sql.NullString
		var category sql.NullString
		var kind sql.NullString
		var templateText sql.NullString
		var status sql.NullString
		var checkStatus sql.NullString
		var checkError sql.NullString
		var lastCheckedAt sql.NullTime
		var latency sql.NullInt64
		var categoryID sql.NullInt64
		var externalSourceID sql.NullInt64
		var externalSourceName sql.NullString
		var warningsJSON string
		var updatedAt sql.NullString
		if err := rows.Scan(&key.ID, &key.Label, &encURL, &categoryID, &category, &kind, &templateText, &status, &checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt, &externalSourceID, &externalSourceName, &key.Protocol, &key.ProfileSchemaVersion, &key.ProfileCompatibility, &warningsJSON, &key.ProfileRevision, &updatedAt); err != nil {
			return nil, err
		}
		if updatedAt.Valid && updatedAt.String != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
				key.UpdatedAt = t
			} else if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				key.UpdatedAt = t
			} else {
				key.UpdatedAt = key.CreatedAt
			}
		} else {
			key.UpdatedAt = key.CreatedAt
		}
		if encURL.Valid && encURL.String != "" && r.keyring != nil {
			if dec, err := profilestorage.Decrypt(encURL.String, r.keyring, key.ID); err == nil {
				key.URL = dec.Reveal()
			}
		}
		if err := json.Unmarshal([]byte(warningsJSON), &key.ProfileWarnings); err != nil || key.ProfileWarnings == nil {
			key.ProfileWarnings = []string{}
		}
		if categoryID.Valid {
			key.CategoryID = categoryID.Int64
		}
		key.Category = strings.TrimSpace(category.String)
		key.Kind, _ = model.NormalizeKeyKind(kind.String)
		if key.Kind == "" {
			key.Kind = model.KeyKindReal
		}
		key.TemplateText = strings.TrimSpace(templateText.String)
		key.Status, _ = model.NormalizeKeyStatus(status.String)
		if key.Status == "" {
			key.Status = model.KeyStatusActive
		}
		key.StatusLabel = model.KeyStatusLabel(key.Status)
		if key.Kind == model.KeyKindInformational {
			key.URLShort = "Информационный ключ"
			if key.TemplateText != "" {
				key.URLShort = vless.TruncateMiddle(key.TemplateText, 88)
			}
		} else {
			key.URLShort = vless.TruncateMiddle(key.URL, 88)
		}
		key.CheckStatus = model.NormalizeCheckStatus(checkStatus.String)
		key.CheckStatusLabel = model.CheckStatusLabel(key.CheckStatus)
		key.CheckError = strings.TrimSpace(checkError.String)
		if key.Kind == model.KeyKindReal {
			key.EditUUID, key.EditHost, key.EditPort, key.EditQuery, key.EditFragment, _ = vless.ParseVLESSParts(key.URL)
		}
		if latency.Valid {
			key.LastLatencyMS = latency.Int64
		}
		if lastCheckedAt.Valid {
			key.LastCheckedAtText = lastCheckedAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		if externalSourceID.Valid && externalSourceID.Int64 > 0 {
			key.ExternalSourceID = externalSourceID.Int64
		}
		key.ExternalSourceName = strings.TrimSpace(externalSourceName.String)
		key.ClientDisplayName = profileconfig.ClientDisplayNameFromKeyURL(key.URL, key.Label)
		out = append(out, key)
	}
	return out, rows.Err()
}

func (r *ProfileRepository) CreateLegacy(ctx context.Context, params profilepersistence.CreateLegacyKeyParams) (int64, error) {
	activeID, activeKey, err := r.keyring.GetActiveEncryptionKey()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	_, bikKey, err := r.keyring.GetActiveBlindIndexKey()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, params.KeyURL)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.upsertCategoryTx(ctx, tx, params.Category)
	if err != nil {
		return 0, fmt.Errorf("failed to save key category: %w", err)
	}

	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return 0, fmt.Errorf("failed to prepare key order: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(label, url_blind_index, category_id, category, status, check_status, key_kind, template_text, sort_order)
		VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?)
	`, params.Label, blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), nextSortOrder)
	if err != nil {
		return 0, fmt.Errorf("create_failed: %w", err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := profilestorage.Encrypt([]byte(params.KeyURL), activeID, activeKey, keyID)
	if err != nil {
		return 0, fmt.Errorf("failed to encrypt key: %w", err)
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

func (r *ProfileRepository) UpdateLegacy(ctx context.Context, params profilepersistence.UpdateLegacyKeyParams) error {
	activeID, activeKey, err := r.keyring.GetActiveEncryptionKey()
	if err != nil {
		return fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	_, bikKey, err := r.keyring.GetActiveBlindIndexKey()
	if err != nil {
		return fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
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
		return profilepersistence.ErrProfileNotFound
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
			return fmt.Errorf("failed to prepare key order: %w", err)
		}
	}

	categoryIDVal, err := r.upsertCategoryTx(ctx, tx, params.Category)
	if err != nil {
		return fmt.Errorf("failed to save key category: %w", err)
	}

	blindIndex := profilestorage.ComputeBlindIndex(bikKey, params.BuiltURL)
	env, err := profilestorage.Encrypt([]byte(params.BuiltURL), activeID, activeKey, params.ID)
	if err != nil {
		return fmt.Errorf("failed to encrypt key: %w", err)
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

func (r *ProfileRepository) DeleteLegacy(ctx context.Context, id int64) error {
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
		return profilepersistence.ErrProfileNotFound
	}

	_, _ = tx.ExecContext(ctx, `DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, id)
	_, _ = tx.ExecContext(ctx, `DELETE FROM user_keys WHERE key_id = ?`, id)

	return tx.Commit()
}
