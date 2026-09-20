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
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func (a *App) deliveryIdentity(raw string) (string, error) {
	return delivery.Identity(raw, a.profileFingerprintKey)
}

type deliverySelection struct {
	Entries       []delivery.Entry
	Exclusions    []delivery.Exclusion
	Format        string // effective subscription format for this response type
	EligibleCount int
	RealKeysCount int // deliverable real keys, as informational templates count them
}

func formatFor(responseType string, settings model.SubscriptionSettings) string {
	switch responseType {
	case "xray-json":
		return model.SubscriptionFormatXrayJSON
	case "base64", "plain", "mihomo", "sing-box":
		return model.SubscriptionFormatLinks
	}
	return settings.SubscriptionFormat
}

func (a *App) selectSubscriptionEntries(ctx subscriptionDeliveryContext, responseType string) (deliverySelection, subscriptionDenial, error) {
	format := formatFor(responseType, ctx.Settings)
	stored, err := a.store().ListDeliveryEntries(context.Background(), ctx.SubscriptionID)
	if err != nil {
		return deliverySelection{}, subscriptionDenial{}, err
	}

	selection := deliverySelection{Format: format, Entries: []delivery.Entry{}, Exclusions: []delivery.Exclusion{}}
	seen := make(map[string]struct{})
	for _, row := range stored {
		if countsAsRealKey(row, format) {
			selection.RealKeysCount++
		}
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
			log.Printf("operator_event: row_id=%d source_id=%v reason=profile_storage_integrity_error error=%s", entry.ID, entry.SourceID, secretErrorCause(row.SecretError))
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
			return deliverySelection{}, subscriptionDenial{}, err
		}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		selection.Entries = append(selection.Entries, entry)
	}
	if selection.EligibleCount == 0 {
		return selection, deny(503, "", "subscription has no available keys"), nil
	}
	if len(selection.Entries) == 0 && allIntegrityExclusions(selection.Exclusions) {
		return selection, deny(503, "", "subscription has no available keys"), nil
	}
	return selection, subscriptionDenial{}, nil
}

// countsAsRealKey reports whether a stored row is a deliverable real key for
// the given format, as informational templates count them.
func countsAsRealKey(row storage.DeliveryEntry, format string) bool {
	return row.Kind == model.KeyKindReal && row.SecretError == nil &&
		(format != model.SubscriptionFormatLinks || profileconfig.SupportedConfigScheme(row.Raw) != model.SubscriptionFormatXrayJSON)
}

func secretErrorCause(err error) string {
	if errors.Is(err, keymanagement.ErrCredentialMissing) {
		return "missing_secret"
	}
	return "decryption_failed"
}

// allIntegrityExclusions reports whether there is at least one exclusion and
// every one of them is a profile storage integrity error.
func allIntegrityExclusions(exclusions []delivery.Exclusion) bool {
	if len(exclusions) == 0 {
		return false
	}
	for _, ex := range exclusions {
		if ex.Reason != "profile_storage_integrity_error" {
			return false
		}
	}
	return true
}

// generateSubscription renders the subscription for an already prepared
// request context: no settings or user rows are re-read.
func (a *App) generateSubscription(ctx subscriptionDeliveryContext, responseType string) (delivery.Generated, subscriptionDenial, error) {
	selection, denial, err := a.selectSubscriptionEntries(ctx, responseType)
	if err != nil || denial.denied() {
		return delivery.Generated{Exclusions: selection.Exclusions}, denial, err
	}
	generated, err := delivery.Render(responseType, selection.Entries, selection.Format, ctx.templateData(selection.RealKeysCount))
	generated.Exclusions = append(selection.Exclusions, generated.Exclusions...)
	generated.EligibleCount = selection.EligibleCount
	settings := ctx.Settings
	settings.SubscriptionFormat = selection.Format
	generated.OutputFormat = delivery.EffectiveFormat(responseType, settings, a.subscriptionBodyEncoding)
	if err != nil {
		return generated, subscriptionDenial{}, err
	}
	if generated.GeneratedCount == 0 {
		if delivery.IsStructuredFormat(generated.OutputFormat) && len(generated.Exclusions) > 0 {
			generated.Body = ""
			return generated, deny(http.StatusUnprocessableEntity, "", delivery.ReasonAllExcluded), nil
		}
		return generated, deny(503, "", "subscription has no available keys"), nil
	}
	return generated, subscriptionDenial{}, nil
}

// generateSelectedSubscription loads the request context itself. Handlers use
// generateSubscription with the context they already prepared.
func (a *App) generateSelectedSubscription(subscriptionID, responseType string) (delivery.Generated, subscriptionDenial, error) {
	ctx, err := a.loadSubscriptionContext(subscriptionID)
	if err != nil {
		return delivery.Generated{}, subscriptionDenial{}, err
	}
	return a.generateSubscription(ctx, responseType)
}
