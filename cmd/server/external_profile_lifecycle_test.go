package main

import (
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExternalProfileWarningOnlyRefreshPreservesRow(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/warnings")
	alias := "hy2://auth@example.com:443#Alias"
	first, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, alias))
	if err != nil || first.Imported != 1 {
		t.Fatalf("initial alias sync: result=%#v err=%v", first, err)
	}
	var id int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&id); err != nil {
		t.Fatalf("read initial row: %v", err)
	}
	canonicalScheme := "hysteria2://auth@example.com:443#Alias"
	refreshed, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, canonicalScheme))
	if err != nil || refreshed.Counts.Updated != 1 {
		t.Fatalf("warning-only refresh result=%#v err=%v", refreshed, err)
	}
	var currentID int64
	var warningsJSON string
	if err := app.db.QueryRow(`SELECT id, profile_warnings_json FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &warningsJSON); err != nil {
		t.Fatalf("read refreshed row: %v", err)
	}
	if currentID != id || warningsJSON != "[]" {
		t.Fatalf("warning-only refresh replaced row or retained warning: id=%d/%d warnings=%q", id, currentID, warningsJSON)
	}
}

func TestExternalProfileSourceSpecificWarningsAndLifecycle(t *testing.T) {
	app := newIntegrationApp(t)
	firstSource := seedExternalProfileSource(t, app, "https://provider.example/warning-source-a")
	secondSource := seedExternalProfileSource(t, app, "https://provider.example/warning-source-b")
	aliasRaw := "hy2://source-auth@example.com:443#Alias"
	canonicalRaw := "hysteria2://source-auth@example.com:443#Canonical"
	if _, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, aliasRaw)); err != nil {
		t.Fatalf("sync alias source: %v", err)
	}
	if _, err := app.syncExternalSource(secondSource, parseExternalTestBody(t, canonicalRaw)); err != nil {
		t.Fatalf("sync canonical source: %v", err)
	}
	var firstID, secondID int64
	var firstWarnings, secondWarnings, secondCreated, secondFingerprint string
	if err := app.db.QueryRow(`SELECT id, profile_warnings_json FROM vless_keys WHERE external_source_id = ?`, firstSource).Scan(&firstID, &firstWarnings); err != nil {
		t.Fatalf("read alias source: %v", err)
	}
	if err := app.db.QueryRow(`SELECT id, profile_warnings_json, created_at, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, secondSource).Scan(&secondID, &secondWarnings, &secondCreated, &secondFingerprint); err != nil {
		t.Fatalf("read canonical source: %v", err)
	}
	if firstID == secondID || !strings.Contains(firstWarnings, profiles.WarningNonCanonicalScheme) || secondWarnings != "[]" {
		t.Fatalf("source warning state collapsed: first=%d %q second=%d %q", firstID, firstWarnings, secondID, secondWarnings)
	}

	// Source A becomes byte-identical to Source B. It updates only its own row.
	if _, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, canonicalRaw)); err != nil {
		t.Fatalf("canonicalize source A: %v", err)
	}
	var encURL, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter string
	if err := app.db.QueryRow(`SELECT s.encrypted_url, k.profile_warnings_json, k.created_at, k.profile_fingerprint FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ? AND k.external_source_id = ?`, secondID, secondSource).Scan(&encURL, &secondWarningsAfter, &secondCreatedAfter, &secondFingerprintAfter); err != nil {
		t.Fatalf("read unaffected source B: %v", err)
	}
	secB, _ := profilestorage.Decrypt(encURL, app.profileKeyring, secondID)
	secondRawAfter := secB.Reveal()
	if secondRawAfter != canonicalRaw || secondWarningsAfter != secondWarnings || secondCreatedAfter != secondCreated || secondFingerprintAfter != secondFingerprint {
		t.Fatalf("Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)
	}
	var identicalRows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE profile_fingerprint = ?`, secondFingerprint).Scan(&identicalRows); err != nil || identicalRows != 2 {
		t.Fatalf("byte-identical cross-source rows=%d err=%v", identicalRows, err)
	}
}

func TestExternalProfileCreateRollsBackOnPersistenceFailure(t *testing.T) {
	app := newIntegrationApp(t)
	if _, err := app.db.Exec(`CREATE TRIGGER reject_profile_insert BEFORE INSERT ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic profile failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"name": "Rollback", "category": "general", "key_category": "", "key_insert_mode": "bottom",
		"source_url": "https://provider.example/rollback-profile", "enabled": true, "raw_body": externalTestHY2,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.apiV1CreateSource(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("rollback response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var sources, keys int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE source_url = 'https://provider.example/rollback-profile'`).Scan(&sources)
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE protocol = 'hysteria2'`).Scan(&keys)
	if sources != 0 || keys != 0 {
		t.Fatalf("transactional rollback failed: sources=%d keys=%d", sources, keys)
	}
}

func TestExternalProfileSynchronizationRollsBackOnPersistenceFailure(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/sync-rollback")
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, externalTestSS)); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	var originalID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&originalID); err != nil {
		t.Fatalf("read original row: %v", err)
	}
	if _, err := app.db.Exec(`CREATE TRIGGER reject_refresh_update BEFORE UPDATE ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic refresh failure'); END`); err != nil {
		t.Fatalf("create refresh trigger: %v", err)
	}
	changed := strings.Replace(externalTestSS, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, changed)); err == nil {
		t.Fatal("refresh persistence failure was ignored")
	}
	var currentID int64
	var encURL string
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ?`, sourceID).Scan(&currentID, &encURL); err != nil {
		t.Fatalf("read retained row: %v", err)
	}
	secRetained, _ := profilestorage.Decrypt(encURL, app.profileKeyring, currentID)
	currentRaw := secRetained.Reveal()
	if currentID != originalID || currentRaw != externalTestSS {
		t.Fatalf("failed refresh partially mutated source: id=%d/%d raw_equal=%v", originalID, currentID, currentRaw == externalTestSS)
	}
}

func TestExternalProfileProbePolicyMarksOnlyUnimplementedProtocolsUnsupported(t *testing.T) {
	for _, raw := range []string{
		"tuic://33333333-3333-4333-8333-333333333333:password@192.0.2.11:443",
		"hysteria://legacy-auth@192.0.2.12:443",
	} {
		status, detail, _ := checkConfigurationAvailability(raw)
		if status != "unsupported_check" || detail != "protocol_health_check_unsupported" {
			t.Fatalf("probe(%q) = %q %q", raw, status, detail)
		}
	}
}

func TestExternalProfileUnsupportedProbeDoesNotAccumulateHealthFailures(t *testing.T) {
	app := newIntegrationApp(t)
	raw := "hysteria://legacy-auth@192.0.2.10:443"
	activeID, activeKey, _ := app.profileKeyring.GetActiveEncryptionKey()
	_, bikKey, _ := app.profileKeyring.GetActiveBlindIndexKey()
	result, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, protocol, health_failure_count) VALUES('legacy-hysteria', ?, 'hysteria', 2)`, profilestorage.ComputeBlindIndex(bikKey, raw))
	if err != nil {
		t.Fatalf("insert Hysteria profile: %v", err)
	}
	id, _ := result.LastInsertId()
	env, _ := profilestorage.Encrypt([]byte(raw), activeID, activeKey, id)
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id, env); err != nil {
		t.Fatalf("insert Hysteria secret: %v", err)
	}
	if err := app.checkAndPersistKey(t.Context(), id, raw); err != nil {
		t.Fatalf("persist unsupported probe: %v", err)
	}
	var status, detail string
	var failures int
	if err := app.db.QueryRow(`SELECT check_status, COALESCE(check_error, ''), health_failure_count FROM vless_keys WHERE id = ?`, id).Scan(&status, &detail, &failures); err != nil {
		t.Fatalf("read probe result: %v", err)
	}
	if status != "unsupported_check" || detail != "protocol_health_check_unsupported" || failures != 2 {
		t.Fatalf("probe persistence = status=%q detail=%q failures=%d", status, detail, failures)
	}
}
