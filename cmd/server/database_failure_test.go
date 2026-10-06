package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/platform/configuration"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

// Only the parent test supplies these paths, all within its disposable directory.
func TestStartupDatabaseFixtureProcess(t *testing.T) {
	args := flag.Args()
	if len(args) != 3 {
		return
	}
	if args[0] == "r02-lock" {
		holdStartupDatabaseLock(t, args[1])
		return
	}
	if args[0] != "r02-crash-wal" {
		return
	}
	keyring, err := profilestorage.LoadKeyringFile(args[2])
	requireStartupSuccess(t, err)
	db := openStartupDatabase(t, configuration.Config{
		DBPath: args[1], SQLiteJournalMode: "WAL", ProfileKeyring: keyring,
	})
	db.SetMaxOpenConns(1)
	execStartupSQL(t, db, "PRAGMA synchronous = FULL; PRAGMA wal_autocheckpoint = 0")
	var busy, frames, checkpointed int
	scanStartupSQL(t, db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)"), &busy, &frames, &checkpointed)
	requireStartupEqual(t, "schema checkpoint completed", busy, 0)
	requireStartupEqual(t, "schema WAL truncated", frames, 0)
	execStartupSQL(t, db, `INSERT INTO admins(username, password_hash, role) VALUES('existing-admin', 'existing-hash', 'operator');
		INSERT INTO users(name, token, subscription_id) VALUES('existing-user', 'fixture-token', 'fixture-subscription')`)
	seedStartupEncryptedProfile(t, db, keyring)
	// Exit without closing the database: the committed accounts and encrypted
	// profile exist only in the WAL, while the schema is in the main file.
	os.Exit(23)
}

func crashedStartupSnapshot(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db")
	keyringBytes, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	requireStartupSuccess(t, err)
	requireStartupSuccess(t, os.WriteFile(path+".keyring.json", keyringBytes, 0o600))
	executable, err := os.Executable()
	requireStartupSuccess(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, executable, "-test.run=^TestStartupDatabaseFixtureProcess$", "--", "r02-crash-wal", path, path+".keyring.json")
	output, err := child.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
		t.Fatalf("startup fixture did not reach committed crash: %v\n%s", err, output)
	}
	snapshot := make(map[string][]byte)
	for _, suffix := range []string{"", "-wal", "-shm", ".keyring.json"} {
		snapshot[suffix], err = os.ReadFile(path + suffix)
		requireStartupSuccess(t, err)
	}
	requireStartupEqual(t, "committed WAL frames present", len(snapshot["-wal"]) > 32, true)
	return snapshot
}

func copyStartupSnapshot(t *testing.T, snapshot map[string][]byte) configuration.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "app.db")
	requireStartupSuccess(t, os.MkdirAll(filepath.Dir(path), 0o700))
	for suffix, data := range snapshot {
		requireStartupSuccess(t, os.WriteFile(path+suffix, data, 0o600))
	}
	keyring, err := profilestorage.LoadKeyringFile(path + ".keyring.json")
	requireStartupSuccess(t, err)
	return configuration.Config{DBPath: path, SQLiteJournalMode: "WAL", ProfileKeyring: keyring}
}

func assertStartupSnapshotRetained(t *testing.T, path string, snapshot map[string][]byte) {
	t.Helper()
	for suffix, original := range snapshot {
		retained, err := os.ReadFile(path + suffix)
		requireStartupSuccess(t, err)
		requireStartupEqual(t, "failed startup retained file: "+suffix, bytes.Equal(retained, original), true)
	}
}

func assertRecoveredStartupState(t *testing.T, config configuration.Config) {
	t.Helper()
	db := openStartupDatabase(t, config)
	_, err := buildApp(config, db)
	requireStartupSuccess(t, err)
	assertStartupAccountsAndSchema(t, db, "WAL")
	var profile startupProfile
	scanStartupSQL(t, db.QueryRow("SELECT vless_key_id, encrypted_url FROM vless_key_secrets"), &profile.id, &profile.envelope)
	assertStartupEncryptedProfile(t, db, config.ProfileKeyring, profile)
	requireStartupSuccess(t, db.Close())
}

func TestOpenDatabaseFailureRetainsCommittedWAL(t *testing.T) {
	snapshot := crashedStartupSnapshot(t)
	for _, test := range []struct {
		name string
		err  error
	}{
		{"disk IO", errors.New("disk I/O error")},
		{"SHM resize", errors.New("SQLite (4874)")},
		{"checkpoint", fmt.Errorf("checkpoint: %w", errors.New("disk I/O error"))},
		{"checkpoint and close", errors.Join(errors.New("checkpoint: disk I/O error"), errors.New("close failed"))},
		{"permission", os.ErrPermission},
		{"locked", errors.New("database is locked (5)")},
		{"corrupt", errors.New("file is not a database (26)")},
		{"migration", errors.New("migrate db: no such column: version")},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := copyStartupSnapshot(t, snapshot)
			calls := 0
			db, err := openDatabaseWithInitializer(config, func(path, mode string, keyring *profilestorage.Keyring) (*sql.DB, error) {
				calls++
				requireStartupEqual(t, "initializer database path", path, config.DBPath)
				requireStartupEqual(t, "initializer journal mode", mode, config.SQLiteJournalMode)
				requireStartupEqual(t, "initializer keyring", keyring, config.ProfileKeyring)
				return nil, test.err
			})
			requireStartupEqual(t, "failed startup database", db, (*sql.DB)(nil))
			requireStartupEqual(t, "initialization error retained", errors.Is(err, test.err), true)
			requireStartupEqual(t, "failed startup attempted once", calls, 1)
			assertStartupSnapshotRetained(t, config.DBPath, snapshot)
			// An explicit later startup uses SQLite recovery, retaining accounts,
			// migration state and decryptable credential material.
			assertRecoveredStartupState(t, config)
		})
	}
}

func TestOpenDatabaseDirectoryFailureDoesNotInitialize(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	requireStartupSuccess(t, os.WriteFile(blocker, []byte("keep"), 0o600))
	calls := 0
	db, err := openDatabaseWithInitializer(configuration.Config{DBPath: filepath.Join(blocker, "app.db")},
		func(string, string, *profilestorage.Keyring) (*sql.DB, error) {
			calls++
			return nil, errors.New("unexpected initializer")
		})
	requireStartupEqual(t, "directory failure database", db, (*sql.DB)(nil))
	requireStartupEqual(t, "directory failure returned", err != nil, true)
	requireStartupEqual(t, "directory failure precedes initialization", calls, 0)
}

func holdStartupDatabaseLock(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	requireStartupSuccess(t, err)
	db.SetMaxOpenConns(1)
	execStartupSQL(t, db, "PRAGMA busy_timeout = 0; BEGIN IMMEDIATE")
	_, err = fmt.Fprintln(os.Stdout, "locked")
	requireStartupSuccess(t, err)
	_, err = io.Copy(io.Discard, os.Stdin)
	requireStartupSuccess(t, err)
	execStartupSQL(t, db, "ROLLBACK")
	requireStartupSuccess(t, db.Close())
}

func startStartupLockHolder(t *testing.T, config configuration.Config) func() {
	t.Helper()
	executable, err := os.Executable()
	requireStartupSuccess(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	child := exec.CommandContext(ctx, executable, "-test.run=^TestStartupDatabaseFixtureProcess$", "--", "r02-lock", config.DBPath, config.DBPath+".keyring.json")
	input, err := child.StdinPipe()
	requireStartupSuccess(t, err)
	output, err := child.StdoutPipe()
	requireStartupSuccess(t, err)
	requireStartupSuccess(t, child.Start())
	released := false
	t.Cleanup(func() {
		if !released {
			cancel()
			_ = input.Close()
			_ = child.Wait()
		}
	})
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(output).ReadString('\n')
		if err == nil && strings.TrimSpace(line) != "locked" {
			err = errors.New("child did not report an acquired lock")
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		requireStartupSuccess(t, err)
	case <-ctx.Done():
		t.Fatal("database lock holder timed out")
	}
	return func() {
		requireStartupSuccess(t, input.Close())
		err := child.Wait()
		released = true
		requireStartupSuccess(t, err)
	}
}

func TestOpenDatabaseLockedByAnotherProcessRetainsState(t *testing.T) {
	config := copyStartupSnapshot(t, crashedStartupSnapshot(t))
	db := openStartupDatabase(t, config)
	// Force a real migration write during the attempted startup. Removing the
	// baseline marker is safe in this disposable fixture: startup records it
	// again without rebuilding the already-versioned schema.
	execStartupSQL(t, db, "DELETE FROM schema_migrations WHERE version = 0")
	requireStartupSuccess(t, db.Close())
	release := startStartupLockHolder(t, config)
	// SQLite may update reader bookkeeping in SHM while the writer is locked.
	// The database, committed WAL and keyring must retain their bytes.
	snapshot := make(map[string][]byte)
	for _, suffix := range []string{"", "-wal", ".keyring.json"} {
		data, err := os.ReadFile(config.DBPath + suffix)
		requireStartupSuccess(t, err)
		snapshot[suffix] = data
	}
	failed, err := openDatabase(config)
	requireStartupEqual(t, "locked startup database", failed, (*sql.DB)(nil))
	requireStartupEqual(t, "lock failure returned", err != nil, true)
	requireStartupEqual(t, "lock diagnostic retained", strings.Contains(err.Error(), "locked"), true)
	assertStartupSnapshotRetained(t, config.DBPath, snapshot)
	release()
	assertRecoveredStartupState(t, config)
}
