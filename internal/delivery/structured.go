package delivery

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

// structuredMapper is what distinguishes one structured client output from
// another: how a parsed profile or a legacy link draft becomes a document
// item, and under which key the items are emitted.
type structuredMapper struct {
	format     string
	payloadKey string
	profile    func(*profiles.Profile, string) (map[string]any, string)
	legacy     func(profileconfig.LinkConfigurationDraft, string) map[string]any
}

// renderStructured is the shared mihomo/sing-box loop: every non-informational
// entry becomes zero or more items or exactly one exclusion.
func renderStructured(entries []Entry, mapper structuredMapper) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	items := make([]map[string]any, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		converted, exclusion := mapper.convert(entry, names)
		result.Exclusions = append(result.Exclusions, exclusionList(exclusion)...)
		items = append(items, converted...)
	}
	payload, err := json.MarshalIndent(map[string]any{mapper.payloadKey: items}, "", "  ")
	if err != nil {
		return result, errors.New(ReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(items)
	return result, nil
}

func (mapper structuredMapper) convert(entry Entry, names *UniqueNames) ([]map[string]any, *Exclusion) {
	if isProfileScheme(profileconfig.SupportedConfigScheme(entry.Raw)) {
		return mapper.convertProfile(entry, names)
	}
	drafts, err := entryDrafts(entry.Raw)
	if err != nil {
		return nil, entry.exclude(entry.protocolName(), mapper.format, ReasonInvalidStored)
	}
	items := make([]map[string]any, 0, len(drafts))
	for index, draft := range drafts {
		name := names.Next(firstNonEmpty(entry.ClientDisplayName, draftDisplayName(draft, index)), entry.Label)
		items = append(items, mapper.legacy(draft, name))
	}
	return items, nil
}

func (mapper structuredMapper) convertProfile(entry Entry, names *UniqueNames) ([]map[string]any, *Exclusion) {
	profile, exclusion := structuredProfile(entry, mapper.format)
	if exclusion != nil {
		return nil, exclusion
	}
	item, reason := mapper.profile(profile, profileName(entry, profile, names))
	if reason != "" {
		return nil, entry.exclude(string(profile.Protocol), mapper.format, reason)
	}
	return []map[string]any{item}, nil
}

// isProfileScheme names the schemes handled by the typed profiles parser
// rather than the legacy link drafts.
func isProfileScheme(scheme string) bool {
	switch scheme {
	case "ss", "hysteria2", "hy2", "tuic":
		return true
	}
	return false
}

func structuredProfile(entry Entry, format string) (*profiles.Profile, *Exclusion) {
	profile, err := profiles.Parse(entry.Raw)
	if err != nil {
		return nil, entry.exclude(entry.protocolName(), format, ReasonInvalidStored)
	}
	if len(profile.UnknownQueryParameters) > 0 {
		return nil, entry.exclude(string(profile.Protocol), format, ReasonUnrepresentable)
	}
	for _, warning := range profile.Warnings {
		if isAmbiguousWarning(warning.Code) {
			return nil, entry.exclude(string(profile.Protocol), format, ReasonAmbiguous)
		}
	}
	return profile, nil
}

func isAmbiguousWarning(code string) bool {
	switch code {
	case profiles.WarningDuplicateParameter, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference:
		return true
	}
	return false
}

func entryDrafts(raw string) ([]profileconfig.LinkConfigurationDraft, error) {
	if profileconfig.SupportedConfigScheme(raw) == model.SubscriptionFormatXrayJSON {
		return profileconfig.ParseXrayJSONDrafts(raw)
	}
	draft, err := profileconfig.ParseLinkConfiguration(raw)
	if err != nil {
		return nil, err
	}
	return []profileconfig.LinkConfigurationDraft{draft}, nil
}

func portNumber(port profiles.PortSpec) int {
	if len(port.Ranges) == 0 {
		return 0
	}
	return int(port.Ranges[0].Start)
}

func tuicHasFieldClass(data profiles.TUICData, class profiles.TUICFieldProvenance) bool {
	for _, observation := range data.FieldObservations {
		if observation.FieldClass == class {
			return true
		}
	}
	return false
}

func durationMilliseconds(raw string) (int64, bool) {
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return 0, false
	}
	if duration%time.Millisecond != 0 {
		return 0, false
	}
	return duration.Milliseconds(), true
}
