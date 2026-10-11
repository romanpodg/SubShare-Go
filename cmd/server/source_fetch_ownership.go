package main

import (
	"context"
	"database/sql"
	"errors"
	"log"

	"github.com/romanpodg/SubShare-Go/internal/sources"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

var errStaleSourceResult = errors.New("source synchronization result is stale")

type sourceFetchLease struct {
	sourceID int64
	sequence uint64
	url      string
	hwid     sources.HWIDProfile
}

func (a *App) beginSourceFetch(source externalSourceRow) sourceFetchLease {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sourceFetchOwners == nil {
		a.sourceFetchOwners = make(map[int64]uint64)
	}
	a.sourceFetchSequence++
	a.sourceFetchOwners[source.ID] = a.sourceFetchSequence
	return sourceFetchLease{source.ID, a.sourceFetchSequence, source.SourceURL,
		sources.NormalizeHWIDProfile(source.PassHWID, source.HWIDVersion, source.HWIDModelName, source.HWIDValue)}
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
	state, err := storage.ReadSourceSyncTarget(context.Background(), a.db, lease.sourceID)
	if err != nil {
		log.Printf("markSourceFetchStatus: source_id=%d err=%v", lease.sourceID, err)
		return
	}
	if !sourceFetchInputMatches(lease, state) {
		a.resetInvalidatedSourceFetchLocked(lease)
		return
	}
	result, err := a.db.Exec(`
		UPDATE external_subscription_sources
		SET import_status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND source_url = ? AND (pass_hwid <> 0) = ?
		  AND COALESCE(hwid_version, '') = ? AND COALESCE(hwid_model_name, '') = ? AND COALESCE(hwid_value, '') = ?
	`, normalizeImportStatus(status), nullStringValue(message), lease.sourceID, state.URL, state.PassHWID,
		state.HWIDVersion, state.HWIDModelName, state.HWIDValue)
	if err != nil {
		log.Printf("markSourceFetchStatus: source_id=%d err=%v", lease.sourceID, err)
		return
	}
	if status == "error" {
		a.finishSourceFailurePublicationLocked(lease, result)
	}
}

func (a *App) finishSourceFailurePublicationLocked(lease sourceFetchLease, result sql.Result) {
	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf("finishSourceFailurePublication: source_id=%d err=%v", lease.sourceID, err)
		return
	}
	if affected == 0 {
		a.resetInvalidatedSourceFetchLocked(lease)
	}
}

func sourceFetchInputMatches(lease sourceFetchLease, state storage.SourceSyncTargetState) bool {
	current := sources.NormalizeHWIDProfile(state.PassHWID, state.HWIDVersion, state.HWIDModelName, state.HWIDValue)
	return state.URL == lease.url && current == lease.hwid
}

// Caller holds the fetch/reconciliation mutex and has verified this lease owns
// the source. Preserve a newer completed state and only end lingering syncing.
func (a *App) resetInvalidatedSourceFetchLocked(lease sourceFetchLease) {
	_, err := a.db.Exec(`
		UPDATE external_subscription_sources
		SET import_status = 'error', last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND import_status = 'syncing'
	`, errStaleSourceResult.Error(), lease.sourceID)
	if err != nil {
		log.Printf("resetInvalidatedSourceFetch: source_id=%d err=%v", lease.sourceID, err)
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
	if !sourceFetchInputMatches(lease, state) {
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
