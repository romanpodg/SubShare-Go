package main

import (
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"strings"
	"testing"
)

func TestExternalProfileSyncRepairsHistoricalDuplicatesAndMergesAssignments(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/historical-repair")
	firstUser := seedExternalRepairUser(t, app, "first")
	secondUser := seedExternalRepairUser(t, app, "second")
	firstRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Historical first", 1)
	secondRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Historical second", 1)
	blankLowerID := insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: firstRaw, Protocol: "vless", Fingerprint: "", UserIDs: []int64{firstUser}})
	currentFingerprint := parseExternalTestBody(t, externalTestVLESS).Keys[0].Fingerprint
	ownerID := insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: secondRaw, Protocol: "vless", Fingerprint: currentFingerprint, UserIDs: []int64{secondUser}})
	requireFixturef(t, ownerID > blankLowerID, "test fixture IDs are not ordered: blank=%d owner=%d", blankLowerID, ownerID)

	{
		_, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = 'Historical override' WHERE id = ?`, blankLowerID)
		requireFixturef(t, err == nil, "set duplicate override: %v", err)
	}

	result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, externalTestVLESS))
	requireFixturef(t, err == nil, "repair sync failed: %v", err)

	{
		got := sourceProfileCount(t, app, sourceID)
		requireFixturef(t, got == 1, "source rows=%d want=1 result=%#v", got, result)
	}

	var survivorID int64
	{
		err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&survivorID)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, survivorID == ownerID, "survivor=%d want current-fingerprint owner=%d", survivorID, ownerID)

	var clientDisplayName string
	{
		err := app.db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, survivorID).Scan(&clientDisplayName)
		requireFixturef(t, err == nil, "merged client override=%q err=%v", clientDisplayName, err)
		requireFixturef(t, clientDisplayName == "Historical override", "merged client override=%q err=%v", clientDisplayName, err)
	}

	var assignments int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE key_id = ? AND user_id IN (?, ?)`, survivorID, firstUser, secondUser).Scan(&assignments)
		requireFixturef(t, err == nil, "merged assignments=%d want=2 err=%v", assignments, err)
		requireFixturef(t, assignments == 2, "merged assignments=%d want=2 err=%v", assignments, err)
	}

}

func TestExternalProfileSyncRepairsBlankDuplicatesDuringPartialImport(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/partial-repair")
	firstRaw := strings.Replace(externalTestHY2, "#HY2", "#first", 1)
	secondRaw := strings.Replace(externalTestHY2, "#HY2", "#second", 1)
	secondRaw = strings.Replace(secondRaw, "hysteria2://", "hy2://", 1)
	firstID := insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: firstRaw, Protocol: "hysteria2", Fingerprint: ""})
	insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: secondRaw, Protocol: "hysteria2", Fingerprint: ""})
	missingID := insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: externalTestVMess, Protocol: "vmess", Fingerprint: ""})

	parsed := parseExternalTestBody(t, externalTestHY2+"\n"+"hysteria://unsupported.example:443")
	result, err := app.syncExternalSource(sourceID, parsed)
	requireFixturef(t, err == nil, "partial repair sync failed: %v", err)

	requireFixturef(t, result.Counts.Removed == 1, "partial repair result=%#v rows=%d", result, sourceProfileCount(t, app, sourceID))
	requireFixturef(t, sourceProfileCount(t, app, sourceID) == 2, "partial repair result=%#v rows=%d", result, sourceProfileCount(t, app, sourceID))

	for _, keyID := range []int64{firstID, missingID} {
		var exists int
		{
			err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, keyID).Scan(&exists)
			requireFixturef(t, err == nil, "preserved key %d exists=%d err=%v", keyID, exists, err)
			requireFixturef(t, exists == 1, "preserved key %d exists=%d err=%v", keyID, exists, err)
		}

	}
}

func TestExternalProfileSyncLeavesRawXrayJSONOnExactRawIdentity(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/xray-exact-repair")
	secondRaw := strings.Replace(externalTestXray, `"tag":"Xray"`, `"tag":"Xray renamed"`, 1)
	firstID := insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: externalTestXray, Protocol: "xray-json", Fingerprint: ""})
	secondID := insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: secondRaw, Protocol: "xray-json", Fingerprint: ""})

	parsed := parseExternalTestBody(t, externalTestXray+"\n"+"hysteria://unsupported.example:443")
	result, err := app.syncExternalSource(sourceID, parsed)
	requireFixturef(t, err == nil, "XRAY-JSON partial sync failed: %v", err)

	requireFixturef(t, result.Counts.Removed == 0, "XRAY-JSON exact-raw rows were reported removed: result=%#v", result)

	for _, keyID := range []int64{firstID, secondID} {
		var exists int
		{
			err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, keyID).Scan(&exists)
			requireFixturef(t, err == nil, "XRAY-JSON exact-raw key %d exists=%d err=%v", keyID, exists, err)
			requireFixturef(t, exists == 1, "XRAY-JSON exact-raw key %d exists=%d err=%v", keyID, exists, err)
		}

	}
}

func TestExternalProfileSyncRepairsEightyRowsToFortyAndRemainsStable(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/eighty-to-forty")
	otherSourceID := seedExternalProfileSource(t, app, "https://provider.example/eighty-to-forty-other")
	current := make([]string, 0, 40)
	for index := 1; index <= 40; index++ {
		raw := fmt.Sprintf("vless://%08x-1111-4111-8111-%012x@node-%02d.example:443?security=tls&type=tcp#Current-%02d", index, index, index, index)
		variant := strings.Replace(raw, "?security=tls&type=tcp", "?type=tcp&security=tls", 1)
		variant = strings.Replace(variant, fmt.Sprintf("#Current-%02d", index), fmt.Sprintf("#Historical-%02d", index), 1)
		insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: raw, Protocol: "vless", Fingerprint: ""})
		insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: sourceID, Raw: variant, Protocol: "vless", Fingerprint: ""})
		current = append(current, raw)
	}
	insertHistoricalExternalProfile(t, app, historicalExternalProfileSeed{SourceID: otherSourceID, Raw: current[0], Protocol: "vless", Fingerprint: ""})
	{
		got := sourceProfileCount(t, app, sourceID)
		requireFixturef(t, got == 80, "corrupted fixture rows=%d want=80", got)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, strings.Join(current, "\n")))
		requireFixturef(t, err == nil, "sync %d failed: %v", attempt, err)

		{
			got := sourceProfileCount(t, app, sourceID)
			requireFixturef(t, got == 40, "sync %d rows=%d want=40 result=%#v", attempt, got, result)
		}

		if attempt == 1 {
			requireFixturef(t, result.Counts.Removed == 40, "repair classifications=%#v", result.Counts)
			requireFixturef(t, result.Counts.Updated == 40, "repair classifications=%#v", result.Counts)
			requireFixturef(t, result.Counts.Added == 0, "repair classifications=%#v", result.Counts)

		} else {
			requireFixturef(t, result.Counts.Removed == 0, "stable sync %d classifications=%#v", attempt, result.Counts)
			requireFixturef(t, result.Counts.Updated == 0, "stable sync %d classifications=%#v", attempt, result.Counts)
			requireFixturef(t, result.Counts.Unchanged == 40, "stable sync %d classifications=%#v", attempt, result.Counts)
		}
	}
	{
		got := sourceProfileCount(t, app, otherSourceID)
		requireFixturef(t, got == 1, "cross-source row count=%d want=1", got)
	}

}

func TestExternalProfileLegacyRowWithoutFingerprintKeepsIDOnRename(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/legacy-fingerprint-backfill")
	userID := seedSubscriptionUser(t, app, "active")
	legacyID := insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, SourceID: &sourceID, Label: "Old name", Raw: externalTestVLESS, Protocol: "vless", Compatibility: "legacy", SortOrder: 1})
	{
		_, err := app.db.Exec(`UPDATE vless_keys SET external_key_ref = ?, profile_fingerprint = NULL WHERE id = ?`, sources.KeyRef(externalTestVLESS), legacyID)
		requireFixturef(t, err == nil, "prepare legacy row: %v", err)
	}

	renamed := strings.Replace(externalTestVLESS, "#VLESS", "#New name", 1)
	result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, renamed))
	requireFixturef(t, err == nil, "legacy rename result=%#v err=%v", result, err)
	requireFixturef(t, result.Counts.Added == 0, "legacy rename result=%#v err=%v", result, err)
	requireFixturef(t, result.Counts.Updated == 1, "legacy rename result=%#v err=%v", result, err)

	var currentID int64
	var fingerprint string
	{
		err := app.db.QueryRow(`SELECT id, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &fingerprint)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, currentID == legacyID, "legacy identity changed: id=%d want=%d fingerprint=%q", currentID, legacyID, fingerprint)
	requireFixturef(t, strings.HasPrefix(fingerprint, "pf1_"), "legacy identity changed: id=%d want=%d fingerprint=%q", currentID, legacyID, fingerprint)

}

func TestExternalProfileKeyRotationMatchesPreviousFingerprint(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/rotation")
	parsed := parseExternalTestBody(t, externalTestTUIC)
	{
		_, err := app.syncExternalSource(sourceID, parsed)
		requireFixturef(t, err == nil, "initial sync: %v", err)
	}

	var id int64
	var oldFingerprint string
	{
		err := app.db.QueryRow(`SELECT id, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&id, &oldFingerprint)
		requireFixturef(t, err == nil, "read old fingerprint: %v", err)
	}

	newKey := []byte("abcdef0123456789abcdef0123456789")
	app.profileFingerprintOldKeys = [][]byte{append([]byte(nil), app.profileFingerprintKey...)}
	app.profileFingerprintKey = newKey
	rotated, err := sources.ParseBody(externalTestTUIC, app.externalProfileFingerprintKeys())
	requireFixturef(t, err == nil, "parse rotated keyring: %v", err)

	result, err := app.syncExternalSource(sourceID, rotated)
	requireFixturef(t, err == nil, "rotation sync result=%#v err=%v", result, err)
	requireFixturef(t, result.Counts.Updated == 1, "rotation sync result=%#v err=%v", result, err)

	var currentID int64
	var newFingerprint string
	{
		err := app.db.QueryRow(`SELECT id, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &newFingerprint)
		requireFixturef(t, err == nil, "read rotated fingerprint: %v", err)
	}

	requireFixturef(t, currentID == id, "rotation failed to preserve row and replace digest: id=%d/%d old=%q new=%q", id, currentID, oldFingerprint, newFingerprint)
	requireFixturef(t, newFingerprint != oldFingerprint, "rotation failed to preserve row and replace digest: id=%d/%d old=%q new=%q", id, currentID, oldFingerprint, newFingerprint)

}
