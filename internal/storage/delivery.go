package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

// DeliveryEntry is one key eligible for a subscription, with its secret already
// decrypted. Callers never see envelopes or the keyring.
type DeliveryEntry struct {
	ID                int64
	SourceID          sql.NullInt64
	Kind              string
	TemplateText      string
	Label             string
	ClientDisplayName string
	StoredProtocol    string
	Compatibility     string
	// Raw is the decrypted profile URI. It is empty for informational keys and
	// whenever SecretError is set.
	Raw string
	// SecretError is nil, keymanagement.ErrCredentialMissing, or a decryption
	// failure. Delivery decides how to report it.
	SecretError error
}

// ListDeliveryEntries returns every active key assigned to the subscription in
// delivery order: uncategorised keys first, then by category order, key order,
// source and id. Real keys with three or more consecutive health failures are
// excluded; informational keys always pass.
func (r *Repository) ListDeliveryEntries(ctx context.Context, subscriptionID string) ([]DeliveryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT k.id, k.external_source_id, s.encrypted_url, k.key_kind, COALESCE(k.template_text, ''),
		       COALESCE(k.label, ''), COALESCE(k.client_display_name, ''),
		       COALESCE(k.protocol, 'legacy'), COALESCE(k.profile_compatibility, 'legacy')
		FROM users u
		JOIN user_keys uk ON uk.user_id = u.id
		JOIN vless_keys k ON k.id = uk.key_id
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		LEFT JOIN key_categories kc ON kc.id = k.category_id
		WHERE u.subscription_id = ?
		  AND k.status = 'active'
		  AND (k.key_kind = 'informational' OR COALESCE(k.health_failure_count, 0) < 3)
		ORDER BY
		  CASE WHEN k.category_id IS NULL THEN 0 ELSE 1 END,
		  COALESCE(kc.sort_order, 2147483647),
		  k.sort_order,
		  COALESCE(k.external_source_id, 0),
		  k.id
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]DeliveryEntry, 0)
	for rows.Next() {
		var entry DeliveryEntry
		var encURL sql.NullString
		if err := rows.Scan(&entry.ID, &entry.SourceID, &encURL, &entry.Kind, &entry.TemplateText, &entry.Label, &entry.ClientDisplayName, &entry.StoredProtocol, &entry.Compatibility); err != nil {
			return nil, err
		}
		if entry.Kind != "informational" {
			if !encURL.Valid || encURL.String == "" {
				entry.SecretError = keymanagement.ErrCredentialMissing
			} else if raw, err := r.credentials.decrypt(encURL.String, entry.ID); err != nil {
				entry.SecretError = err
			} else {
				entry.Raw = raw
			}
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// VerifySecrets decrypts every stored secret and returns how many were checked.
// It is used to validate backups and fails on the first unreadable row.
func (r *Repository) VerifySecrets(ctx context.Context) (int, error) {
	if r.credentials == nil || r.credentials.keyring == nil {
		return 0, keymanagement.ErrEncryptionUnavailable
	}
	lastID := int64(0)
	scanned := 0
	for {
		rows, err := r.db.QueryContext(ctx, `SELECT vless_key_id, encrypted_url FROM vless_key_secrets WHERE vless_key_id > ? ORDER BY vless_key_id ASC LIMIT 1000`, lastID)
		if err != nil {
			return scanned, fmt.Errorf("query secrets: %w", err)
		}
		count := 0
		for rows.Next() {
			count++
			scanned++
			var id int64
			var env string
			if err := rows.Scan(&id, &env); err != nil {
				rows.Close()
				return scanned, fmt.Errorf("scan secret: %w", err)
			}
			lastID = id
			raw, err := r.credentials.decrypt(env, id)
			if err != nil {
				rows.Close()
				return scanned, fmt.Errorf("row %d decryption failure: %w", id, err)
			}
			if raw == "" {
				rows.Close()
				return scanned, fmt.Errorf("row %d decrypted to zero bytes", id)
			}
		}
		rows.Close()
		if count == 0 {
			return scanned, nil
		}
	}
}

// Execer is satisfied by *sql.DB and *sql.Tx so key statements can join a
// caller's transaction.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// SetSourceKeysStatus flips every key owned by the source to status.
func SetSourceKeysStatus(ctx context.Context, exec Execer, sourceID int64, status string) error {
	_, err := exec.ExecContext(ctx, `UPDATE vless_keys SET status = ? WHERE external_source_id = ?`, status, sourceID)
	return err
}

// CountSourceKeys returns how many keys the source owns.
func CountSourceKeys(ctx context.Context, exec Execer, sourceID int64) (int, error) {
	var count int
	err := exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&count)
	return count, err
}

// DeleteSourceKeys removes every key owned by the source and returns how many
// were removed.
func DeleteSourceKeys(ctx context.Context, exec Execer, sourceID int64) (int, error) {
	count, err := CountSourceKeys(ctx, exec, sourceID)
	if err != nil {
		return 0, err
	}
	if _, err := exec.ExecContext(ctx, `DELETE FROM vless_keys WHERE external_source_id = ?`, sourceID); err != nil {
		return 0, err
	}
	return count, nil
}

// AssignAllKeysToUser links every existing key to the user.
func AssignAllKeysToUser(ctx context.Context, exec Execer, userID int64) error {
	_, err := exec.ExecContext(ctx, `INSERT OR IGNORE INTO user_keys(user_id, key_id) SELECT ?, id FROM vless_keys`, userID)
	return err
}

// AssignKeyToUser links one key to the user; false means the key does not exist.
func AssignKeyToUser(ctx context.Context, exec Execer, userID, keyID int64) (bool, error) {
	result, err := exec.ExecContext(ctx, `INSERT OR IGNORE INTO user_keys(user_id, key_id) SELECT ?, id FROM vless_keys WHERE id = ?`, userID, keyID)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected > 0, nil
}

// KeyHealthCounts summarises check_status across all keys for the dashboard.
type KeyHealthCounts struct {
	Total, Up, Down, Unknown int
}

func LoadKeyHealthCounts(ctx context.Context, exec Execer) (KeyHealthCounts, error) {
	var c KeyHealthCounts
	err := exec.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN check_status = 'up' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN check_status = 'down' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN check_status IS NULL OR check_status = '' OR check_status = 'unknown' THEN 1 ELSE 0 END), 0)
		FROM vless_keys
	`).Scan(&c.Total, &c.Up, &c.Down, &c.Unknown)
	return c, err
}
