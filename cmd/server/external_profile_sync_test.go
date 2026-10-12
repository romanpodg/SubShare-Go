package main

import (
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"strings"
	"sync"
	"testing"
)

func TestExternalProfileSyncPreservesLocalStatusAndClientDisplayName(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/client-name-override")
	initialRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Source A", 1)
	syncExternalFixtureBody(t, app, sourceID, initialRaw)
	var keyID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`UPDATE vless_keys SET status = 'non-active', client_display_name = 'CUSTOM' WHERE id = ?`, keyID); err != nil {
		t.Fatal(err)
	}

	renamedRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Source B", 1)
	connectionUpdatedRaw := strings.Replace(renamedRaw, "vless.example", "rotated-vless.example", 1)
	latestRaw := strings.Replace(connectionUpdatedRaw, "#Source B", "#Source C", 1)
	phases := []struct {
		name, raw, label string
		override         sql.NullString
		resetOverride    bool
		effectiveName    string
	}{
		{name: "source rename preserves local metadata", raw: renamedRaw, label: "Source B", override: sql.NullString{String: "CUSTOM", Valid: true}},
		{name: "connection update preserves local metadata", raw: connectionUpdatedRaw, label: "Source B", override: sql.NullString{String: "CUSTOM", Valid: true}},
		{name: "override reset restores source display name", raw: latestRaw, label: "Source C", resetOverride: true, effectiveName: "Source C"},
	}
	// These phases intentionally share the imported row; a failed predecessor
	// must stop the story rather than run successors against incomplete state.
	for _, phase := range phases {
		if !t.Run(phase.name, func(t *testing.T) {
			if phase.resetOverride {
				if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = NULL WHERE id = ?`, keyID); err != nil {
					t.Fatal(err)
				}
			}
			syncExternalFixtureBody(t, app, sourceID, phase.raw)
			assertExternalFixtureMetadata(t, app, keyID, phase.label, "non-active", phase.override)
			raw := profileFixtureRaw(t, app, keyID)
			if raw != phase.raw {
				t.Fatal("source-controlled configuration was not updated")
			}
			if phase.effectiveName != "" {
				if effective := profileconfig.EffectiveClientDisplayName("", raw, phase.label, true); effective != phase.effectiveName {
					t.Fatalf("post-reset effective client name=%q", effective)
				}
			}
		}) {
			return
		}
	}

	t.Run("inactive profile remains excluded from delivery", func(t *testing.T) {
		userID := seedSubscriptionUser(t, app, "active")
		assignProfileFixtureUsers(t, app, keyID, userID)
		generated, _, err := app.generateSelectedSubscription("subscription-token", "plain")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(generated.Body, "vless.example") {
			t.Fatal("inactive source profile was delivered")
		}
	})
}

func TestExternalProfileDuplicateWithinSourcePersistsOnce(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/source-duplicate")
	body := externalTestHY2 + "\n" + strings.Replace(externalTestHY2, "#HY2", "#duplicate-tag", 1)
	parsed := parseExternalTestBody(t, body)
	if parsed.Counts.Duplicate != 1 || len(parsed.Keys) != 1 {
		t.Fatalf("parser duplicate result=%#v", parsed)
	}
	result, err := app.syncExternalSource(sourceID, parsed)
	if err != nil || result.Imported != 1 || result.Counts.Duplicate != 1 {
		t.Fatalf("source duplicate sync result=%#v err=%v", result, err)
	}
	var rows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("source duplicate rows=%d err=%v", rows, err)
	}
}

func TestExternalProfilePersistenceAndSynchronization(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/profiles")
	result := syncExternalFixtureBody(t, app, sourceID, externalTestSS+"\n"+externalTestHY2)
	assertExternalSyncFixtureCounts(t, result, externalSyncCounts{syncImported: 2})
	initial := externalFixtureProfile(t, app, sourceID, "shadowsocks")
	assertInitialExternalSSFixture(t, app, initial)
	tagChanged := strings.Replace(externalTestSS, "#SS", "#Renamed", 1)
	phases := []externalSyncFixturePhase{
		{Name: "tag refresh preserves semantic identity", Body: tagChanged + "\n" + externalTestHY2, Rows: 2, Counts: externalSyncCounts{syncUpdated: 1, syncUnchanged: 1}, Check: func(t *testing.T) {
			updated := externalFixtureProfile(t, app, sourceID, "shadowsocks")
			if updated.ID != initial.ID || profileFixtureRaw(t, app, updated.ID) != tagChanged || updated.Ref != initial.Ref {
				t.Fatal("semantic update changed identity or raw output")
			}
		}},
		{Name: "partial refresh retains valid stored profile", Body: tagChanged + "\n" + "tuic://not-a-uuid:password@example.com:443", Rows: 2},
		{Name: "complete refresh removes missing profile", Body: tagChanged, Rows: 1},
		{Name: "credential change gets new fingerprint", Body: strings.Replace(tagChanged, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1), Rows: 1, Counts: externalSyncCounts{syncImported: 1}, Check: func(t *testing.T) {
			if changed := externalFixtureProfile(t, app, sourceID, "shadowsocks"); changed.Fingerprint == initial.Fingerprint {
				t.Fatal("changed credential retained the old semantic fingerprint")
			}
		}},
	}
	runExternalSyncFixturePhases(t, app, sourceID, phases)
}

func TestExternalProfileSynchronizationIsIdempotentAndSourceScoped(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/idempotent")
	initialBody := strings.Join([]string{externalTestVLESS, externalTestVMess, externalTestTrojan}, "\n")
	renamedVLESS := strings.Replace(externalTestVLESS, "#VLESS", "#Renamed VLESS", 1)
	renamedBody := strings.Join([]string{renamedVLESS, externalTestVMess, externalTestTrojan}, "\n")
	var stableVLESSID int64
	phases := []externalSyncFixturePhase{
		{Name: "first import", Body: initialBody, Rows: 3, Counts: externalSyncCounts{syncAdded: 3, syncUpdated: 0, syncUnchanged: 0}},
		{Name: "repeat 2", Body: initialBody, Rows: 3, Counts: externalSyncCounts{syncAdded: 0, syncUpdated: 0, syncUnchanged: 3}},
		{Name: "repeat 3", Body: initialBody, Rows: 3, Counts: externalSyncCounts{syncAdded: 0, syncUpdated: 0, syncUnchanged: 3}},
		{Name: "reorder", Body: strings.Join([]string{externalTestTrojan, externalTestVLESS, externalTestVMess}, "\n"), Rows: 3, Counts: externalSyncCounts{syncAdded: 0, syncUnchanged: 3}, Check: func(t *testing.T) {
			stableVLESSID = externalFixtureProfile(t, app, sourceID, "vless").ID
		}},
		{Name: "rename retains row identity", Body: renamedBody, Rows: 3, Counts: externalSyncCounts{syncAdded: 0, syncUpdated: 1, syncUnchanged: 2}, Check: func(t *testing.T) {
			if id := externalFixtureProfile(t, app, sourceID, "vless").ID; id != stableVLESSID {
				t.Fatalf("renamed VLESS id=%d want=%d", id, stableVLESSID)
			}
		}},
		{Name: "add profile", Body: renamedBody + "\n" + externalTestSS, Rows: 4, Counts: externalSyncCounts{syncAdded: 1, syncUnchanged: 3}},
		{Name: "partial refresh retains missing profiles", Body: renamedVLESS + "\n" + "hysteria://unsupported@example.com:443", Rows: 4, Counts: externalSyncCounts{syncRemoved: 0}},
		{Name: "complete refresh removes missing profiles", Body: renamedVLESS, Rows: 1, Counts: externalSyncCounts{syncRemoved: 3, syncUnchanged: 1}},
	}
	runExternalSyncFixturePhases(t, app, sourceID, phases)
	if t.Failed() {
		return
	}
	otherSourceID := seedExternalProfileSource(t, app, "https://provider.example/idempotent-other")
	other := syncExternalFixtureBody(t, app, otherSourceID, renamedVLESS)
	assertExternalSyncFixtureCounts(t, other, externalSyncCounts{syncAdded: 1})
	crossSourceRows := profileFixtureCount(t, app, `SELECT COUNT(*) FROM vless_keys WHERE protocol = 'vless' AND external_source_id IN (?, ?)`, sourceID, otherSourceID)
	if crossSourceRows != 2 {
		t.Fatalf("cross-source rows=%d", crossSourceRows)
	}
}

func TestExternalProfileConcurrentRetryDoesNotDuplicateAndHistoryCountsAreAccurate(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/concurrent")
	parsed := parseExternalTestBody(t, strings.Join([]string{externalTestVLESS, externalTestVMess, externalTestTrojan}, "\n"))

	results := make(chan sources.SyncResult, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := app.syncExternalSource(sourceID, parsed)
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent sync failed: %v", err)
		}
	}
	addedTotal := 0
	unchangedTotal := 0
	var historyResult sources.SyncResult
	for result := range results {
		addedTotal += result.Counts.Added
		unchangedTotal += result.Counts.Unchanged
		if result.Counts.Unchanged == 3 {
			historyResult = result
		}
	}
	if addedTotal != 3 || unchangedTotal != 3 {
		t.Fatalf("concurrent classifications added=%d unchanged=%d", addedTotal, unchangedTotal)
	}
	var rows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&rows); err != nil || rows != 3 {
		t.Fatalf("concurrent source rows=%d err=%v", rows, err)
	}

	runID := app.startSourceSyncRun(sourceID)
	app.finishSourceSyncRun(runID, historyResult, nil)
	counts := externalFixtureHistoryCounts(t, app, runID)
	if counts.Added != 0 || counts.Updated != 0 || counts.Unchanged != 3 {
		t.Fatalf("history counts=%#v", counts)
	}
}

func TestExternalProfileCrossSourceAndTUICV4Compatibility(t *testing.T) {
	app := newIntegrationApp(t)
	firstSource := seedExternalProfileSource(t, app, "https://provider.example/first")
	secondSource := seedExternalProfileSource(t, app, "https://provider.example/second")
	parsed := parseExternalTestBody(t, externalTestTUICV4)
	var first, second externalSourceIdentityFixture
	var userID int64
	phases := []externalFixtureCheck{
		{Name: "persist compatibility-only v4", Check: func(t *testing.T) {
			result, err := app.syncExternalSource(firstSource, parsed)
			if err != nil || result.Counts.CompatibilityOnly != 1 {
				t.Fatalf("TUIC v4 first sync result=%#v err=%v", result, err)
			}
			assertTUICV4StoredFixture(t, app, firstSource)
		}},
		{Name: "second source retains independent encrypted row", Check: func(t *testing.T) {
			result, err := app.syncExternalSource(secondSource, parsed)
			if err != nil || result.Counts.CompatibilityOnly != 1 || result.Imported != 1 {
				t.Fatalf("second source did not retain its own exact profile: result=%#v err=%v", result, err)
			}
			if rows := countExternalFixtureRawRows(t, app, externalTestTUICV4); rows != 2 {
				t.Fatalf("source-owned exact profiles were merged: rows=%d", rows)
			}
			first = externalFixtureSourceIdentity(t, app, firstSource)
			second = externalFixtureSourceIdentity(t, app, secondSource)
			if first.ID == second.ID || first.Ref != second.Ref {
				t.Fatalf("identity roles collapsed: first=%#v second=%#v", first, second)
			}
		}},
		{Name: "assigned source rows refresh independently", Check: func(t *testing.T) {
			userID = assignCrossSourceFixtureKeys(t, app, first.ID, second.ID)
			for sourceID, label := range map[int64]string{firstSource: "first", secondSource: "second"} {
				refreshed, err := app.syncExternalSource(sourceID, parsed)
				if err != nil || refreshed.Counts.Unchanged != 1 {
					t.Fatalf("independent %s refresh result=%#v err=%v", label, refreshed, err)
				}
			}
		}},
		{Name: "deletion is scoped to source and assignment", Check: func(t *testing.T) {
			syncExternalFixtureBody(t, app, firstSource, externalTestSS)
			assertSourceScopedFixtureDeletion(t, app, userID, first.ID, second.ID, secondSource)
		}},
		{Name: "tagged v5 remains separately source-owned", Check: func(t *testing.T) {
			tagged := strings.Replace(externalTestTUIC, "#TUIC", "#Other-source", 1)
			syncExternalFixtureBody(t, app, firstSource, externalTestTUIC)
			thirdSource := seedExternalProfileSource(t, app, "https://provider.example/third")
			result := syncExternalFixtureBody(t, app, thirdSource, tagged)
			assertExternalSyncFixtureCounts(t, result, externalSyncCounts{syncImported: 1})
		}},
	}
	runExternalFixtureChecks(t, phases)
}
