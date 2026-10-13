package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExternalProfilePreviewApplyIsAuthoritativeAndSecretSafe(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")
	requestBody := fmt.Sprintf(`{"name":"Profiles","category":"general","key_category":"","key_insert_mode":"bottom","source_url":"https://provider.example/manual","enabled":true,"raw_body":%q,"selected_item_refs":["ir1_tampered"]}`, externalTestTUIC)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	recorder := httptest.NewRecorder()
	app.apiV1CreateSource(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusBadRequest, "tampered apply response: status=%d body=%s", recorder.Code, recorder.Body.String())
	requireFixturef(t, strings.Contains(recorder.Body.String(), "source_selection_invalid"), "tampered apply response: status=%d body=%s", recorder.Code, recorder.Body.String())

	var sources int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE source_url = 'https://provider.example/manual'`).Scan(&sources)
		requireFixturef(t, err == nil, "tampered apply persisted a source: count=%d err=%v", sources, err)
		requireFixturef(t, sources == 0, "tampered apply persisted a source: count=%d err=%v", sources, err)
	}

	for _, secret := range []string{"33333333-3333-4333-8333-333333333333", "tuic-password", "tuic://"} {
		requireFixturef(t, !(strings.Contains(recorder.Body.String(), secret)), "error response exposed %q: %s", secret, recorder.Body.String())

	}
}

func TestExternalProfilePreviewAndAuditNeverExposeSecrets(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")
	previewPayload, _ := json.Marshal(map[string]any{"source_url": "https://provider.example/preview", "raw_body": externalTestSS + "\n" + externalTestTUIC})
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sources/preview", strings.NewReader(string(previewPayload)))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRequest.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	previewRecorder := httptest.NewRecorder()
	app.apiV1PreviewSource(previewRecorder, previewRequest)
	requireFixturef(t, previewRecorder.Code == http.StatusOK, "preview status=%d body=%s", previewRecorder.Code, previewRecorder.Body.String())

	var preview map[string]any
	{
		err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview)
		requireFixturef(t, err == nil, "decode preview: %v", err)
	}

	items, ok := preview["keys"].([]any)
	requireFixturef(t, ok, "safe preview items = %#v", preview["keys"])
	requireFixturef(t, len(items) == 2, "safe preview items = %#v", preview["keys"])

	createPayload, _ := json.Marshal(map[string]any{
		"name": "Safe audit", "category": "general", "key_category": "", "key_insert_mode": "bottom",
		"source_url": "https://provider.example/audit", "enabled": true, "raw_body": externalTestSS,
	})
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(string(createPayload)))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	createRecorder := httptest.NewRecorder()
	app.apiV1CreateSource(createRecorder, createRequest)
	requireFixturef(t, createRecorder.Code == http.StatusCreated, "create status=%d body=%s", createRecorder.Code, createRecorder.Body.String())

	var auditMetadata string
	{
		err := app.db.QueryRow(`SELECT metadata_json FROM audit_events WHERE action = 'external_source.create' ORDER BY id DESC LIMIT 1`).Scan(&auditMetadata)
		requireFixturef(t, err == nil, "read audit event: %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	listRecorder := httptest.NewRecorder()
	app.keys().queries.ListKeys(listRecorder, listRequest)
	requireFixturef(t, listRecorder.Code == http.StatusOK, "safe key listing status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	requireFixturef(t, strings.Contains(listRecorder.Body.String(), `"protocol":"shadowsocks"`), "safe key listing status=%d body=%s", listRecorder.Code, listRecorder.Body.String())

	combined := previewRecorder.Body.String() + createRecorder.Body.String() + listRecorder.Body.String() + auditMetadata
	for _, secret := range []string{"shadow-password", "plugin-secret", "33333333-3333-4333-8333-333333333333", "tuic-password", "ss://", "tuic://"} {
		requireFixturef(t, !(strings.Contains(combined, secret)), "preview/apply/audit exposed %q: %s", secret, combined)

	}
	var unsafeMetadataRows int
	{
		err := app.db.QueryRow(`
		SELECT
		  (SELECT COUNT(*) FROM vless_keys WHERE profile_warnings_json LIKE '%shadow-password%' OR profile_warnings_json LIKE '%plugin-secret%') +
		  (SELECT COUNT(*) FROM audit_events WHERE metadata_json LIKE '%shadow-password%' OR metadata_json LIKE '%plugin-secret%') +
		  (SELECT COUNT(*) FROM source_sync_runs WHERE result_counts_json LIKE '%shadow-password%' OR error_message LIKE '%shadow-password%')
	`).Scan(&unsafeMetadataRows)
		requireFixturef(t, err == nil, "raw URI was duplicated into metadata/audit/sync storage: rows=%d err=%v", unsafeMetadataRows, err)
		requireFixturef(t, unsafeMetadataRows == 0, "raw URI was duplicated into metadata/audit/sync storage: rows=%d err=%v", unsafeMetadataRows, err)
	}

}

func TestExternalProfileErrorsDoNotContainSecrets(t *testing.T) {
	secret := "do-not-log-profile-secret"
	parsed, err := sources.ParseBody("tuic://not-a-uuid:"+secret+"@example.com:70000", [][]byte{externalTestFingerprintKey})
	requireFixturef(t, err == nil, "partial parser should return an item: %v", err)

	formatted := fmt.Sprintf("%v %#v", parsed.Items, parsed.Items)
	requireFixturef(t, !(strings.Contains(formatted, secret)), "secret escaped through error/formatting: %s", formatted)
	requireFixturef(t, !(strings.Contains(parsed.Items[0].ErrorCode, secret)), "secret escaped through error/formatting: %s", formatted)

	requireFixture(t, profiles.ErrorCodeOf(fmt.Errorf("wrapped: %w", &profiles.Error{Code: profiles.ErrorInvalidProfile, Protocol: profiles.ProtocolTUIC, Field: "password"})) != "", "profile error wrapping regression")

	internalKey := parseExternalTestBody(t, externalTestTUIC).Keys[0]
	{
		_, err := json.Marshal(internalKey)
		requireFixturef(t, err != nil, "internal persistence DTO JSON boundary error = %v", err)
		requireFixturef(t, !(strings.Contains(err.Error(), secret)), "internal persistence DTO JSON boundary error = %v", err)
	}

}

func TestExternalProfileLoggingStyleFormattingIsSecretSafe(t *testing.T) {
	profile, err := profiles.Parse(externalTestTUIC)
	requireFixturef(t, err == nil, "parse TUIC: %v", err)

	parsed := parseExternalTestBody(t, externalTestTUIC)
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	log.Printf("profile=%s value=%v detail=%+v go=%#v", profile, profile, profile, profile)
	log.Printf("import_items=%v", parsed.Items)
	log.Printf("internal_parse=%s value=%v detail=%+v go=%#v key=%#v", parsed, parsed, parsed, parsed, parsed.Keys[0])
	logged := output.String()
	for _, secret := range []string{"33333333-3333-4333-8333-333333333333", "tuic-password", "tuic://"} {
		requireFixturef(t, !(strings.Contains(logged, secret)), "logging exposed %q: %s", secret, logged)

	}
}
