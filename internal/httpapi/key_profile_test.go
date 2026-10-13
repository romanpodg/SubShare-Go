package httpapi

import (
	"database/sql"
	"net/http"
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
	requireFixturef(t, err == nil, "generate keyring: %v", err)

	kr, err := profilestorage.LoadKeyringJSON(data)
	requireFixturef(t, err == nil, "load keyring: %v", err)

	return kr
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	requireFixturef(t, err == nil, "open db: %v", err)

	{
		_, err := db.Exec(`PRAGMA foreign_keys = ON`)
		requireFixturef(t, err == nil, "enable foreign keys: %v", err)
	}

	{
		err := storage.MigrateWithKeyring(db, newTestKeyringForHTTPAPI(t))
		requireFixturef(t, err == nil, "migrate: %v", err)
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
	svc := keymanagement.NewService(storage.NewRepository(db, kr), nil)

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
	recGet404 := serveProfileRequest(handler.GetKey, profileRequest{method: "GET", path: "/api/v1/keys/999", id: "999", body: ""})
	requireFixturef(t, recGet404.Code == http.StatusNotFound, "expected 404, got %d", recGet404.Code)

	{
		cache := recGet404.Header().Get("Cache-Control")
		requireFixturef(t, cache == "no-store, private", "unexpected Cache-Control: %q", cache)
	}

	// 2. Create Key Profile -> 201
	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example.com:8443#TestSS"
	recCreate := serveProfileRequest(handler.CreateKeyProfile, profileRequest{method: "POST", path: "/api/v1/keys", id: "", body: `{"label":"SS Key","status":"active","kind":"real","creation_mode":"raw","raw_uri":"` + ssURI + `"}`})
	requireFixturef(t, recCreate.Code == http.StatusCreated, "CreateKeyProfile expected 201, got %d: %s", recCreate.Code, recCreate.Body.String())

	requireFixturef(t, len(auditLog) == 1, "unexpected audit log after create: %#v", auditLog)
	requireFixturef(t, auditLog[0].eventName == "key.create", "unexpected audit log after create: %#v", auditLog)
	requireFixturef(t, auditLog[0].entityID == "1", "unexpected audit log after create: %#v", auditLog)

	// Verify audit metadata contains NO secret
	for k, v := range auditLog[0].metadata {
		requireFixturef(t, k != "raw_uri", "secret leaked into audit metadata: %s=%v", k, v)
		requireFixturef(t, k != "password", "secret leaked into audit metadata: %s=%v", k, v)
		requireFixturef(t, v != ssURI, "secret leaked into audit metadata: %s=%v", k, v)

	}

	// 3. Get Key Profile Detail -> 200 with safe headers
	recGet := serveProfileRequest(handler.GetKey, profileRequest{method: "GET", path: "/api/v1/keys/1", id: "1", body: ""})
	requireFixturef(t, recGet.Code == http.StatusOK, "GetKey expected 200, got %d", recGet.Code)

	{
		cache := recGet.Header().Get("Cache-Control")
		requireFixturef(t, cache == "no-store, private", "unexpected Cache-Control: %q", cache)
	}

	// 4. Reveal Key -> 200 with strict no-cache headers
	recReveal := serveProfileRequest(handler.RevealKey, profileRequest{method: "POST", path: "/api/v1/keys/1/reveal", id: "1", body: `{"target":"raw","profile_revision":1}`})
	requireFixturef(t, recReveal.Code == http.StatusOK, "RevealKey expected 200, got %d", recReveal.Code)

	{
		cache := recReveal.Header().Get("Cache-Control")
		requireFixturef(t, cache == "no-store, no-cache, must-revalidate, private", "unexpected reveal Cache-Control: %q", cache)
	}

	requireFixturef(t, len(auditLog) == 2, "unexpected audit log after reveal: %#v", auditLog)
	requireFixturef(t, auditLog[1].eventName == "key.reveal", "unexpected audit log after reveal: %#v", auditLog)

	// 5. Reveal Key Revision Conflict -> 409 (no audit event)
	auditCountBefore := len(auditLog)
	recRevealConflict := serveProfileRequest(handler.RevealKey, profileRequest{method: "POST", path: "/api/v1/keys/1/reveal", id: "1", body: `{"target":"raw","profile_revision":99}`})
	requireFixturef(t, recRevealConflict.Code == http.StatusConflict, "expected 409 on reveal conflict, got %d", recRevealConflict.Code)

	requireFixturef(t, len(auditLog) == auditCountBefore, "audit logged on rejected reveal request")

	// 6. Reveal Key Invalid Target -> 400
	recRevealInvalid := serveProfileRequest(handler.RevealKey, profileRequest{method: "POST", path: "/api/v1/keys/1/reveal", id: "1", body: `{"target":"invalid_target","profile_revision":1}`})
	requireFixturef(t, recRevealInvalid.Code == http.StatusBadRequest, "expected 400 on invalid reveal target, got %d", recRevealInvalid.Code)

	// 7. Create Key Profile with Unknown JSON Field -> 400
	recUnknownField := serveProfileRequest(handler.CreateKeyProfile, profileRequest{method: "POST", path: "/api/v1/keys", id: "", body: `{"label":"Fail","status":"active","kind":"real","unknown_field":"bad"}`})
	requireFixturef(t, recUnknownField.Code == http.StatusBadRequest, "expected 400 on unknown field, got %d", recUnknownField.Code)

	// 8. Create Key Profile with Trailing JSON -> 400
	recTrailingJSON := serveProfileRequest(handler.CreateKeyProfile, profileRequest{method: "POST", path: "/api/v1/keys", id: "", body: `{"label":"Fail","status":"active","kind":"real","creation_mode":"raw","raw_uri":"` + ssURI + `"} extra_junk`})
	requireFixturef(t, recTrailingJSON.Code == http.StatusBadRequest, "expected 400 on trailing JSON, got %d", recTrailingJSON.Code)

	// 9. Update Key Profile -> 200
	recUpdate := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":1,"label":"Updated SS","status":"active","kind":"real","patch_mode":"raw","raw_uri":"` + ssURI + `"}`})
	requireFixturef(t, recUpdate.Code == http.StatusOK, "UpdateKeyProfile expected 200, got %d", recUpdate.Code)

	// 10. Source-owned client-name-only update is allowed without rewriting the secret.
	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 100 WHERE id = 1`)
		requireFixturef(t, err == nil, "set external_source_id: %v", err)
	}

	var sourceEnvelopeBefore string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = 1`).Scan(&sourceEnvelopeBefore)
		requireFixture(t, err == nil, err)
	}

	recSourceName := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":2,"label":"Updated SS","client_display_name":"Source override","status":"active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`})
	requireFixturef(t, recSourceName.Code == http.StatusOK, "source client-name update status=%d body=%s", recSourceName.Code, recSourceName.Body.String())
	requireFixturef(t, strings.Contains(recSourceName.Body.String(), `"client_display_name":"Source override"`), "source client-name update status=%d body=%s", recSourceName.Code, recSourceName.Body.String())
	requireFixturef(t, strings.Contains(recSourceName.Body.String(), `"client_display_name_overridden":true`), "source client-name update status=%d body=%s", recSourceName.Code, recSourceName.Body.String())

	var sourceEnvelopeAfter string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = 1`).Scan(&sourceEnvelopeAfter)
		requireFixturef(t, err == nil, "source metadata update rewrote secret=%v err=%v", sourceEnvelopeAfter != sourceEnvelopeBefore, err)
		requireFixturef(t, sourceEnvelopeAfter == sourceEnvelopeBefore, "source metadata update rewrote secret=%v err=%v", sourceEnvelopeAfter != sourceEnvelopeBefore, err)
	}

	// Source-owned delivery status is local metadata and supports both directions.
	recSourceInactive := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":3,"label":"Updated SS","status":"non-active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`})
	requireFixturef(t, recSourceInactive.Code == http.StatusOK, "source deactivate status=%d body=%s", recSourceInactive.Code, recSourceInactive.Body.String())
	requireFixturef(t, strings.Contains(recSourceInactive.Body.String(), `"status":"non-active"`), "source deactivate status=%d body=%s", recSourceInactive.Code, recSourceInactive.Body.String())
	requireFixturef(t, strings.Contains(recSourceInactive.Body.String(), `"client_display_name":"Source override"`), "source deactivate status=%d body=%s", recSourceInactive.Code, recSourceInactive.Body.String())

	recSourceActive := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":4,"label":"Updated SS","status":"active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`})
	requireFixturef(t, recSourceActive.Code == http.StatusOK, "source reactivate status=%d body=%s", recSourceActive.Code, recSourceActive.Body.String())
	requireFixturef(t, strings.Contains(recSourceActive.Body.String(), `"status":"active"`), "source reactivate status=%d body=%s", recSourceActive.Code, recSourceActive.Body.String())

	// Full configuration and source label mutations remain forbidden.
	recUpdateExt := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":5,"label":"Updated SS","client_display_name":"Source override","status":"active","kind":"real","patch_mode":"raw","raw_uri":"` + ssURI + `"}`})
	requireFixturef(t, recUpdateExt.Code == http.StatusForbidden, "expected 403 on source-owned update, got %d", recUpdateExt.Code)

	recRenameExt := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":5,"label":"Update Ext","client_display_name":"Source override","status":"active","kind":"real","patch_mode":"structured"}`})
	requireFixturef(t, recRenameExt.Code == http.StatusForbidden, "expected 403 on source label update, got %d", recRenameExt.Code)

	recResetSourceName := serveProfileRequest(handler.UpdateKeyProfile, profileRequest{method: "PUT", path: "/api/v1/keys/1", id: "1", body: `{"profile_revision":5,"label":"Updated SS","client_display_name":"","status":"active","kind":"real","category":"","template_text":"","patch_mode":"structured"}`})
	requireFixturef(t, recResetSourceName.Code == http.StatusOK, "source client-name reset status=%d body=%s", recResetSourceName.Code, recResetSourceName.Body.String())
	requireFixturef(t, strings.Contains(recResetSourceName.Body.String(), `"client_display_name_overridden":false`), "source client-name reset status=%d body=%s", recResetSourceName.Code, recResetSourceName.Body.String())

	var storedSourceOverride sql.NullString
	{
		err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = 1`).Scan(&storedSourceOverride)
		requireFixturef(t, err == nil, "source reset override=%#v err=%v", storedSourceOverride, err)
		requireFixturef(t, !(storedSourceOverride.Valid), "source reset override=%#v err=%v", storedSourceOverride, err)
	}

	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = NULL WHERE id = 1`)
		requireFixturef(t, err == nil, "reset external_source_id: %v", err)
	}

	// 11. Clone Key Profile -> 201
	recClone := serveProfileRequest(handler.CloneKey, profileRequest{method: "POST", path: "/api/v1/keys/1/clone", id: "1", body: `{"expected_profile_revision":6,"new_label":"Cloned SS"}`})
	requireFixturef(t, recClone.Code == http.StatusCreated, "CloneKey expected 201, got %d", recClone.Code)

	// 12. Storage Integrity Error Mapping -> 500
	{
		_, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = 1`)
		requireFixturef(t, err == nil, "delete secrets row: %v", err)
	}

	recCloneIntegrity := serveProfileRequest(handler.CloneKey, profileRequest{method: "POST", path: "/api/v1/keys/1/clone", id: "1", body: `{"expected_profile_revision":6,"new_label":"Clone Fail"}`})
	requireFixturef(t, recCloneIntegrity.Code == http.StatusInternalServerError, "expected 500 on storage integrity failure, got %d", recCloneIntegrity.Code)

	requireFixture(t, !(strings.Contains(recCloneIntegrity.Body.String(), ssURI)), "clone integrity response leaked decrypted credential material")

	recGetIntegrity := serveProfileRequest(handler.GetKey, profileRequest{method: "GET", path: "/api/v1/keys/1", id: "1", body: ""})
	requireFixturef(t, recGetIntegrity.Code == http.StatusInternalServerError, "detail integrity mapping changed: status=%d body=%s", recGetIntegrity.Code, recGetIntegrity.Body.String())
	requireFixturef(t, strings.Contains(recGetIntegrity.Body.String(), `"code":"storage_integrity_error"`), "detail integrity mapping changed: status=%d body=%s", recGetIntegrity.Code, recGetIntegrity.Body.String())

	requireFixture(t, !(strings.Contains(recGetIntegrity.Body.String(), ssURI)), "detail integrity response leaked decrypted credential material")

	auditCountBefore = len(auditLog)
	recRevealIntegrity := serveProfileRequest(handler.RevealKey, profileRequest{method: "POST", path: "/api/v1/keys/1/reveal", id: "1", body: `{"target":"raw","profile_revision":2}`})
	requireFixturef(t, recRevealIntegrity.Code == http.StatusInternalServerError, "reveal integrity mapping changed: status=%d body=%s", recRevealIntegrity.Code, recRevealIntegrity.Body.String())

	requireFixturef(t, len(auditLog) == auditCountBefore, "rejected reveal was audited or leaked secret: audit=%#v body=%s", auditLog, recRevealIntegrity.Body.String())
	requireFixturef(t, !(strings.Contains(recRevealIntegrity.Body.String(), ssURI)), "rejected reveal was audited or leaked secret: audit=%#v body=%s", auditLog, recRevealIntegrity.Body.String())

	{
		_, err := db.Exec(`CREATE TRIGGER reject_profile_create BEFORE INSERT ON vless_keys BEGIN SELECT RAISE(ABORT, 'forced profile create failure'); END`)
		requireFixturef(t, err == nil, "create profile failure trigger: %v", err)
	}

	recCreateConflict := serveProfileRequest(handler.CreateKeyProfile, profileRequest{method: "POST", path: "/api/v1/keys", id: "", body: `{"label":"Conflict","status":"active","kind":"real","creation_mode":"raw","raw_uri":"` + ssURI + `"}`})
	requireFixturef(t, recCreateConflict.Code == http.StatusConflict, "typed create failure mapping changed: status=%d body=%s", recCreateConflict.Code, recCreateConflict.Body.String())
	requireFixturef(t, strings.Contains(recCreateConflict.Body.String(), `"code":"create_failed"`), "typed create failure mapping changed: status=%d body=%s", recCreateConflict.Code, recCreateConflict.Body.String())

	requireFixturef(t, len(auditLog) == auditCountBefore, "failed create was audited or leaked secret: audit=%#v body=%s", auditLog, recCreateConflict.Body.String())
	requireFixturef(t, !(strings.Contains(recCreateConflict.Body.String(), ssURI)), "failed create was audited or leaked secret: audit=%#v body=%s", auditLog, recCreateConflict.Body.String())

	// 13. Get Editor Schema -> 200
	recSchema := serveProfileRequest(handler.GetKeyEditorSchema, profileRequest{method: "GET", path: "/api/v1/key-editor-schema", id: "", body: ""})
	requireFixturef(t, recSchema.Code == http.StatusOK, "GetKeyEditorSchema expected 200, got %d", recSchema.Code)

}
