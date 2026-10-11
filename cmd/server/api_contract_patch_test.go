package main

import (
	"database/sql"
	"net/http"
	"testing"
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
