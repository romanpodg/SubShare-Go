package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func runProfileContractCommand(f profileContractFixture, command string) (*model.VLESSKey, string, error) {
	ctx := context.Background()
	switch command {
	case "create":
		params := profileContractCreateParams()
		params.BuiltURI = f.updateParams().NewURI
		params.Category = "New category"
		return f.repo.CreateLocal(ctx, params)
	case "update":
		return f.repo.UpdateLocal(ctx, f.updateParams())
	case "source metadata":
		name := "Subscriber override"
		return f.repo.UpdateSourceOwnedMetadata(ctx, keymanagement.UpdateSourceOwnedMetadataParams{ID: f.key.ID, ExpectedRevision: 1, Status: "non-active", ClientDisplayName: &name})
	case "clone":
		return f.repo.CloneLocal(ctx, keymanagement.CloneProfileParams{ID: f.key.ID, ExpectedRevision: 1, NewLabel: "Clone"})
	default:
		panic("unknown test command")
	}
}

func TestProfileCommandsSQLFailuresRollBackFullUnit(t *testing.T) {
	for _, tc := range []struct{ command, stage, trigger string }{
		{"create", "parent after category", `BEFORE INSERT ON vless_keys`},
		{"create", "secret after parent", `AFTER INSERT ON vless_key_secrets`},
		{"update", "parent after category", `AFTER UPDATE ON vless_keys`},
		{"update", "secret after parent", `AFTER UPDATE ON vless_key_secrets`},
		{"source metadata", "parent", `AFTER UPDATE ON vless_keys`},
		{"clone", "parent", `AFTER INSERT ON vless_keys`},
		{"clone", "secret after parent", `AFTER INSERT ON vless_key_secrets`},
		{"clone", "assignment after first user", `AFTER INSERT ON user_keys WHEN NEW.user_id = 2`},
	} {
		t.Run(tc.command+"/"+tc.stage, func(t *testing.T) {
			f := newProfileContractFixture(t)
			if tc.command == "source metadata" {
				f.makeSourceOwned(t)
			}
			before := profileContractSnapshot(t, f.db)
			profileContractExec(t, f.db, `CREATE TRIGGER fail_profile_write `+tc.trigger+` BEGIN SELECT RAISE(ABORT, 'r06 staged write failure'); END`)
			key, raw, err := runProfileContractCommand(f, tc.command)
			profileContractNoResult(t, key, raw)
			profileContractEqual(t, "staged failure returned", err != nil, true)
			profileContractEqual(t, "staged failure reached", strings.Contains(err.Error(), "r06 staged write failure"), true)
			assertProfileContractSnapshot(t, f.db, before)
			profileContractExec(t, f.db, `DROP TRIGGER fail_profile_write`)
			if _, _, err := runProfileContractCommand(f, tc.command); err != nil {
				t.Fatalf("retry after rollback failed: %v", err)
			}
		})
	}
}

func TestProfileCommandsBeginAndRejectedCommitRollBack(t *testing.T) {
	for _, command := range []string{"create", "update", "source metadata", "clone"} {
		for _, stage := range []string{"begin", "commit"} {
			t.Run(command+"/"+stage, func(t *testing.T) {
				f := newProfileContractFixture(t)
				if command == "source metadata" {
					f.makeSourceOwned(t)
				}
				faults := f.withFaultDriver(t)
				before := profileContractSnapshot(t, f.db)
				if stage == "begin" {
					faults.beginErr = errProfileContractFault
				} else {
					faults.commitErr = errProfileContractFault
				}
				key, raw, err := runProfileContractCommand(f, command)
				profileContractNoResult(t, key, raw)
				profileContractError(t, err, errProfileContractFault)
				assertProfileContractSnapshot(t, f.db, before)
				faults.beginErr, faults.commitErr = nil, nil
				if _, _, err := runProfileContractCommand(f, command); err != nil {
					t.Fatalf("retry after known rollback failed: %v", err)
				}
			})
		}
	}
}

// Defect characterization, not a desired invariant: CreateLocal discards the
// assignment statement's error and commits the parent and secret. CloneLocal
// instead rolls back (covered above). Correcting CreateLocal is a separate PR.
func TestProfileCreateCharacterizesIgnoredAssignmentFailure(t *testing.T) {
	f := newProfileContractFixture(t)
	profileContractExec(t, f.db, `CREATE TRIGGER fail_profile_assignment AFTER INSERT ON user_keys WHEN NEW.user_id = 2 BEGIN SELECT RAISE(ABORT, 'r06 assignment failure'); END`)
	key, raw, err := runProfileContractCommand(f, "create")
	profileContractSuccess(t, err)
	profileContractEqual(t, "partial-success key present", key != nil, true)
	profileContractEqual(t, "partial-success configuration", raw, f.updateParams().NewURI)
	assignments := profileContractCount(t, f.db, `SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, key.ID)
	secrets := profileContractCount(t, f.db, `SELECT COUNT(*) FROM vless_key_secrets WHERE vless_key_id = ?`, key.ID)
	profileContractEqual(t, "failed assignment statement retained no assignments", assignments, 0)
	profileContractEqual(t, "committed secret", secrets, 1)
	originalAssignments := profileContractCount(t, f.db, `SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, f.key.ID)
	profileContractEqual(t, "existing assignments", originalAssignments, 2)
}

func TestProfileCommandsPostCommitReloadFailureAndRetry(t *testing.T) {
	for _, command := range []string{"create", "update", "source metadata", "clone"} {
		t.Run(command, func(t *testing.T) {
			f := newProfileContractFixture(t)
			if command == "source metadata" {
				f.makeSourceOwned(t)
			}
			faults := f.withFaultDriver(t)
			faults.reloadErr = errProfileContractFault
			key, raw, err := runProfileContractCommand(f, command)
			profileContractNoResult(t, key, raw)
			profileContractError(t, err, errProfileContractFault)
			profileContractEqual(t, "commit succeeded before reload", faults.committed, true)
			faults.reloadErr = nil
			id, wantRevision := f.key.ID, int64(2)
			if command == "create" || command == "clone" {
				wantRevision = 1
				profileContractSuccess(t, f.db.QueryRow(`SELECT MAX(id) FROM vless_keys`).Scan(&id))
			}
			committed, committedRaw, err := f.repo.GetByID(context.Background(), id)
			profileContractSuccess(t, err)
			profileContractEqual(t, "committed revision", committed.ProfileRevision, wantRevision)
			profileContractEqual(t, "committed configuration present", committedRaw != "", true)
			beforeRetry := profileContractSnapshot(t, f.db)
			assertProfileReloadRetry(t, f, command, beforeRetry)
		})
	}
}

func assertProfileReloadRetry(t *testing.T, f profileContractFixture, command string, before map[string][][]any) {
	t.Helper()
	_, _, err := runProfileContractCommand(f, command)
	switch command {
	case "create":
		profileContractError(t, err, keymanagement.ErrProfileCreateConflict)
	case "update", "source metadata":
		profileContractError(t, err, keymanagement.ErrProfileRevisionConflict)
	case "clone":
		// Cloning has no idempotency token; a retry creates another clone.
		profileContractSuccess(t, err)
		profileContractEqual(t, "non-idempotent clone retry", profileContractCount(t, f.db, `SELECT COUNT(*) FROM vless_keys`), 3)
		return
	}
	assertProfileContractSnapshot(t, f.db, before)
}

func TestProfileDuplicateBlindIndexRollsBackCategoryAndSecret(t *testing.T) {
	f := newProfileContractFixture(t)
	before := profileContractSnapshot(t, f.db)
	params := profileContractCreateParams()
	params.Category = "Must roll back"
	key, raw, err := f.repo.CreateLocal(context.Background(), params)
	profileContractNoResult(t, key, raw)
	profileContractError(t, err, keymanagement.ErrProfileCreateConflict)
	assertProfileContractSnapshot(t, f.db, before)
	other, _, err := runProfileContractCommand(f, "create")
	if err != nil {
		t.Fatal(err)
	}
	before = profileContractSnapshot(t, f.db)
	update := f.updateParams()
	update.ID, update.NewURI, update.Category = other.ID, contractProfileURI, "Must roll back"
	key, raw, err = f.repo.UpdateLocal(context.Background(), update)
	profileContractNoResult(t, key, raw)
	profileContractEqual(t, "duplicate update rejected", err != nil, true)
	assertProfileContractSnapshot(t, f.db, before)
}

func TestProfileCloneHasIndependentLocalIdentityAndAllModeAssignments(t *testing.T) {
	f := newProfileContractFixture(t)
	f.makeSourceOwned(t)
	profileContractExec(t, f.db, `INSERT INTO user_keys(user_id, key_id) VALUES(3, ?)`, f.key.ID)
	before := profileContractSnapshot(t, f.db)
	clone, raw, err := runProfileContractCommand(f, "clone")
	if err != nil {
		t.Fatal(err)
	}
	profileContractEqual(t, "new clone ID", clone.ID != f.key.ID, true)
	profileContractEqual(t, "local clone", clone.ExternalSourceID, int64(0))
	profileContractEqual(t, "clone revision", clone.ProfileRevision, int64(1))
	profileContractEqual(t, "clone category", clone.CategoryID, f.key.CategoryID)
	profileContractEqual(t, "clone raw configuration", raw, contractProfileURI)
	var originalIndex, originalEnvelope, cloneIndex, cloneEnvelope string
	profileContractSuccess(t, f.db.QueryRow(`SELECT k.url_blind_index, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON s.vless_key_id=k.id WHERE k.id=?`, f.key.ID).Scan(&originalIndex, &originalEnvelope))
	profileContractSuccess(t, f.db.QueryRow(`SELECT k.url_blind_index, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON s.vless_key_id=k.id WHERE k.id=?`, clone.ID).Scan(&cloneIndex, &cloneEnvelope))
	profileContractEqual(t, "independent clone index", cloneIndex != originalIndex, true)
	profileContractEqual(t, "independent clone ciphertext", cloneEnvelope != originalEnvelope, true)
	_, err = profilestorage.Decrypt(cloneEnvelope, f.keyring, f.key.ID)
	profileContractEqual(t, "ciphertext bound to clone row ID", err != nil, true)
	var sourceRef, fingerprint any
	profileContractSuccess(t, f.db.QueryRow(`SELECT external_key_ref, profile_fingerprint FROM vless_keys WHERE id=?`, clone.ID).Scan(&sourceRef, &fingerprint))
	profileContractEqual(t, "no clone source reference", sourceRef, nil)
	profileContractEqual(t, "no clone source fingerprint", fingerprint, nil)
	rows, err := f.db.Query(`SELECT user_id FROM user_keys WHERE key_id=? ORDER BY user_id`, clone.ID)
	if err != nil {
		t.Fatal(err)
	}
	assignments := profileContractRows(t, rows)
	profileContractEqual(t, "only all-mode clone assignments", assignments, [][]any{{int64(1)}, {int64(2)}})
	after := profileContractSnapshot(t, f.db)
	profileContractEqual(t, "one clone parent", len(after["vless_keys"]), len(before["vless_keys"])+1)
	profileContractEqual(t, "one clone secret", len(after["vless_key_secrets"]), len(before["vless_key_secrets"])+1)
}
