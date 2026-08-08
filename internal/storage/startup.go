package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func VerifyStartupEnvelopesAndInvariants(ctx context.Context, db *sql.DB, keyring *profilestorage.Keyring) error {
	var maxVersion int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxVersion); err != nil {
		return err
	}
	if maxVersion < 11 {
		return nil
	}

	if keyring == nil {
		return profilestorage.ErrMissingKeyring
	}

	lastID := int64(0)
	for {
		rows, err := db.QueryContext(ctx, `SELECT vless_key_id, encrypted_url FROM vless_key_secrets WHERE vless_key_id > ? ORDER BY vless_key_id ASC LIMIT 1000`, lastID)
		if err != nil {
			return fmt.Errorf("scan secrets: %w", err)
		}
		var count int
		for rows.Next() {
			count++
			var id int64
			var env string
			if err := rows.Scan(&id, &env); err != nil {
				rows.Close()
				return fmt.Errorf("scan secret row: %w", err)
			}
			lastID = id
			keyID, _, _, err := profilestorage.InspectEnvelope(env)
			if err != nil {
				rows.Close()
				return fmt.Errorf("row %d: %w", id, err)
			}
			if _, ok := keyring.GetEncryptionKey(keyID); !ok {
				rows.Close()
				return fmt.Errorf("row %d: %w: key %q", id, profilestorage.ErrUnknownKeyID, keyID)
			}
		}
		rows.Close()
		if count == 0 {
			break
		}
	}

	var missingSecrets int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE s.vless_key_id IS NULL`).Scan(&missingSecrets); err != nil {
		return err
	}
	if missingSecrets > 0 {
		return fmt.Errorf("%w: %d parents without secrets", profilestorage.ErrMigrationVerificationFailed, missingSecrets)
	}

	return nil
}
