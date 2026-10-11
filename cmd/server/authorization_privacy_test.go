package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"testing"
)

func TestActualAuthPrivacyRetainsHashedCredentials(t *testing.T) {
	f := newUserMutationFixture(t)
	var ownerID int64
	cookieSum := sha256.Sum256([]byte(f.session))
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT admin_id FROM admin_sessions WHERE id = ?`, hex.EncodeToString(cookieSum[:])).Scan(&ownerID))
	token := seedAuthorizationRouteToken(t, f.app, authorizationRouteTokenSeed{ownerID, 1, "read"})
	tokenSum := sha256.Sum256([]byte(token))
	requireRepositoryEqual(t, "raw cookie is not stored", mutationCount(t, f.app, `SELECT COUNT(*) FROM admin_sessions WHERE id = ?`, f.session), 0)
	requireRepositoryEqual(t, "raw bearer is not stored", mutationCount(t, f.app, `SELECT COUNT(*) FROM api_tokens WHERE token_hash = ?`, token), 0)
	requireRepositoryEqual(t, "SHA256 bearer is stored", mutationCount(t, f.app, `SELECT COUNT(*) FROM api_tokens WHERE token_hash = ?`, hex.EncodeToString(tokenSum[:])), 1)
	var capture bytes.Buffer
	writer := log.Writer()
	log.SetOutput(&capture)
	t.Cleanup(func() { log.SetOutput(writer) })
	contract := authorizationRouteContract{Pattern: "POST /api/v1/users"}
	for _, principal := range []authorizationRoutePrincipal{{cookie: f.session}, {bearer: token}} {
		recorder := authorizationRouteRequest(f, contract, principal, http.MethodPost)
		requireRepositoryEqual(t, "privacy request denied", recorder.Code, http.StatusForbidden)
		if strings.Contains(recorder.Body.String(), f.session) || strings.Contains(recorder.Body.String(), token) {
			t.Fatal("raw credential appeared in an auth error")
		}
	}
	log.SetOutput(writer)
	if strings.Contains(capture.String(), f.session) || strings.Contains(capture.String(), token) {
		t.Fatal("raw credential appeared in logs")
	}
}
