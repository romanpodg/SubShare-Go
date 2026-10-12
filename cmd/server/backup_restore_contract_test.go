package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestBackupAssurancePopulatedPairRestoresIDsAssignmentsAndCredentials(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	keyring := f.exportedKeyring(t)
	before := backupAssuranceBytes(t, f.path)
	backupAssuranceSuccess(t, runValidateBackup(f.path, keyring))
	backupAssuranceUnchanged(t, f.path, before)
	backupAssuranceExec(t, f.app.db, `UPDATE users SET name='Live later' WHERE id=41`)
	backupAssuranceCreateKey(t, f.app.db, keyring, "trojan://later-fixture@later.example:443#Later")
	restoredPath := filepath.Join(f.root, "restored.db")
	backupAssuranceSuccess(t, os.WriteFile(restoredPath, before, 0600))
	restored, err := storage.InitializeSQLiteWithJournalMode(restoredPath, "DELETE", keyring)
	backupAssuranceSuccess(t, err)
	t.Cleanup(func() { _ = restored.Close() })
	backupAssuranceEqual(t, "credential and immutable row identity", backupAssuranceRaw(t, restored, keyring, f.keyID), backupAssuranceURI)
	backupAssuranceEqual(t, "snapshot user", backupAssuranceCount(t, restored, `SELECT COUNT(*) FROM users WHERE id=41 AND name='Backup user'`), int64(1))
	backupAssuranceEqual(t, "snapshot keys", backupAssuranceCount(t, restored, `SELECT COUNT(*) FROM vless_keys`), int64(1))
	backupAssuranceEqual(t, "assignment", backupAssuranceCount(t, restored, `SELECT COUNT(*) FROM user_keys WHERE user_id=41 AND key_id=?`, f.keyID), int64(1))
	backupAssuranceEqual(t, "foreign keys", backupAssuranceCount(t, restored, `PRAGMA foreign_keys`), int64(1))
	nextID := backupAssuranceCreateKey(t, restored, keyring, "trojan://next-fixture@next.example:443#Next")
	backupAssuranceEqual(t, "sequence retained", nextID > f.keyID, true)
}

func TestBackupAssuranceSameKeyIDWrongMaterialPassesShapeFailsAuthentication(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	data, err := profilestorage.GenerateKeyringJSON("backup-key", "backup-index")
	backupAssuranceSuccess(t, err)
	wrong, err := profilestorage.LoadKeyringJSON(data)
	backupAssuranceSuccess(t, err)
	db := backupAssuranceOpen(t, f.path)
	backupAssuranceSuccess(t, storage.VerifyStartupEnvelopesAndInvariants(context.Background(), db, wrong))
	backupAssuranceError(t, runValidateBackup(f.path, wrong), profilestorage.ErrAuthenticationFailed)
}

func TestBackupAssuranceRowBindingPassesShapeFailsAuthentication(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	keyring := f.exportedKeyring(t)
	activeID, activeKey, err := keyring.GetActiveEncryptionKey()
	backupAssuranceSuccess(t, err)
	envelope, err := profilestorage.Encrypt([]byte(backupAssuranceURI), activeID, activeKey, f.keyID+1)
	backupAssuranceSuccess(t, err)
	db := backupAssuranceOpen(t, f.path)
	backupAssuranceExec(t, db, `UPDATE vless_key_secrets SET encrypted_url=? WHERE vless_key_id=?`, envelope, f.keyID)
	backupAssuranceSuccess(t, storage.VerifyStartupEnvelopesAndInvariants(context.Background(), db, keyring))
	backupAssuranceError(t, runValidateBackup(f.path, keyring), profilestorage.ErrAuthenticationFailed)
}

func TestBackupAssuranceMissingKeyringIsRejected(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	backupAssuranceError(t, runValidateBackup(f.path, nil), profilestorage.ErrMissingKeyring)
}

func TestBackupAssuranceMissingSecretCannotBeCertifiedRestorable(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	db := backupAssuranceOpen(t, f.path)
	backupAssuranceExec(t, db, `DELETE FROM vless_key_secrets WHERE vless_key_id=?`, f.keyID)
	backupAssuranceError(t, storage.VerifyStartupEnvelopesAndInvariants(context.Background(), db, f.app.profileKeyring), profilestorage.ErrMigrationVerificationFailed)
	backupAssuranceError(t, runValidateBackup(f.path, f.app.profileKeyring), profilestorage.ErrMigrationVerificationFailed)
}

func TestBackupAssuranceVacuumFailureRetainsPreviousPublishedBackup(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	before := backupAssuranceBytes(t, f.path)
	backupAssuranceSuccess(t, f.app.db.Close())
	err := f.app.performBackup(f.path)
	backupAssuranceEqual(t, "vacuum failure surfaced", strings.Contains(backupAssuranceFailure(t, err), "create SQLite backup"), true)
	backupAssuranceUnchanged(t, f.path, before)
	backupAssuranceMissing(t, f.path+".tmp")
	backupAssuranceMissing(t, f.path+".previous")
}

func TestBackupAssurancePreserveFailureRetainsPreviousAndRemovesCandidate(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	f.backup(t)
	before := backupAssuranceBytes(t, f.path)
	backupAssuranceSuccess(t, os.Mkdir(f.path+".previous", 0700))
	backupAssuranceSuccess(t, os.WriteFile(filepath.Join(f.path+".previous", "blocker"), []byte("fixture"), 0600))
	err := f.app.performBackup(f.path)
	backupAssuranceEqual(t, "previous backup failure surfaced", strings.Contains(backupAssuranceFailure(t, err), "preserve previous backup"), true)
	backupAssuranceUnchanged(t, f.path, before)
	backupAssuranceMissing(t, f.path+".tmp")
}

func TestBackupAssuranceDirectoryFailureDoesNotChangeLiveData(t *testing.T) {
	f := newBackupAssuranceFixture(t)
	blocked := filepath.Join(f.root, "blocked")
	backupAssuranceSuccess(t, os.WriteFile(blocked, []byte("fixture"), 0600))
	err := f.app.performBackup(filepath.Join(blocked, "backup.db"))
	backupAssuranceEqual(t, "directory failure surfaced", strings.Contains(backupAssuranceFailure(t, err), "create backup directory"), true)
	backupAssuranceEqual(t, "live credential intact", backupAssuranceRaw(t, f.app.db, f.app.profileKeyring, f.keyID), backupAssuranceURI)
}

func TestBackupAssuranceCorruptedSQLiteFailsIntegrity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.db")
	backupAssuranceSuccess(t, os.WriteFile(path, []byte("invalid sqlite fixture"), 0600))
	backupAssuranceFailure(t, verifySQLiteBackup(path))
}
