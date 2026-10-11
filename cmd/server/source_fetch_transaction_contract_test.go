package main

import (
	"strings"
	"testing"
	"time"
)

func TestJobSourceURLEditAfterTransactionalReadCannotCommitOldProfiles(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	var journal string
	requireRepositorySuccess(t, f.app.db.QueryRow(`PRAGMA journal_mode = WAL`).Scan(&journal))
	requireRepositoryEqual(t, "disposable fixture uses WAL", strings.ToLower(journal), "wal")
	sourceID, fetch := configureJobSourceFixture(t, f)
	reachedWrite := make(chan struct{})
	resumeWrite := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resumeWrite:
		default:
			close(resumeWrite)
		}
	})
	faults.beforeExec = func(query string) {
		if strings.Contains(query, "INSERT INTO vless_keys(") {
			close(reachedWrite)
			<-resumeWrite
		}
	}
	jobID, done := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, fetch)
	close(fetch.release)
	select {
	case <-reachedWrite:
	case <-time.After(5 * time.Second):
		t.Fatal("reconciliation did not reach the post-read write barrier")
	}
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET source_url = 'https://provider.example/new-feed' WHERE id = ?`, sourceID)
	close(resumeWrite)
	waitControlledSourceJob(t, done)
	status, _, _, _ := jobLifecycleState(t, f.app, jobID)
	requireRepositoryEqual(t, "stale transactional snapshot fails", status, "failed")
	requireRepositoryEqual(t, "stale snapshot creates no profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys`), 0)
	requireRepositoryEqual(t, "stale snapshot creates no secret", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_key_secrets`), 0)
}
