package storage

// SourceSyncTargetState is read inside the reconciliation transaction so a
// fetched result cannot be committed against a different source URL.
type SourceSyncTargetState struct {
	URL, KeyCategory, KeyInsertMode string
	Enabled                         bool
}

func (s *SourceSync) ReadTargetState(sourceID int64) (SourceSyncTargetState, error) {
	var state SourceSyncTargetState
	var enabled int
	err := s.tx.QueryRowContext(s.ctx, `
		SELECT source_url, enabled, key_category, key_insert_mode
		FROM external_subscription_sources WHERE id = ?
	`, sourceID).Scan(&state.URL, &enabled, &state.KeyCategory, &state.KeyInsertMode)
	state.Enabled = enabled != 0
	return state, err
}
