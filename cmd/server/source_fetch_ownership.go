package main

import (
	"context"
	"errors"
	"log"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

var errStaleSourceResult = errors.New("source synchronization result is stale")

type sourceFetchLease struct {
	sourceID int64
	sequence uint64
	url      string
}

func (a *App) beginSourceFetch(source externalSourceRow) sourceFetchLease {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sourceFetchOwners == nil {
		a.sourceFetchOwners = make(map[int64]uint64)
	}
	a.sourceFetchSequence++
	a.sourceFetchOwners[source.ID] = a.sourceFetchSequence
	return sourceFetchLease{source.ID, a.sourceFetchSequence, source.SourceURL}
}

// sourceFetchOwnedLocked requires the same mutex used by reconciliation.
func (a *App) sourceFetchOwnedLocked(lease sourceFetchLease) bool {
	return a.sourceFetchOwners[lease.sourceID] == lease.sequence
}

func (a *App) releaseSourceFetch(lease sourceFetchLease) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sourceFetchOwnedLocked(lease) {
		delete(a.sourceFetchOwners, lease.sourceID)
	}
}

func (a *App) markSourceFetchStatus(lease sourceFetchLease, status, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.sourceFetchOwnedLocked(lease) {
		return
	}
	_, err := a.db.Exec(`
		UPDATE external_subscription_sources
		SET import_status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND source_url = ?
	`, normalizeImportStatus(status), nullStringValue(message), lease.sourceID, lease.url)
	if err != nil {
		log.Printf("markSourceFetchStatus: source_id=%d err=%v", lease.sourceID, err)
	}
}

func (a *App) syncOwnedSourceResult(lease sourceFetchLease, parsed sources.ParseResult) (sources.SyncResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.sourceFetchOwnedLocked(lease) {
		return sources.SyncResult{}, errStaleSourceResult
	}
	sync, err := a.store().BeginSourceSync(context.Background())
	if err != nil {
		return sources.SyncResult{}, err
	}
	defer sync.Rollback()
	state, err := sync.ReadTargetState(lease.sourceID)
	if err != nil {
		return sources.SyncResult{}, err
	}
	if state.URL != lease.url {
		return sources.SyncResult{}, errStaleSourceResult
	}
	target := sources.SyncTarget{ID: lease.sourceID, Enabled: state.Enabled, KeyCategory: state.KeyCategory, KeyInsertMode: state.KeyInsertMode}
	result, err := sources.Sync(sync, target, parsed, a.externalProfileFingerprintKeys())
	if err != nil {
		return result, err
	}
	if err := sync.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
