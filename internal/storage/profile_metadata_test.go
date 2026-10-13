package storage

import (
	"context"
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"testing"
)

func TestProfileRepository_ClientDisplayNameMetadataDoesNotRewriteSecretAndClonePreservesIt(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db, newTestKeyringForRepo(t))
	raw := "vless://11111111-1111-4111-8111-111111111111@example.com:443#Embedded"
	created, _, err := repo.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label: "Panel", ClientDisplayName: "Subscriber", Status: "active", Kind: "real", Protocol: "vless", BuiltURI: raw,
	})
	requireFixture(t, err == nil, err)

	var envelopeBefore string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeBefore)
		requireFixture(t, err == nil, err)
	}

	clientName := "Subscriber renamed"
	updated, updatedRaw, err := repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID: created.ID, ExpectedRevision: 1, Label: "Panel", ClientDisplayName: &clientName,
		Status: "active", Kind: "real", Protocol: "vless", NewURI: raw,
	})
	requireFixture(t, err == nil, err)

	var envelopeAfter string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeAfter)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, updatedRaw == raw, "metadata update raw=%q envelope_changed=%v client=%q", updatedRaw, envelopeAfter != envelopeBefore, updated.ClientDisplayName)
	requireFixturef(t, envelopeAfter == envelopeBefore, "metadata update raw=%q envelope_changed=%v client=%q", updatedRaw, envelopeAfter != envelopeBefore, updated.ClientDisplayName)
	requireFixturef(t, updated.ClientDisplayName == clientName, "metadata update raw=%q envelope_changed=%v client=%q", updatedRaw, envelopeAfter != envelopeBefore, updated.ClientDisplayName)

	clone, cloneRaw, err := repo.CloneLocal(ctx, keymanagement.CloneProfileParams{ID: created.ID, ExpectedRevision: 2, NewLabel: "Panel clone"})
	requireFixture(t, err == nil, err)

	requireFixturef(t, cloneRaw == raw, "clone=%#v raw=%q", clone, cloneRaw)
	requireFixturef(t, clone.ClientDisplayName == clientName, "clone=%#v raw=%q", clone, cloneRaw)
	requireFixturef(t, clone.Label == "Panel clone", "clone=%#v raw=%q", clone, cloneRaw)

}

func TestProfileRepository_SourceLocalMetadataDoesNotRewriteSecret(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db, newTestKeyringForRepo(t))
	raw := "vless://11111111-1111-4111-8111-111111111111@example.com:443#Source%20name"
	created, _, err := repo.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label: "Source name", Status: "active", Kind: "real", Protocol: "vless", BuiltURI: raw,
	})
	requireFixture(t, err == nil, err)

	{
		_, err := db.Exec(`INSERT INTO external_subscription_sources(id, name, source_url) VALUES(77, 'Provider', 'https://provider.example/sub')`)
		requireFixture(t, err == nil, err)
	}

	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 77 WHERE id = ?`, created.ID)
		requireFixture(t, err == nil, err)
	}

	var envelopeBefore string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeBefore)
		requireFixture(t, err == nil, err)
	}

	override := "Subscriber override"
	updated, updatedRaw, err := repo.UpdateSourceOwnedMetadata(ctx, keymanagement.UpdateSourceOwnedMetadataParams{
		ID: created.ID, ExpectedRevision: 1, Status: "non-active", ClientDisplayName: &override,
	})
	requireFixture(t, err == nil, err)

	requireFixturef(t, updatedRaw == raw, "updated=%#v raw=%q", updated, updatedRaw)
	requireFixturef(t, updated.Status == "non-active", "updated=%#v raw=%q", updated, updatedRaw)
	requireFixturef(t, updated.ClientDisplayName == override, "updated=%#v raw=%q", updated, updatedRaw)
	requireFixturef(t, updated.ClientDisplayNameOverridden, "updated=%#v raw=%q", updated, updatedRaw)
	requireFixturef(t, updated.ProfileRevision == 2, "updated=%#v raw=%q", updated, updatedRaw)

	var envelopeAfter string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeAfter)
		requireFixture(t, err == nil, err)
	}

	requireFixture(t, envelopeAfter == envelopeBefore, "client display-name override rewrote encrypted configuration")

	clone, cloneRaw, err := repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID: created.ID, ExpectedRevision: 2, NewLabel: "Local clone",
	})
	requireFixture(t, err == nil, err)

	requireFixturef(t, cloneRaw == raw, "clone=%#v raw=%q", clone, cloneRaw)
	requireFixturef(t, clone.ClientDisplayName == override, "clone=%#v raw=%q", clone, cloneRaw)
	requireFixturef(t, clone.ClientDisplayNameOverridden, "clone=%#v raw=%q", clone, cloneRaw)

	empty := ""
	reset, resetRaw, err := repo.UpdateSourceOwnedMetadata(ctx, keymanagement.UpdateSourceOwnedMetadataParams{
		ID: created.ID, ExpectedRevision: 2, Status: "active", ClientDisplayName: &empty,
	})
	requireFixture(t, err == nil, err)

	requireFixturef(t, resetRaw == raw, "reset=%#v raw=%q", reset, resetRaw)
	requireFixturef(t, reset.Status == "active", "reset=%#v raw=%q", reset, resetRaw)
	requireFixturef(t, reset.ClientDisplayName == "Source name", "reset=%#v raw=%q", reset, resetRaw)
	requireFixturef(t, !(reset.ClientDisplayNameOverridden), "reset=%#v raw=%q", reset, resetRaw)

	var storedOverride sql.NullString
	{
		err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, created.ID).Scan(&storedOverride)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, !(storedOverride.Valid), "reset stored override=%#v, want NULL", storedOverride)

	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeAfter)
		requireFixturef(t, err == nil, "reset envelope changed=%v err=%v", envelopeAfter != envelopeBefore, err)
		requireFixturef(t, envelopeAfter == envelopeBefore, "reset envelope changed=%v err=%v", envelopeAfter != envelopeBefore, err)
	}

	fallbackClone, fallbackCloneRaw, err := repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID: created.ID, ExpectedRevision: 3, NewLabel: "Fallback clone",
	})
	requireFixture(t, err == nil, err)

	requireFixturef(t, fallbackCloneRaw == raw, "fallback clone=%#v raw=%q", fallbackClone, fallbackCloneRaw)
	requireFixturef(t, fallbackClone.ClientDisplayName == "Source name", "fallback clone=%#v raw=%q", fallbackClone, fallbackCloneRaw)
	requireFixturef(t, !(fallbackClone.ClientDisplayNameOverridden), "fallback clone=%#v raw=%q", fallbackClone, fallbackCloneRaw)

	var clonedStoredOverride sql.NullString
	{
		err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, fallbackClone.ID).Scan(&clonedStoredOverride)
		requireFixturef(t, err == nil, "fallback clone materialized override=%#v err=%v", clonedStoredOverride, err)
		requireFixturef(t, !(clonedStoredOverride.Valid), "fallback clone materialized override=%#v err=%v", clonedStoredOverride, err)
	}

}

func TestProfileRepository_SourceXrayUsesLabelAndClonePreservesEffectiveName(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	keyring := newTestKeyringForRepo(t)
	repo := NewRepository(db, keyring)
	raw := `{"outbounds":[{"tag":"proxy","protocol":"trojan","settings":{"servers":[{"address":"edge.example","port":443,"password":"secret"}]}}]}`
	created, _, err := repo.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label: "🌟 Human source name", Status: "active", Kind: "real", Protocol: "xray-json", BuiltURI: raw,
	})
	requireFixture(t, err == nil, err)

	{
		_, err := db.Exec(`INSERT INTO external_subscription_sources(id, name, source_url) VALUES(88, 'Provider', 'https://provider.example/sub')`)
		requireFixture(t, err == nil, err)
	}

	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 88 WHERE id = ?`, created.ID)
		requireFixture(t, err == nil, err)
	}

	var envelopeBefore string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeBefore)
		requireFixture(t, err == nil, err)
	}

	loaded, loadedRaw, err := repo.GetByID(ctx, created.ID)
	requireFixture(t, err == nil, err)

	requireFixturef(t, loadedRaw == raw, "source effective name=%q overridden=%v raw_changed=%v", loaded.ClientDisplayName, loaded.ClientDisplayNameOverridden, loadedRaw != raw)
	requireFixturef(t, loaded.ClientDisplayName == "🌟 Human source name", "source effective name=%q overridden=%v raw_changed=%v", loaded.ClientDisplayName, loaded.ClientDisplayNameOverridden, loadedRaw != raw)
	requireFixturef(t, !(loaded.ClientDisplayNameOverridden), "source effective name=%q overridden=%v raw_changed=%v", loaded.ClientDisplayName, loaded.ClientDisplayNameOverridden, loadedRaw != raw)

	list, err := NewRepository(db, keyring).ListLegacy(ctx)
	requireFixturef(t, err == nil, "key list source name=%#v err=%v", list, err)
	requireFixturef(t, len(list) == 1, "key list source name=%#v err=%v", list, err)
	requireFixturef(t, list[0].ClientDisplayName == "🌟 Human source name", "key list source name=%#v err=%v", list, err)

	clone, cloneRaw, err := repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID: created.ID, ExpectedRevision: 1, NewLabel: "Local clone",
	})
	requireFixture(t, err == nil, err)

	requireFixturef(t, cloneRaw == raw, "clone=%#v raw_changed=%v", clone, cloneRaw != raw)
	requireFixturef(t, clone.ClientDisplayName == "🌟 Human source name", "clone=%#v raw_changed=%v", clone, cloneRaw != raw)
	requireFixturef(t, clone.ClientDisplayNameOverridden, "clone=%#v raw_changed=%v", clone, cloneRaw != raw)

	var storedSourceName sql.NullString
	{
		err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, created.ID).Scan(&storedSourceName)
		requireFixturef(t, err == nil, "source fallback was persisted: %#v err=%v", storedSourceName, err)
		requireFixturef(t, !(storedSourceName.Valid), "source fallback was persisted: %#v err=%v", storedSourceName, err)
	}

	var envelopeAfter string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, created.ID).Scan(&envelopeAfter)
		requireFixturef(t, err == nil, "source secret changed=%v err=%v", envelopeAfter != envelopeBefore, err)
		requireFixturef(t, envelopeAfter == envelopeBefore, "source secret changed=%v err=%v", envelopeAfter != envelopeBefore, err)
	}

}
