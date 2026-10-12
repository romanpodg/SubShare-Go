package sources_test

import (
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func TestLocalSourceSyncAddsEncryptsAssignsAndRemainsIdempotent(t *testing.T) {
	f := newSyncContractFixture(t)
	first := f.sync(t, 1, syncOne+"\n"+syncTwo)
	sourceEqual(t, "added", first.Counts.Added, 2)
	sourceEqual(t, "imported", first.Imported, 2)
	one := f.row(t, 1, "One")
	sourceEqual(t, "decrypted URI", one.raw, syncOne)
	sourceEqual(t, "active new row", one.status, "active")
	sourceEqual(t, "all-user assignments", f.count(t, `SELECT COUNT(*) FROM user_keys WHERE user_id=1`), 2)
	sourceEqual(t, "selected-user assignments", f.count(t, `SELECT COUNT(*) FROM user_keys WHERE user_id=2`), 0)
	second := f.sync(t, 1, syncTwo+"\n"+syncOne)
	sourceEqual(t, "unchanged", second.Counts.Unchanged, 2)
	sourceEqual(t, "added on repeat", second.Counts.Added, 0)
	sourceEqual(t, "stable row and raw", f.row(t, 1, "One"), one)
}

func TestLocalSourceSyncMatchesSemanticRenameAndKeepsLocalMetadata(t *testing.T) {
	f := newSyncContractFixture(t)
	f.sync(t, 1, syncOne)
	one := f.row(t, 1, "One")
	f.exec(t, `UPDATE vless_keys SET status='non-active', client_display_name='Local display' WHERE id=?`, one.id)
	renamed := strings.Replace(syncOne, "#One", "#Renamed", 1)
	result := f.sync(t, 1, renamed)
	sourceEqual(t, "updated", result.Counts.Updated, 1)
	got := f.row(t, 1, "Renamed")
	sourceEqual(t, "stable id", got.id, one.id)
	sourceEqual(t, "stable ref", got.ref, one.ref)
	sourceEqual(t, "provider raw update", got.raw, renamed)
	sourceEqual(t, "local status", got.status, "non-active")
	sourceEqual(t, "local display", got.display, "Local display")
}

func TestLocalSourceSyncUniqueLabelRotationKeepsAssignments(t *testing.T) {
	f := newSyncContractFixture(t)
	f.sync(t, 1, syncOne)
	one := f.row(t, 1, "One")
	rotated := strings.Replace(syncOne, "fixture-one", "fixture-rotated", 1)
	result := f.sync(t, 1, rotated)
	sourceEqual(t, "updated", result.Counts.Updated, 1)
	got := f.row(t, 1, "One")
	sourceEqual(t, "stable id", got.id, one.id)
	sourceEqual(t, "rotated credential", got.raw, rotated)
	sourceEqual(t, "assignment preserved", f.count(t, `SELECT COUNT(*) FROM user_keys WHERE user_id=1 AND key_id=?`, one.id), 1)
}

func TestLocalSourceSyncPartialRefreshProtectsMissingRows(t *testing.T) {
	for _, bad := range []string{"trojan://", "https://unsupported.example/profile"} {
		t.Run(bad, func(t *testing.T) {
			f := newSyncContractFixture(t)
			f.sync(t, 1, syncOne+"\n"+syncTwo)
			two := f.row(t, 1, "Two")
			partial := f.sync(t, 1, syncOne+"\n"+bad)
			sourceEqual(t, "no partial removal", partial.Counts.Removed, 0)
			sourceEqual(t, "missing row retained", f.row(t, 1, "Two"), two)
			complete := f.sync(t, 1, syncOne)
			sourceEqual(t, "confirmed removal", complete.Counts.Removed, 1)
			sourceEqual(t, "removed secret", f.count(t, `SELECT COUNT(*) FROM vless_key_secrets WHERE vless_key_id=?`, two.id), 0)
			sourceEqual(t, "removed assignment", f.count(t, `SELECT COUNT(*) FROM user_keys WHERE key_id=?`, two.id), 0)
		})
	}
}

func TestLocalSourceSyncScopeAndDisabledNewKeys(t *testing.T) {
	f := newSyncContractFixture(t)
	f.sync(t, 1, syncOne)
	other := f.row(t, 1, "One")
	parsed := syncParsed(t, syncOne+"\n"+syncTwo)
	parsed.Metadata = sources.Metadata{Title: "Provider", RefreshHours: 12, Announce: "Message"}
	result, err := f.apply(t, sources.SyncTarget{ID: 2, Enabled: false, KeyCategory: "Imported", KeyInsertMode: "top"}, parsed, syncFingerprintKeys)
	sourceSuccess(t, err)
	sourceEqual(t, "added second source", result.Counts.Added, 2)
	sourceEqual(t, "other source unchanged", f.row(t, 1, "One"), other)
	sourceEqual(t, "new disabled row", f.row(t, 2, "One").status, "non-active")
	sourceEqual(t, "category", f.count(t, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id=2 AND category='Imported'`), 2)
	sourceEqual(t, "published metadata", f.count(t, `SELECT COUNT(*) FROM external_subscription_sources WHERE id=2 AND import_status='ok' AND last_import_count=2 AND meta_title='Provider' AND meta_refresh_hours=12 AND meta_announce='Message'`), 1)
}

func TestLocalSourceSyncAdmissionFailuresPreservePopulatedState(t *testing.T) {
	cases := []struct {
		name, want string
		empty      bool
	}{
		{"empty feed", "no_keys_to_import", true},
		{"missing fingerprint key", "profile_fingerprint_key_unavailable", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newSyncContractFixture(t)
			f.sync(t, 1, syncOne)
			before := f.snapshot(t)
			parsed := sources.ParseResult{}
			if !c.empty {
				parsed = syncParsed(t, syncTwo)
			}
			_, err := f.apply(t, sources.SyncTarget{ID: 1, Enabled: true, KeyCategory: "Transient"}, parsed, nil)
			sourceEqual(t, "admission error", sourceFailure(t, err), c.want)
			sourceEqual(t, "complete rollback", f.snapshot(t), before)
		})
	}
}

func TestLocalSourceSyncPublicationFailureRollsBackWholeUnit(t *testing.T) {
	f := newSyncContractFixture(t)
	f.sync(t, 1, syncOne)
	f.exec(t, `CREATE TRIGGER fail_sync_publication BEFORE UPDATE ON external_subscription_sources BEGIN SELECT RAISE(ABORT, 'fixture_publication_failure'); END`)
	before := f.snapshot(t)
	_, err := f.apply(t, sources.SyncTarget{ID: 1, Enabled: true, KeyCategory: "Transient"}, syncParsed(t, syncTwo), syncFingerprintKeys)
	sourceEqual(t, "injected failure returned", strings.Contains(sourceFailure(t, err), "fixture_publication_failure"), true)
	sourceEqual(t, "metadata/key/secret/category/assignment rollback", f.snapshot(t), before)
}

func TestLocalSourceSyncDuplicateIncomingRefIsCountedOnce(t *testing.T) {
	f := newSyncContractFixture(t)
	parsed := syncParsed(t, syncOne)
	duplicate := parsed.Keys[0]
	duplicate.ItemRef = "fixture-duplicate-item"
	parsed.Keys = append(parsed.Keys, duplicate)
	duplicateItem := parsed.Items[0]
	duplicateItem.ItemRef = duplicate.ItemRef
	parsed.Items = append(parsed.Items, duplicateItem)
	result, err := f.apply(t, sources.SyncTarget{ID: 1, Enabled: true}, parsed, syncFingerprintKeys)
	sourceSuccess(t, err)
	sourceEqual(t, "imported once", result.Imported, 1)
	sourceEqual(t, "duplicate skipped", result.Skipped, 1)
	sourceEqual(t, "duplicate status", result.Items[1].Status, sources.StatusDuplicate)
	sourceEqual(t, "one persisted row", f.count(t, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id=1`), 1)
}

func TestLocalSourceSyncAmbiguousLabelsUseConservativeReplacement(t *testing.T) {
	f := newSyncContractFixture(t)
	first := strings.Replace(syncOne, "#One", "#Shared", 1)
	second := strings.Replace(syncTwo, "#Two", "#Shared", 1)
	f.sync(t, 1, first+"\n"+second)
	rotated := strings.Replace(first, "fixture-one", "fixture-rotated", 1)
	result := f.sync(t, 1, rotated)
	sourceEqual(t, "new identity", result.Counts.Added, 1)
	sourceEqual(t, "both ambiguous old identities removed", result.Counts.Removed, 2)
	sourceEqual(t, "one remaining row", f.count(t, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id=1`), 1)
	sourceEqual(t, "new raw credential", f.row(t, 1, "Shared").raw, rotated)
}
