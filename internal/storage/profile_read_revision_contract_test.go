package storage

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestProfileMetadataUpdatesPreserveCiphertextAndBlindIndex(t *testing.T) {
	for _, ownership := range []string{"local", "source"} {
		for _, field := range []string{"name", "status", "name reset", "omitted name"} {
			t.Run(ownership+"/"+field, func(t *testing.T) {
				assertProfileMetadataContract(t, ownership, field)
			})
		}
	}
}

func profileMetadataParams(f profileContractFixture, ownership, field string) (keymanagement.UpdateLocalParams, string) {
	params := keymanagement.UpdateLocalParams{ID: f.key.ID, ProfileRevision: 1, Label: "Panel", Status: "active", Kind: "real", Category: "Original", PatchMode: "structured"}
	if ownership == "local" {
		params.StructuredPatch = &model.StructuredProfilePatch{}
	}
	name := "Subscriber"
	switch field {
	case "name":
		params.ClientDisplayName = &name
		return params, name
	case "status":
		params.Status = "non-active"
	case "name reset":
		name = ""
		params.ClientDisplayName = &name
		return params, "Panel"
	}
	return params, "Original override"
}

func assertProfileMetadataContract(t *testing.T, ownership, field string) {
	t.Helper()
	f := newProfileContractFixture(t)
	if ownership == "source" {
		f.makeSourceOwned(t)
	}
	profileContractExec(t, f.db, `UPDATE vless_keys SET client_display_name='Original override' WHERE id=?`, f.key.ID)
	before := profileContractSnapshot(t, f.db)
	var indexBefore, indexAfter string
	profileContractSuccess(t, f.db.QueryRow(`SELECT url_blind_index FROM vless_keys WHERE id=?`, f.key.ID).Scan(&indexBefore))
	service := keymanagement.NewService(f.repo, nil)
	params, wantName := profileMetadataParams(f, ownership, field)
	detail, err := service.UpdateLocal(context.Background(), params)
	profileContractSuccess(t, err)
	profileContractEqual(t, "metadata revision", detail.ProfileRevision, int64(2))
	profileContractEqual(t, "metadata status", detail.Status, params.Status)
	after := profileContractSnapshot(t, f.db)
	profileContractEqual(t, "unchanged ciphertext", after["vless_key_secrets"], before["vless_key_secrets"])
	profileContractSuccess(t, f.db.QueryRow(`SELECT url_blind_index FROM vless_keys WHERE id=?`, f.key.ID).Scan(&indexAfter))
	profileContractEqual(t, "unchanged blind index", indexAfter, indexBefore)
	key, raw, err := f.repo.GetByID(context.Background(), f.key.ID)
	profileContractSuccess(t, err)
	profileContractEqual(t, "unchanged raw bytes", raw, contractProfileURI)
	profileContractEqual(t, "unchanged category", key.CategoryID, f.key.CategoryID)
	profileContractEqual(t, "display name", key.ClientDisplayName, wantName)
	result, err := service.UpdateLocal(context.Background(), params)
	profileContractEqual(t, "no stale detail", result == nil, true)
	profileContractError(t, err, keymanagement.ErrProfileRevisionConflict)
	assertProfileContractSnapshot(t, f.db, after)
}

func TestProfileConcurrentSameRevisionHasOneWinner(t *testing.T) {
	for _, ownership := range []string{"local", "source"} {
		t.Run(ownership, func(t *testing.T) {
			f := newProfileContractFixture(t)
			if ownership == "source" {
				f.makeSourceOwned(t)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			start := make(chan struct{})
			ready := make(chan struct{}, 2)
			results := make(chan error, 2)
			for i := 0; i < 2; i++ {
				go func() {
					ready <- struct{}{}
					<-start
					results <- runProfileRevisionContender(ctx, f, ownership)
				}()
			}
			<-ready
			<-ready
			close(start)
			counts := make(map[string]int)
			for i := 0; i < 2; i++ {
				select {
				case err := <-results:
					counts[profileRevisionResultKind(t, err)]++
				case <-ctx.Done():
					t.Fatal("revision contenders did not complete")
				}
			}
			key, raw, err := f.repo.GetByID(context.Background(), f.key.ID)
			profileContractSuccess(t, err)
			profileContractEqual(t, "one winner and one conflict", counts, map[string]int{"winner": 1, "conflict": 1})
			profileContractEqual(t, "one revision increment", key.ProfileRevision, int64(2))
			profileContractEqual(t, "unchanged concurrent metadata bytes", raw, contractProfileURI)
		})
	}
}

func runProfileRevisionContender(ctx context.Context, f profileContractFixture, ownership string) error {
	if ownership == "local" {
		p := f.updateParams()
		p.NewURI = contractProfileURI
		_, _, err := f.repo.UpdateLocal(ctx, p)
		return err
	}
	_, _, err := f.repo.UpdateSourceOwnedMetadata(ctx, keymanagement.UpdateSourceOwnedMetadataParams{ID: f.key.ID, ExpectedRevision: 1, Status: "non-active"})
	return err
}

func profileRevisionResultKind(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return "winner"
	}
	profileContractError(t, err, keymanagement.ErrProfileRevisionConflict)
	return "conflict"
}

func TestProfileReadsFailClosedForMissingCorruptAndWrongRowSecrets(t *testing.T) {
	for _, fault := range []string{"missing", "corrupt", "wrong row", "unavailable key"} {
		t.Run(fault, func(t *testing.T) {
			f := newProfileContractFixture(t)
			want := keymanagement.ErrStorageIntegrity
			switch fault {
			case "missing":
				profileContractExec(t, f.db, `DELETE FROM vless_key_secrets WHERE vless_key_id=?`, f.key.ID)
			case "corrupt":
				profileContractExec(t, f.db, `UPDATE vless_key_secrets SET encrypted_url='v1:key-1:corrupt' WHERE vless_key_id=?`, f.key.ID)
			case "wrong row":
				other, _, err := runProfileContractCommand(f, "create")
				if err != nil {
					t.Fatal(err)
				}
				profileContractExec(t, f.db, `UPDATE vless_key_secrets SET encrypted_url=(SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id=?) WHERE vless_key_id=?`, other.ID, f.key.ID)
			case "unavailable key":
				f.repo = NewRepository(f.db, nil)
				want = keymanagement.ErrEncryptionUnavailable
			}
			before := profileContractSnapshot(t, f.db)
			key, raw, err := f.repo.GetByID(context.Background(), f.key.ID)
			profileContractNoResult(t, key, raw)
			profileContractError(t, err, want)
			list, err := f.repo.ListLegacy(context.Background())
			profileContractEqual(t, "no partial list", list == nil, true)
			profileContractError(t, err, want)
			service := keymanagement.NewService(f.repo, nil)
			detail, err := service.GetDetail(context.Background(), f.key.ID)
			profileContractEqual(t, "no unsafe detail", detail == nil, true)
			profileContractError(t, err, want)
			reveal, err := service.Reveal(context.Background(), keymanagement.RevealParams{ID: f.key.ID, Target: "raw", ProfileRevision: 1})
			profileContractEqual(t, "no unsafe reveal", reveal == nil, true)
			profileContractError(t, err, want)
			clone, raw, err := runProfileContractCommand(f, "clone")
			profileContractNoResult(t, clone, raw)
			profileContractError(t, err, want)
			updated, raw, err := runProfileContractCommand(f, "update")
			profileContractNoResult(t, updated, raw)
			profileContractError(t, err, want)
			assertProfileContractSnapshot(t, f.db, before)
		})
	}
}

func TestProfileSourceProjectionSafeDetailAndRevealParity(t *testing.T) {
	f := newProfileContractFixture(t)
	f.makeSourceOwned(t)
	profileContractExec(t, f.db, `UPDATE vless_keys SET profile_warnings_json='invalid-json', updated_at='invalid-time', check_error=? WHERE id=?`, contractProfileURI, f.key.ID)
	key, raw, err := f.repo.GetByID(context.Background(), f.key.ID)
	profileContractSuccess(t, err)
	profileContractEqual(t, "source raw bytes", raw, contractProfileURI)
	profileContractEqual(t, "trimmed source name", key.ExternalSourceName, "Provider")
	profileContractEqual(t, "source label fallback", key.ClientDisplayName, "Panel")
	profileContractEqual(t, "empty invalid warnings", key.ProfileWarnings, []string{})
	profileContractEqual(t, "invalid update time fallback", key.UpdatedAt.Equal(key.CreatedAt), true)
	list, err := f.repo.ListLegacy(context.Background())
	profileContractSuccess(t, err)
	profileContractEqual(t, "one listed source", len(list), 1)
	profileContractEqual(t, "list/get source ID", list[0].ExternalSourceID, key.ExternalSourceID)
	profileContractEqual(t, "list/get source name", list[0].ExternalSourceName, key.ExternalSourceName)
	profileContractEqual(t, "list/get display name", list[0].ClientDisplayName, key.ClientDisplayName)
	profileContractEqual(t, "list/get raw bytes", list[0].URL, raw)
	service := keymanagement.NewService(f.repo, nil)
	detail, err := service.GetDetail(context.Background(), f.key.ID)
	profileContractSuccess(t, err)
	encoded, err := json.Marshal(detail)
	profileContractSuccess(t, err)
	for _, secret := range []string{contractProfileURI, "contract-auth", "private-value", "encrypted_url", "raw_uri"} {
		profileContractEqual(t, "no secret or unknown value in detail", strings.Contains(string(encoded), secret), false)
	}
	profileContractEqual(t, "safe source ownership", detail.Ownership, model.OwnershipExternalSource)
	profileContractEqual(t, "unknown parameter count", len(detail.UnknownQueryParameters), 1)
	profileContractEqual(t, "unknown parameter key only", detail.UnknownQueryParameters[0].Key, "x-extra")
	reveal, err := service.Reveal(context.Background(), keymanagement.RevealParams{ID: f.key.ID, Target: "raw", ProfileRevision: 1})
	profileContractSuccess(t, err)
	profileContractEqual(t, "explicit reveal raw bytes", reveal.RawURI, raw)
	profileContractEqual(t, "confirmed reveal revision", reveal.ConfirmedProfileRevision, int64(1))
	// Reproduce a missing joined source without modifying production FK policy.
	profileContractExec(t, f.db, `PRAGMA foreign_keys=OFF`)
	profileContractExec(t, f.db, `DELETE FROM external_subscription_sources WHERE id=77`)
	key, raw, err = f.repo.GetByID(context.Background(), f.key.ID)
	profileContractSuccess(t, err)
	profileContractEqual(t, "orphan source ID retained", key.ExternalSourceID, int64(77))
	profileContractEqual(t, "orphan source name empty", key.ExternalSourceName, "")
	profileContractEqual(t, "orphan source display fallback", key.ClientDisplayName, "Panel")
	profileContractEqual(t, "orphan source raw bytes", raw, contractProfileURI)
}
