package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func profileCreatePersistenceCounts(t *testing.T, db *sql.DB) [4]int64 {
	t.Helper()
	var counts [4]int64
	profileHTTPSuccess(t, db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM vless_keys),
		(SELECT COUNT(*) FROM vless_key_secrets),
		(SELECT COUNT(*) FROM key_categories),
		(SELECT COUNT(*) FROM user_keys)
	`).Scan(&counts[0], &counts[1], &counts[2], &counts[3]))
	return counts
}

func TestProfileHTTPCreateAssignmentFailureDoesNotReportSuccess(t *testing.T) {
	f := newProfileHTTPContractFixture(t)
	_, err := f.db.Exec(`INSERT INTO users(id, name, token, key_assignment_mode) VALUES(1, 'one', 'r08c-one', 'all'), (2, 'two', 'r08c-two', 'all'), (3, 'selected', 'r08c-selected', 'selected')`)
	profileHTTPSuccess(t, err)
	before := profileCreatePersistenceCounts(t, f.db)
	_, err = f.db.Exec(`CREATE TRIGGER fail_http_profile_assignment AFTER INSERT ON user_keys WHEN NEW.user_id=2 BEGIN SELECT RAISE(ABORT, 'r08c private SQL failure'); END`)
	profileHTTPSuccess(t, err)
	newURI := strings.Replace(profileHTTPContractURI, "edge.example", "new.example", 1)
	body, err := json.Marshal(map[string]any{"creation_mode": "raw", "raw_uri": newURI, "label": "New", "status": "active", "kind": "real", "category": "New HTTP"})
	profileHTTPSuccess(t, err)
	response := httptest.NewRecorder()
	f.handler.CreateKeyProfile(response, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(string(body))))
	assertProfileHTTPError(t, response, http.StatusInternalServerError, "create_failed")
	profileHTTPEqual(t, "no SQL failure details", strings.Contains(response.Body.String(), "r08c private SQL failure"), false)
	profileHTTPEqual(t, "no success audit", len(*f.audits), 0)
	profileHTTPEqual(t, "no partial persisted creation", profileCreatePersistenceCounts(t, f.db), before)
	_, err = f.db.Exec(`DROP TRIGGER fail_http_profile_assignment`)
	profileHTTPSuccess(t, err)
	response = httptest.NewRecorder()
	f.handler.CreateKeyProfile(response, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(string(body))))
	profileHTTPEqual(t, "retry creation status", response.Code, http.StatusCreated)
	profileHTTPEqual(t, "one retry success audit", len(*f.audits), 1)
	profileHTTPEqual(t, "retry creation and all-mode assignments", profileCreatePersistenceCounts(t, f.db), [4]int64{before[0] + 1, before[1] + 1, before[2] + 1, before[3] + 2})
	assertProfileHTTPSecretFree(t, response.Body.String())
}
