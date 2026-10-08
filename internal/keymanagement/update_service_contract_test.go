package keymanagement

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type updateContractRepo struct {
	*fakeRepo
	localCalls, sourceCalls int
	writeErr                error
	localParams             UpdateProfileParams
	sourceParams            UpdateSourceOwnedMetadataParams
}

func updateContractEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from contract", label)
	}
}

func updateContractError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error=%v want=%v", err, want)
	}
}

func (r *updateContractRepo) UpdateLocal(ctx context.Context, params UpdateProfileParams) (*model.VLESSKey, string, error) {
	r.localCalls++
	r.localParams = params
	if r.writeErr != nil {
		return nil, "", r.writeErr
	}
	return r.fakeRepo.UpdateLocal(ctx, params)
}

func (r *updateContractRepo) UpdateSourceOwnedMetadata(ctx context.Context, params UpdateSourceOwnedMetadataParams) (*model.VLESSKey, string, error) {
	r.sourceCalls++
	r.sourceParams = params
	if r.writeErr != nil {
		return nil, "", r.writeErr
	}
	return r.fakeRepo.UpdateSourceOwnedMetadata(ctx, params)
}

func newUpdateContractService(source bool) (*Service, *updateContractRepo, UpdateLocalParams) {
	r := &updateContractRepo{fakeRepo: newFakeRepo()}
	r.keys[1] = &model.VLESSKey{ID: 1, Label: "Panel", Status: "active", Kind: "real", Category: "Provider", CategoryID: 8, Protocol: "hysteria2", ProfileRevision: 3}
	r.uris[1] = patchHY2
	if source {
		r.keys[1].ExternalSourceID = 77
	}
	params := UpdateLocalParams{ID: 1, ProfileRevision: 3, Label: "Panel", Status: "active", Kind: "real", Category: "Provider"}
	return NewService(r, nil), r, params
}

func TestUpdateServiceSourceOwnershipAndRevisionMatrix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*UpdateLocalParams)
		want   error
	}{
		{"status allowed", func(p *UpdateLocalParams) { p.Status = "non-active" }, nil},
		{"name allowed", func(p *UpdateLocalParams) { v := "Subscriber"; p.ClientDisplayName = &v }, nil},
		{"name reset allowed", func(p *UpdateLocalParams) { v := ""; p.ClientDisplayName = &v }, nil},
		{"matching category ID allowed", func(p *UpdateLocalParams) { v := int64(8); p.CategoryID = &v }, nil},
		{"normalized metadata allowed", func(p *UpdateLocalParams) {
			p.Label = " Panel "
			p.Category = " Provider "
			p.PatchMode = " STRUCTURED "
		}, nil},
		{"label rejected", func(p *UpdateLocalParams) { p.Label = "Renamed" }, ErrSourceOwnedReadOnly},
		{"kind rejected", func(p *UpdateLocalParams) { p.Kind = "info" }, ErrSourceOwnedReadOnly},
		{"category rejected", func(p *UpdateLocalParams) { p.Category = "Other" }, ErrSourceOwnedReadOnly},
		{"category ID rejected", func(p *UpdateLocalParams) { v := int64(9); p.CategoryID = &v }, ErrSourceOwnedReadOnly},
		{"zero category ID rejected", func(p *UpdateLocalParams) { v := int64(0); p.CategoryID = &v }, ErrSourceOwnedReadOnly},
		{"template rejected", func(p *UpdateLocalParams) { p.TemplateText = "new template" }, ErrSourceOwnedReadOnly},
		{"raw replacement rejected", func(p *UpdateLocalParams) { p.RawURI = patchHY2 }, ErrSourceOwnedReadOnly},
		{"empty structured patch rejected", func(p *UpdateLocalParams) { p.StructuredPatch = &model.StructuredProfilePatch{} }, ErrSourceOwnedReadOnly},
		{"raw mode rejected", func(p *UpdateLocalParams) { p.PatchMode = "raw" }, ErrSourceOwnedReadOnly},
		{"invalid mode rejected", func(p *UpdateLocalParams) { p.PatchMode = "other" }, ErrSourceOwnedReadOnly},
		{"invalid status rejected", func(p *UpdateLocalParams) { p.Status = "other" }, ErrSourceOwnedReadOnly},
		{"long name rejected", func(p *UpdateLocalParams) { v := strings.Repeat("x", 256); p.ClientDisplayName = &v }, ErrLabelTooLong},
		{"ownership precedes long name", func(p *UpdateLocalParams) {
			p.Label = "Renamed"
			v := strings.Repeat("x", 256)
			p.ClientDisplayName = &v
		}, ErrSourceOwnedReadOnly},
		{"stale revision precedes ownership", func(p *UpdateLocalParams) { p.ProfileRevision = 2; p.Label = "Renamed" }, ErrProfileRevisionConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, repo, params := newUpdateContractService(true)
			before := *repo.keys[1]
			tc.mutate(&params)
			result, err := service.UpdateLocal(context.Background(), params)
			updateContractError(t, err, tc.want)
			updateContractEqual(t, "no local writes", repo.localCalls, 0)
			if tc.want != nil {
				updateContractEqual(t, "no rejected detail", result == nil, true)
				updateContractEqual(t, "no source writes", repo.sourceCalls, 0)
				updateContractEqual(t, "unchanged row", *repo.keys[1], before)
				updateContractEqual(t, "unchanged raw bytes", repo.uris[1], patchHY2)
				return
			}
			updateContractEqual(t, "one source write", repo.sourceCalls, 1)
			updateContractEqual(t, "revision increment", result.ProfileRevision, int64(4))
			updateContractEqual(t, "source ownership", result.Ownership, model.OwnershipExternalSource)
			updateContractEqual(t, "unchanged raw bytes", repo.uris[1], patchHY2)
		})
	}
}

func TestUpdateServiceLocalPatchModeMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, mode, raw string
		patch           *model.StructuredProfilePatch
		want            error
	}{
		{"implicit raw", "", patchHY2, nil, nil},
		{"normalized raw", " RAW ", " \n" + patchHY2 + "\n ", nil, nil},
		{"implicit structured", "", "", &model.StructuredProfilePatch{}, nil},
		{"explicit structured", "structured", "", &model.StructuredProfilePatch{}, nil},
		{"missing raw", "raw", "", nil, ErrRawURIRequired},
		{"implicit whitespace raw", "", " \n ", nil, ErrRawURIRequired},
		{"missing structured", "structured", "", nil, ErrStructuredPatchRequired},
		{"invalid mode", "other", "", nil, ErrInvalidPatchMode},
		{"raw with structured", "raw", patchHY2, &model.StructuredProfilePatch{}, ErrMutuallyExclusiveMode},
		{"structured with raw", "structured", patchHY2, &model.StructuredProfilePatch{}, ErrMutuallyExclusiveMode},
		{"invalid raw URI", "raw", "malformed secret bytes", nil, ErrInvalidProfileURI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, repo, params := newUpdateContractService(false)
			before := *repo.keys[1]
			params.PatchMode, params.RawURI, params.StructuredPatch = tc.mode, tc.raw, tc.patch
			result, err := service.UpdateLocal(context.Background(), params)
			updateContractError(t, err, tc.want)
			updateContractEqual(t, "no source writes", repo.sourceCalls, 0)
			if tc.want != nil {
				updateContractEqual(t, "no rejected detail", result == nil, true)
				updateContractEqual(t, "no local writes", repo.localCalls, 0)
				updateContractEqual(t, "unchanged row", *repo.keys[1], before)
				updateContractEqual(t, "unchanged raw bytes", repo.uris[1], patchHY2)
				return
			}
			updateContractEqual(t, "one local write", repo.localCalls, 1)
			updateContractEqual(t, "revision increment", result.ProfileRevision, int64(4))
			updateContractEqual(t, "unchanged raw bytes", repo.uris[1], patchHY2)
		})
	}
}

func TestUpdateServiceDisplayNameResetDependsOnOwnership(t *testing.T) {
	for _, source := range []bool{false, true} {
		service, repo, params := newUpdateContractService(source)
		reset := "  "
		params.ClientDisplayName = &reset
		want := ""
		if !source {
			params.StructuredPatch = &model.StructuredProfilePatch{}
			want = "Panel"
		}
		_, err := service.UpdateLocal(context.Background(), params)
		updateContractError(t, err, nil)
		name := repo.localParams.ClientDisplayName
		if source {
			name = repo.sourceParams.ClientDisplayName
		}
		if name == nil {
			t.Fatal("reset omitted the display name command field")
		}
		updateContractEqual(t, "display name reset command", *name, want)
		updateContractEqual(t, "reset leaves configuration bytes", repo.uris[1], patchHY2)
	}
}

func TestUpdateServiceLocalDisplayNameErrorPrecedesInvalidStatus(t *testing.T) {
	service, repo, params := newUpdateContractService(false)
	name := strings.Repeat("x", 256)
	params.ClientDisplayName = &name
	params.Status = "invalid"
	params.StructuredPatch = &model.StructuredProfilePatch{}
	result, err := service.UpdateLocal(context.Background(), params)
	updateContractError(t, err, ErrLabelTooLong)
	updateContractEqual(t, "no rejected detail", result == nil, true)
	updateContractEqual(t, "no invalid writes", repo.localCalls+repo.sourceCalls, 0)
}

func TestUpdateServicePersistenceFailuresReturnNoDetail(t *testing.T) {
	for _, source := range []bool{false, true} {
		for _, failure := range []error{ErrProfileRevisionConflict, ErrStorageIntegrity, ErrEncryptionUnavailable, errors.New("injected write failure")} {
			t.Run(fmtServiceFailureName(source, failure), func(t *testing.T) {
				service, repo, params := newUpdateContractService(source)
				if !source {
					params.StructuredPatch = &model.StructuredProfilePatch{}
				}
				repo.writeErr = failure
				result, err := service.UpdateLocal(context.Background(), params)
				updateContractError(t, err, failure)
				updateContractEqual(t, "no detail on write failure", result == nil, true)
				updateContractEqual(t, "one attempted write", repo.localCalls+repo.sourceCalls, 1)
				updateContractEqual(t, "unchanged revision", repo.keys[1].ProfileRevision, int64(3))
			})
		}
	}
}

func fmtServiceFailureName(source bool, err error) string {
	if source {
		return "source/" + err.Error()
	}
	return "local/" + err.Error()
}
