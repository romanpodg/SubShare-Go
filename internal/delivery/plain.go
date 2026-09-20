package delivery

import (
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func RenderPlain(entries []Entry, format string, templateData TemplateData) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	lines := make([]string, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		projected, exclusions := plainEntryLines(entry, format, templateData, names)
		result.Exclusions = append(result.Exclusions, exclusions...)
		lines = append(lines, projected...)
	}
	if format == model.SubscriptionFormatXrayJSON {
		body, err := xrayJSONArray(lines)
		if err != nil {
			return Generated{}, err
		}
		result.Body = body
	} else {
		result.Body = strings.Join(lines, "\n")
	}
	result.GeneratedCount = len(lines)
	return result, nil
}

func plainEntryLines(entry Entry, format string, templateData TemplateData, names *UniqueNames) ([]string, []Exclusion) {
	if entry.Kind == model.KeyKindInformational {
		return plainInformationalLines(entry, format, templateData), nil
	}
	if format == model.SubscriptionFormatXrayJSON {
		converted, exclusion := xrayJSONForEntryWithNames(entry, names)
		return converted, exclusionList(exclusion)
	}
	return projectEntryShareLinks(entry, names)
}

func projectEntryShareLinks(entry Entry, names *UniqueNames) ([]string, []Exclusion) {
	if profileconfig.SupportedConfigScheme(entry.Raw) == model.SubscriptionFormatXrayJSON {
		return projectXrayJSONShareLinks(entry, names)
	}
	link, err := ShareURIWithDisplayName(entry.Raw, entry.ClientDisplayName)
	if err != nil || !isShareLink(link) {
		return nil, exclusionList(entry.exclude(entry.protocolName(), "plain", ReasonUnrepresentable))
	}
	return []string{link}, nil
}

// isShareLink reports whether link is a URI-shaped profile a plain
// subscription can carry.
func isShareLink(link string) bool {
	scheme := profileconfig.SupportedConfigScheme(link)
	return scheme != "" && scheme != model.SubscriptionFormatXrayJSON
}

func projectXrayJSONShareLinks(entry Entry, names *UniqueNames) ([]string, []Exclusion) {
	drafts, rejected, err := profileconfig.ProjectXrayJSONDrafts(entry.Raw)
	if err != nil {
		return nil, exclusionList(entry.exclude("xray-json", "plain", ReasonInvalidStored))
	}
	links := make([]string, 0, len(drafts))
	exclusions := make([]Exclusion, 0, rejected)
	for index, draft := range drafts {
		link, ok := shareLinkFromDraft(entry, draft, index, names)
		if !ok {
			exclusions = append(exclusions, *entry.exclude(draft.Protocol, "plain", ReasonUnrepresentable))
			continue
		}
		links = append(links, link)
	}
	for index := 0; index < rejected; index++ {
		exclusions = append(exclusions, *entry.exclude("xray-json", "plain", ReasonUnrepresentable))
	}
	if len(links) == 0 && len(exclusions) == 0 {
		exclusions = append(exclusions, *entry.exclude("xray-json", "plain", ReasonUnsupportedProtocol))
	}
	return links, exclusions
}

func shareLinkFromDraft(entry Entry, draft profileconfig.LinkConfigurationDraft, index int, names *UniqueNames) (string, bool) {
	fallback := draft.Remark
	if entry.ClientDisplayName != "" {
		fallback = entry.ClientDisplayName
	}
	draft.Remark = names.Next(fallback, firstNonEmpty(entry.Label, fmt.Sprintf("proxy-%d", index+1)))
	link, err := profileconfig.BuildShareLinkFromDraft(draft)
	return link, err == nil && isShareLink(link)
}

func ShareURIWithDisplayName(raw, displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || profileconfig.ClientDisplayNameFromKeyURL(raw, "") == displayName {
		return raw, nil
	}
	profile, err := profiles.Parse(raw)
	if err == nil {
		profile.DisplayName = displayName
		serialized, serializeErr := profiles.Serialize(profile, profiles.CanonicalSerialization)
		if serializeErr != nil {
			return "", serializeErr
		}
		return serialized.URI.Reveal(), nil
	}
	draft, err := profileconfig.ParseLinkConfiguration(raw)
	if err != nil {
		return "", err
	}
	draft.Remark = displayName
	return profileconfig.BuildShareLinkFromDraft(draft)
}
