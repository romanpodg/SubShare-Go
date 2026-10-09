package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

var responsePublicPaths = []string{
	"/sub/subscription-token",
	"/sub/subscription-token/subbody",
	"/sub/subscription-token/subbody/plain",
}

func responsePublicFixture(t *testing.T, responseType, headers string) userMutationFixture {
	t.Helper()
	f := newUserMutationFixture(t)
	userID := seedSubscriptionUser(t, f.app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, f.app, userID, nil, "Fixture", responsePreviewURI, "vless", "full", 0)
	execRepositoryFixtureSQL(t, f.app, `DELETE FROM response_rules`)
	execRepositoryFixtureSQL(t, f.app, `INSERT INTO response_rules(name, enabled, priority, operator, conditions_json, response_type, headers_json) VALUES('Fixture', 1, 0, 'AND', '[]', ?, ?)`, responseType, headers)
	return f
}

func TestResponsePolicyRegisteredPublicRulePrecedence(t *testing.T) {
	cases := []struct {
		responseType, status string
		code                 int
	}{
		{"block", "blocked", http.StatusForbidden},
		{"not-found", "not-found", http.StatusNotFound},
		{"browser", "", http.StatusOK},
	}
	for _, test := range cases {
		t.Run(test.responseType, func(t *testing.T) {
			f := responsePublicFixture(t, test.responseType, "[]")
			for _, path := range responsePublicPaths {
				t.Run(path, func(t *testing.T) {
					recorder := f.request(http.MethodGet, path, "")
					requireRepositoryEqual(t, "public rule status", recorder.Code, test.code)
					if test.status != "" {
						requireRepositoryEqual(t, "public denial metadata", recorder.Header().Get("Subscription-Status"), test.status)
					}
					assertResponsePublicBrowserDispatch(t, test.responseType, path, recorder)
				})
			}
		})
	}
}

func assertResponsePublicBrowserDispatch(t *testing.T, responseType, path string, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if responseType != "browser" {
		return
	}
	if path == responsePublicPaths[0] {
		if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/html") {
			t.Fatal("explicit browser rule did not select the browser page")
		}
		return
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/plain") {
		t.Fatal("subbody must retain its explicit body format even for a browser rule")
	}
}

func TestResponsePolicyRegisteredLegacyHeadersCannotOverwriteMetadata(t *testing.T) {
	headers := `[{"key":"PROFILE-TITLE","value":"override"},{"key":"SUBSCRIPTION-USERINFO","value":"override"},{"key":"Content-Type","value":"override"},{"key":"Set-Cookie","value":"override"},{"key":"X-Fixture","value":"kept"}]`
	f := responsePublicFixture(t, "plain", headers)
	for _, path := range responsePublicPaths {
		t.Run(path, func(t *testing.T) {
			recorder := f.request(http.MethodGet, path, "")
			requireRepositoryEqual(t, "metadata delivery status", recorder.Code, http.StatusOK)
			for _, key := range []string{"Profile-Title", "Subscription-Userinfo", "Content-Type", "Set-Cookie"} {
				if recorder.Header().Get(key) == "override" {
					t.Fatalf("legacy custom header overwrote canonical %s", key)
				}
			}
			requireRepositoryEqual(t, "active canonical status", recorder.Header().Get("Subscription-Status"), "active")
		})
	}
	root := f.request(http.MethodGet, responsePublicPaths[0], "")
	requireRepositoryEqual(t, "safe root rule header", root.Header().Get("X-Fixture"), "kept")
}

func TestResponsePolicyRegisteredBrowserCryptoFailureKeepsPlainFallback(t *testing.T) {
	crypto := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer crypto.Close()
	f := responsePublicFixture(t, "browser", "[]")
	f.app.baseURL, f.app.happCryptoAPIURL = "https://subscription.example", crypto.URL
	recorder := f.request(http.MethodGet, responsePublicPaths[0], "")
	requireRepositoryEqual(t, "optional crypto failure status", recorder.Code, http.StatusOK)
	if !strings.Contains(recorder.Body.String(), "https://subscription.example/sub/subscription-token") {
		t.Fatal("optional crypto failure removed the plain subscription fallback")
	}
}

func TestResponsePolicyRegisteredTemplateDeletionKeepsDefaultDelivery(t *testing.T) {
	f := responsePublicFixture(t, "plain", "[]")
	input := templateInput{Name: "Prefix", Format: "plain", Content: "prefix{{subscription}}", Enabled: true}
	created := f.request(http.MethodPost, "/api/v1/templates", mutationJSON(t, input))
	requireRepositoryEqual(t, "referenced template created", created.Code, http.StatusCreated)
	id := fmt.Sprint(decodeJSONMap(t, created)["id"])
	execRepositoryFixtureSQL(t, f.app, `UPDATE response_rules SET template_id = ?`, id)
	before := f.request(http.MethodGet, responsePublicPaths[0], "")
	requireRepositoryEqual(t, "templated delivery status", before.Code, http.StatusOK)
	if !strings.HasPrefix(before.Body.String(), "prefix") {
		t.Fatal("enabled referenced template was not applied")
	}
	assertResponseRouteMessage(t, f.request(http.MethodDelete, "/api/v1/templates/"+id, ""), "template deleted")
	var reference sql.NullInt64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT template_id FROM response_rules`).Scan(&reference))
	requireRepositoryEqual(t, "deleted template reference cleared", reference.Valid, false)
	after := f.request(http.MethodGet, responsePublicPaths[0], "")
	requireRepositoryEqual(t, "default delivery after deletion", after.Code, http.StatusOK)
	if !strings.HasPrefix(after.Body.String(), "vless://") {
		t.Fatal("deleted template did not retain default generated delivery")
	}
}
