package keymanagement

import (
	"context"
	"errors"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

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
	requireFixturef(t, err == nil, "CreateLocal: %v", err)

	requireFixturef(t, detail.ID == 1, "unexpected detail: %#v", detail)
	requireFixturef(t, detail.Protocol == "shadowsocks", "unexpected detail: %#v", detail)
	requireFixturef(t, detail.ProfileRevision == 1, "unexpected detail: %#v", detail)

	// Safe detail should NOT contain raw password
	requireFixturef(t, detail.SafeStructured != nil, "safe structured profile mismatch: %#v", detail.SafeStructured)
	requireFixturef(t, detail.SafeStructured.Server == "ss.example.com", "safe structured profile mismatch: %#v", detail.SafeStructured)

	// Reveal raw secret
	revealRaw, err := svc.Reveal(ctx, RevealParams{
		ID:              detail.ID,
		Target:          "raw",
		ProfileRevision: 1,
	})
	requireFixturef(t, err == nil, "Reveal raw: %v", err)

	requireFixturef(t, revealRaw.RawURI == ssURI, "rawURI = %q, want %q", revealRaw.RawURI, ssURI)

	// Reveal structured secrets
	revealStruct, err := svc.Reveal(ctx, RevealParams{
		ID:              detail.ID,
		Target:          "structured-secrets",
		ProfileRevision: 1,
	})
	requireFixturef(t, err == nil, "Reveal structured: %v", err)

	requireFixturef(t, revealStruct.Secrets.Password == "MTIzNDU2Nzg5MDEyMzQ1Ng==", "secrets password = %q, want MTIzNDU2Nzg5MDEyMzQ1Ng==", revealStruct.Secrets.Password)

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
	requireFixturef(t, err == ErrProfileRevisionConflict, "expected ErrProfileRevisionConflict, got %v", err)

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
	requireFixturef(t, err == nil, "UpdateLocal: %v", err)

	requireFixturef(t, updatedDetail.ProfileRevision == 2, "unexpected updated detail: %#v", updatedDetail)
	requireFixturef(t, updatedDetail.Label == "Updated SS", "unexpected updated detail: %#v", updatedDetail)

	// Clone
	clonedDetail, err := svc.CloneAsLocal(ctx, CloneParams{
		ID:                      detail.ID,
		ExpectedProfileRevision: 2,
		NewLabel:                "Cloned SS",
	})
	requireFixturef(t, err == nil, "CloneAsLocal: %v", err)

	requireFixturef(t, clonedDetail.ID == 2, "unexpected cloned detail: %#v", clonedDetail)
	requireFixturef(t, clonedDetail.Label == "Cloned SS", "unexpected cloned detail: %#v", clonedDetail)
	requireFixturef(t, clonedDetail.ProfileRevision == 1, "unexpected cloned detail: %#v", clonedDetail)

}

func TestKeyManagementService_XrayJSONRawCreateUpdateAndMetadataPreservation(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newFakeService(repo)
	raw := `{"outbounds":[{"protocol":"vless","tag":"Compact","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111"}]}]}}]}`

	detail, err := svc.CreateLocal(ctx, CreateLocalParams{
		Label: "Xray", Status: "active", Kind: "real", CreationMode: "raw", RawURI: raw,
	})
	requireFixturef(t, err == nil, "CreateLocal XRAY-JSON: %v", err)

	requireFixturef(t, detail.Protocol == "xray-json", "unexpected XRAY-JSON detail: %#v", detail)
	requireFixturef(t, detail.SafeStructured != nil, "unexpected XRAY-JSON detail: %#v", detail)
	requireFixturef(t, detail.SafeStructured.Server == "xray.example", "unexpected XRAY-JSON detail: %#v", detail)

	updated, err := svc.UpdateLocal(ctx, UpdateLocalParams{
		ID: detail.ID, ProfileRevision: detail.ProfileRevision, Label: "Renamed Xray",
		Status: "active", Kind: "real", PatchMode: "structured", StructuredPatch: &model.StructuredProfilePatch{},
	})
	requireFixturef(t, err == nil, "metadata-only XRAY-JSON update: %v", err)

	requireFixturef(t, repo.uris[detail.ID] == raw, "metadata update changed raw/profile: raw=%q detail=%#v", repo.uris[detail.ID], updated)
	requireFixturef(t, updated.Protocol == "xray-json", "metadata update changed raw/profile: raw=%q detail=%#v", repo.uris[detail.ID], updated)

	{
		_, err := svc.UpdateLocal(ctx, UpdateLocalParams{
			ID: detail.ID, ProfileRevision: updated.ProfileRevision, Label: "Broken Xray",
			Status: "active", Kind: "real", PatchMode: "raw", RawURI: `{"outbounds":`,
		})
		requireFixturef(t, errors.Is(err, ErrInvalidProfileURI), "invalid XRAY-JSON update error=%v, want ErrInvalidProfileURI", err)
	}

}

func TestKeyManagementService_EditorSchema(t *testing.T) {
	fake := newFakeRepo()
	svc := newFakeService(fake)
	schema := svc.EditorSchema()
	requireFixturef(t, len(schema.Protocols) == 3, "protocols count = %d, want 3", len(schema.Protocols))

	requireFixture(t, len(schema.ExclusionReasonCodes) != 0, "exclusion reason catalog empty")

}
