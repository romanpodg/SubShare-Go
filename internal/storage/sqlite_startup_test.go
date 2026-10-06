package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This child process deliberately bypasses SQLite's close/checkpoint. It only
// runs when the parent supplies a disposable fixture path after the test flags.
func TestSQLiteCrashWALFixtureProcess(t *testing.T) {
	args := flag.Args()
	if len(args) != 2 || args[0] != "r01-crash-wal" {
		return
	}
	db, err := sql.Open("sqlite", filepath.ToSlash(args[1]))
	requireStartupSuccess(t, err)
	db.SetMaxOpenConns(1)
	var mode string
	scanStartupSQL(t, db.QueryRow("PRAGMA journal_mode = WAL"), &mode)
	requireStartupEqual(t, "WAL enabled", mode, "wal")
	for _, statement := range []string{
		"PRAGMA synchronous = FULL",
		"PRAGMA wal_autocheckpoint = 0",
		"CREATE TABLE committed_rows (id INTEGER PRIMARY KEY, value TEXT NOT NULL)",
	} {
		execStartupSQL(t, db, statement)
	}
	var busy, frames, checkpointed int
	scanStartupSQL(t, db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)"), &busy, &frames, &checkpointed)
	requireStartupEqual(t, "schema checkpoint not busy", busy, 0)
	requireStartupEqual(t, "schema WAL truncated", frames, 0)
	execStartupSQL(t, db, "BEGIN; INSERT INTO committed_rows VALUES (7, 'committed before crash'); COMMIT;")
	// A distinct exit status proves the fixture reached the committed write.
	os.Exit(23)
}

func crashWALSnapshot(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "crashed.db")
	executable, err := os.Executable()
	requireStartupSuccess(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, executable, "-test.run=^TestSQLiteCrashWALFixtureProcess$", "--", "r01-crash-wal", fixture)
	output, err := child.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("crash fixture timed out: %v\n%s", ctx.Err(), output)
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("crash fixture did not exit with a status: %v\n%s", err, output)
	}
	if exitError.ExitCode() != 23 {
		t.Fatalf("crash fixture did not commit: %v\n%s", err, output)
	}
	snapshot := make(map[string][]byte)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		snapshot[suffix] = readStartupFixture(t, fixture+suffix)
	}
	requireStartupEqual(t, "crash left WAL frames", len(snapshot["-wal"]) > 32, true)
	requireStartupEqual(t, "crash left shared memory", len(snapshot["-shm"]) > 0, true)
	return fixture, snapshot
}

func copyStartupDatabase(t *testing.T, snapshot map[string][]byte, sidecars bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "copy.db")
	for suffix, data := range snapshot {
		if suffix == "" || sidecars {
			writeStartupFixture(t, path+suffix, data)
		}
	}
	return path
}

func TestCleanupSQLiteSidecarsCharacterizesCommittedWALLoss(t *testing.T) {
	fixture, snapshot := crashWALSnapshot(t)
	// The checkpointed main file has the schema, but not the committed row.
	assertStartupRowCount(t, copyStartupDatabase(t, snapshot, false), 0)
	preserved := copyStartupDatabase(t, snapshot, true)
	db, err := InitializeSQLiteWithJournalMode(preserved, "WAL", newTestKeyring(t))
	requireStartupSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var value string
	scanStartupSQL(t, db.QueryRow("SELECT value FROM committed_rows WHERE id = 7"), &value)
	requireStartupEqual(t, "committed WAL row recovered by initializer", value, "committed before crash")
	cleaned := copyStartupDatabase(t, snapshot, true)
	requireStartupSuccess(t, CleanupSQLiteSidecars(cleaned))
	requireStartupEqual(t, "cleanup left main database unchanged", bytes.Equal(readStartupFixture(t, cleaned), snapshot[""]), true)
	// R01 records the defect, not a safety requirement. R02 must replace this
	// destructive observation with a preservation assertion for its new policy.
	assertStartupRowCount(t, cleaned, 0)
	for suffix, original := range snapshot {
		requireStartupEqual(t, "reference crash fixture unchanged: "+suffix, bytes.Equal(readStartupFixture(t, fixture+suffix), original), true)
	}
}

func assertStartupRowCount(t *testing.T, path string, want int) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	requireStartupSuccess(t, err)
	defer db.Close()
	var count int
	scanStartupSQL(t, db.QueryRow("SELECT COUNT(*) FROM committed_rows"), &count)
	requireStartupEqual(t, "committed row count", count, want)
}

func TestCleanupSQLiteSidecarsMissingAndJunkFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	for file, data := range map[string]string{path: "database sentinel", path + ".keyring.json": "keyring sentinel"} {
		writeStartupFixture(t, file, []byte(data))
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		writeStartupFixture(t, path+suffix, []byte("junk sidecar"))
	}
	for attempt := 0; attempt < 2; attempt++ {
		requireStartupSuccess(t, CleanupSQLiteSidecars(path))
		for _, suffix := range []string{"-wal", "-shm"} {
			_, err := os.Stat(path + suffix)
			requireStartupEqual(t, "sidecar absent after cleanup: "+suffix, os.IsNotExist(err), true)
		}
	}
	requireStartupEqual(t, "main file retained", string(readStartupFixture(t, path)), "database sentinel")
	requireStartupEqual(t, "keyring file retained", string(readStartupFixture(t, path+".keyring.json")), "keyring sentinel")
}

func TestCleanupSQLiteSidecarsStopsAtFilesystemFailure(t *testing.T) {
	for _, test := range []struct {
		blocked, other string
		otherSurvives  bool
	}{
		{"-shm", "-wal", true},
		{"-wal", "-shm", false},
	} {
		t.Run(test.blocked, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "app.db")
			writeStartupFixture(t, path, []byte("database sentinel"))
			// Nonempty directories fail removal on Windows and Unix without
			// depending on the runner's permission model or administrator rights.
			requireStartupSuccess(t, os.Mkdir(path+test.blocked, 0o700))
			writeStartupFixture(t, filepath.Join(path+test.blocked, "keep"), []byte("obstacle"))
			writeStartupFixture(t, path+test.other, []byte("sidecar sentinel"))
			err := CleanupSQLiteSidecars(path)
			var pathError *os.PathError
			requireStartupEqual(t, "filesystem error returned", errors.As(err, &pathError), true)
			requireStartupEqual(t, "failure identifies blocked sidecar", pathError.Path, filepath.ToSlash(path)+test.blocked)
			if test.otherSurvives {
				requireStartupEqual(t, "WAL retained after SHM failure", string(readStartupFixture(t, path+test.other)), "sidecar sentinel")
			} else {
				_, err := os.Stat(path + test.other)
				requireStartupEqual(t, "SHM removed before WAL failure", os.IsNotExist(err), true)
			}
			requireStartupEqual(t, "database retained after failure", string(readStartupFixture(t, path)), "database sentinel")
		})
	}
}

func TestIsRecoverableSQLiteIOCharacterization(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"wrapped mixed case IO", fmt.Errorf("initialize: %w", errors.New("DISK I/O ERROR")), true},
		{"wrapped SHMSIZE code", fmt.Errorf("migrate: %w", errors.New("SQLite (4874)")), true},
		{"unrelated code", errors.New("SQLite (48740)"), false},
		{"locked", errors.New("database is locked (5)"), false},
		{"permission", os.ErrPermission, false},
		{"corrupt", errors.New("file is not a database (26)"), false},
		{"migration", errors.New("no such column: version"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireStartupEqual(t, "I/O classification", IsRecoverableSQLiteIO(test.err), test.want)
		})
	}
}

func execStartupSQL(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	_, err := db.Exec(query)
	requireStartupSuccess(t, err)
}

func scanStartupSQL(t *testing.T, row *sql.Row, destinations ...any) {
	t.Helper()
	requireStartupSuccess(t, row.Scan(destinations...))
}

func writeStartupFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	requireStartupSuccess(t, os.WriteFile(path, data, 0o600))
}

func readStartupFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	requireStartupSuccess(t, err)
	return data
}

func requireStartupSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func requireStartupEqual[T comparable](t *testing.T, invariant string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("startup invariant failed: %s", invariant)
	}
}
