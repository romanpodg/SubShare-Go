package main

import (
	"database/sql"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func syncExternalFixtureBody(t *testing.T, app *App, sourceID int64, raw string) sources.SyncResult {
	t.Helper()
	result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, raw))
	requireFixturef(t, err == nil, "sync external profile fixture: %v", err)

	return result
}

type externalMetadataExpectation struct {
	Label, Status string
	Override      sql.NullString
}

func assertExternalFixtureMetadata(t *testing.T, app *App, keyID int64, want externalMetadataExpectation) {
	t.Helper()
	var label, status string
	var override sql.NullString
	{
		err := app.db.QueryRow(`SELECT label, status, client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&label, &status, &override)
		requireFixturef(t, err == nil, "read external profile metadata: %v", err)
	}

	requireFixturef(t, label == want.Label, "external profile metadata label=%q status=%q override=%#v; want label=%q status=%q override=%#v", label, status, override, want.Label, want.Status, want.Override)
	requireFixturef(t, status == want.Status, "external profile metadata label=%q status=%q override=%#v; want label=%q status=%q override=%#v", label, status, override, want.Label, want.Status, want.Override)
	requireFixturef(t, override == want.Override, "external profile metadata label=%q status=%q override=%#v; want label=%q status=%q override=%#v", label, status, override, want.Label, want.Status, want.Override)

}

type externalMetadataScope struct{ SourceID, KeyID int64 }
type externalMetadataPhase struct {
	name, raw, label string
	override         sql.NullString
	resetOverride    bool
	effectiveName    string
}

func syncExternalFixtureMetadataPhase(t *testing.T, app *App, scope externalMetadataScope, phase externalMetadataPhase) {
	t.Helper()

	if phase.resetOverride {
		{
			_, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = NULL WHERE id = ?`, scope.KeyID)
			requireFixture(t, err == nil, err)
		}

	}
	syncExternalFixtureBody(t, app, scope.SourceID, phase.raw)
	assertExternalFixtureMetadata(t, app, scope.KeyID, externalMetadataExpectation{Label: phase.label, Status: "non-active", Override: phase.override})
	raw := profileFixtureRaw(t, app, scope.KeyID)
	requireFixture(t, raw == phase.raw, "source-controlled configuration was not updated")

	if phase.effectiveName != "" {
		{
			effective := profileconfig.EffectiveClientDisplayName("", raw, phase.label, true)
			requireFixturef(t, effective == phase.effectiveName, "post-reset effective client name=%q", effective)
		}

	}
}
