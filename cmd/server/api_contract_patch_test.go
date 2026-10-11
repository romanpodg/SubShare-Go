package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestAPIContractRegisteredSubscriptionPatches(t *testing.T) {
	for name, c := range loadAPIContracts(t) {
		if c.Method != http.MethodPatch {
			continue
		}
		t.Run(name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			execRepositoryFixtureSQL(t, f.app, `INSERT INTO users(id,name,token,status,subscription_name,subscription_info_url,subscription_refresh_hours) VALUES(1,'Contract user','fixture-token','active','Override','https://user.example/info',24)`)
			w := apiContractRequest(f, c)
			requireRepositoryEqual(t, "status", w.Code, c.Status)
			requireRepositoryEqual(t, "response", decodeJSONMap(t, w), apiContractExpected(t, name, c.Response))
			requireRepositoryEqual(t, "persisted patch semantics", apiContractStoredPatch(t, f.app), apiContractExpected(t, name, c.Stored))
		})
	}
}

func TestAPIContractOmittedEmptyCreateProtocolKeepsCommandValue(t *testing.T) {
	var before, after model.CreateKeyProfileRequest
	requireRepositorySuccess(t, json.Unmarshal([]byte(`{"label":"fixture","status":"active","kind":"real","creation_mode":"structured","protocol":"","structured":{}}`), &before))
	requireRepositorySuccess(t, json.Unmarshal([]byte(`{"label":"fixture","status":"active","kind":"real","creation_mode":"structured","structured":{}}`), &after))
	requireRepositoryEqual(t, "empty protocol omission preserves decoded command", after, before)
}

func apiContractStoredPatch(t *testing.T, app *App) map[string]any {
	t.Helper()
	var title, info sql.NullString
	var refresh int
	var status string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT subscription_name,subscription_info_url,subscription_refresh_hours,status FROM users WHERE id=1`).Scan(&title, &info, &refresh, &status))
	stored := map[string]any{"title": nil, "info_url": nil, "refresh_hours": float64(refresh), "status": status}
	if title.Valid {
		stored["title"] = title.String
	}
	if info.Valid {
		stored["info_url"] = info.String
	}
	return stored
}

func TestAPIContractBlankProtocolDoesNotEnableRealStructuredCreation(t *testing.T) {
	for _, kind := range []string{"informational", "real"} {
		t.Run(kind, func(t *testing.T) {
			f := newUserMutationFixture(t)
			mode := "raw"
			if kind == "real" {
				mode = "structured"
			}
			payload := map[string]any{"label": "Contract info", "status": "active", "kind": kind, "creation_mode": mode, "protocol": "", "template_text": "literal info"}
			if kind == "real" {
				payload["structured"] = map[string]any{}
			}
			body := mutationJSON(t, payload)
			w := f.request(http.MethodPost, "/api/v1/keys", body)
			if kind == "real" {
				requireRepositoryEqual(t, "existing unsupported structured protocol failure", w.Code, http.StatusInternalServerError)
				requireRepositoryEqual(t, "safe existing failure code", decodeJSONMap(t, w)["code"], "create_failed")
				requireRepositoryEqual(t, "no invalid real profile committed", mutationCount(t, f.app, "SELECT COUNT(*) FROM vless_keys"), 0)
				return
			}
			requireRepositoryEqual(t, "informational empty wire protocol accepted", w.Code, http.StatusCreated)
			requireRepositoryEqual(t, "existing informational creation normalization", decodeJSONMap(t, w)["data"].(map[string]any)["protocol"], "legacy")
		})
	}
}
