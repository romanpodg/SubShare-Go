package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestSourceCreateResponseReadFailureRetainsCommittedUnitWithoutAudit(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	seedSourceCreateAllModeUser(t, f.app)
	faults.queryErr = func(query string) error {
		if strings.Contains(strings.Join(strings.Fields(query), " "), "FROM external_subscription_sources WHERE id = ?") {
			return errors.New("fixture source response read failure")
		}
		return nil
	}
	recorder := sourceCreateContractRequest(t, f)
	requireRepositoryEqual(t, "committed response read failure", recorder.Code, http.StatusInternalServerError)
	payload := decodeJSONMap(t, recorder)
	requireRepositoryEqual(t, "committed response read code", payload["code"], "source_create_failed")
	requireRepositoryEqual(t, "committed response safe error", payload["error"], "source created but response could not be loaded")
	requireRepositoryEqual(t, "failed response withholds source DTO", payload["data"], nil)
	requireRepositoryEqual(t, "committed source retained", mutationCount(t, f.app, `SELECT COUNT(*) FROM external_subscription_sources`), 1)
	requireRepositoryEqual(t, "committed profiles retained", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys`), 2)
	requireRepositoryEqual(t, "committed credentials retained", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_key_secrets`), 2)
	requireRepositoryEqual(t, "committed assignments retained", mutationCount(t, f.app, `SELECT COUNT(*) FROM user_keys`), 2)
	requireRepositoryEqual(t, "failed response audit remains absent", mutationCount(t, f.app, `SELECT COUNT(*) FROM audit_events WHERE action = 'external_source.create'`), 0)
}
