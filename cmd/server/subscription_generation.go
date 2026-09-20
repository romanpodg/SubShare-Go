package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

func (a *App) deliveryIdentity(raw string) (string, error) {
	return delivery.Identity(raw, a.profileFingerprintKey)
}

type deliverySelection struct {
	Entries       []delivery.Entry
	Exclusions    []delivery.Exclusion
	Settings      model.SubscriptionSettings
	EligibleCount int
}

func (a *App) selectSubscriptionEntries(subscriptionID, responseType string) (deliverySelection, int, string, error) {
	settings, _, _, err := a.effectiveSubscriptionSettings(subscriptionID)
	if err != nil {
		return deliverySelection{}, 0, "", err
	}
	switch responseType {
	case "xray-json":
		settings.SubscriptionFormat = model.SubscriptionFormatXrayJSON
	case "base64", "plain", "mihomo", "sing-box":
		settings.SubscriptionFormat = model.SubscriptionFormatLinks
	}

	stored, err := a.store().ListDeliveryEntries(context.Background(), subscriptionID)
	if err != nil {
		return deliverySelection{}, 0, "", err
	}

	selection := deliverySelection{Settings: settings, Entries: []delivery.Entry{}, Exclusions: []delivery.Exclusion{}}
	seen := make(map[string]struct{})
	for _, row := range stored {
		entry := delivery.Entry{
			ID: row.ID, SourceID: row.SourceID, Raw: row.Raw, Kind: row.Kind, TemplateText: row.TemplateText,
			Label: row.Label, ClientDisplayName: row.ClientDisplayName, StoredProtocol: row.StoredProtocol, Compatibility: row.Compatibility,
		}
		kind, _ := model.NormalizeKeyKind(entry.Kind)
		entry.Kind = kind
		selection.EligibleCount++
		if kind == model.KeyKindInformational {
			selection.Entries = append(selection.Entries, entry)
			continue
		}
		if row.SecretError != nil {
			cause := "decryption_failed"
			if errors.Is(row.SecretError, keymanagement.ErrCredentialMissing) {
				cause = "missing_secret"
			}
			log.Printf("operator_event: row_id=%d source_id=%v reason=profile_storage_integrity_error error=%s", entry.ID, entry.SourceID, cause)
			selection.Exclusions = append(selection.Exclusions, delivery.Exclusion{RecordRef: entry.SafeRef(), Protocol: "unknown", Format: responseType, Reason: "profile_storage_integrity_error"})
			continue
		}
		entry.ClientDisplayName = profileconfig.EffectiveClientDisplayName(
			entry.ClientDisplayName, entry.Raw, entry.Label, entry.SourceID.Valid && entry.SourceID.Int64 > 0,
		)
		protocol := delivery.SafeProtocolName(entry.Raw, entry.StoredProtocol)
		if delivery.HasUnsafeStoredControl(entry.Raw) {
			selection.Exclusions = append(selection.Exclusions, delivery.Exclusion{RecordRef: entry.SafeRef(), Protocol: protocol, Format: responseType, Reason: delivery.ReasonUnsafeControl})
			continue
		}
		if err := delivery.ValidateStoredEntry(entry.Raw); err != nil {
			selection.Exclusions = append(selection.Exclusions, delivery.Exclusion{RecordRef: entry.SafeRef(), Protocol: protocol, Format: responseType, Reason: delivery.ReasonInvalidStored})
			continue
		}
		identity, err := a.deliveryIdentity(entry.Raw)
		if err != nil {
			return deliverySelection{}, 0, "", err
		}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		selection.Entries = append(selection.Entries, entry)
	}
	if selection.EligibleCount == 0 {
		return selection, 503, "subscription has no available keys", nil
	}
	if len(selection.Entries) == 0 {
		allCorrupt := len(selection.Exclusions) > 0
		for _, ex := range selection.Exclusions {
			if ex.Reason != "profile_storage_integrity_error" {
				allCorrupt = false
				break
			}
		}
		if allCorrupt {
			return selection, 503, "subscription has no available keys", nil
		}
	}
	return selection, 0, "", nil
}

func (a *App) generateSelectedSubscription(subscriptionID, responseType string) (delivery.Generated, model.SubscriptionSettings, int, string, error) {
	selection, denyCode, denyReason, err := a.selectSubscriptionEntries(subscriptionID, responseType)
	if err != nil || denyCode != 0 {
		return delivery.Generated{Exclusions: selection.Exclusions}, selection.Settings, denyCode, denyReason, err
	}
	templateData, err := a.buildSubscriptionTemplateData(subscriptionID, selection.Settings.SubscriptionFormat)
	if err != nil {
		return delivery.Generated{}, selection.Settings, 0, "", err
	}
	generated, err := delivery.Render(responseType, selection.Entries, selection.Settings.SubscriptionFormat, templateData)
	generated.Exclusions = append(selection.Exclusions, generated.Exclusions...)
	generated.EligibleCount = selection.EligibleCount
	generated.OutputFormat = delivery.EffectiveFormat(responseType, selection.Settings, a.subscriptionBodyEncoding)
	if err != nil {
		return generated, selection.Settings, 0, "", err
	}
	if generated.GeneratedCount == 0 {
		if delivery.IsStructuredFormat(generated.OutputFormat) && len(generated.Exclusions) > 0 {
			generated.Body = ""
			return generated, selection.Settings, http.StatusUnprocessableEntity, delivery.ReasonAllExcluded, nil
		}
		return generated, selection.Settings, 503, "subscription has no available keys", nil
	}
	return generated, selection.Settings, 0, "", nil
}
