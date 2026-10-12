package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

const backupAssuranceURI = "trojan://restore-fixture@restore.example:443#Restore"

type backupAssuranceFixture struct {
	app                     *App
	root, path, keyringPath string
	keyID                   int64
}

func backupAssuranceSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func backupAssuranceFailure(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected backup failure")
	}
	return err.Error()
}

func backupAssuranceEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from the backup contract", label)
	}
}

func backupAssuranceError(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func newBackupAssuranceFixture(t *testing.T) backupAssuranceFixture {
	t.Helper()
	root := t.TempDir()
	keyringPath := filepath.Join(root, "keyring.json")
	data, err := profilestorage.GenerateKeyringJSON("backup-key", "backup-index")
	backupAssuranceSuccess(t, err)
	backupAssuranceSuccess(t, os.WriteFile(keyringPath, data, 0600))
	keyring, err := profilestorage.LoadKeyringJSON(data)
	backupAssuranceSuccess(t, err)
	dbPath := filepath.Join(root, "live.db")
	db, err := storage.InitializeSQLiteWithJournalMode(dbPath, "DELETE", keyring)
	backupAssuranceSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	f := backupAssuranceFixture{app: &App{db: db, dbPath: dbPath, profileKeyring: keyring}, root: root, path: filepath.Join(root, "archive", "backup.db"), keyringPath: keyringPath}
	backupAssuranceExec(t, db, `INSERT INTO users(id, name, token, key_assignment_mode) VALUES(41, 'Backup user', 'backup-fixture-user', 'all')`)
	f.keyID = backupAssuranceCreateKey(t, db, keyring, backupAssuranceURI)
	return f
}

func backupAssuranceCreateKey(t *testing.T, db *sql.DB, keyring *profilestorage.Keyring, raw string) int64 {
	t.Helper()
	key, _, err := storage.NewRepository(db, keyring).CreateLocal(context.Background(), keymanagement.CreateProfileParams{Label: "Restore", Status: "active", Kind: "real", Protocol: "trojan", BuiltURI: raw, Category: "Restore category"})
	backupAssuranceSuccess(t, err)
	return key.ID
}

func backupAssuranceExec(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	_, err := db.Exec(statement, args...)
	backupAssuranceSuccess(t, err)
}

func (f backupAssuranceFixture) backup(t *testing.T) {
	t.Helper()
	backupAssuranceSuccess(t, f.app.performBackup(f.path))
}

func backupAssuranceBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	backupAssuranceSuccess(t, err)
	return data
}

func backupAssuranceUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()
	backupAssuranceEqual(t, "previous backup bytes", bytes.Equal(backupAssuranceBytes(t, path), before), true)
}

func backupAssuranceMissing(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	backupAssuranceEqual(t, "temporary file removed", os.IsNotExist(err), true)
}

func backupAssuranceOpen(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	backupAssuranceSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func backupAssuranceCount(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var got int64
	backupAssuranceSuccess(t, db.QueryRow(query, args...).Scan(&got))
	return got
}

func (f backupAssuranceFixture) exportedKeyring(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data := backupAssuranceBytes(t, f.keyringPath)
	keyring, err := profilestorage.LoadKeyringJSON(data)
	backupAssuranceSuccess(t, err)
	return keyring
}

func backupAssuranceRaw(t *testing.T, db *sql.DB, keyring *profilestorage.Keyring, id int64) string {
	t.Helper()
	var envelope string
	backupAssuranceSuccess(t, db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id=?`, id).Scan(&envelope))
	secret, err := profilestorage.Decrypt(envelope, keyring, id)
	backupAssuranceSuccess(t, err)
	return secret.Reveal()
}
