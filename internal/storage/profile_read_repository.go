package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

// profileReadRow holds the nullable database projection before endpoint-specific
// mapping. Credential decryption stays in the existing credential store.
type profileReadRow struct {
	key                                                model.VLESSKey
	encryptedURL, category, kind, templateText, status sql.NullString
	checkStatus, checkError                            sql.NullString
	lastCheckedAt                                      sql.NullTime
	latency, categoryID, externalSourceID              sql.NullInt64
	externalSourceName, clientDisplayName              sql.NullString
	warningsJSON                                       string
	revision                                           sql.NullInt64
	updatedAt                                          sql.NullString
}

func (row *profileReadRow) scan(scan scanFunc, revision any) error {
	return scan(
		&row.key.ID, &row.key.Label, &row.clientDisplayName, &row.encryptedURL, &row.categoryID, &row.category,
		&row.kind, &row.templateText, &row.status, &row.checkStatus, &row.checkError, &row.lastCheckedAt,
		&row.latency, &row.key.CreatedAt, &row.externalSourceID, &row.externalSourceName, &row.key.Protocol,
		&row.key.ProfileSchemaVersion, &row.key.ProfileCompatibility, &row.warningsJSON, revision, &row.updatedAt,
	)
}

func (row *profileReadRow) baseKey() model.VLESSKey {
	key := row.key
	if err := json.Unmarshal([]byte(row.warningsJSON), &key.ProfileWarnings); err != nil || key.ProfileWarnings == nil {
		key.ProfileWarnings = []string{}
	}
	if row.categoryID.Valid {
		key.CategoryID = row.categoryID.Int64
	}
	key.Category = strings.TrimSpace(row.category.String)
	key.Kind, _ = model.NormalizeKeyKind(row.kind.String)
	if key.Kind == "" {
		key.Kind = model.KeyKindReal
	}
	key.TemplateText = strings.TrimSpace(row.templateText.String)
	key.Status, _ = model.NormalizeKeyStatus(row.status.String)
	if key.Status == "" {
		key.Status = model.KeyStatusActive
	}
	key.CheckStatus = model.NormalizeCheckStatus(row.checkStatus.String)
	if row.latency.Valid {
		key.LastLatencyMS = row.latency.Int64
	}
	key.UpdatedAt = parseUpdatedAt(row.updatedAt, key.CreatedAt)
	key.ExternalSourceName = strings.TrimSpace(row.externalSourceName.String)
	key.ClientDisplayNameOverridden = strings.TrimSpace(row.clientDisplayName.String) != ""
	return key
}

func (row *profileReadRow) detailKey(rawURI string) *model.VLESSKey {
	key := row.baseKey()
	key.CheckError = row.checkError.String
	if row.lastCheckedAt.Valid {
		key.LastCheckedAtText = row.lastCheckedAt.Time.Format("2006-01-02 15:04:05")
	}
	key.ProfileRevision = row.revision.Int64
	if key.ProfileRevision < 1 {
		key.ProfileRevision = 1
	}
	if row.externalSourceID.Valid {
		key.ExternalSourceID = row.externalSourceID.Int64
	}
	row.setDisplayName(&key, rawURI)
	return &key
}

func (row *profileReadRow) legacyKey(rawURI string) model.VLESSKey {
	key := row.baseKey()
	key.URL = rawURI
	key.StatusLabel = model.KeyStatusLabel(key.Status)
	key.CheckStatusLabel = model.CheckStatusLabel(key.CheckStatus)
	key.CheckError = strings.TrimSpace(row.checkError.String)
	if row.lastCheckedAt.Valid {
		key.LastCheckedAtText = row.lastCheckedAt.Time.Local().Format("2006-01-02 15:04:05")
	}
	if row.externalSourceID.Valid && row.externalSourceID.Int64 > 0 {
		key.ExternalSourceID = row.externalSourceID.Int64
	}
	row.setDisplayName(&key, rawURI)
	return key
}

func (row *profileReadRow) setDisplayName(key *model.VLESSKey, rawURI string) {
	key.ClientDisplayName = profileconfig.EffectiveClientDisplayName(
		row.clientDisplayName.String, rawURI, key.Label, key.ExternalSourceID > 0,
	)
}

// parseUpdatedAt reads the stored updated_at text, accepting the SQLite and
// RFC 3339 layouts, and falls back to created_at otherwise.
func parseUpdatedAt(updatedAt sql.NullString, fallback time.Time) time.Time {
	if !updatedAt.Valid || updatedAt.String == "" {
		return fallback
	}
	if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
		return t
	}
	return fallback
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	return loadKeyByID(ctx, r.db, r.credentials, id)
}

func loadKeyByID(ctx context.Context, db *sql.DB, credentials *credentialStore, id int64) (*model.VLESSKey, string, error) {
	var row profileReadRow
	result := db.QueryRowContext(ctx, `
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
 `, id)
	err := row.scan(result.Scan, &row.revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return nil, "", err
	}

	decryptedURI, err := credentials.decrypt(row.encryptedURL.String, row.key.ID)
	if err != nil {
		if credentialKeyUnavailable(err) {
			return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
		}
		return nil, "", fmt.Errorf("%w: %w", keymanagement.ErrStorageIntegrity, err)
	}
	return row.detailKey(decryptedURI), decryptedURI, nil
}

func (r *Repository) ListLegacy(ctx context.Context) ([]model.VLESSKey, error) {
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
		var row profileReadRow
		if err := row.scan(rows.Scan, &row.key.ProfileRevision); err != nil {
			return nil, err
		}
		decryptedURL, decryptErr := r.credentials.decrypt(row.encryptedURL.String, row.key.ID)
		if decryptErr != nil {
			return nil, mapKeyCredentialError(decryptErr)
		}
		out = append(out, row.legacyKey(decryptedURL))
	}
	return out, rows.Err()
}

func (r *Repository) GetLegacyByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	key, rawURL, err := loadKeyByID(ctx, r.db, r.credentials, id)
	if errors.Is(err, errCredentialMissing) {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrCredentialMissing, err)
	}
	if isClassifiedProfileReadError(err) {
		return nil, "", err
	}
	if err != nil {
		return nil, "", mapKeyCredentialError(err)
	}
	return key, rawURL, nil
}

func isClassifiedProfileReadError(err error) bool {
	for _, classified := range []error{keymanagement.ErrKeyNotFound, keymanagement.ErrEncryptionUnavailable, keymanagement.ErrStorageIntegrity} {
		if errors.Is(err, classified) {
			return true
		}
	}
	return false
}
