package main

import (
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

// assignedDeliveryKeySeed accepts malformed raw profiles intentionally; delivery
// exclusion tests must seed storage directly rather than validate through import.
type assignedDeliveryKeySeed struct {
	UserID        int64
	SourceID      *int64 // nil preserves a SQL NULL for manually owned keys.
	Label         string
	Raw           string
	Protocol      string
	Compatibility string
	SortOrder     int
}

func insertAssignedDeliveryKey(t *testing.T, app *App, seed assignedDeliveryKeySeed) int64 {
	t.Helper()
	_, bikKey, err := app.profileKeyring.GetActiveBlindIndexKey()
	if err != nil {
		t.Fatalf("load fixture blind-index key: %v", err)
	}
	blindIndex := profilestorage.ComputeBlindIndex(bikKey, seed.Raw)
	result, err := app.db.Exec(`
  INSERT INTO vless_keys(
   label, url_blind_index, status, key_kind, health_failure_count, external_source_id,
   protocol, profile_compatibility, sort_order
  ) VALUES(?, ?, 'active', 'real', 0, ?, ?, ?, ?)
 `, seed.Label, blindIndex, seed.SourceID, seed.Protocol, seed.Compatibility, seed.SortOrder)
	if err != nil {
		t.Fatalf("insert delivery key: %v", err)
	}
	keyID := profileFixtureInsertID(t, result)
	insertProfileFixtureSecret(t, app, keyID, seed.Raw)
	assignProfileFixtureUsers(t, app, keyID, seed.UserID)
	return keyID
}
