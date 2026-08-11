package httpapi

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func newTestKeyringForHTTPAPI(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	kr, err := profilestorage.LoadKeyringJSON(data)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	return kr
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_httpapi.db")
	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	schema := `
		CREATE TABLE key_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			color TEXT NOT NULL DEFAULT '#4B5563',
			sort_order INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE external_subscription_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			key_category_id INTEGER,
			key_category TEXT
		);
		CREATE TABLE vless_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			client_display_name TEXT,
			url_blind_index TEXT,
			category_id INTEGER,
			category TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			check_status TEXT NOT NULL DEFAULT 'unknown',
			check_error TEXT,
			last_checked_at DATETIME,
			last_latency_ms INTEGER,
			health_failure_count INTEGER NOT NULL DEFAULT 0,
			key_kind TEXT NOT NULL DEFAULT 'real',
			template_text TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			external_source_id INTEGER,
			external_key_ref TEXT,
			protocol TEXT NOT NULL DEFAULT 'vless',
			profile_schema_version INTEGER NOT NULL DEFAULT 1,
			profile_compatibility TEXT NOT NULL DEFAULT 'full',
			profile_warnings_json TEXT NOT NULL DEFAULT '[]',
			profile_revision INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE vless_key_secrets (
			vless_key_id INTEGER PRIMARY KEY REFERENCES vless_keys(id) ON DELETE CASCADE,
			encrypted_url TEXT
		);
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_assignment_mode TEXT NOT NULL DEFAULT 'all'
		);
		CREATE TABLE user_keys (
			user_id INTEGER NOT NULL,
			key_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, key_id)
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	return db
}

type auditRecord struct {
	eventName  string
	entityType string
	entityID   string
	metadata   map[string]any
}

func TestKeyProfileHandler_FullSuite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	profileRepo := storage.NewProfileRepository(db, kr)
	keyRepo := storage.NewKeyRepository(db, kr)
	svc := keymanagement.NewService(profileRepo, keyRepo, nil)

	var auditLog []auditRecord
	recordAudit := func(r *http.Request, eventName, entityType, entityID string, metadata map[string]any) {
		auditLog = append(auditLog, auditRecord{
			eventName:  eventName,
			entityType: entityType,
			entityID:   entityID,
			metadata:   metadata,
		})
	}

	handler := NewKeyProfileHandler(svc, recordAudit)

	// 1. Get non-existent key -> 404
	reqGet404 := httptest.NewRequest("GET", "/api/v1/keys/999", nil)
	reqGet404.SetPathValue("id", "999")
	recGet404 := httptest.NewRecorder()
	handler.GetKey(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recGet404.Code)
	}
	if cache := recGet404.Header().Get("Cache-Control"); cache != "no-store, private" {
		t.Fatalf("unexpected Cache-Control: %q", cache)
	}

	// 2. Create Key Profile -> 201
	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example.com:8443#TestSS"
	bodyCreate := bytes.NewBufferString(`{"label":"SS Key","status":"active","kind":"real","creation_mode":"raw","raw_uri":"` + ssURI + `"}`)
	reqCreate := httptest.NewRequest("POST", "/api/v1/keys", bodyCreate)
	recCreate := httptest.NewRecorder()
	handler.CreateKeyProfile(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("CreateKeyProfile expected 201, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	if len(auditLog) != 1 || auditLog[0].eventName != "key.create" || auditLog[0].entityID != "1" {
		t.Fatalf("unexpected audit log after create: %#v", auditLog)
	}

	// Verify audit metadata contains NO secret
	for k, v := range auditLog[0].metadata {
		if k == "raw_uri" || k == "password" || v == ssURI {
			t.Fatalf("secret leaked into audit metadata: %s=%v", k, v)
		}
	}

	// 3. Get Key Profile Detail -> 200 with safe headers
	reqGet := httptest.NewRequest("GET", "/api/v1/keys/1", nil)
	reqGet.SetPathValue("id", "1")
	recGet := httptest.NewRecorder()
	handler.GetKey(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GetKey expected 200, got %d", recGet.Code)
	}
	if cache := recGet.Header().Get("Cache-Control"); cache != "no-store, private" {
		t.Fatalf("unexpected Cache-Control: %q", cache)
	}

	// 4. Reveal Key -> 200 with strict no-cache headers
	bodyReveal := bytes.NewBufferString(`{"target":"raw","profile_revision":1}`)
	reqReveal := httptest.NewRequest("POST", "/api/v1/keys/1/reveal", bodyReveal)
	reqReveal.SetPathValue("id", "1")
	recReveal := httptest.NewRecorder()
	handler.RevealKey(recReveal, reqReveal)
	if recReveal.Code != http.StatusOK {
		t.Fatalf("RevealKey expected 200, got %d", recReveal.Code)
	}
	if cache := recReveal.Header().Get("Cache-Control"); cache != "no-store, no-cache, must-revalidate, private" {
		t.Fatalf("unexpected reveal Cache-Control: %q", cache)
	}
	if len(auditLog) != 2 || auditLog[1].eventName != "key.reveal" {
		t.Fatalf("unexpected audit log after reveal: %#v", auditLog)
	}

	// 5. Reveal Key Revision Conflict -> 409 (no audit event)
	auditCountBefore := len(auditLog)
	bodyRevealConflict := bytes.NewBufferString(`{"target":"raw","profile_revision":99}`)
	reqRevealConflict := httptest.NewRequest("POST", "/api/v1/keys/1/reveal", bodyRevealConflict)
	reqRevealConflict.SetPathValue("id", "1")
	recRevealConflict := httptest.NewRecorder()
	handler.RevealKey(recRevealConflict, reqRevealConflict)
	if recRevealConflict.Code != http.StatusConflict {
		t.Fatalf("expected 409 on reveal conflict, got %d", recRevealConflict.Code)
	}
	if len(auditLog) != auditCountBefore {
		t.Fatalf("audit logged on rejected reveal request")
	}

	// 6. Reveal Key Invalid Target -> 400
	bodyRevealInvalid := bytes.NewBufferString(`{"target":"invalid_target","profile_revision":1}`)
	reqRevealInvalid := httptest.NewRequest("POST", "/api/v1/keys/1/reveal", bodyRevealInvalid)
	reqRevealInvalid.SetPathValue("id", "1")
	recRevealInvalid := httptest.NewRecorder()
	handler.RevealKey(recRevealInvalid, reqRevealInvalid)
	if recRevealInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid reveal target, got %d", recRevealInvalid.Code)
	}

	// 7. Create Key Profile with Unknown JSON Field -> 400
	bodyUnknownField := bytes.NewBufferString(`{"label":"Fail","status":"active","kind":"real","unknown_field":"bad"}`)
	reqUnknownField := httptest.NewRequest("POST", "/api/v1/keys", bodyUnknownField)
	recUnknownField := httptest.NewRecorder()
	handler.CreateKeyProfile(recUnknownField, reqUnknownField)
	if recUnknownField.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unknown field, got %d", recUnknownField.Code)
	}

	// 8. Create Key Profile with Trailing JSON -> 400
	bodyTrailingJSON := bytes.NewBufferString(`{"label":"Fail","status":"active","kind":"real","creation_mode":"raw","raw_uri":"` + ssURI + `"} extra_junk`)
	reqTrailingJSON := httptest.NewRequest("POST", "/api/v1/keys", bodyTrailingJSON)
	recTrailingJSON := httptest.NewRecorder()
	handler.CreateKeyProfile(recTrailingJSON, reqTrailingJSON)
	if recTrailingJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on trailing JSON, got %d", recTrailingJSON.Code)
	}

	// 9. Update Key Profile -> 200
	bodyUpdate := bytes.NewBufferString(`{"profile_revision":1,"label":"Updated SS","status":"active","kind":"real","patch_mode":"raw","raw_uri":"` + ssURI + `"}`)
	reqUpdate := httptest.NewRequest("PUT", "/api/v1/keys/1", bodyUpdate)
	reqUpdate.SetPathValue("id", "1")
	recUpdate := httptest.NewRecorder()
	handler.UpdateKeyProfile(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("UpdateKeyProfile expected 200, got %d", recUpdate.Code)
	}

	// 10. Source-owned client-name-only update is allowed without rewriting the secret.
	if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = 100 WHERE id = 1`); err != nil {
		t.Fatalf("set external_source_id: %v", err)
	}
	var sourceEnvelopeBefore string
	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = 1`).Scan(&sourceEnvelopeBefore); err != nil {
		t.Fatal(err)
	}
	bodySourceName := bytes.NewBufferString(`{"profile_revision":2,"label":"Updated SS","client_display_name":"Source override","status":"active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`)
	reqSourceName := httptest.NewRequest("PUT", "/api/v1/keys/1", bodySourceName)
	reqSourceName.SetPathValue("id", "1")
	recSourceName := httptest.NewRecorder()
	handler.UpdateKeyProfile(recSourceName, reqSourceName)
	if recSourceName.Code != http.StatusOK || !strings.Contains(recSourceName.Body.String(), `"client_display_name":"Source override"`) || !strings.Contains(recSourceName.Body.String(), `"client_display_name_overridden":true`) {
		t.Fatalf("source client-name update status=%d body=%s", recSourceName.Code, recSourceName.Body.String())
	}
	var sourceEnvelopeAfter string
	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = 1`).Scan(&sourceEnvelopeAfter); err != nil || sourceEnvelopeAfter != sourceEnvelopeBefore {
		t.Fatalf("source metadata update rewrote secret=%v err=%v", sourceEnvelopeAfter != sourceEnvelopeBefore, err)
	}

	// Source-owned delivery status is local metadata and supports both directions.
	bodySourceInactive := bytes.NewBufferString(`{"profile_revision":3,"label":"Updated SS","status":"non-active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`)
	reqSourceInactive := httptest.NewRequest("PUT", "/api/v1/keys/1", bodySourceInactive)
	reqSourceInactive.SetPathValue("id", "1")
	recSourceInactive := httptest.NewRecorder()
	handler.UpdateKeyProfile(recSourceInactive, reqSourceInactive)
	if recSourceInactive.Code != http.StatusOK || !strings.Contains(recSourceInactive.Body.String(), `"status":"non-active"`) || !strings.Contains(recSourceInactive.Body.String(), `"client_display_name":"Source override"`) {
		t.Fatalf("source deactivate status=%d body=%s", recSourceInactive.Code, recSourceInactive.Body.String())
	}
	bodySourceActive := bytes.NewBufferString(`{"profile_revision":4,"label":"Updated SS","status":"active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`)
	reqSourceActive := httptest.NewRequest("PUT", "/api/v1/keys/1", bodySourceActive)
	reqSourceActive.SetPathValue("id", "1")
	recSourceActive := httptest.NewRecorder()
	handler.UpdateKeyProfile(recSourceActive, reqSourceActive)
	if recSourceActive.Code != http.StatusOK || !strings.Contains(recSourceActive.Body.String(), `"status":"active"`) {
		t.Fatalf("source reactivate status=%d body=%s", recSourceActive.Code, recSourceActive.Body.String())
	}

	// Full configuration and source label mutations remain forbidden.
	bodyUpdateExt := bytes.NewBufferString(`{"profile_revision":5,"label":"Updated SS","client_display_name":"Source override","status":"active","kind":"real","patch_mode":"raw","raw_uri":"` + ssURI + `"}`)
	reqUpdateExt := httptest.NewRequest("PUT", "/api/v1/keys/1", bodyUpdateExt)
	reqUpdateExt.SetPathValue("id", "1")
	recUpdateExt := httptest.NewRecorder()
	handler.UpdateKeyProfile(recUpdateExt, reqUpdateExt)
	if recUpdateExt.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on source-owned update, got %d", recUpdateExt.Code)
	}
	bodyRenameExt := bytes.NewBufferString(`{"profile_revision":5,"label":"Update Ext","client_display_name":"Source override","status":"active","kind":"real","patch_mode":"structured"}`)
	reqRenameExt := httptest.NewRequest("PUT", "/api/v1/keys/1", bodyRenameExt)
	reqRenameExt.SetPathValue("id", "1")
	recRenameExt := httptest.NewRecorder()
	handler.UpdateKeyProfile(recRenameExt, reqRenameExt)
	if recRenameExt.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on source label update, got %d", recRenameExt.Code)
	}

	bodyResetSourceName := bytes.NewBufferString(`{"profile_revision":5,"label":"Updated SS","client_display_name":"","status":"active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`)
	reqResetSourceName := httptest.NewRequest("PUT", "/api/v1/keys/1", bodyResetSourceName)
	reqResetSourceName.SetPathValue("id", "1")
	recResetSourceName := httptest.NewRecorder()
	handler.UpdateKeyProfile(recResetSourceName, reqResetSourceName)
	if recResetSourceName.Code != http.StatusOK || !strings.Contains(recResetSourceName.Body.String(), `"client_display_name_overridden":false`) {
		t.Fatalf("source client-name reset status=%d body=%s", recResetSourceName.Code, recResetSourceName.Body.String())
	}
	var storedSourceOverride sql.NullString
	if err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = 1`).Scan(&storedSourceOverride); err != nil || storedSourceOverride.Valid {
		t.Fatalf("source reset override=%#v err=%v", storedSourceOverride, err)
	}
	if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = NULL WHERE id = 1`); err != nil {
		t.Fatalf("reset external_source_id: %v", err)
	}

	// 11. Clone Key Profile -> 201
	bodyClone := bytes.NewBufferString(`{"expected_profile_revision":6,"new_label":"Cloned SS"}`)
	reqClone := httptest.NewRequest("POST", "/api/v1/keys/1/clone", bodyClone)
	reqClone.SetPathValue("id", "1")
	recClone := httptest.NewRecorder()
	handler.CloneKey(recClone, reqClone)
	if recClone.Code != http.StatusCreated {
		t.Fatalf("CloneKey expected 201, got %d", recClone.Code)
	}

	// 12. Storage Integrity Error Mapping -> 500
	if _, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = 1`); err != nil {
		t.Fatalf("delete secrets row: %v", err)
	}
	bodyCloneIntegrity := bytes.NewBufferString(`{"expected_profile_revision":6,"new_label":"Clone Fail"}`)
	reqCloneIntegrity := httptest.NewRequest("POST", "/api/v1/keys/1/clone", bodyCloneIntegrity)
	reqCloneIntegrity.SetPathValue("id", "1")
	recCloneIntegrity := httptest.NewRecorder()
	handler.CloneKey(recCloneIntegrity, reqCloneIntegrity)
	if recCloneIntegrity.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on storage integrity failure, got %d", recCloneIntegrity.Code)
	}
	if strings.Contains(recCloneIntegrity.Body.String(), ssURI) {
		t.Fatal("clone integrity response leaked decrypted credential material")
	}

	reqGetIntegrity := httptest.NewRequest("GET", "/api/v1/keys/1", nil)
	reqGetIntegrity.SetPathValue("id", "1")
	recGetIntegrity := httptest.NewRecorder()
	handler.GetKey(recGetIntegrity, reqGetIntegrity)
	if recGetIntegrity.Code != http.StatusInternalServerError || !strings.Contains(recGetIntegrity.Body.String(), `"code":"storage_integrity_error"`) {
		t.Fatalf("detail integrity mapping changed: status=%d body=%s", recGetIntegrity.Code, recGetIntegrity.Body.String())
	}
	if strings.Contains(recGetIntegrity.Body.String(), ssURI) {
		t.Fatal("detail integrity response leaked decrypted credential material")
	}

	auditCountBefore = len(auditLog)
	bodyRevealIntegrity := bytes.NewBufferString(`{"target":"raw","profile_revision":2}`)
	reqRevealIntegrity := httptest.NewRequest("POST", "/api/v1/keys/1/reveal", bodyRevealIntegrity)
	reqRevealIntegrity.SetPathValue("id", "1")
	recRevealIntegrity := httptest.NewRecorder()
	handler.RevealKey(recRevealIntegrity, reqRevealIntegrity)
	if recRevealIntegrity.Code != http.StatusInternalServerError {
		t.Fatalf("reveal integrity mapping changed: status=%d body=%s", recRevealIntegrity.Code, recRevealIntegrity.Body.String())
	}
	if len(auditLog) != auditCountBefore || strings.Contains(recRevealIntegrity.Body.String(), ssURI) {
		t.Fatalf("rejected reveal was audited or leaked secret: audit=%#v body=%s", auditLog, recRevealIntegrity.Body.String())
	}

	if _, err := db.Exec(`CREATE TRIGGER reject_profile_create BEFORE INSERT ON vless_keys BEGIN SELECT RAISE(ABORT, 'forced profile create failure'); END`); err != nil {
		t.Fatalf("create profile failure trigger: %v", err)
	}
	bodyCreateConflict := bytes.NewBufferString(`{"label":"Conflict","status":"active","kind":"real","creation_mode":"raw","raw_uri":"` + ssURI + `"}`)
	reqCreateConflict := httptest.NewRequest("POST", "/api/v1/keys", bodyCreateConflict)
	recCreateConflict := httptest.NewRecorder()
	handler.CreateKeyProfile(recCreateConflict, reqCreateConflict)
	if recCreateConflict.Code != http.StatusConflict || !strings.Contains(recCreateConflict.Body.String(), `"code":"create_failed"`) {
		t.Fatalf("typed create failure mapping changed: status=%d body=%s", recCreateConflict.Code, recCreateConflict.Body.String())
	}
	if len(auditLog) != auditCountBefore || strings.Contains(recCreateConflict.Body.String(), ssURI) {
		t.Fatalf("failed create was audited or leaked secret: audit=%#v body=%s", auditLog, recCreateConflict.Body.String())
	}

	// 13. Get Editor Schema -> 200
	reqSchema := httptest.NewRequest("GET", "/api/v1/key-editor-schema", nil)
	recSchema := httptest.NewRecorder()
	handler.GetKeyEditorSchema(recSchema, reqSchema)
	if recSchema.Code != http.StatusOK {
		t.Fatalf("GetKeyEditorSchema expected 200, got %d", recSchema.Code)
	}
}
