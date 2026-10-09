package main

import (
	"errors"
	"strings"
	"testing"
)

func TestJobSourceWorkerBeginAndCommitFailureRollBack(t *testing.T) {
	for _, phase := range []string{"begin", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			sourceID, fetch := configureJobSourceFixture(t, f)
			before := seedJobRollbackState(t, f.app, sourceID)
			fetch.body = strings.Replace(externalTestVLESS, "vless.example", "changed.example", 1)
			if phase == "begin" {
				faults.beginErr = errors.New("fixture begin failure")
			} else {
				faults.commitErr = errors.New("fixture commit failure")
			}
			jobID, done := startControlledSourceJob(t, f, sourceID)
			waitJobFetch(t, fetch)
			close(fetch.release)
			waitControlledSourceJob(t, done)
			status, _, _, _ := jobLifecycleState(t, f.app, jobID)
			requireRepositoryEqual(t, "transaction failure job status", status, "failed")
			assertJobRollbackState(t, f.app, sourceID, before)
		})
	}
}

func TestJobSourceWorkerRollsBackProfilesSecretsAndAssignments(t *testing.T) {
	cases := []struct{ name, trigger string }{
		{"parent", `CREATE TRIGGER fixture_sync_failure BEFORE INSERT ON vless_keys WHEN NEW.label = 'Second' BEGIN SELECT RAISE(ABORT, 'fixture parent failure'); END`},
		{"secret", `CREATE TRIGGER fixture_sync_failure BEFORE INSERT ON vless_key_secrets WHEN (SELECT label FROM vless_keys WHERE id = NEW.vless_key_id) = 'Second' BEGIN SELECT RAISE(ABORT, 'fixture secret failure'); END`},
		{"assignment", `CREATE TRIGGER fixture_sync_failure BEFORE INSERT ON user_keys WHEN (SELECT label FROM vless_keys WHERE id = NEW.key_id) = 'Second' BEGIN SELECT RAISE(ABORT, 'fixture assignment failure'); END`},
		{"source finish", `CREATE TRIGGER fixture_sync_failure BEFORE UPDATE ON external_subscription_sources WHEN NEW.import_status = 'ok' BEGIN SELECT RAISE(ABORT, 'fixture source finish failure'); END`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, sourceID, fetch := newJobSourceFixture(t)
			before := seedJobRollbackState(t, f.app, sourceID)
			first := strings.Replace(strings.Replace(externalTestVLESS, "vless.example", "first.example", 1), "#VLESS", "#First", 1)
			second := strings.Replace(strings.Replace(externalTestVLESS, "vless.example", "second.example", 1), "#VLESS", "#Second", 1)
			fetch.body = first + "\n" + second
			execRepositoryFixtureSQL(t, f.app, test.trigger)
			jobID, done := startControlledSourceJob(t, f, sourceID)
			waitJobFetch(t, fetch)
			close(fetch.release)
			waitControlledSourceJob(t, done)
			status, _, _, _ := jobLifecycleState(t, f.app, jobID)
			requireRepositoryEqual(t, "rollback worker fails", status, "failed")
			assertJobRollbackState(t, f.app, sourceID, before)
		})
	}
}

type jobRollbackState struct {
	keyID    int64
	envelope string
}

func seedJobRollbackState(t *testing.T, app *App, sourceID int64) jobRollbackState {
	t.Helper()
	old := strings.Replace(externalTestVLESS, "#VLESS", "#Old", 1)
	_, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, old))
	requireRepositorySuccess(t, err)
	var state jobRollbackState
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT k.id, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON s.vless_key_id = k.id WHERE k.external_source_id = ?`, sourceID).Scan(&state.keyID, &state.envelope))
	selected := seedExternalRepairUser(t, app, "rollback-selected")
	execRepositoryFixtureSQL(t, app, `INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, selected, state.keyID)
	execRepositoryFixtureSQL(t, app, `INSERT INTO users(name, token, status, key_assignment_mode) VALUES('all-fixture', 'all-fixture-token', 'active', 'all')`)
	return state
}

func assertJobRollbackState(t *testing.T, app *App, sourceID int64, before jobRollbackState) {
	t.Helper()
	requireRepositoryEqual(t, "rollback retains original profile only", mutationCount(t, app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID), 1)
	requireRepositoryEqual(t, "rollback retains original credential only", mutationCount(t, app, `SELECT COUNT(*) FROM vless_key_secrets`), 1)
	requireRepositoryEqual(t, "rollback retains original assignment only", mutationCount(t, app, `SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, before.keyID), 1)
	requireRepositoryEqual(t, "rollback removes tentative all-mode assignments", mutationCount(t, app, `SELECT COUNT(*) FROM user_keys k JOIN users u ON u.id = k.user_id WHERE u.key_assignment_mode = 'all'`), 0)
	var envelope string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, before.keyID).Scan(&envelope))
	if envelope != before.envelope {
		t.Fatal("rollback changed the original credential envelope")
	}
}
