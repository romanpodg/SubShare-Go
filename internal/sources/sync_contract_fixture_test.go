package sources_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

const syncOne = "trojan://fixture-one@one.example:443#One"
const syncTwo = "trojan://fixture-two@two.example:443#Two"

var syncFingerprintKeys = [][]byte{[]byte("0123456789abcdef0123456789abcdef")}

type syncContractFixture struct {
	db      *sql.DB
	repo    *storage.Repository
	keyring *profilestorage.Keyring
}

func sourceSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func sourceFailure(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected source reconciliation rejection")
	}
	return err.Error()
}

func sourceEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from the frozen contract", label)
	}
}

func newSyncContractFixture(t *testing.T) syncContractFixture {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("fixture-key", "fixture-index")
	sourceSuccess(t, err)
	keyring, err := profilestorage.LoadKeyringJSON(data)
	sourceSuccess(t, err)
	db, err := storage.InitializeSQLiteWithJournalMode(filepath.Join(t.TempDir(), "sync.db"), "DELETE", keyring)
	sourceSuccess(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	f := syncContractFixture{db: db, repo: storage.NewRepository(db, keyring), keyring: keyring}
	f.exec(t, `INSERT INTO external_subscription_sources(id, name, source_url) VALUES(1, 'One', 'https://one.example/feed'), (2, 'Two', 'https://two.example/feed')`)
	f.exec(t, `INSERT INTO users(id, name, token, key_assignment_mode) VALUES(1, 'All', 'fixture-all', 'all'), (2, 'Selected', 'fixture-selected', 'selected')`)
	return f
}

func (f syncContractFixture) exec(t *testing.T, statement string, args ...any) {
	t.Helper()
	_, err := f.db.Exec(statement, args...)
	sourceSuccess(t, err)
}

func syncParsed(t *testing.T, raw string) sources.ParseResult {
	t.Helper()
	parsed, err := sources.ParseBody(raw, syncFingerprintKeys)
	sourceSuccess(t, err)
	return parsed
}

func (f syncContractFixture) apply(t *testing.T, target sources.SyncTarget, parsed sources.ParseResult, keys [][]byte) (sources.SyncResult, error) {
	t.Helper()
	tx, err := f.repo.BeginSourceSync(context.Background())
	sourceSuccess(t, err)
	defer tx.Rollback()
	result, err := sources.Sync(tx, target, parsed, keys)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (f syncContractFixture) sync(t *testing.T, id int64, raw string) sources.SyncResult {
	t.Helper()
	result, err := f.apply(t, sources.SyncTarget{ID: id, Enabled: true}, syncParsed(t, raw), syncFingerprintKeys)
	sourceSuccess(t, err)
	return result
}

type syncContractRow struct {
	id                               int64
	ref, label, raw, status, display string
}

func (f syncContractFixture) row(t *testing.T, id int64, label string) syncContractRow {
	t.Helper()
	var row syncContractRow
	var envelope string
	err := f.db.QueryRow(`SELECT k.id, k.external_key_ref, k.label, s.encrypted_url, k.status, COALESCE(k.client_display_name, '') FROM vless_keys k JOIN vless_key_secrets s ON s.vless_key_id=k.id WHERE k.external_source_id=? AND k.label=?`, id, label).Scan(&row.id, &row.ref, &row.label, &envelope, &row.status, &row.display)
	sourceSuccess(t, err)
	secret, err := profilestorage.Decrypt(envelope, f.keyring, row.id)
	sourceSuccess(t, err)
	row.raw = secret.Reveal()
	return row
}

func (f syncContractFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var count int
	sourceSuccess(t, f.db.QueryRow(query, args...).Scan(&count))
	return count
}

// Preserve complete persisted rows, including randomized envelopes, on failure.
// Comparison failures identify the table without printing credential contents.
func (f syncContractFixture) snapshot(t *testing.T) map[string][][]any {
	t.Helper()
	result := make(map[string][][]any)
	for _, table := range []string{"vless_keys", "vless_key_secrets", "key_categories", "user_keys", "external_subscription_sources"} {
		result[table] = snapshotSyncTable(t, f.db, table)
	}
	return result
}

func snapshotSyncTable(t *testing.T, db *sql.DB, table string) [][]any {
	t.Helper()
	rows, err := db.Query("SELECT * FROM " + table + " ORDER BY rowid")
	sourceSuccess(t, err)
	defer rows.Close()
	columns, err := rows.Columns()
	sourceSuccess(t, err)
	var result [][]any
	for rows.Next() {
		values, destinations := make([]any, len(columns)), make([]any, len(columns))
		for i := range values {
			destinations[i] = &values[i]
		}
		sourceSuccess(t, rows.Scan(destinations...))
		result = append(result, values)
	}
	sourceSuccess(t, rows.Err())
	return result
}
