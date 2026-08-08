package keymanagement

import (
	"context"
	"errors"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
)

func TestService_StrictCredentialErrorsReachProfileAndLegacyOperations(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newFakeService(repo)

	repo.profileGetErr = profilepersistence.ErrStorageIntegrity
	if _, err := svc.GetDetail(ctx, 1); !errors.Is(err, ErrStorageIntegrity) {
		t.Fatalf("detail error = %v", err)
	}
	if _, err := svc.Reveal(ctx, RevealParams{ID: 1, Target: "raw", ProfileRevision: 1}); !errors.Is(err, ErrStorageIntegrity) {
		t.Fatalf("reveal error = %v", err)
	}
	if _, err := svc.UpdateLocal(ctx, UpdateLocalParams{ID: 1}); !errors.Is(err, ErrStorageIntegrity) {
		t.Fatalf("update error = %v", err)
	}

	repo.profileGetErr = nil
	repo.profileCloneErr = profilepersistence.ErrStorageIntegrity
	if _, err := svc.CloneAsLocal(ctx, CloneParams{ID: 1, ExpectedProfileRevision: 1}); !errors.Is(err, ErrStorageIntegrity) {
		t.Fatalf("clone error = %v", err)
	}

	repo.legacyGetErr = keypersistence.ErrStorageIntegrity
	_, err := svc.UpdateLegacy(ctx, 1, UpdateLegacyParams{
		Label: "Legacy", Kind: "real", Status: "active",
		RawURL: "vless://11111111-1111-1111-1111-111111111111@example.com:443#legacy",
	})
	if !errors.Is(err, ErrStorageIntegrity) {
		t.Fatalf("legacy update error = %v", err)
	}
}

func TestService_BulkValidationAndPersistenceErrorTranslation(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newFakeService(repo)

	tests := []struct {
		name   string
		params BulkUpdateKeysParams
		want   error
	}{
		{name: "empty", params: BulkUpdateKeysParams{Status: "active"}, want: ErrBulkKeyIDsRequired},
		{name: "invalid id", params: BulkUpdateKeysParams{IDs: []int64{0}, Status: "active"}, want: ErrInvalidBulkKeyID},
		{name: "duplicate", params: BulkUpdateKeysParams{IDs: []int64{1, 1}, Status: "active"}, want: ErrDuplicateBulkKeyIDs},
		{name: "invalid status", params: BulkUpdateKeysParams{IDs: []int64{1}, Status: "invalid"}, want: ErrInvalidKeyStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := svc.BulkUpdateKeys(ctx, test.params); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}

	repo.ensureErr = keypersistence.ErrEncryptionUnavailable
	if _, err := svc.BulkUpdateKeys(ctx, BulkUpdateKeysParams{IDs: []int64{1}, Status: "active", Category: "Category"}); !errors.Is(err, ErrEncryptionUnavailable) {
		t.Fatalf("category persistence error = %v", err)
	}
	repo.ensureErr = nil
	repo.bulkUpdateErr = keypersistence.KeyNotFoundError{ID: 42}
	if _, err := svc.BulkUpdateKeys(ctx, BulkUpdateKeysParams{IDs: []int64{1}, Status: "active"}); err == nil {
		t.Fatal("expected missing-key error")
	} else if id, ok := MissingKeyID(err); !ok || id != 42 {
		t.Fatalf("missing key identity = (%d, %v), err=%v", id, ok, err)
	}

	repo.bulkDeleteErr = keypersistence.KeyNotFoundError{ID: 77}
	if _, err := svc.BulkDeleteKeys(ctx, []int64{1}); err == nil {
		t.Fatal("expected bulk delete missing-key error")
	} else if id, ok := MissingKeyID(err); !ok || id != 77 {
		t.Fatalf("missing delete identity = (%d, %v), err=%v", id, ok, err)
	}
}
