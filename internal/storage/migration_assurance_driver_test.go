package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"modernc.org/sqlite"
)

var migrationAssuranceDriverSequence atomic.Uint64
var errMigrationAssurance = errors.New("injected migration interruption")

type migrationAssuranceFault struct {
	mu       sync.Mutex
	query    string
	at, seen int
	enabled  bool
}

func (f *migrationAssuranceFault) interrupt(query string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.enabled || !strings.Contains(query, f.query) {
		return nil
	}
	f.seen++
	if f.seen == f.at {
		return errMigrationAssurance
	}
	return nil
}

func (f *migrationAssuranceFault) disable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = false
}

type migrationAssuranceDriver struct{ fault *migrationAssuranceFault }
type migrationAssuranceConn struct {
	driver.Conn
	fault *migrationAssuranceFault
}

func (d migrationAssuranceDriver) Open(name string) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &migrationAssuranceConn{conn, d.fault}, nil
}

func (c *migrationAssuranceConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
}

func (c *migrationAssuranceConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.fault.interrupt(query); err != nil {
		return nil, err
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *migrationAssuranceConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (f *migrationAssuranceFixture) interrupt(t *testing.T, query string, at int) *migrationAssuranceFault {
	t.Helper()
	profileContractSuccess(t, f.db.Close())
	fault := &migrationAssuranceFault{query: query, at: at, enabled: true}
	name := fmt.Sprintf("migration-assurance-%d", migrationAssuranceDriverSequence.Add(1))
	sql.Register(name, migrationAssuranceDriver{fault})
	db, err := sql.Open(name, sqliteDataSourceName(f.path))
	profileContractSuccess(t, err)
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = db.Close() })
	f.db = db
	return fault
}
