package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/vless"
)

type ProfileRepository struct {
	db          *sql.DB
	credentials *credentialStore
	categories  *categoryStore
}

func NewProfileRepository(db *sql.DB, keyring *profilestorage.Keyring) *ProfileRepository {
	return &ProfileRepository{
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

func (r *ProfileRepository) GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	return loadKeyByID(ctx, r.db, r.credentials, id)
}

func loadKeyByID(ctx context.Context, db *sql.DB, credentials *credentialStore, id int64) (*model.VLESSKey, string, error) {
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
	var storedClientDisplayName sql.NullString
	var warningsJSON string
	var revision sql.NullInt64
	var updatedAt sql.NullString

	err := db.QueryRowContext(ctx, `
		SELECT k.id, k.label, k.client_display_name, s.encrypted_url, k.category_id, COALESCE(kc.name, k.category),
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
		&key.ID, &key.Label, &storedClientDisplayName, &encURL, &categoryID, &category, &kind, &templateText, &status,
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

	decryptedURI, err := credentials.decrypt(encURL.String, key.ID)
	if err != nil {
		if credentialKeyUnavailable(err) {
			return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
		}
		return nil, "", fmt.Errorf("%w: %w", profilepersistence.ErrStorageIntegrity, err)
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
	key.ClientDisplayNameOverridden = strings.TrimSpace(storedClientDisplayName.String) != ""
	key.ClientDisplayName = profileconfig.EffectiveClientDisplayName(
		storedClientDisplayName.String, decryptedURI, key.Label, key.ExternalSourceID > 0,
	)

	return &key, decryptedURI, nil
}

func (r *ProfileRepository) CreateLocal(ctx context.Context, params profilepersistence.CreateProfileParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.BuiltURI)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
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
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrProfileCreateConflict, err)
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
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.NewURI)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
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

	var extSourceID sql.NullInt64
	var storedRev sql.NullInt64
	var storedBlindIndex string
	var storedEncryptedURL sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT k.external_source_id, COALESCE(k.profile_revision, 1), k.url_blind_index, s.encrypted_url
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON s.vless_key_id = k.id
		WHERE k.id = ?
	`, params.ID).Scan(&extSourceID, &storedRev, &storedBlindIndex, &storedEncryptedURL)
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
	if !storedEncryptedURL.Valid || storedEncryptedURL.String == "" {
		return nil, "", profilepersistence.ErrStorageIntegrity
	}
	storedURI, decryptErr := r.credentials.decrypt(storedEncryptedURL.String, params.ID)
	if decryptErr != nil {
		if credentialKeyUnavailable(decryptErr) {
			return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, decryptErr)
		}
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrStorageIntegrity, decryptErr)
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
		return nil, "", profilepersistence.ErrProfileRevisionConflict
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

// UpdateSourceOwnedMetadata changes only locally administered delivery metadata.
// Source-controlled fields and the encrypted configuration are deliberately
// excluded from the UPDATE.
func (r *ProfileRepository) UpdateSourceOwnedMetadata(ctx context.Context, params profilepersistence.UpdateSourceOwnedMetadataParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
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
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to load key metadata: %w", err)
	}
	if storedRevision != params.ExpectedRevision {
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}
	if !storedEncryptedURL.Valid || storedEncryptedURL.String == "" {
		return nil, "", profilepersistence.ErrStorageIntegrity
	}
	if _, decryptErr := r.credentials.decrypt(storedEncryptedURL.String, params.ID); decryptErr != nil {
		if credentialKeyUnavailable(decryptErr) {
			return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, decryptErr)
		}
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrStorageIntegrity, decryptErr)
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
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("failed to commit source-owned metadata: %w", err)
	}
	return r.GetByID(ctx, params.ID)
}

func (r *ProfileRepository) CloneLocal(ctx context.Context, params profilepersistence.CloneProfileParams) (*model.VLESSKey, string, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, err)
	}
	if _, err := r.credentials.blindIndex(""); err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var sourceLabel, sourceCategory, sourceKind, sourceStatus, sourceProtocol string
	var sourceTemplateText, sourceWarnings, sourceEncURL, sourceClientDisplayName sql.NullString
	var sourceCategoryID, sourceExternalSourceID sql.NullInt64
	var sourceSchemaVer int
	var sourceRevision int64
	err = tx.QueryRowContext(ctx, `
		SELECT k.label, k.client_display_name, k.category, k.key_kind, k.template_text, k.status, k.protocol,
		       k.profile_schema_version, k.profile_warnings_json, s.encrypted_url,
		       k.category_id, k.external_source_id, COALESCE(k.profile_revision, 1)
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.id = ?
	`, params.ID).Scan(
		&sourceLabel, &sourceClientDisplayName, &sourceCategory, &sourceKind, &sourceTemplateText, &sourceStatus,
		&sourceProtocol, &sourceSchemaVer, &sourceWarnings, &sourceEncURL, &sourceCategoryID, &sourceExternalSourceID, &sourceRevision,
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

	decryptedURI, decErr := r.credentials.decrypt(sourceEncURL.String, params.ID)
	if decErr != nil {
		if credentialKeyUnavailable(decErr) {
			return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrEncryptionUnavailable, decErr)
		}
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrStorageIntegrity, decErr)
	}

	newLabel := strings.TrimSpace(params.NewLabel)
	if newLabel == "" {
		newLabel = sourceLabel + " (Копия)"
	}
	cloneClientDisplayName := strings.TrimSpace(sourceClientDisplayName.String)
	if cloneClientDisplayName == "" && sourceExternalSourceID.Valid && sourceExternalSourceID.Int64 > 0 {
		sourceEffectiveName := profileconfig.EffectiveClientDisplayName("", decryptedURI, sourceLabel, true)
		localFallbackName := profileconfig.EffectiveClientDisplayName("", decryptedURI, newLabel, false)
		if sourceEffectiveName != localFallbackName {
			cloneClientDisplayName = sourceEffectiveName
		}
	}

	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return nil, "", fmt.Errorf("failed to prepare key order: %w", err)
	}

	blindIndex, err := r.credentials.cloneBlindIndex(decryptedURI)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", profilepersistence.ErrBlindIndexUnavailable, err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(
			label, client_display_name, url_blind_index, category_id, category, status, check_status,
			key_kind, template_text, sort_order, external_source_id, external_key_ref,
			protocol, profile_schema_version, profile_compatibility, profile_warnings_json,
			profile_revision, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, 'unknown', ?, ?, ?, NULL, NULL, ?, ?, 'full', ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, newLabel, nullStringValue(cloneClientDisplayName), blindIndex, nullInt64Value(sourceCategoryID.Int64), sourceCategory, sourceStatus, sourceKind, nullStringValue(sourceTemplateText.String), nextSortOrder, sourceProtocol, sourceSchemaVer, nullStringValue(sourceWarnings.String))
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

func (r *KeyRepository) ListLegacy(ctx context.Context) ([]model.VLESSKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT k.id, k.label, k.client_display_name, s.encrypted_url, k.category_id, COALESCE(kc.name, k.category), k.key_kind, k.template_text, k.status, k.check_status, k.check_error, k.last_checked_at, k.last_latency_ms, k.created_at, k.external_source_id, COALESCE(es.name, ''), k.protocol, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json, COALESCE(k.profile_revision, 1), COALESCE(k.updated_at, k.created_at)
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
		var storedClientDisplayName sql.NullString
		var warningsJSON string
		var updatedAt sql.NullString
		if err := rows.Scan(&key.ID, &key.Label, &storedClientDisplayName, &encURL, &categoryID, &category, &kind, &templateText, &status, &checkStatus, &checkError, &lastCheckedAt, &latency, &key.CreatedAt, &externalSourceID, &externalSourceName, &key.Protocol, &key.ProfileSchemaVersion, &key.ProfileCompatibility, &warningsJSON, &key.ProfileRevision, &updatedAt); err != nil {
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
		decryptedURL, decryptErr := r.credentials.decrypt(encURL.String, key.ID)
		if decryptErr != nil {
			return nil, mapKeyCredentialError(decryptErr)
		}
		key.URL = decryptedURL
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
		key.ClientDisplayNameOverridden = strings.TrimSpace(storedClientDisplayName.String) != ""
		key.ClientDisplayName = profileconfig.EffectiveClientDisplayName(
			storedClientDisplayName.String, key.URL, key.Label, key.ExternalSourceID > 0,
		)
		out = append(out, key)
	}
	return out, rows.Err()
}

func (r *KeyRepository) CreateLegacy(ctx context.Context, params keypersistence.CreateLegacyKeyParams) (int64, error) {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return 0, fmt.Errorf("%w: %v", keypersistence.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.KeyURL)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keypersistence.ErrBlindIndexUnavailable, err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	categoryIDVal, err := r.categories.upsertTx(ctx, tx, params.Category, "#4B5563")
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keypersistence.ErrCategoryPersistence, err)
	}

	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys`).Scan(&nextSortOrder); err != nil {
		return 0, fmt.Errorf("%w: %v", keypersistence.ErrKeyOrderPersistence, err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO vless_keys(label, url_blind_index, category_id, category, status, check_status, key_kind, template_text, sort_order)
		VALUES(?, ?, ?, ?, ?, 'unknown', ?, ?, ?)
	`, params.Label, blindIndex, categoryIDVal, params.Category, params.Status, params.Kind, nullStringValue(params.TemplateText), nextSortOrder)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keypersistence.ErrKeyCreateConflict, err)
	}

	keyID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get key ID: %w", err)
	}

	env, err := r.credentials.encrypt(params.KeyURL, keyID)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", keypersistence.ErrCredentialEncryption, err)
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

func (r *KeyRepository) UpdateLegacy(ctx context.Context, params keypersistence.UpdateLegacyKeyParams) error {
	if err := r.credentials.encryptionAvailable(); err != nil {
		return fmt.Errorf("%w: %v", keypersistence.ErrEncryptionUnavailable, err)
	}
	blindIndex, err := r.credentials.blindIndex(params.BuiltURL)
	if err != nil {
		return fmt.Errorf("%w: %v", keypersistence.ErrBlindIndexUnavailable, err)
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
		return keypersistence.ErrKeyNotFound
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
			return fmt.Errorf("%w: %v", keypersistence.ErrKeyOrderPersistence, err)
		}
	}

	categoryIDVal, err := r.categories.upsertTx(ctx, tx, params.Category, "#4B5563")
	if err != nil {
		return fmt.Errorf("%w: %v", keypersistence.ErrCategoryPersistence, err)
	}

	env, err := r.credentials.encrypt(params.BuiltURL, params.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", keypersistence.ErrCredentialEncryption, err)
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

func (r *KeyRepository) DeleteLegacy(ctx context.Context, id int64) error {
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
		return keypersistence.ErrKeyNotFound
	}

	_, _ = tx.ExecContext(ctx, `DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, id)
	_, _ = tx.ExecContext(ctx, `DELETE FROM user_keys WHERE key_id = ?`, id)

	return tx.Commit()
}

var categoryHexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func normalizeCategoryColor(raw string) string {
	val := strings.TrimSpace(raw)
	if val == "" {
		return "#D8B33D"
	}
	if categoryHexColorRegex.MatchString(val) {
		return strings.ToUpper(val)
	}
	return "#D8B33D"
}

func normalizeCategoryName(raw string) string {
	val := strings.TrimSpace(raw)
	if len(val) > 24 {
		val = val[:24]
	}
	return val
}

func (r *KeyRepository) ListKeyCategories(ctx context.Context) ([]model.KeyCategory, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, color FROM key_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to load key categories: %w", err)
	}
	defer rows.Close()

	countByName := make(map[string]int)
	colorByName := make(map[string]string)
	categories := make([]model.KeyCategory, 0, 16)
	for rows.Next() {
		var id int64
		var name sql.NullString
		var color sql.NullString
		if err := rows.Scan(&id, &name, &color); err != nil {
			return nil, fmt.Errorf("failed to scan key category: %w", err)
		}
		normalized := normalizeCategoryName(name.String)
		if normalized == "" {
			continue
		}
		if _, exists := countByName[normalized]; exists {
			continue
		}
		countByName[normalized] = 0
		colorByName[normalized] = normalizeCategoryColor(color.String)
		categories = append(categories, model.KeyCategory{ID: id, Name: normalized, Color: colorByName[normalized], KeysCount: 0})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read key categories: %w", err)
	}

	countRows, err := r.db.QueryContext(ctx, `
		SELECT kc.name, COUNT(*)
		FROM vless_keys k
		JOIN key_categories kc ON kc.id = k.category_id
		GROUP BY k.category_id, kc.name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load key category counts: %w", err)
	}
	defer countRows.Close()

	for countRows.Next() {
		var category sql.NullString
		var count int64
		if err := countRows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("failed to scan category count: %w", err)
		}
		normalized := normalizeCategoryName(category.String)
		if normalized == "" {
			continue
		}
		if _, exists := countByName[normalized]; !exists {
			colorByName[normalized] = "#D8B33D"
			var id int64
			_ = r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, normalized).Scan(&id)
			categories = append(categories, model.KeyCategory{ID: id, Name: normalized, Color: colorByName[normalized], KeysCount: 0})
		}
		countByName[normalized] += int(count)
	}
	if err := countRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read category counts: %w", err)
	}

	for index := range categories {
		categories[index].KeysCount = countByName[categories[index].Name]
		categories[index].Color = normalizeCategoryColor(colorByName[categories[index].Name])
	}
	return categories, nil
}

func (r *KeyRepository) GetCategoryColor(ctx context.Context, name string) (string, error) {
	var currentColor sql.NullString
	_ = r.db.QueryRowContext(ctx, `SELECT color FROM key_categories WHERE name = ?`, normalizeCategoryName(name)).Scan(&currentColor)
	return currentColor.String, nil
}

func (r *KeyRepository) CreateKeyCategory(ctx context.Context, params keypersistence.CreateCategoryParams) (model.KeyCategory, error) {
	name := normalizeCategoryName(params.Name)
	color := params.Color

	var nextSortOrder int64
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to prepare category order: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET color = excluded.color, updated_at = CURRENT_TIMESTAMP
	`, name, color, nextSortOrder); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to create key category: %w", err)
	}

	var catID int64
	_ = r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, name).Scan(&catID)

	return model.KeyCategory{
		ID:    catID,
		Name:  name,
		Color: color,
	}, nil
}

func (r *KeyRepository) UpdateKeyCategory(ctx context.Context, params keypersistence.UpdateCategoryParams) (model.KeyCategory, error) {
	oldName := normalizeCategoryName(params.OldName)
	newName := normalizeCategoryName(params.NewName)
	color := params.Color

	var keyCount int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM vless_keys
		 WHERE category_id = (SELECT id FROM key_categories WHERE name = ?)
		    OR (category_id IS NULL AND category = ?)
	`, oldName, oldName).Scan(&keyCount); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to count category keys: %w", err)
	}

	var categoryCount int64
	var existingSortOrder int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM key_categories WHERE name = ?`, oldName).Scan(&categoryCount); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to count category: %w", err)
	}
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(sort_order, 0) FROM key_categories WHERE name = ?`, oldName).Scan(&existingSortOrder)

	if keyCount == 0 && categoryCount == 0 {
		return model.KeyCategory{}, keypersistence.ErrCategoryNotFound
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET color = excluded.color, sort_order = COALESCE(NULLIF(key_categories.sort_order, 0), excluded.sort_order), updated_at = CURRENT_TIMESTAMP
	`, newName, color, existingSortOrder); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to upsert key category: %w", err)
	}

	if oldName != newName {
		var newCategoryID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, newName).Scan(&newCategoryID); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to resolve key category ID: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE vless_keys
			   SET category_id = ?, category = ?
			 WHERE category_id = (SELECT id FROM key_categories WHERE name = ?)
			    OR (category_id IS NULL AND category = ?)
		`, newCategoryID, newName, oldName, oldName); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to update key category references: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE external_subscription_sources
			   SET key_category_id = ?, key_category = ?
			 WHERE key_category_id = (SELECT id FROM key_categories WHERE name = ?)
			    OR (key_category_id IS NULL AND key_category = ?)
		`, newCategoryID, newName, oldName, oldName); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to update source category references: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM key_categories WHERE name = ?`, oldName); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to delete old category: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE key_categories SET color = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, color, newName); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to update category color: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to commit category update: %w", err)
	}

	var catID int64
	_ = r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, newName).Scan(&catID)

	return model.KeyCategory{
		ID:    catID,
		Name:  newName,
		Color: color,
	}, nil
}

func (r *KeyRepository) DeleteKeyCategory(ctx context.Context, params keypersistence.DeleteCategoryParams) error {
	name := normalizeCategoryName(params.Name)
	mode := strings.TrimSpace(params.Mode)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var categoryID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, name).Scan(&categoryID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to resolve key category: %w", err)
	}

	if mode == "delete_with_keys" {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM vless_keys
			 WHERE category_id = ? OR (category_id IS NULL AND category = ?)
		`, categoryID, name); err != nil {
			return fmt.Errorf("failed to delete keys in category: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE vless_keys SET category_id = NULL, category = ''
			 WHERE category_id = ? OR (category_id IS NULL AND category = ?)
		`, categoryID, name); err != nil {
			return fmt.Errorf("failed to clear keys category: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM key_categories WHERE name = ?`, name); err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	return tx.Commit()
}

func (r *KeyRepository) ReorderKeyCategories(ctx context.Context, names []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE key_categories SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare reorder statement: %w", err)
	}
	defer stmt.Close()

	for index, name := range names {
		if _, err := stmt.ExecContext(ctx, index+1, name); err != nil {
			return fmt.Errorf("failed to update category order for %s: %w", name, err)
		}
	}

	return tx.Commit()
}

func (r *KeyRepository) ReorderKeys(ctx context.Context, ids []int64) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM vless_keys ORDER BY sort_order, id`)
	if err != nil {
		return fmt.Errorf("failed to load keys for reorder: %w", err)
	}
	defer rows.Close()

	existingIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to read key id: %w", err)
		}
		existingIDs = append(existingIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate key ids: %w", err)
	}

	if len(existingIDs) != len(ids) {
		return keypersistence.ErrInvalidKeyOrderCount
	}

	allowed := make(map[int64]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		allowed[id] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return keypersistence.ErrUnknownKeyInOrder
		}
		if _, ok := seen[id]; ok {
			return keypersistence.ErrDuplicateKeyInOrder
		}
		seen[id] = struct{}{}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE vless_keys SET sort_order = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare key reorder statement: %w", err)
	}
	defer stmt.Close()

	for index, id := range ids {
		if _, err := stmt.ExecContext(ctx, index+1, id); err != nil {
			return fmt.Errorf("failed to update key sort order: %w", err)
		}
	}

	return tx.Commit()
}
