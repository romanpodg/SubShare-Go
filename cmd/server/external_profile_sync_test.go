package main

import (
	"database/sql"
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
	{
		err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&keyID)
		requireFixture(t, err == nil, err)
	}

	{
		_, err := app.db.Exec(`UPDATE vless_keys SET status = 'non-active', client_display_name = 'CUSTOM' WHERE id = ?`, keyID)
		requireFixture(t, err == nil, err)
	}

	renamedRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Source B", 1)
	connectionUpdatedRaw := strings.Replace(renamedRaw, "vless.example", "rotated-vless.example", 1)
	latestRaw := strings.Replace(connectionUpdatedRaw, "#Source B", "#Source C", 1)
	phases := []externalMetadataPhase{
		{name: "source rename preserves local metadata", raw: renamedRaw, label: "Source B", override: sql.NullString{String: "CUSTOM", Valid: true}},
		{name: "connection update preserves local metadata", raw: connectionUpdatedRaw, label: "Source B", override: sql.NullString{String: "CUSTOM", Valid: true}},
		{name: "override reset restores source display name", raw: latestRaw, label: "Source C", resetOverride: true, effectiveName: "Source C"},
	}
	// These phases intentionally share the imported row; a failed predecessor
	// must stop the story rather than run successors against incomplete state.
	for _, phase := range phases {
		if !t.Run(phase.name, func(t *testing.T) {
			syncExternalFixtureMetadataPhase(t, app, externalMetadataScope{SourceID: sourceID, KeyID: keyID}, phase)
		}) {
			return
		}
	}

	t.Run("inactive profile remains excluded from delivery", func(t *testing.T) {
		userID := seedSubscriptionUser(t, app, "active")
		assignProfileFixtureUsers(t, app, keyID, userID)
		generated, _, err := app.generateSelectedSubscription("subscription-token", "plain")
		requireFixture(t, err == nil, err)

		requireFixture(t, !(strings.Contains(generated.Body, "vless.example")), "inactive source profile was delivered")

	})
}

func TestExternalProfileDuplicateWithinSourcePersistsOnce(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/source-duplicate")
	body := externalTestHY2 + "\n" + strings.Replace(externalTestHY2, "#HY2", "#duplicate-tag", 1)
	parsed := parseExternalTestBody(t, body)
	requireFixturef(t, parsed.Counts.Duplicate == 1, "parser duplicate result=%#v", parsed)
	requireFixturef(t, len(parsed.Keys) == 1, "parser duplicate result=%#v", parsed)

	result, err := app.syncExternalSource(sourceID, parsed)
	requireFixturef(t, err == nil, "source duplicate sync result=%#v err=%v", result, err)
	requireFixturef(t, result.Imported == 1, "source duplicate sync result=%#v err=%v", result, err)
	requireFixturef(t, result.Counts.Duplicate == 1, "source duplicate sync result=%#v err=%v", result, err)

	var rows int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&rows)
		requireFixturef(t, err == nil, "source duplicate rows=%d err=%v", rows, err)
		requireFixturef(t, rows == 1, "source duplicate rows=%d err=%v", rows, err)
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
			requireFixture(t, updated.ID == initial.ID, "semantic update changed identity or raw output")
			requireFixture(t, profileFixtureRaw(t, app, updated.ID) == tagChanged, "semantic update changed identity or raw output")
			requireFixture(t, updated.Ref == initial.Ref, "semantic update changed identity or raw output")

		}},
		{Name: "partial refresh retains valid stored profile", Body: tagChanged + "\n" + "tuic://not-a-uuid:password@example.com:443", Rows: 2},
		{Name: "complete refresh removes missing profile", Body: tagChanged, Rows: 1},
		{Name: "credential change gets new fingerprint", Body: strings.Replace(tagChanged, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1), Rows: 1, Counts: externalSyncCounts{syncImported: 1}, Check: func(t *testing.T) {
			{
				changed := externalFixtureProfile(t, app, sourceID, "shadowsocks")
				requireFixture(t, changed.Fingerprint != initial.Fingerprint, "changed credential retained the old semantic fingerprint")
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
			{
				id := externalFixtureProfile(t, app, sourceID, "vless").ID
				requireFixturef(t, id == stableVLESSID, "renamed VLESS id=%d want=%d", id, stableVLESSID)
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
	requireFixturef(t, crossSourceRows == 2, "cross-source rows=%d", crossSourceRows)

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
		requireFixturef(t, err == nil, "concurrent sync failed: %v", err)

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
	requireFixturef(t, addedTotal == 3, "concurrent classifications added=%d unchanged=%d", addedTotal, unchangedTotal)
	requireFixturef(t, unchangedTotal == 3, "concurrent classifications added=%d unchanged=%d", addedTotal, unchangedTotal)

	var rows int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&rows)
		requireFixturef(t, err == nil, "concurrent source rows=%d err=%v", rows, err)
		requireFixturef(t, rows == 3, "concurrent source rows=%d err=%v", rows, err)
	}

	runID := app.startSourceSyncRun(sourceID)
	app.finishSourceSyncRun(runID, historyResult, nil)
	counts := externalFixtureHistoryCounts(t, app, runID)
	requireFixturef(t, counts.Added == 0, "history counts=%#v", counts)
	requireFixturef(t, counts.Updated == 0, "history counts=%#v", counts)
	requireFixturef(t, counts.Unchanged == 3, "history counts=%#v", counts)

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
			requireFixturef(t, err == nil, "TUIC v4 first sync result=%#v err=%v", result, err)
			requireFixturef(t, result.Counts.CompatibilityOnly == 1, "TUIC v4 first sync result=%#v err=%v", result, err)

			assertTUICV4StoredFixture(t, app, firstSource)
		}},
		{Name: "second source retains independent encrypted row", Check: func(t *testing.T) {
			result, err := app.syncExternalSource(secondSource, parsed)
			requireFixturef(t, err == nil, "second source did not retain its own exact profile: result=%#v err=%v", result, err)
			requireFixturef(t, result.Counts.CompatibilityOnly == 1, "second source did not retain its own exact profile: result=%#v err=%v", result, err)
			requireFixturef(t, result.Imported == 1, "second source did not retain its own exact profile: result=%#v err=%v", result, err)

			{
				rows := countExternalFixtureRawRows(t, app, externalTestTUICV4)
				requireFixturef(t, rows == 2, "source-owned exact profiles were merged: rows=%d", rows)
			}

			first = externalFixtureSourceIdentity(t, app, firstSource)
			second = externalFixtureSourceIdentity(t, app, secondSource)
			requireFixturef(t, first.ID != second.ID, "identity roles collapsed: first=%#v second=%#v", first, second)
			requireFixturef(t, first.Ref == second.Ref, "identity roles collapsed: first=%#v second=%#v", first, second)

		}},
		{Name: "assigned source rows refresh independently", Check: func(t *testing.T) {
			userID = assignCrossSourceFixtureKeys(t, app, first.ID, second.ID)
			for sourceID, label := range map[int64]string{firstSource: "first", secondSource: "second"} {
				refreshed, err := app.syncExternalSource(sourceID, parsed)
				requireFixturef(t, err == nil, "independent %s refresh result=%#v err=%v", label, refreshed, err)
				requireFixturef(t, refreshed.Counts.Unchanged == 1, "independent %s refresh result=%#v err=%v", label, refreshed, err)

			}
		}},
		{Name: "deletion is scoped to source and assignment", Check: func(t *testing.T) {
			syncExternalFixtureBody(t, app, firstSource, externalTestSS)
			assertSourceScopedFixtureDeletion(t, app, sourceDeletionFixtureScope{UserID: userID, FirstID: first.ID, SecondID: second.ID, SecondSource: secondSource})
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
