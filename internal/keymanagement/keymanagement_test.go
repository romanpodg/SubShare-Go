package keymanagement

import (
	"context"
	"errors"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

type fakeRepo struct {
	keys            map[int64]*model.VLESSKey
	uris            map[int64]string
	profileGetErr   error
	profileCloneErr error
	legacyGetErr    error
	ensureErr       error
	bulkUpdateErr   error
	bulkDeleteErr   error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		keys: make(map[int64]*model.VLESSKey),
		uris: make(map[int64]string),
	}
}

func newFakeService(repo *fakeRepo) *Service {
	return NewService(repo, nil)
}

func (f *fakeRepo) GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	if f.profileGetErr != nil {
		return nil, "", f.profileGetErr
	}
	key, ok := f.keys[id]
	if !ok {
		return nil, "", ErrKeyNotFound
	}
	return key, f.uris[id], nil
}

func (f *fakeRepo) CreateLocal(ctx context.Context, params CreateProfileParams) (*model.VLESSKey, string, error) {
	id := int64(len(f.keys) + 1)
	key := &model.VLESSKey{
		ID:                          id,
		Label:                       params.Label,
		ClientDisplayName:           params.ClientDisplayName,
		ClientDisplayNameOverridden: params.ClientDisplayName != "",
		Status:                      params.Status,
		Kind:                        params.Kind,
		Category:                    params.Category,
		Protocol:                    params.Protocol,
		ProfileRevision:             1,
	}
	f.keys[id] = key
	f.uris[id] = params.BuiltURI
	return key, params.BuiltURI, nil
}

func (f *fakeRepo) UpdateLocal(ctx context.Context, params UpdateProfileParams) (*model.VLESSKey, string, error) {
	key, ok := f.keys[params.ID]
	if !ok {
		return nil, "", ErrKeyNotFound
	}
	if key.ExternalSourceID > 0 {
		return nil, "", ErrSourceOwnedReadOnly
	}
	if key.ProfileRevision != params.ExpectedRevision {
		return nil, "", ErrProfileRevisionConflict
	}
	key.Label = params.Label
	if params.ClientDisplayName != nil {
		key.ClientDisplayName = *params.ClientDisplayName
		key.ClientDisplayNameOverridden = *params.ClientDisplayName != ""
	}
	key.Status = params.Status
	key.Kind = params.Kind
	key.Category = params.Category
	key.Protocol = params.Protocol
	key.ProfileRevision++
	f.uris[params.ID] = params.NewURI
	return key, params.NewURI, nil
}

func (f *fakeRepo) UpdateSourceOwnedMetadata(ctx context.Context, params UpdateSourceOwnedMetadataParams) (*model.VLESSKey, string, error) {
	key, ok := f.keys[params.ID]
	if !ok {
		return nil, "", ErrKeyNotFound
	}
	if key.ProfileRevision != params.ExpectedRevision {
		return nil, "", ErrProfileRevisionConflict
	}
	key.Status = params.Status
	if params.ClientDisplayName != nil {
		if *params.ClientDisplayName == "" {
			key.ClientDisplayName = profileconfig.EffectiveClientDisplayName("", f.uris[params.ID], key.Label, key.ExternalSourceID > 0)
			key.ClientDisplayNameOverridden = false
		} else {
			key.ClientDisplayName = *params.ClientDisplayName
			key.ClientDisplayNameOverridden = true
		}
	}
	key.ProfileRevision++
	return key, f.uris[params.ID], nil
}

func (f *fakeRepo) CloneLocal(ctx context.Context, params CloneProfileParams) (*model.VLESSKey, string, error) {
	if f.profileCloneErr != nil {
		return nil, "", f.profileCloneErr
	}
	source, ok := f.keys[params.ID]
	if !ok {
		return nil, "", ErrKeyNotFound
	}
	if source.ProfileRevision != params.ExpectedRevision {
		return nil, "", ErrProfileRevisionConflict
	}
	newID := int64(len(f.keys) + 1)
	label := params.NewLabel
	if label == "" {
		label = source.Label + " (Копия)"
	}
	cloned := &model.VLESSKey{
		ID:                          newID,
		Label:                       label,
		ClientDisplayName:           source.ClientDisplayName,
		ClientDisplayNameOverridden: source.ClientDisplayNameOverridden,
		Status:                      source.Status,
		Kind:                        source.Kind,
		Category:                    source.Category,
		Protocol:                    source.Protocol,
		ProfileRevision:             1,
	}
	f.keys[newID] = cloned
	f.uris[newID] = f.uris[params.ID]
	return cloned, f.uris[newID], nil
}

func (f *fakeRepo) ListLegacy(ctx context.Context) ([]model.VLESSKey, error) {
	out := make([]model.VLESSKey, 0, len(f.keys))
	for _, key := range f.keys {
		out = append(out, *key)
	}
	return out, nil
}

func TestSourceOwnedProfileAllowsOnlyLocalStatusAndClientDisplayName(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.keys[1] = &model.VLESSKey{
		ID: 1, Label: "Source A", ClientDisplayName: "Source A", Status: "active", Kind: "real",
		Category: "Provider", Protocol: "vless", ProfileRevision: 1, ExternalSourceID: 77,
	}
	repo.uris[1] = "vless://11111111-1111-4111-8111-111111111111@example.com:443#Source%20A"
	service := newFakeService(repo)

	override := "CUSTOM"
	updated, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 1, Label: "Source A", ClientDisplayName: &override,
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	if err != nil {
		t.Fatalf("client-name-only update: %v", err)
	}
	if updated.Label != "Source A" || updated.ClientDisplayName != override || !updated.ClientDisplayNameOverridden || updated.ProfileRevision != 2 {
		t.Fatalf("updated detail=%#v", updated)
	}

	statusOnly, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 2, Label: "Source A",
		Status: "non-active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	if err != nil {
		t.Fatalf("status-only update: %v", err)
	}
	if statusOnly.Status != "non-active" || statusOnly.ClientDisplayName != override || statusOnly.ProfileRevision != 3 {
		t.Fatalf("status-only detail=%#v", statusOnly)
	}

	reactivated, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 3, Label: "Source A",
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	if err != nil {
		t.Fatalf("reactivate update: %v", err)
	}
	if reactivated.Status != "active" || reactivated.ClientDisplayName != override || reactivated.ProfileRevision != 4 {
		t.Fatalf("reactivated detail=%#v", reactivated)
	}

	mutatedRaw := "vless://11111111-1111-4111-8111-111111111111@attacker.example:443#Changed"
	if _, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 4, Label: "Source A", ClientDisplayName: &override,
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "raw", RawURI: mutatedRaw,
	}); !errors.Is(err, ErrSourceOwnedReadOnly) {
		t.Fatalf("source configuration mutation error=%v", err)
	}
	if repo.uris[1] == mutatedRaw {
		t.Fatal("source configuration was mutated")
	}
	if _, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 4, Label: "Renamed panel label", ClientDisplayName: &override,
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
	}); !errors.Is(err, ErrSourceOwnedReadOnly) {
		t.Fatalf("source label mutation error=%v", err)
	}

	// Simulate a source rename: the explicit local override remains effective.
	repo.keys[1].Label = "Source B"
	repo.uris[1] = "vless://11111111-1111-4111-8111-111111111111@example.com:443#Source%20B"
	if repo.keys[1].ClientDisplayName != "CUSTOM" || !repo.keys[1].ClientDisplayNameOverridden {
		t.Fatalf("source rename lost override: %#v", repo.keys[1])
	}
	empty := ""
	reset, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 4, Label: "Source B", ClientDisplayName: &empty,
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	if err != nil {
		t.Fatalf("reset override: %v", err)
	}
	if reset.ClientDisplayName != "Source B" || reset.ClientDisplayNameOverridden || reset.ProfileRevision != 5 {
		t.Fatalf("reset detail=%#v", reset)
	}
}

func (f *fakeRepo) GetLegacyByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	if f.legacyGetErr != nil {
		return nil, "", f.legacyGetErr
	}
	return f.GetByID(ctx, id)
}

func (f *fakeRepo) CreateLegacy(ctx context.Context, params CreateLegacyKeyParams) (int64, error) {
	id := int64(len(f.keys) + 1)
	key := &model.VLESSKey{
		ID:       id,
		Label:    params.Label,
		Status:   params.Status,
		Kind:     params.Kind,
		Category: params.Category,
		URL:      params.KeyURL,
	}
	f.keys[id] = key
	f.uris[id] = params.KeyURL
	return id, nil
}

func (f *fakeRepo) UpdateLegacy(ctx context.Context, params UpdateLegacyKeyParams) error {
	key, ok := f.keys[params.ID]
	if !ok {
		return ErrKeyNotFound
	}
	key.Label = params.Label
	key.Status = params.Status
	key.Kind = params.Kind
	key.Category = params.Category
	key.URL = params.BuiltURL
	f.uris[params.ID] = params.BuiltURL
	return nil
}

func (f *fakeRepo) DeleteLegacy(ctx context.Context, id int64) error {
	if _, ok := f.keys[id]; !ok {
		return ErrKeyNotFound
	}
	delete(f.keys, id)
	delete(f.uris, id)
	return nil
}

func (f *fakeRepo) ListKeyCategories(ctx context.Context) ([]model.KeyCategory, error) {
	return []model.KeyCategory{}, nil
}

func (f *fakeRepo) CreateKeyCategory(ctx context.Context, params CreateCategoryParams) (model.KeyCategory, error) {
	return model.KeyCategory{ID: 1, Name: params.Name, Color: params.Color}, nil
}

func (f *fakeRepo) UpdateKeyCategory(ctx context.Context, params UpdateCategoryParams) (model.KeyCategory, error) {
	if params.OldName == "notfound" {
		return model.KeyCategory{}, ErrKeyNotFound
	}
	return model.KeyCategory{ID: 1, Name: params.NewName, Color: params.Color}, nil
}

func (f *fakeRepo) DeleteKeyCategory(ctx context.Context, params DeleteCategoryParams) error {
	return nil
}

func (f *fakeRepo) ReorderKeyCategories(ctx context.Context, names []string) error {
	return nil
}

func (f *fakeRepo) ReorderKeys(ctx context.Context, ids []int64) error {
	return nil
}

func (f *fakeRepo) GetCategoryColor(ctx context.Context, name string) (string, error) {
	return "#D8B33D", nil
}

func (f *fakeRepo) EnsureKeyCategory(ctx context.Context, name string) (int64, error) {
	return 1, f.ensureErr
}

func (f *fakeRepo) BulkUpdateKeys(ctx context.Context, params BulkKeyUpdate) error {
	return f.bulkUpdateErr
}

func (f *fakeRepo) BulkDeleteKeys(ctx context.Context, ids []int64) error {
	return f.bulkDeleteErr
}

func (f *fakeRepo) GetHealthCheckTarget(ctx context.Context, id int64) (HealthCheckTarget, error) {
	return HealthCheckTarget{ID: id, URL: f.uris[id]}, nil
}

func (f *fakeRepo) ListHealthCheckTargets(ctx context.Context) ([]HealthCheckTarget, error) {
	return []HealthCheckTarget{}, nil
}

func (f *fakeRepo) SaveHealthCheckResult(ctx context.Context, params SaveHealthCheckResultParams) error {
	return nil
}

func (f *fakeRepo) GetHealthCheckResult(ctx context.Context, id int64) (HealthCheckResult, error) {
	return HealthCheckResult{ID: id}, nil
}

func (f *fakeRepo) ListHealthCheckResults(ctx context.Context) ([]HealthCheckResult, error) {
	return []HealthCheckResult{}, nil
}

func TestKeyManagementService_Create_Reveal_Update_Clone(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newFakeService(repo)

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

func TestKeyManagementService_XrayJSONRawCreateUpdateAndMetadataPreservation(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newFakeService(repo)
	raw := `{"outbounds":[{"protocol":"vless","tag":"Compact","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111"}]}]}}]}`

	detail, err := svc.CreateLocal(ctx, CreateLocalParams{
		Label: "Xray", Status: "active", Kind: "real", CreationMode: "raw", RawURI: raw,
	})
	if err != nil {
		t.Fatalf("CreateLocal XRAY-JSON: %v", err)
	}
	if detail.Protocol != "xray-json" || detail.SafeStructured == nil || detail.SafeStructured.Server != "xray.example" {
		t.Fatalf("unexpected XRAY-JSON detail: %#v", detail)
	}

	updated, err := svc.UpdateLocal(ctx, UpdateLocalParams{
		ID: detail.ID, ProfileRevision: detail.ProfileRevision, Label: "Renamed Xray",
		Status: "active", Kind: "real", PatchMode: "structured", StructuredPatch: &model.StructuredProfilePatch{},
	})
	if err != nil {
		t.Fatalf("metadata-only XRAY-JSON update: %v", err)
	}
	if repo.uris[detail.ID] != raw || updated.Protocol != "xray-json" {
		t.Fatalf("metadata update changed raw/profile: raw=%q detail=%#v", repo.uris[detail.ID], updated)
	}

	if _, err := svc.UpdateLocal(ctx, UpdateLocalParams{
		ID: detail.ID, ProfileRevision: updated.ProfileRevision, Label: "Broken Xray",
		Status: "active", Kind: "real", PatchMode: "raw", RawURI: `{"outbounds":`,
	}); !errors.Is(err, ErrInvalidProfileURI) {
		t.Fatalf("invalid XRAY-JSON update error=%v, want ErrInvalidProfileURI", err)
	}
}

func TestKeyManagementService_EditorSchema(t *testing.T) {
	fake := newFakeRepo()
	svc := newFakeService(fake)
	schema := svc.EditorSchema()
	if len(schema.Protocols) != 3 {
		t.Fatalf("protocols count = %d, want 3", len(schema.Protocols))
	}
	if len(schema.ExclusionReasonCodes) == 0 {
		t.Fatal("exclusion reason catalog empty")
	}
}
