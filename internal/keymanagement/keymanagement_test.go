package keymanagement

import (
	"context"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
)

type fakeRepo struct {
	keys map[int64]*model.VLESSKey
	uris map[int64]string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		keys: make(map[int64]*model.VLESSKey),
		uris: make(map[int64]string),
	}
}

func (f *fakeRepo) GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	key, ok := f.keys[id]
	if !ok {
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	return key, f.uris[id], nil
}

func (f *fakeRepo) CreateLocal(ctx context.Context, params profilepersistence.CreateProfileParams) (*model.VLESSKey, string, error) {
	id := int64(len(f.keys) + 1)
	key := &model.VLESSKey{
		ID:              id,
		Label:           params.Label,
		Status:          params.Status,
		Kind:            params.Kind,
		Category:        params.Category,
		Protocol:        params.Protocol,
		ProfileRevision: 1,
	}
	f.keys[id] = key
	f.uris[id] = params.BuiltURI
	return key, params.BuiltURI, nil
}

func (f *fakeRepo) UpdateLocal(ctx context.Context, params profilepersistence.UpdateProfileParams) (*model.VLESSKey, string, error) {
	key, ok := f.keys[params.ID]
	if !ok {
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	if key.ExternalSourceID > 0 {
		return nil, "", profilepersistence.ErrSourceOwnedProfile
	}
	if key.ProfileRevision != params.ExpectedRevision {
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}
	key.Label = params.Label
	key.Status = params.Status
	key.Kind = params.Kind
	key.Category = params.Category
	key.Protocol = params.Protocol
	key.ProfileRevision++
	f.uris[params.ID] = params.NewURI
	return key, params.NewURI, nil
}

func (f *fakeRepo) CloneLocal(ctx context.Context, params profilepersistence.CloneProfileParams) (*model.VLESSKey, string, error) {
	source, ok := f.keys[params.ID]
	if !ok {
		return nil, "", profilepersistence.ErrProfileNotFound
	}
	if source.ProfileRevision != params.ExpectedRevision {
		return nil, "", profilepersistence.ErrProfileRevisionConflict
	}
	newID := int64(len(f.keys) + 1)
	label := params.NewLabel
	if label == "" {
		label = source.Label + " (Копия)"
	}
	cloned := &model.VLESSKey{
		ID:              newID,
		Label:           label,
		Status:          source.Status,
		Kind:            source.Kind,
		Category:        source.Category,
		Protocol:        source.Protocol,
		ProfileRevision: 1,
	}
	f.keys[newID] = cloned
	f.uris[newID] = f.uris[params.ID]
	return cloned, f.uris[newID], nil
}

func TestKeyManagementService_Create_Reveal_Update_Clone(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	// Create Local SS Key
	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example.com:8443#TestSS"
	detail, err := svc.CreateLocal(ctx, CreateLocalParams{
		Label:        "SS Key",
		Status:       "active",
		Kind:         "real",
		CreationMode: "raw",
		RawURI:       ssURI,
	})
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if detail.ID != 1 || detail.Protocol != "shadowsocks" || detail.ProfileRevision != 1 {
		t.Fatalf("unexpected detail: %#v", detail)
	}

	// Safe detail should NOT contain raw password
	if detail.SafeStructured == nil || detail.SafeStructured.Server != "ss.example.com" {
		t.Fatalf("safe structured profile mismatch: %#v", detail.SafeStructured)
	}

	// Reveal raw secret
	revealRaw, err := svc.Reveal(ctx, RevealParams{
		ID:              detail.ID,
		Target:          "raw",
		ProfileRevision: 1,
	})
	if err != nil {
		t.Fatalf("Reveal raw: %v", err)
	}
	if revealRaw.RawURI != ssURI {
		t.Fatalf("rawURI = %q, want %q", revealRaw.RawURI, ssURI)
	}

	// Reveal structured secrets
	revealStruct, err := svc.Reveal(ctx, RevealParams{
		ID:              detail.ID,
		Target:          "structured-secrets",
		ProfileRevision: 1,
	})
	if err != nil {
		t.Fatalf("Reveal structured: %v", err)
	}
	if revealStruct.Secrets.Password != "MTIzNDU2Nzg5MDEyMzQ1Ng==" {
		t.Fatalf("secrets password = %q, want MTIzNDU2Nzg5MDEyMzQ1Ng==", revealStruct.Secrets.Password)
	}

	// Update with stale revision should fail with conflict
	_, err = svc.UpdateLocal(ctx, UpdateLocalParams{
		ID:              detail.ID,
		ProfileRevision: 99, // stale revision
		Label:           "Updated SS",
		Status:          "active",
		Kind:            "real",
		PatchMode:       "raw",
		RawURI:          ssURI,
	})
	if err != ErrProfileRevisionConflict {
		t.Fatalf("expected ErrProfileRevisionConflict, got %v", err)
	}

	// Update with correct revision
	updatedDetail, err := svc.UpdateLocal(ctx, UpdateLocalParams{
		ID:              detail.ID,
		ProfileRevision: 1,
		Label:           "Updated SS",
		Status:          "active",
		Kind:            "real",
		PatchMode:       "raw",
		RawURI:          ssURI,
	})
	if err != nil {
		t.Fatalf("UpdateLocal: %v", err)
	}
	if updatedDetail.ProfileRevision != 2 || updatedDetail.Label != "Updated SS" {
		t.Fatalf("unexpected updated detail: %#v", updatedDetail)
	}

	// Clone
	clonedDetail, err := svc.CloneAsLocal(ctx, CloneParams{
		ID:                      detail.ID,
		ExpectedProfileRevision: 2,
		NewLabel:                "Cloned SS",
	})
	if err != nil {
		t.Fatalf("CloneAsLocal: %v", err)
	}
	if clonedDetail.ID != 2 || clonedDetail.Label != "Cloned SS" || clonedDetail.ProfileRevision != 1 {
		t.Fatalf("unexpected cloned detail: %#v", clonedDetail)
	}
}

func TestKeyManagementService_EditorSchema(t *testing.T) {
	svc := NewService(newFakeRepo(), nil)
	schema := svc.EditorSchema()
	if len(schema.Protocols) != 3 {
		t.Fatalf("protocols count = %d, want 3", len(schema.Protocols))
	}
	if len(schema.ExclusionReasonCodes) == 0 {
		t.Fatal("exclusion reason catalog empty")
	}
}
