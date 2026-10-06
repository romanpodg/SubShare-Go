package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"modernc.org/sqlite"
)

const contractProfileURI = "hysteria2://contract-auth@edge.example:443?sni=edge.example&x-extra=private-value#Embedded"

type profileContractFixture struct {
	db      *sql.DB
	repo    *Repository
	keyring *profilestorage.Keyring
	key     *model.VLESSKey
}

func newProfileContractFixture(t *testing.T) profileContractFixture {
	t.Helper()
	db := setupTestDB(t)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	profileContractExec(t, db, `PRAGMA foreign_keys=ON`)
	var foreignKeys int
	profileContractSuccess(t, db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys))
	profileContractEqual(t, "fixture foreign keys enabled", foreignKeys, 1)
	keyring := newTestKeyringForRepo(t)
	repo := NewRepository(db, keyring)
	profileContractExec(t, db, `INSERT INTO users(id, name, token, key_assignment_mode) VALUES(1, 'all-one', 'r06-one', 'all'), (2, 'all-two', 'r06-two', 'all'), (3, 'selected', 'r06-three', 'selected')`)
	key, _, err := repo.CreateLocal(context.Background(), profileContractCreateParams())
	if err != nil {
		t.Fatal(err)
	}
	return profileContractFixture{db, repo, keyring, key}
}

func profileContractCreateParams() keymanagement.CreateProfileParams {
	return keymanagement.CreateProfileParams{Label: "Panel", Status: "active", Kind: "real", Protocol: "hysteria2", BuiltURI: contractProfileURI, Category: "Original"}
}

func (f profileContractFixture) updateParams() keymanagement.UpdateProfileParams {
	return keymanagement.UpdateProfileParams{ID: f.key.ID, ExpectedRevision: 1, Label: "Changed", Status: "non-active", Kind: "real", Protocol: "hysteria2", Category: "New category", NewURI: "hysteria2://replacement-auth@other.example:8443#New"}
}

func (f profileContractFixture) makeSourceOwned(t *testing.T) {
	t.Helper()
	profileContractExec(t, f.db, `INSERT INTO external_subscription_sources(id, name, source_url, key_category, key_category_id) VALUES(77, ' Provider ', 'https://provider.example/sub', 'Original', ?)`, f.key.CategoryID)
	profileContractExec(t, f.db, `UPDATE vless_keys SET external_source_id = 77, external_key_ref = 'source-ref', profile_fingerprint = 'source-fingerprint' WHERE id = ?`, f.key.ID)
}

func profileContractExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

// Snapshot full rows, including secrets, revision, timestamps, category/source
// references and assignments. Failure assertions never print these contents.
func profileContractSnapshot(t *testing.T, db *sql.DB) map[string][][]any {
	t.Helper()
	snapshot := make(map[string][][]any)
	for _, table := range []string{"vless_keys", "vless_key_secrets", "key_categories", "user_keys", "external_subscription_sources"} {
		rows, err := db.Query("SELECT * FROM " + table + " ORDER BY 1, 2")
		if err != nil {
			t.Fatal(err)
		}
		snapshot[table] = profileContractRows(t, rows)
	}
	return snapshot
}

func profileContractRows(t *testing.T, rows *sql.Rows) [][]any {
	t.Helper()
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	result := make([][]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertProfileContractSnapshot(t *testing.T, db *sql.DB, before map[string][][]any) {
	t.Helper()
	after := profileContractSnapshot(t, db)
	for table, rows := range before {
		if !reflect.DeepEqual(rows, after[table]) {
			t.Fatalf("failed command changed %s", table)
		}
	}
}

func profileContractEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from contract", label)
	}
}

func profileContractSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func profileContractError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error=%v want=%v", err, want)
	}
}

func profileContractNoResult(t *testing.T, key *model.VLESSKey, raw string) {
	t.Helper()
	profileContractEqual(t, "no returned key", key == nil, true)
	profileContractEqual(t, "no returned plaintext", raw, "")
}

func profileContractCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	profileContractSuccess(t, db.QueryRow(query, args...).Scan(&count))
	return count
}

type profileContractFaults struct {
	beginErr, commitErr, reloadErr error
	committed                      bool
}

type profileContractDriver struct{ faults *profileContractFaults }
type profileContractConn struct {
	driver.Conn
	faults *profileContractFaults
}
type profileContractTx struct {
	driver.Tx
	faults *profileContractFaults
}

var profileContractDriverSequence atomic.Uint64
var errProfileContractFault = errors.New("injected profile boundary failure")

func (d profileContractDriver) Open(name string) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &profileContractConn{conn, d.faults}, nil
}

func (c *profileContractConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *profileContractConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.faults.beginErr != nil {
		return nil, c.faults.beginErr
	}
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &profileContractTx{tx, c.faults}, nil
}

func (c *profileContractConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *profileContractConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.isFailedReload(query) {
		return nil, c.faults.reloadErr
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *profileContractConn) isFailedReload(query string) bool {
	if !c.faults.committed || c.faults.reloadErr == nil {
		return false
	}
	return strings.Contains(query, "COALESCE(es.name")
}

func (tx *profileContractTx) Commit() error {
	if tx.faults.commitErr != nil {
		// This fault represents a rejected commit with a known rollback. It does
		// not simulate SQLite committing successfully and losing acknowledgment.
		_ = tx.Rollback()
		return tx.faults.commitErr
	}
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	tx.faults.committed = true
	return nil
}

func (f *profileContractFixture) withFaultDriver(t *testing.T) *profileContractFaults {
	t.Helper()
	var seq int
	var name, path string
	if err := f.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	faults := &profileContractFaults{}
	driverName := fmt.Sprintf("profile-contract-%d", profileContractDriverSequence.Add(1))
	sql.Register(driverName, profileContractDriver{faults})
	db, err := sql.Open(driverName, filepath.ToSlash(path)+"?_pragma=foreign_keys%3DON&_pragma=busy_timeout%3D5000")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	f.db = db
	f.repo = NewRepository(db, f.keyring)
	return faults
}
