package storage

import (
	"testing"
)

func TestPopulatedMigrationAssuranceAuthenticHistoricalUpgrade(t *testing.T) {
	f := newMigrationAssuranceFixture(t, 11, 3)
	profileContractSuccess(t, MigrateWithKeyring(f.db, f.keyring))
	f.assertUpgraded(t, 3)
	before := f.snapshot(t)
	profileContractSuccess(t, MigrateWithKeyring(f.db, f.keyring))
	profileContractEqual(t, "idempotent rows/envelopes/sequence/identities", f.snapshot(t), before)
}

func TestPopulatedMigrationAssuranceInterruptedEncryptionBatchResumes(t *testing.T) {
	for _, at := range []int{2, 101} {
		t.Run(migrationAssuranceURI(at), func(t *testing.T) {
			f := newMigrationAssuranceFixture(t, 11, 101)
			fault := f.interrupt(t, "INSERT INTO vless_key_secrets", at)
			profileContractError(t, MigrateWithKeyring(f.db, f.keyring), errMigrationAssurance)
			committed := 0
			if at > 100 {
				committed = 100
			}
			before := f.envelopes(t)
			profileContractEqual(t, "failed batch rolled back", len(before), committed)
			profileContractEqual(t, "encryption version not falsely published", profileContractCount(t, f.db, `SELECT COUNT(*) FROM schema_migrations WHERE version=12`), 0)
			profileContractEqual(t, "all plaintext parents retained", profileContractCount(t, f.db, `SELECT COUNT(*) FROM vless_keys WHERE url<>''`), 101)
			fault.disable()
			profileContractSuccess(t, MigrateWithKeyring(f.db, f.keyring))
			f.assertUpgraded(t, 101)
			after := f.envelopes(t)
			for id, envelope := range before {
				profileContractEqual(t, "committed envelope retained", after[id], envelope)
			}
		})
	}
}

func TestPopulatedMigrationAssuranceInterruptedRebuildRetainsRowsAndFKs(t *testing.T) {
	for _, query := range []string{"INSERT INTO vless_keys_new", "ALTER TABLE vless_keys_new RENAME TO vless_keys"} {
		t.Run(query, func(t *testing.T) {
			f := newMigrationAssuranceFixture(t, 11, 3)
			// Complete encryption independently, leaving the real populated
			// version-twelve schema for an isolated table-rebuild interruption.
			applyAssuranceVersionTwelve(t, f)
			before := f.snapshot(t)
			fault := f.interrupt(t, query, 1)
			profileContractError(t, MigrateWithKeyring(f.db, f.keyring), errMigrationAssurance)
			profileContractEqual(t, "whole rebuild transaction rollback", f.snapshot(t), before)
			profileContractEqual(t, "FK enforcement restored after interruption", profileContractCount(t, f.db, `PRAGMA foreign_keys`), 1)
			fault.disable()
			profileContractSuccess(t, MigrateWithKeyring(f.db, f.keyring))
			f.assertUpgraded(t, 3)
		})
	}
}

func applyAssuranceVersionTwelve(t *testing.T, f migrationAssuranceFixture) {
	t.Helper()
	conn, err := f.db.Conn(t.Context())
	profileContractSuccess(t, err)
	defer conn.Close()
	for _, migration := range SchemaMigrations {
		if migration.Version == 12 {
			applyAssuranceMigration(t, f, conn, migration)
			return
		}
	}
	t.Fatal("historical encryption migration missing")
}

func TestPopulatedMigrationAssuranceFailedFKRestoreDoesNotReturnDirtyConnection(t *testing.T) {
	f := newMigrationAssuranceFixture(t, 11, 3)
	applyAssuranceVersionTwelve(t, f)
	fault := f.interrupt(t, "PRAGMA foreign_keys = ON", 1)
	profileContractError(t, MigrateWithKeyring(f.db, f.keyring), errMigrationAssurance)
	profileContractEqual(t, "next connection retains FK enforcement", profileContractCount(t, f.db, `PRAGMA foreign_keys`), 1)
	fault.disable()
	profileContractSuccess(t, MigrateWithKeyring(f.db, f.keyring))
	f.assertUpgraded(t, 3)
}
