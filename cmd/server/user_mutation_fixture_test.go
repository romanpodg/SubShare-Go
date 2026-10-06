package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"modernc.org/sqlite"
)

type userMutationFixture struct {
	app           *App
	handler       http.Handler
	session, csrf string
}

func newUserMutationFixture(t *testing.T) userMutationFixture {
	t.Helper()
	return userMutationFixtureForApp(t, newIntegrationApp(t))
}

func userMutationFixtureForApp(t *testing.T, app *App) userMutationFixture {
	t.Helper()
	session, csrf, _ := seedIntegrationSession(t, app, "owner")
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	return userMutationFixture{app, mux, session, csrf}
}

func (f userMutationFixture) request(method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: f.session})
	request.Header.Set("X-CSRF-Token", f.csrf)
	request = request.WithContext(context.WithValue(request.Context(), middleware.CtxKeyRequestID, "mutation-fixture"))
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)
	return recorder
}

func mutationJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	requireRepositorySuccess(t, err)
	return string(data)
}

func mutationCount(t *testing.T, app *App, query string, args ...any) int {
	t.Helper()
	var count int
	requireRepositorySuccess(t, app.db.QueryRow(query, args...).Scan(&count))
	return count
}

type mutationResponseWant struct {
	status  int
	message string
}

func assertMutationResponse(t *testing.T, recorder *httptest.ResponseRecorder, path string, want mutationResponseWant) {
	t.Helper()
	requireRepositoryEqual(t, "mutation HTTP status", recorder.Code, want.status)
	payload := decodeJSONMap(t, recorder)
	if want.status == http.StatusOK {
		requireRepositoryEqual(t, "mutation success message", payload["message"], want.message)
		requireRepositoryEqual(t, "mutation success envelope", len(payload), 1)
		return
	}
	requireRepositoryEqual(t, "mutation error message", payload["error"], want.message)
	if strings.HasPrefix(path, "/api/v1/") {
		requireRepositoryEqual(t, "mutation v1 message", payload["message"], want.message)
		requireRepositoryEqual(t, "mutation request ID", payload["request_id"], "mutation-fixture")
		requireRepositoryEqual(t, "mutation v1 error code present", payload["code"] != nil, true)
		requireRepositoryEqual(t, "mutation field errors", payload["field_errors"], map[string]any{})
		return
	}
	requireRepositoryEqual(t, "legacy mutation error envelope", len(payload), 1)
}

// This driver wraps real disposable SQLite connections. Faults are configured
// before requests start. It preserves real SQL/transactions and adds controlled
// begin/commit failures or an execution barrier for concurrent PATCH tests.
type mutationSQLFaults struct {
	beginErr, commitErr  error
	patchRowsAffectedErr error
	beforeExec           func(string)
}

type mutationSQLDriver struct{ faults *mutationSQLFaults }
type mutationSQLConn struct {
	driver.Conn
	faults *mutationSQLFaults
}
type mutationSQLTx struct {
	driver.Tx
	faults *mutationSQLFaults
}

var mutationDriverSequence atomic.Uint64

func (d mutationSQLDriver) Open(name string) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &mutationSQLConn{conn, d.faults}, nil
}

func (c *mutationSQLConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *mutationSQLConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.faults.beginErr != nil {
		return nil, c.faults.beginErr
	}
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &mutationSQLTx{tx, c.faults}, nil
}

func (c *mutationSQLConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.faults.beforeExec != nil {
		c.faults.beforeExec(query)
	}
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if c.faults.patchRowsAffectedErr != nil && isSubscriptionPatchWrite(query) {
		return mutationSQLResult{result, c.faults.patchRowsAffectedErr}, nil
	}
	return result, nil
}

type mutationSQLResult struct {
	driver.Result
	err error
}

func (result mutationSQLResult) RowsAffected() (int64, error) { return 0, result.err }

func (c *mutationSQLConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (tx *mutationSQLTx) Commit() error {
	if tx.faults.commitErr != nil {
		// Model a commit rejected with rollback, rather than leaving a fake
		// driver's transaction open after database/sql marks it done.
		_ = tx.Rollback()
		return tx.faults.commitErr
	}
	return tx.Tx.Commit()
}

func newFaultedMutationFixture(t *testing.T) (userMutationFixture, *mutationSQLFaults) {
	t.Helper()
	app := newIntegrationApp(t)
	var seq int
	var name, path string
	requireRepositorySuccess(t, app.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path))
	requireRepositorySuccess(t, app.db.Close())
	faults := &mutationSQLFaults{}
	driverName := "mutation-sqlite-" + strconv.FormatUint(mutationDriverSequence.Add(1), 10)
	sql.Register(driverName, mutationSQLDriver{faults})
	db, err := sql.Open(driverName, filepath.ToSlash(path)+"?_pragma=busy_timeout%3D5000&_pragma=foreign_keys%3DON")
	requireRepositorySuccess(t, err)
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = db.Close() })
	app.db = db
	return userMutationFixtureForApp(t, app), faults
}
