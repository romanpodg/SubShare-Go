package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func externalFixtureHistoryCounts(t *testing.T, app *App, runID int64) sources.Counts {
	t.Helper()
	var raw string
	if err := app.db.QueryRow(`SELECT result_counts_json FROM source_sync_runs WHERE id = ?`, runID).Scan(&raw); err != nil {
		t.Fatalf("read sync history: %v", err)
	}
	var counts sources.Counts
	if err := json.Unmarshal([]byte(raw), &counts); err != nil {
		t.Fatalf("decode sync history counts: %v", err)
	}
	return counts
}

type externalSyncCount string

const (
	syncAdded     externalSyncCount = "added"
	syncUpdated   externalSyncCount = "updated"
	syncUnchanged externalSyncCount = "unchanged"
	syncRemoved   externalSyncCount = "removed"
	syncImported  externalSyncCount = "imported"
)

// An absent count is deliberately unasserted, matching the original phase's contract.
type externalSyncCounts map[externalSyncCount]int

type externalFixtureCheck struct {
	Name  string
	Check func(*testing.T)
}

func runExternalFixtureChecks(t *testing.T, phases []externalFixtureCheck) {
	t.Helper()
	for _, phase := range phases {
		if !t.Run(phase.Name, phase.Check) {
			return
		}
	}
}

type externalSyncFixturePhase struct {
	Name, Body string
	Rows       int
	Counts     externalSyncCounts
	Check      func(*testing.T)
}

func assertExternalSyncFixtureCounts(t *testing.T, result sources.SyncResult, want externalSyncCounts) {
	t.Helper()
	got := externalSyncCounts{syncAdded: result.Counts.Added, syncUpdated: result.Counts.Updated, syncUnchanged: result.Counts.Unchanged, syncRemoved: result.Counts.Removed, syncImported: result.Imported}
	for field, value := range want {
		if got[field] != value {
			t.Fatalf("sync %s=%d want=%d", field, got[field], value)
		}
	}
}

func runExternalSyncFixturePhases(t *testing.T, app *App, sourceID int64, phases []externalSyncFixturePhase) {
	t.Helper()
	for _, phase := range phases {
		if !t.Run(phase.Name, func(t *testing.T) {
			result := syncExternalFixtureBody(t, app, sourceID, phase.Body)
			assertExternalSyncFixtureCounts(t, result, phase.Counts)
			if got := sourceProfileCount(t, app, sourceID); got != phase.Rows {
				t.Fatalf("source rows=%d want=%d", got, phase.Rows)
			}
			if phase.Check != nil {
				phase.Check(t)
			}
		}) {
			return
		}
	}
}

type externalStoredProfileFixture struct {
	ID                                                            int64
	Envelope, Protocol, Fingerprint, Compatibility, Warnings, Ref string
	SchemaVersion                                                 int
}

func externalFixtureProfile(t *testing.T, app *App, sourceID int64, protocol string) externalStoredProfileFixture {
	t.Helper()
	var row externalStoredProfileFixture
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url, k.protocol, k.profile_fingerprint, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json, k.external_key_ref FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ? AND k.protocol = ?`, sourceID, protocol).Scan(&row.ID, &row.Envelope, &row.Protocol, &row.Fingerprint, &row.SchemaVersion, &row.Compatibility, &row.Warnings, &row.Ref); err != nil {
		t.Fatalf("read persisted external profile: %v", err)
	}
	return row
}

func assertInitialExternalSSFixture(t *testing.T, app *App, row externalStoredProfileFixture) {
	t.Helper()
	if profileFixtureRaw(t, app, row.ID) != externalTestSS || row.Protocol != "shadowsocks" || !strings.HasPrefix(row.Fingerprint, "pf1_") || row.SchemaVersion != sources.ExternalProfileSchemaVersion || row.Compatibility != "full" || row.Warnings != "[]" {
		t.Fatal("unexpected persisted Shadowsocks profile")
	}
}

func countExternalFixtureRawRows(t *testing.T, app *App, want string) int {
	t.Helper()
	rows, err := app.db.Query(`SELECT k.id, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON k.id = s.vless_key_id`)
	if err != nil {
		t.Fatalf("query source profile secrets: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		var envelope string
		if err := rows.Scan(&id, &envelope); err != nil {
			t.Fatalf("scan source profile secret: %v", err)
		}
		secret, err := profilestorage.Decrypt(envelope, app.profileKeyring, id)
		if err != nil {
			t.Fatalf("decrypt source profile secret: %v", err)
		}
		if secret.Reveal() == want {
			count++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate source profile secrets: %v", err)
	}
	return count
}

func assertTUICV4StoredFixture(t *testing.T, app *App, sourceID int64) {
	t.Helper()
	var id int64
	var envelope, fingerprint, compatibility string
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url, COALESCE(k.profile_fingerprint, ''), k.profile_compatibility FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ?`, sourceID).Scan(&id, &envelope, &fingerprint, &compatibility); err != nil {
		t.Fatalf("read TUIC v4: %v", err)
	}
	secret, err := profilestorage.Decrypt(envelope, app.profileKeyring, id)
	if err != nil {
		t.Fatalf("decrypt TUIC v4: %v", err)
	}
	if secret.Reveal() != externalTestTUICV4 || !strings.HasPrefix(fingerprint, "pf1_") || compatibility != "read_only" {
		t.Fatal("unexpected TUIC v4 persistence")
	}
}

type externalSourceIdentityFixture struct {
	ID  int64
	Ref string
}

func externalFixtureSourceIdentity(t *testing.T, app *App, sourceID int64) externalSourceIdentityFixture {
	t.Helper()
	var identity externalSourceIdentityFixture
	if err := app.db.QueryRow(`SELECT id, external_key_ref FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&identity.ID, &identity.Ref); err != nil {
		t.Fatalf("read source identity: %v", err)
	}
	return identity
}

func assignCrossSourceFixtureKeys(t *testing.T, app *App, firstID, secondID int64) int64 {
	t.Helper()
	userID := seedSubscriptionUser(t, app, "active")
	if _, err := app.db.Exec(`UPDATE users SET key_assignment_mode = 'selected' WHERE id = ?`, userID); err != nil {
		t.Fatalf("set selected assignment mode: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?), (?, ?)`, userID, firstID, userID, secondID); err != nil {
		t.Fatalf("assign both source-owned records: %v", err)
	}
	return userID
}

func assertSourceScopedFixtureDeletion(t *testing.T, app *App, userID, firstID, secondID, secondSource int64) {
	t.Helper()
	firstRows := profileFixtureCount(t, app, `SELECT COUNT(*) FROM vless_keys WHERE id = ?`, firstID)
	secondRows := profileFixtureCount(t, app, `SELECT COUNT(*) FROM vless_keys WHERE id = ? AND external_source_id = ? AND profile_compatibility = 'read_only'`, secondID, secondSource)
	assignments := profileFixtureCount(t, app, `SELECT COUNT(*) FROM user_keys WHERE user_id = ? AND key_id = ?`, userID, secondID)
	if firstRows != 0 || secondRows != 1 || assignments != 1 {
		t.Fatalf("source-specific deletion failed: first=%d second=%d assignment=%d", firstRows, secondRows, assignments)
	}
}
