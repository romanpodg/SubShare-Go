package keymanagement

import (
	"context"

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
