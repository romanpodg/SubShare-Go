package main

import (
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func seedExternalProfileSource(t *testing.T, app *App, sourceURL string) int64 {
	t.Helper()
	result, err := app.db.Exec(`INSERT INTO external_subscription_sources(name, category, key_category, key_insert_mode, source_url, enabled, import_status) VALUES('profiles', 'general', '', 'bottom', ?, 1, 'idle')`, sourceURL)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	id := profileFixtureInsertID(t, result)
	return id
}

func seedExternalRepairUser(t *testing.T, app *App, suffix string) int64 {
	t.Helper()
	result, err := app.db.Exec(
		`INSERT INTO users(name, token, status, key_assignment_mode) VALUES(?, ?, 'active', 'selected')`,
		"repair-"+suffix,
		"repair-token-"+suffix,
	)
	if err != nil {
		t.Fatalf("insert repair user: %v", err)
	}
	id := profileFixtureInsertID(t, result)
	return id
}

// historicalExternalProfileSeed deliberately permits blank fingerprints and legacy
// metadata so repair tests exercise persisted historical state, not the importer.
type historicalExternalProfileSeed struct {
	SourceID    int64
	Raw         string
	Protocol    string
	Fingerprint string
	UserIDs     []int64
}

func insertHistoricalExternalProfile(t *testing.T, app *App, seed historicalExternalProfileSeed) int64 {
	t.Helper()
	result, err := app.db.Exec(`
  INSERT INTO vless_keys(
   label, status, key_kind, external_source_id, external_key_ref, protocol,
   profile_fingerprint, profile_schema_version, profile_compatibility
  ) VALUES('historical', 'active', 'real', ?, ?, ?, ?, 0, 'legacy')
 `, seed.SourceID, sources.KeyRef(seed.Raw), seed.Protocol, nullStringValue(seed.Fingerprint))
	if err != nil {
		t.Fatalf("insert historical profile: %v", err)
	}
	keyID := profileFixtureInsertID(t, result)
	insertProfileFixtureSecret(t, app, keyID, seed.Raw)
	assignProfileFixtureUsers(t, app, keyID, seed.UserIDs...)
	return keyID
}

func sourceProfileCount(t *testing.T, app *App, sourceID int64) int {
	t.Helper()
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&count); err != nil {
		t.Fatalf("count source profiles: %v", err)
	}
	return count
}
