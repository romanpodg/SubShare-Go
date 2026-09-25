package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (r *Repository) GetLegacyByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	key, rawURL, err := loadKeyByID(ctx, r.db, r.credentials, id)
	if errors.Is(err, errCredentialMissing) {
		return nil, "", fmt.Errorf("%w: %v", keymanagement.ErrCredentialMissing, err)
	}
	if errors.Is(err, keymanagement.ErrKeyNotFound) || errors.Is(err, keymanagement.ErrEncryptionUnavailable) || errors.Is(err, keymanagement.ErrStorageIntegrity) {
		return nil, "", err
	}
	if err != nil {
		return nil, "", mapKeyCredentialError(err)
	}
	return key, rawURL, nil
}

func (r *Repository) EnsureKeyCategory(ctx context.Context, name string) (int64, error) {
	return r.categories.ensure(ctx, name, "#d8b33d")
}

func (r *Repository) BulkUpdateKeys(ctx context.Context, params keymanagement.BulkKeyUpdate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	query := `UPDATE vless_keys SET status = ? WHERE id = ?`
	if params.ApplyCategory {
		query = `UPDATE vless_keys SET status = ?, category_id = ?, category = ? WHERE id = ?`
	}
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare bulk key update: %w", err)
	}
	defer stmt.Close()

	for _, id := range params.IDs {
		var result sql.Result
		if params.ApplyCategory {
			result, err = stmt.ExecContext(ctx, params.Status, params.CategoryID, params.Category, id)
		} else {
			result, err = stmt.ExecContext(ctx, params.Status, id)
		}
		if err != nil {
			return fmt.Errorf("failed to update key %d: %w", id, err)
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("failed to count updated key %d: %w", id, rowsErr)
		}
		if affected == 0 {
			return keymanagement.KeyNotFoundError{ID: id}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit bulk key update: %w", err)
	}
	return nil
}

func (r *Repository) BulkDeleteKeys(ctx context.Context, ids []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM vless_keys WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare bulk key delete: %w", err)
	}
	defer stmt.Close()
	for _, id := range ids {
		result, execErr := stmt.ExecContext(ctx, id)
		if execErr != nil {
			return fmt.Errorf("failed to delete key %d: %w", id, execErr)
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("failed to count deleted key %d: %w", id, rowsErr)
		}
		if affected == 0 {
			return keymanagement.KeyNotFoundError{ID: id}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit bulk key delete: %w", err)
	}
	return nil
}

func (r *Repository) GetHealthCheckTarget(ctx context.Context, id int64) (keymanagement.HealthCheckTarget, error) {
	var envelope sql.NullString
	var kind string
	err := r.db.QueryRowContext(ctx, `
		SELECT s.encrypted_url, k.key_kind
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.id = ?
	`, id).Scan(&envelope, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return keymanagement.HealthCheckTarget{}, keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return keymanagement.HealthCheckTarget{}, fmt.Errorf("failed to load key health target: %w", err)
	}
	if normalized, _ := model.NormalizeKeyKind(kind); normalized == model.KeyKindInformational {
		return keymanagement.HealthCheckTarget{ID: id}, nil
	}
	rawURL, err := r.credentials.decrypt(envelope.String, id)
	if err != nil {
		return keymanagement.HealthCheckTarget{}, mapKeyCredentialError(err)
	}
	return keymanagement.HealthCheckTarget{ID: id, URL: rawURL}, nil
}

func (r *Repository) ListHealthCheckTargets(ctx context.Context) ([]keymanagement.HealthCheckTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT k.id, s.encrypted_url, k.key_kind, k.status
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		ORDER BY CASE WHEN k.key_kind = 'real' THEN 0 ELSE 1 END, k.sort_order, k.id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load key health targets: %w", err)
	}
	defer rows.Close()

	targets := make([]keymanagement.HealthCheckTarget, 0)
	for rows.Next() {
		var id int64
		var envelope sql.NullString
		var kind, status string
		if err := rows.Scan(&id, &envelope, &kind, &status); err != nil {
			return nil, fmt.Errorf("failed to scan key health target: %w", err)
		}
		target := keymanagement.HealthCheckTarget{ID: id, Status: status, Kind: kind}
		normalizedStatus, _ := model.NormalizeKeyStatus(status)
		if normalized, _ := model.NormalizeKeyKind(kind); normalized == model.KeyKindInformational || normalizedStatus != model.KeyStatusActive {
			targets = append(targets, target)
			continue
		}
		rawURL, decryptErr := r.credentials.decrypt(envelope.String, id)
		if decryptErr != nil {
			target.Unreadable = true
			targets = append(targets, target)
			continue
		}
		target.URL = rawURL
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate key health targets: %w", err)
	}
	return targets, nil
}

func (r *Repository) SaveHealthCheckResult(ctx context.Context, params keymanagement.SaveHealthCheckResultParams) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE vless_keys
		SET check_status = ?, check_error = ?, last_latency_ms = ?, last_checked_at = CURRENT_TIMESTAMP,
		    health_failure_count = CASE
		      WHEN ? = 'up' THEN 0
		      WHEN ? = 'down' THEN COALESCE(health_failure_count, 0) + 1
		      ELSE COALESCE(health_failure_count, 0)
		    END
		WHERE id = ?
	`, params.Status, nullStringValue(params.Error), nullInt64Value(params.Latency), params.Status, params.Status, params.ID)
	if err != nil {
		return fmt.Errorf("failed to save key health result: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count saved key health result: %w", err)
	}
	if affected == 0 {
		return keymanagement.ErrKeyNotFound
	}
	return nil
}

func (r *Repository) GetHealthCheckResult(ctx context.Context, id int64) (keymanagement.HealthCheckResult, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, check_status, check_error, last_checked_at, last_latency_ms
		FROM vless_keys WHERE id = ?
	`, id)
	result, err := scanHealthCheckResult(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return keymanagement.HealthCheckResult{}, keymanagement.ErrKeyNotFound
	}
	if err != nil {
		return keymanagement.HealthCheckResult{}, fmt.Errorf("failed to load key health result: %w", err)
	}
	return result, nil
}

func (r *Repository) ListHealthCheckResults(ctx context.Context) ([]keymanagement.HealthCheckResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, check_status, check_error, last_checked_at, last_latency_ms
		FROM vless_keys
		ORDER BY CASE WHEN key_kind = 'real' THEN 0 ELSE 1 END, sort_order, id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load key health results: %w", err)
	}
	defer rows.Close()
	results := make([]keymanagement.HealthCheckResult, 0)
	for rows.Next() {
		result, scanErr := scanHealthCheckResult(rows.Scan)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan key health result: %w", scanErr)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate key health results: %w", err)
	}
	return results, nil
}

type scanFunc func(dest ...any) error

func scanHealthCheckResult(scan scanFunc) (keymanagement.HealthCheckResult, error) {
	var result keymanagement.HealthCheckResult
	var status, detail sql.NullString
	var checkedAt sql.NullTime
	var latency sql.NullInt64
	if err := scan(&result.ID, &status, &detail, &checkedAt, &latency); err != nil {
		return result, err
	}
	result.Status = model.NormalizeCheckStatus(status.String)
	result.Error = strings.TrimSpace(detail.String)
	if checkedAt.Valid {
		value := checkedAt.Time
		result.LastCheckedAt = &value
	}
	if latency.Valid {
		result.Latency = latency.Int64
	}
	return result, nil
}

func mapKeyCredentialError(err error) error {
	if errors.Is(err, errCredentialMissing) {
		return fmt.Errorf("%w: %v", keymanagement.ErrCredentialMissing, err)
	}
	if credentialKeyUnavailable(err) {
		return fmt.Errorf("%w: %v", keymanagement.ErrEncryptionUnavailable, err)
	}
	return fmt.Errorf("%w: %v", keymanagement.ErrStorageIntegrity, err)
}
