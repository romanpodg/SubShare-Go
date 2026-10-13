package keymanagement

import (
	"context"
	"errors"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

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
	requireFixturef(t, err == nil, "client-name-only update: %v", err)

	requireFixturef(t, updated.Label == "Source A", "updated detail=%#v", updated)
	requireFixturef(t, updated.ClientDisplayName == override, "updated detail=%#v", updated)
	requireFixturef(t, updated.ClientDisplayNameOverridden, "updated detail=%#v", updated)
	requireFixturef(t, updated.ProfileRevision == 2, "updated detail=%#v", updated)

	statusOnly, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 2, Label: "Source A",
		Status: "non-active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	requireFixturef(t, err == nil, "status-only update: %v", err)

	requireFixturef(t, statusOnly.Status == "non-active", "status-only detail=%#v", statusOnly)
	requireFixturef(t, statusOnly.ClientDisplayName == override, "status-only detail=%#v", statusOnly)
	requireFixturef(t, statusOnly.ProfileRevision == 3, "status-only detail=%#v", statusOnly)

	reactivated, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 3, Label: "Source A",
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	requireFixturef(t, err == nil, "reactivate update: %v", err)

	requireFixturef(t, reactivated.Status == "active", "reactivated detail=%#v", reactivated)
	requireFixturef(t, reactivated.ClientDisplayName == override, "reactivated detail=%#v", reactivated)
	requireFixturef(t, reactivated.ProfileRevision == 4, "reactivated detail=%#v", reactivated)

	mutatedRaw := "vless://11111111-1111-4111-8111-111111111111@attacker.example:443#Changed"
	{
		_, err := service.UpdateLocal(ctx, UpdateLocalParams{
			ID: 1, ProfileRevision: 4, Label: "Source A", ClientDisplayName: &override,
			Status: "active", Kind: "real", Category: "Provider", PatchMode: "raw", RawURI: mutatedRaw,
		})
		requireFixturef(t, errors.Is(err, ErrSourceOwnedReadOnly), "source configuration mutation error=%v", err)
	}

	requireFixture(t, repo.uris[1] != mutatedRaw, "source configuration was mutated")

	{
		_, err := service.UpdateLocal(ctx, UpdateLocalParams{
			ID: 1, ProfileRevision: 4, Label: "Renamed panel label", ClientDisplayName: &override,
			Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
		})
		requireFixturef(t, errors.Is(err, ErrSourceOwnedReadOnly), "source label mutation error=%v", err)
	}

	// Simulate a source rename: the explicit local override remains effective.
	repo.keys[1].Label = "Source B"
	repo.uris[1] = "vless://11111111-1111-4111-8111-111111111111@example.com:443#Source%20B"
	requireFixturef(t, repo.keys[1].ClientDisplayName == "CUSTOM", "source rename lost override: %#v", repo.keys[1])
	requireFixturef(t, repo.keys[1].ClientDisplayNameOverridden, "source rename lost override: %#v", repo.keys[1])

	empty := ""
	reset, err := service.UpdateLocal(ctx, UpdateLocalParams{
		ID: 1, ProfileRevision: 4, Label: "Source B", ClientDisplayName: &empty,
		Status: "active", Kind: "real", Category: "Provider", PatchMode: "structured",
	})
	requireFixturef(t, err == nil, "reset override: %v", err)

	requireFixturef(t, reset.ClientDisplayName == "Source B", "reset detail=%#v", reset)
	requireFixturef(t, !(reset.ClientDisplayNameOverridden), "reset detail=%#v", reset)
	requireFixturef(t, reset.ProfileRevision == 5, "reset detail=%#v", reset)

}
