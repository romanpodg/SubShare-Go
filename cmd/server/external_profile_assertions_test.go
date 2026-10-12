package main

import (
	"database/sql"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func syncExternalFixtureBody(t *testing.T, app *App, sourceID int64, raw string) sources.SyncResult {
	t.Helper()
	result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, raw))
	if err != nil {
		t.Fatalf("sync external profile fixture: %v", err)
	}
	return result
}

func assertExternalFixtureMetadata(t *testing.T, app *App, keyID int64, wantLabel, wantStatus string, wantOverride sql.NullString) {
	t.Helper()
	var label, status string
	var override sql.NullString
	if err := app.db.QueryRow(`SELECT label, status, client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&label, &status, &override); err != nil {
		t.Fatalf("read external profile metadata: %v", err)
	}
	if label != wantLabel || status != wantStatus || override != wantOverride {
		t.Fatalf("external profile metadata label=%q status=%q override=%#v; want label=%q status=%q override=%#v", label, status, override, wantLabel, wantStatus, wantOverride)
	}
}
