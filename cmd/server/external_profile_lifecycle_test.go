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
	requireFixturef(t, err == nil, "initial alias sync: result=%#v err=%v", first, err)
	requireFixturef(t, first.Imported == 1, "initial alias sync: result=%#v err=%v", first, err)

	var id int64
	{
		err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&id)
		requireFixturef(t, err == nil, "read initial row: %v", err)
	}

	canonicalScheme := "hysteria2://auth@example.com:443#Alias"
	refreshed, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, canonicalScheme))
	requireFixturef(t, err == nil, "warning-only refresh result=%#v err=%v", refreshed, err)
	requireFixturef(t, refreshed.Counts.Updated == 1, "warning-only refresh result=%#v err=%v", refreshed, err)

	var currentID int64
	var warningsJSON string
	{
		err := app.db.QueryRow(`SELECT id, profile_warnings_json FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &warningsJSON)
		requireFixturef(t, err == nil, "read refreshed row: %v", err)
	}

	requireFixturef(t, currentID == id, "warning-only refresh replaced row or retained warning: id=%d/%d warnings=%q", id, currentID, warningsJSON)
	requireFixturef(t, warningsJSON == "[]", "warning-only refresh replaced row or retained warning: id=%d/%d warnings=%q", id, currentID, warningsJSON)

}

func TestExternalProfileSourceSpecificWarningsAndLifecycle(t *testing.T) {
	app := newIntegrationApp(t)
	firstSource := seedExternalProfileSource(t, app, "https://provider.example/warning-source-a")
	secondSource := seedExternalProfileSource(t, app, "https://provider.example/warning-source-b")
	aliasRaw := "hy2://source-auth@example.com:443#Alias"
	canonicalRaw := "hysteria2://source-auth@example.com:443#Canonical"
	{
		_, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, aliasRaw))
		requireFixturef(t, err == nil, "sync alias source: %v", err)
	}

	{
		_, err := app.syncExternalSource(secondSource, parseExternalTestBody(t, canonicalRaw))
		requireFixturef(t, err == nil, "sync canonical source: %v", err)
	}

	var firstID, secondID int64
	var firstWarnings, secondWarnings, secondCreated, secondFingerprint string
	{
		err := app.db.QueryRow(`SELECT id, profile_warnings_json FROM vless_keys WHERE external_source_id = ?`, firstSource).Scan(&firstID, &firstWarnings)
		requireFixturef(t, err == nil, "read alias source: %v", err)
	}

	{
		err := app.db.QueryRow(`SELECT id, profile_warnings_json, created_at, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, secondSource).Scan(&secondID, &secondWarnings, &secondCreated, &secondFingerprint)
		requireFixturef(t, err == nil, "read canonical source: %v", err)
	}

	requireFixturef(t, firstID != secondID, "source warning state collapsed: first=%d %q second=%d %q", firstID, firstWarnings, secondID, secondWarnings)
	requireFixturef(t, strings.Contains(firstWarnings, profiles.WarningNonCanonicalScheme), "source warning state collapsed: first=%d %q second=%d %q", firstID, firstWarnings, secondID, secondWarnings)
	requireFixturef(t, secondWarnings == "[]", "source warning state collapsed: first=%d %q second=%d %q", firstID, firstWarnings, secondID, secondWarnings)

	// Source A becomes byte-identical to Source B. It updates only its own row.
	{
		_, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, canonicalRaw))
		requireFixturef(t, err == nil, "canonicalize source A: %v", err)
	}

	var encURL, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter string
	{
		err := app.db.QueryRow(`SELECT s.encrypted_url, k.profile_warnings_json, k.created_at, k.profile_fingerprint FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ? AND k.external_source_id = ?`, secondID, secondSource).Scan(&encURL, &secondWarningsAfter, &secondCreatedAfter, &secondFingerprintAfter)
		requireFixturef(t, err == nil, "read unaffected source B: %v", err)
	}

	secB, _ := profilestorage.Decrypt(encURL, app.profileKeyring, secondID)
	secondRawAfter := secB.Reveal()
	requireFixturef(t, secondRawAfter == canonicalRaw, "Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)
	requireFixturef(t, secondWarningsAfter == secondWarnings, "Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)
	requireFixturef(t, secondCreatedAfter == secondCreated, "Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)
	requireFixturef(t, secondFingerprintAfter == secondFingerprint, "Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)

	var identicalRows int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE profile_fingerprint = ?`, secondFingerprint).Scan(&identicalRows)
		requireFixturef(t, err == nil, "byte-identical cross-source rows=%d err=%v", identicalRows, err)
		requireFixturef(t, identicalRows == 2, "byte-identical cross-source rows=%d err=%v", identicalRows, err)
	}

}

func TestExternalProfileCreateRollsBackOnPersistenceFailure(t *testing.T) {
	app := newIntegrationApp(t)
	{
		_, err := app.db.Exec(`CREATE TRIGGER reject_profile_insert BEFORE INSERT ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic profile failure'); END`)
		requireFixturef(t, err == nil, "create failure trigger: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"name": "Rollback", "category": "general", "key_category": "", "key_insert_mode": "bottom",
		"source_url": "https://provider.example/rollback-profile", "enabled": true, "raw_body": externalTestHY2,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.apiV1CreateSource(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusInternalServerError, "rollback response status=%d body=%s", recorder.Code, recorder.Body.String())

	var sources, keys int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE source_url = 'https://provider.example/rollback-profile'`).Scan(&sources)
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE protocol = 'hysteria2'`).Scan(&keys)
	requireFixturef(t, sources == 0, "transactional rollback failed: sources=%d keys=%d", sources, keys)
	requireFixturef(t, keys == 0, "transactional rollback failed: sources=%d keys=%d", sources, keys)

}

func TestExternalProfileSynchronizationRollsBackOnPersistenceFailure(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/sync-rollback")
	{
		_, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, externalTestSS))
		requireFixturef(t, err == nil, "initial sync: %v", err)
	}

	var originalID int64
	{
		err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&originalID)
		requireFixturef(t, err == nil, "read original row: %v", err)
	}

	{
		_, err := app.db.Exec(`CREATE TRIGGER reject_refresh_update BEFORE UPDATE ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic refresh failure'); END`)
		requireFixturef(t, err == nil, "create refresh trigger: %v", err)
	}

	changed := strings.Replace(externalTestSS, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1)
	{
		_, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, changed))
		requireFixture(t, err != nil, "refresh persistence failure was ignored")
	}

	var currentID int64
	var encURL string
	{
		err := app.db.QueryRow(`SELECT k.id, s.encrypted_url FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ?`, sourceID).Scan(&currentID, &encURL)
		requireFixturef(t, err == nil, "read retained row: %v", err)
	}

	secRetained, _ := profilestorage.Decrypt(encURL, app.profileKeyring, currentID)
	currentRaw := secRetained.Reveal()
	requireFixturef(t, currentID == originalID, "failed refresh partially mutated source: id=%d/%d raw_equal=%v", originalID, currentID, currentRaw == externalTestSS)
	requireFixturef(t, currentRaw == externalTestSS, "failed refresh partially mutated source: id=%d/%d raw_equal=%v", originalID, currentID, currentRaw == externalTestSS)

}

func TestExternalProfileProbePolicyMarksOnlyUnimplementedProtocolsUnsupported(t *testing.T) {
	for _, raw := range []string{
		"tuic://33333333-3333-4333-8333-333333333333:password@192.0.2.11:443",
		"hysteria://legacy-auth@192.0.2.12:443",
	} {
		status, detail, _ := checkConfigurationAvailability(raw)
		requireFixturef(t, status == "unsupported_check", "probe(%q) = %q %q", raw, status, detail)
		requireFixturef(t, detail == "protocol_health_check_unsupported", "probe(%q) = %q %q", raw, status, detail)

	}
}

func TestExternalProfileUnsupportedProbeDoesNotAccumulateHealthFailures(t *testing.T) {
	app := newIntegrationApp(t)
	raw := "hysteria://legacy-auth@192.0.2.10:443"
	activeID, activeKey, _ := app.profileKeyring.GetActiveEncryptionKey()
	_, bikKey, _ := app.profileKeyring.GetActiveBlindIndexKey()
	result, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, protocol, health_failure_count) VALUES('legacy-hysteria', ?, 'hysteria', 2)`, profilestorage.ComputeBlindIndex(bikKey, raw))
	requireFixturef(t, err == nil, "insert Hysteria profile: %v", err)

	id, _ := result.LastInsertId()
	env, _ := profilestorage.Encrypt([]byte(raw), activeID, activeKey, id)
	{
		_, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id, env)
		requireFixturef(t, err == nil, "insert Hysteria secret: %v", err)
	}

	{
		err := app.checkAndPersistKey(t.Context(), id, raw)
		requireFixturef(t, err == nil, "persist unsupported probe: %v", err)
	}

	var status, detail string
	var failures int
	{
		err := app.db.QueryRow(`SELECT check_status, COALESCE(check_error, ''), health_failure_count FROM vless_keys WHERE id = ?`, id).Scan(&status, &detail, &failures)
		requireFixturef(t, err == nil, "read probe result: %v", err)
	}

	requireFixturef(t, status == "unsupported_check", "probe persistence = status=%q detail=%q failures=%d", status, detail, failures)
	requireFixturef(t, detail == "protocol_health_check_unsupported", "probe persistence = status=%q detail=%q failures=%d", status, detail, failures)
	requireFixturef(t, failures == 2, "probe persistence = status=%q detail=%q failures=%d", status, detail, failures)

}
