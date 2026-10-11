package storage

import (
	"context"
	"database/sql"
)

// SourceSyncTargetState is read inside the reconciliation transaction so a
// fetched result cannot be committed against different request inputs.
type SourceSyncTargetState struct {
	URL, KeyCategory, KeyInsertMode       string
	PassHWID                              bool
	HWIDVersion, HWIDModelName, HWIDValue string
	Enabled                               bool
}

func (s *SourceSync) ReadTargetState(sourceID int64) (SourceSyncTargetState, error) {
	return scanSourceSyncTarget(s.tx.QueryRowContext(s.ctx, sourceSyncTargetQuery, sourceID))
}

func ReadSourceSyncTarget(ctx context.Context, db *sql.DB, sourceID int64) (SourceSyncTargetState, error) {
	return scanSourceSyncTarget(db.QueryRowContext(ctx, sourceSyncTargetQuery, sourceID))
}

const sourceSyncTargetQuery = `
	SELECT source_url, enabled, key_category, key_insert_mode, pass_hwid,
	       COALESCE(hwid_version, ''), COALESCE(hwid_model_name, ''), COALESCE(hwid_value, '')
	FROM external_subscription_sources WHERE id = ?
`

func scanSourceSyncTarget(row *sql.Row) (SourceSyncTargetState, error) {
	var state SourceSyncTargetState
	var enabled, passHWID int
	err := row.Scan(&state.URL, &enabled, &state.KeyCategory, &state.KeyInsertMode, &passHWID,
		&state.HWIDVersion, &state.HWIDModelName, &state.HWIDValue)
	state.Enabled = enabled != 0
	state.PassHWID = passHWID != 0
	return state, err
}
