// Package delivery renders a subscription body from stored profile entries.
// It has no database, keyring or HTTP dependencies: callers select the entries
// and pass them in; the module owns every output format and its exclusion
// policy.
package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"gopkg.in/yaml.v3"
)

const (
	TargetMihomoVersion      = "1.19.28"
	TargetSingBoxVersion     = "1.13.12"
	TargetXrayMinimumVersion = "26.3.27"
	TargetXrayCurrentVersion = "26.7.28"
)

func ExclusionReasonCodes() []string {
	return []string{
		ReasonAmbiguous,
		ReasonClientVersion,
		ReasonCompatibility,
		ReasonUnrepresentable,
		ReasonInvalidStored,
		ReasonSerialization,
		ReasonUnsupportedProtocol,
		ReasonPlugin,
		ReasonUnsafeControl,
	}
}

const (
	ReasonUnsupportedProtocol = "protocol_unsupported_by_format"
	ReasonClientVersion       = "client_version_unsupported"
	ReasonPlugin              = "required_plugin_unsupported"
	ReasonCompatibility       = "compatibility_only_profile"
	ReasonInvalidStored       = "invalid_stored_raw_uri"
	ReasonUnsafeControl       = "unsafe_control_character"
	ReasonUnrepresentable     = "field_not_representable"
	ReasonAmbiguous           = "ambiguous_profile"
	ReasonSerialization       = "generator_serialization_failure"
	ReasonAllExcluded         = "all_profiles_excluded"
)

type CapabilitySupport string

const (
	CapabilitySupported     CapabilitySupport = "supported"
	CapabilityUnsupported   CapabilitySupport = "unsupported"
	CapabilityConditional   CapabilitySupport = "conditionally_supported"
	CapabilityCompatibility CapabilitySupport = "compatibility_only"
)

type OutputCapability struct {
	Status                  CapabilitySupport `json:"status"`
	ReasonCode              string            `json:"reason_code,omitempty"`
	TargetVersion           string            `json:"target_version,omitempty"`
	MinimumVersion          string            `json:"minimum_version,omitempty"`
	SyntaxValidation        string            `json:"syntax_validation,omitempty"`
	RuntimeInteroperability string            `json:"runtime_interoperability,omitempty"`
}

type ProtocolCapability struct {
	Protocol   string                      `json:"protocol"`
	Generation string                      `json:"generation"`
	Outputs    map[string]OutputCapability `json:"outputs"`
}

// CapabilityMatrix is the single backend source of truth for
// delivery, editing, and probing claims. Structured generators are pinned to
// explicit client schemas; an absent generator is never interpreted as support.

// CapabilityMatrix is the single backend source of truth for
// delivery, editing, and probing claims. Structured generators are pinned to
// explicit client schemas; an absent generator is never interpreted as support.
func CapabilityMatrix() []ProtocolCapability {
	structural := func(version string) OutputCapability {
		return OutputCapability{Status: CapabilitySupported, TargetVersion: version, SyntaxValidation: "structurally_generated", RuntimeInteroperability: "not_tested"}
	}
	clientSupported := func(version, minimum string) OutputCapability {
		return OutputCapability{Status: CapabilitySupported, TargetVersion: version, MinimumVersion: minimum, SyntaxValidation: "official_binary", RuntimeInteroperability: "not_tested"}
	}
	conditional := func(reason, version, minimum, syntax string) OutputCapability {
		return OutputCapability{Status: CapabilityConditional, ReasonCode: reason, TargetVersion: version, MinimumVersion: minimum, SyntaxValidation: syntax, RuntimeInteroperability: "not_tested"}
	}
	unsupported := func(reason, version string) OutputCapability {
		return OutputCapability{Status: CapabilityUnsupported, ReasonCode: reason, TargetVersion: version}
	}
	compatibility := func(reason string) OutputCapability {
		return OutputCapability{Status: CapabilityCompatibility, ReasonCode: reason}
	}
	legacy := func(protocol string) ProtocolCapability {
		return ProtocolCapability{Protocol: protocol, Generation: "current", Outputs: map[string]OutputCapability{
			"plain": structural("sip-uri"), "base64": structural("whole-body-standard-base64"),
			"mihomo": clientSupported(TargetMihomoVersion, TargetMihomoVersion), "sing-box": clientSupported(TargetSingBoxVersion, TargetSingBoxVersion),
			"xray-json": clientSupported(TargetXrayCurrentVersion, TargetXrayMinimumVersion), "structured-editing": structural("subshare-current"),
			"connectivity-probe": conditional("tcp_reachability_only", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(ReasonUnsupportedProtocol, ""),
		}}
	}
	matrix := []ProtocolCapability{legacy("vless"), legacy("vmess"), legacy("trojan")}
	matrix = append(matrix,
		ProtocolCapability{Protocol: "shadowsocks", Generation: "sip002/sip022", Outputs: map[string]OutputCapability{
			"plain": structural("sip002/sip022"), "base64": structural("whole-body-standard-base64"),
			"mihomo":             conditional("method_or_plugin_dependent", TargetMihomoVersion, TargetMihomoVersion, "official_binary"),
			"sing-box":           conditional("method_or_plugin_dependent", TargetSingBoxVersion, TargetSingBoxVersion, "official_binary"),
			"xray-json":          conditional("plugin_free_only", TargetXrayCurrentVersion, TargetXrayMinimumVersion, "official_binary"),
			"structured-editing": unsupported("frontend_editor_deferred", ""),
			"connectivity-probe": conditional("tcp_reachability_only", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(ReasonUnsupportedProtocol, ""),
		}},
		ProtocolCapability{Protocol: "hysteria2", Generation: "2", Outputs: map[string]OutputCapability{
			"plain": structural("hysteria2-uri"), "base64": structural("whole-body-standard-base64"),
			"mihomo":             conditional("field_and_version_dependent", TargetMihomoVersion, TargetMihomoVersion, "official_binary"),
			"sing-box":           conditional("field_and_version_dependent", TargetSingBoxVersion, TargetSingBoxVersion, "official_binary"),
			"xray-json":          conditional("obfuscation_or_extension_dependent", TargetXrayCurrentVersion, TargetXrayMinimumVersion, "official_binary"),
			"structured-editing": unsupported("frontend_editor_deferred", ""),
			"connectivity-probe": conditional("dns_only_udp_quic_probe_unavailable", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(ReasonUnsupportedProtocol, ""),
		}},
		ProtocolCapability{Protocol: "tuic", Generation: "5", Outputs: map[string]OutputCapability{
			"plain": structural("compatibility-uri"), "base64": structural("whole-body-standard-base64"),
			"mihomo":             conditional("provenance_and_field_dependent", TargetMihomoVersion, TargetMihomoVersion, "official_binary"),
			"sing-box":           conditional("provenance_and_field_dependent", TargetSingBoxVersion, TargetSingBoxVersion, "official_binary"),
			"xray-json":          unsupported(ReasonUnsupportedProtocol, TargetXrayCurrentVersion),
			"structured-editing": unsupported("frontend_editor_deferred", ""),
			"connectivity-probe": conditional("dns_only_udp_quic_probe_unavailable", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(ReasonUnsupportedProtocol, ""),
		}},
		ProtocolCapability{Protocol: "tuic", Generation: "4", Outputs: map[string]OutputCapability{
			"plain": compatibility("raw_delivery_only"), "base64": compatibility("raw_delivery_only"),
			"mihomo":             unsupported(ReasonCompatibility, TargetMihomoVersion),
			"sing-box":           unsupported(ReasonCompatibility, TargetSingBoxVersion),
			"xray-json":          unsupported(ReasonCompatibility, TargetXrayCurrentVersion),
			"structured-editing": unsupported(ReasonCompatibility, ""),
			"connectivity-probe": conditional("dns_only_udp_quic_probe_unavailable", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  compatibility("raw_delivery_only"),
		}},
	)
	return matrix
}

type Exclusion struct {
	RecordRef string `json:"record_ref"`
	Protocol  string `json:"protocol"`
	Format    string `json:"format"`
	Reason    string `json:"reason_code"`
}

func (item Exclusion) Error() string {
	return "subscription_generation_exclusion{record_ref=" + item.RecordRef + ", protocol=" + item.Protocol + ", format=" + item.Format + ", reason=" + item.Reason + "}"
}

type Entry struct {
	ID                int64
	SourceID          sql.NullInt64
	Raw               string
	Kind              string
	TemplateText      string
	Label             string
	ClientDisplayName string
	StoredProtocol    string
	Compatibility     string
}

func (entry Entry) SafeRef() string { return "key:" + strconv.FormatInt(entry.ID, 10) }

type Generated struct {
	Body           string
	Exclusions     []Exclusion
	OutputFormat   string
	EligibleCount  int
	GeneratedCount int
}

type Failure struct {
	ErrorCode       string         `json:"error_code"`
	OutputFormat    string         `json:"output_format"`
	EligibleCount   int            `json:"eligible_count"`
	ExcludedCount   int            `json:"excluded_count"`
	ExclusionCounts map[string]int `json:"exclusion_counts"`
}

func HasUnsafeControl(raw string) bool {
	for _, char := range raw {
		if char == 0x7f || char < 0x20 {
			return true
		}
	}
	return false
}

func HasUnsafeStoredControl(raw string) bool {
	if profileconfig.SupportedConfigScheme(raw) != model.SubscriptionFormatXrayJSON {
		return HasUnsafeControl(raw)
	}
	for _, char := range raw {
		if char == 0x7f || (char < 0x20 && char != '\n' && char != '\r' && char != '\t') {
			return true
		}
	}
	return false
}

func SafeProtocolName(raw, fallback string) string {
	scheme := profileconfig.SupportedConfigScheme(raw)
	if scheme == "" {
		scheme = strings.ToLower(strings.TrimSpace(fallback))
	}
	if scheme == "hy2" {
		return "hysteria2"
	}
	if scheme == "ss" {
		return "shadowsocks"
	}
	return scheme
}

// Identity is the delivery-time dedupe identity of a stored profile: a keyed
// fingerprint when the URI parses as a profile, otherwise a keyed digest of
// the exact bytes.
func Identity(raw string, key []byte) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if profile, err := profiles.Parse(raw); err == nil {
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, key)
		if fingerprintErr != nil {
			return "", fmt.Errorf("profile_fingerprint_failed")
		}
		return "profile:" + fingerprint, nil
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte("subshare-delivery-raw-v1\x00"))
	_, _ = digest.Write([]byte(raw))
	return "raw:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func ValidateStoredEntry(raw string) error {
	switch profileconfig.SupportedConfigScheme(raw) {
	case "vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic":
		if _, err := profiles.Parse(raw); err == nil {
			return nil
		}
		if _, err := profileconfig.ParseLinkConfiguration(raw); err == nil {
			return nil
		}
		return errors.New(ReasonInvalidStored)
	case model.SubscriptionFormatXrayJSON:
		var root map[string]any
		if err := json.Unmarshal([]byte(raw), &root); err != nil || len(profileconfig.AsArray(root["outbounds"])) == 0 {
			return errors.New(ReasonInvalidStored)
		}
		return nil
	default:
		return errors.New(ReasonInvalidStored)
	}
}

func EffectiveFormat(responseType string, settings model.SubscriptionSettings, defaultEncoding string) string {
	if responseType != "" {
		return responseType
	}
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		return "xray-json"
	}
	if defaultEncoding == "base64" {
		return "base64"
	}
	return "plain"
}

func IsStructuredFormat(format string) bool {
	switch format {
	case "mihomo", "sing-box", "xray-json":
		return true
	default:
		return false
	}
}

// Render produces the subscription body for responseType. Informational
// entries are expanded with tpl; subscriptionFormat only matters for the
// plain/base64 path.
func Render(responseType string, entries []Entry, subscriptionFormat string, tpl TemplateData) (Generated, error) {
	switch responseType {
	case "mihomo":
		return RenderMihomo(MaterializeInformational(entries, tpl, false))
	case "sing-box":
		return RenderSingBox(MaterializeInformational(entries, tpl, false))
	case "xray-json":
		return RenderXray(MaterializeInformational(entries, tpl, true))
	default:
		return RenderPlain(entries, subscriptionFormat, tpl)
	}
}

// MaterializeInformational turns informational entries into synthetic real
// entries rendered from tpl, dropping those that render to nothing.
func MaterializeInformational(entries []Entry, templateData TemplateData, xray bool) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != model.KeyKindInformational {
			result = append(result, entry)
			continue
		}
		textTemplate := strings.TrimSpace(entry.TemplateText)
		if textTemplate == "" {
			textTemplate = strings.TrimSpace(entry.Label)
		}
		rendered := RenderInfoTemplate(textTemplate, templateData)
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		entry.Kind = model.KeyKindReal
		if xray {
			entry.Raw = InformationalXrayJSON(rendered)
			entry.StoredProtocol = "xray-json"
		} else {
			entry.Raw = InformationalVLESSURL(rendered)
			entry.StoredProtocol = "vless"
		}
		result = append(result, entry)
	}
	return result
}

func RenderPlain(entries []Entry, format string, templateData TemplateData) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	lines := make([]string, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			textTemplate := strings.TrimSpace(entry.TemplateText)
			if textTemplate == "" {
				textTemplate = strings.TrimSpace(entry.Label)
			}
			rendered := RenderInfoTemplate(textTemplate, templateData)
			if strings.TrimSpace(rendered) == "" {
				continue
			}
			if format == model.SubscriptionFormatXrayJSON {
				if info := InformationalXrayJSON(rendered); strings.TrimSpace(info) != "" {
					lines = append(lines, info)
				}
			} else {
				lines = append(lines, InformationalVLESSURL(rendered))
			}
			continue
		}
		if format == model.SubscriptionFormatXrayJSON {
			converted, exclusion := xrayJSONForEntryWithNames(entry, names)
			if exclusion != nil {
				result.Exclusions = append(result.Exclusions, *exclusion)
				continue
			}
			lines = append(lines, converted...)
			continue
		}
		projected, exclusions := projectEntryShareLinks(entry, names)
		result.Exclusions = append(result.Exclusions, exclusions...)
		lines = append(lines, projected...)
	}
	if format == model.SubscriptionFormatXrayJSON {
		items := make([]json.RawMessage, 0, len(lines))
		for _, line := range lines {
			if !json.Valid([]byte(line)) {
				return Generated{}, errors.New(ReasonSerialization)
			}
			items = append(items, json.RawMessage(line))
		}
		payload, marshalErr := json.Marshal(items)
		if marshalErr != nil {
			return Generated{}, errors.New(ReasonSerialization)
		}
		result.Body = string(payload)
		result.GeneratedCount = len(items)
		return result, nil
	}
	result.Body = strings.Join(lines, "\n")
	result.GeneratedCount = len(lines)
	return result, nil
}

func projectEntryShareLinks(entry Entry, names *UniqueNames) ([]string, []Exclusion) {
	if profileconfig.SupportedConfigScheme(entry.Raw) == model.SubscriptionFormatXrayJSON {
		drafts, rejected, err := profileconfig.ProjectXrayJSONDrafts(entry.Raw)
		if err != nil {
			return nil, []Exclusion{{entry.SafeRef(), "xray-json", "plain", ReasonInvalidStored}}
		}
		links := make([]string, 0, len(drafts))
		exclusions := make([]Exclusion, 0, rejected)
		for index, draft := range drafts {
			fallback := draft.Remark
			if entry.ClientDisplayName != "" {
				fallback = entry.ClientDisplayName
			}
			draft.Remark = names.Next(fallback, firstNonEmpty(entry.Label, fmt.Sprintf("proxy-%d", index+1)))
			link, buildErr := profileconfig.BuildShareLinkFromDraft(draft)
			if buildErr != nil || profileconfig.SupportedConfigScheme(link) == "" || profileconfig.SupportedConfigScheme(link) == model.SubscriptionFormatXrayJSON {
				exclusions = append(exclusions, Exclusion{entry.SafeRef(), draft.Protocol, "plain", ReasonUnrepresentable})
				continue
			}
			links = append(links, link)
		}
		for index := 0; index < rejected; index++ {
			exclusions = append(exclusions, Exclusion{entry.SafeRef(), "xray-json", "plain", ReasonUnrepresentable})
		}
		if len(links) == 0 && len(exclusions) == 0 {
			exclusions = append(exclusions, Exclusion{entry.SafeRef(), "xray-json", "plain", ReasonUnsupportedProtocol})
		}
		return links, exclusions
	}

	link, err := ShareURIWithDisplayName(entry.Raw, entry.ClientDisplayName)
	if err != nil || profileconfig.SupportedConfigScheme(link) == "" || profileconfig.SupportedConfigScheme(link) == model.SubscriptionFormatXrayJSON {
		return nil, []Exclusion{{entry.SafeRef(), SafeProtocolName(entry.Raw, entry.StoredProtocol), "plain", ReasonUnrepresentable}}
	}
	return []string{link}, nil
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

func structuredProfile(entry Entry, format string) (*profiles.Profile, *Exclusion) {
	profile, err := profiles.Parse(entry.Raw)
	if err != nil {
		return nil, &Exclusion{entry.SafeRef(), SafeProtocolName(entry.Raw, entry.StoredProtocol), format, ReasonInvalidStored}
	}
	if len(profile.UnknownQueryParameters) > 0 {
		return nil, &Exclusion{entry.SafeRef(), string(profile.Protocol), format, ReasonUnrepresentable}
	}
	for _, warning := range profile.Warnings {
		if warning.Code == profiles.WarningDuplicateParameter || warning.Code == profiles.WarningAmbiguousParameter || warning.Code == profiles.WarningConflictingPreference {
			return nil, &Exclusion{entry.SafeRef(), string(profile.Protocol), format, ReasonAmbiguous}
		}
	}
	return profile, nil
}

type UniqueNames struct{ counts map[string]int }

func (names *UniqueNames) Next(raw, fallback string) string {
	name := safeGeneratedName(raw)
	if name == "" {
		name = safeGeneratedName(fallback)
	}
	if name == "" {
		name = "proxy"
	}
	if names.counts == nil {
		names.counts = make(map[string]int)
	}
	names.counts[name]++
	if names.counts[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s (%d)", name, names.counts[name])
}

func safeGeneratedName(raw string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(raw) {
		if char == 0x7f || unicode.IsControl(char) {
			continue
		}
		builder.WriteRune(char)
	}
	return strings.TrimSpace(builder.String())
}

func profileName(entry Entry, profile *profiles.Profile, names *UniqueNames) string {
	return names.Next(firstNonEmpty(entry.ClientDisplayName, profile.DisplayName), firstNonEmpty(entry.Label, string(profile.Protocol)+"-"+profile.Server))
}

func SyntheticEntries(raw string) []Entry {
	parts := splitSubscriptionEntries(raw)
	entries := make([]Entry, 0, len(parts))
	for index, part := range parts {
		entries = append(entries, Entry{ID: int64(index + 1), Raw: part, Kind: model.KeyKindReal, Label: fmt.Sprintf("proxy-%d", index+1)})
	}
	return entries
}

func RenderMihomo(entries []Entry) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	proxiesOut := make([]map[string]any, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		scheme := profileconfig.SupportedConfigScheme(entry.Raw)
		if scheme == "ss" || scheme == "hysteria2" || scheme == "hy2" || scheme == "tuic" {
			profile, exclusion := structuredProfile(entry, "mihomo")
			if exclusion != nil {
				result.Exclusions = append(result.Exclusions, *exclusion)
				continue
			}
			proxy, reason := mihomoProxy(profile, profileName(entry, profile, names))
			if reason != "" {
				result.Exclusions = append(result.Exclusions, Exclusion{entry.SafeRef(), string(profile.Protocol), "mihomo", reason})
				continue
			}
			proxiesOut = append(proxiesOut, proxy)
			continue
		}
		drafts, err := entryDrafts(entry.Raw)
		if err != nil {
			result.Exclusions = append(result.Exclusions, Exclusion{entry.SafeRef(), SafeProtocolName(entry.Raw, entry.StoredProtocol), "mihomo", ReasonInvalidStored})
			continue
		}
		for index, draft := range drafts {
			name := names.Next(firstNonEmpty(entry.ClientDisplayName, draftDisplayName(draft, index)), entry.Label)
			proxiesOut = append(proxiesOut, mihomoLegacyProxy(draft, name))
		}
	}
	payload, err := json.MarshalIndent(map[string]any{"proxies": proxiesOut}, "", "  ")
	if err != nil {
		return result, errors.New(ReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(proxiesOut)
	return result, nil
}

func RenderSingBox(entries []Entry) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	outbounds := make([]map[string]any, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		scheme := profileconfig.SupportedConfigScheme(entry.Raw)
		if scheme == "ss" || scheme == "hysteria2" || scheme == "hy2" || scheme == "tuic" {
			profile, exclusion := structuredProfile(entry, "sing-box")
			if exclusion != nil {
				result.Exclusions = append(result.Exclusions, *exclusion)
				continue
			}
			outbound, reason := singBoxOutbound(profile, profileName(entry, profile, names))
			if reason != "" {
				result.Exclusions = append(result.Exclusions, Exclusion{entry.SafeRef(), string(profile.Protocol), "sing-box", reason})
				continue
			}
			outbounds = append(outbounds, outbound)
			continue
		}
		drafts, err := entryDrafts(entry.Raw)
		if err != nil {
			result.Exclusions = append(result.Exclusions, Exclusion{entry.SafeRef(), SafeProtocolName(entry.Raw, entry.StoredProtocol), "sing-box", ReasonInvalidStored})
			continue
		}
		for index, draft := range drafts {
			name := names.Next(firstNonEmpty(entry.ClientDisplayName, draftDisplayName(draft, index)), entry.Label)
			outbounds = append(outbounds, singBoxLegacyOutbound(draft, name))
		}
	}
	payload, err := json.MarshalIndent(map[string]any{"outbounds": outbounds}, "", "  ")
	if err != nil {
		return result, errors.New(ReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(outbounds)
	return result, nil
}

func RenderXray(entries []Entry) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	items := make([]json.RawMessage, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		converted, exclusion := xrayJSONForEntryWithNames(entry, names)
		if exclusion != nil {
			result.Exclusions = append(result.Exclusions, *exclusion)
			continue
		}
		for _, item := range converted {
			if !json.Valid([]byte(item)) {
				return result, errors.New(ReasonSerialization)
			}
			items = append(items, json.RawMessage(item))
		}
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return result, errors.New(ReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(items)
	return result, nil
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

func mihomoLegacyProxy(draft profileconfig.LinkConfigurationDraft, name string) map[string]any {
	proxy := map[string]any{"name": name, "type": draft.Protocol, "server": draft.Server, "port": draft.Port, "udp": true}
	switch draft.Protocol {
	case "vless":
		proxy["uuid"] = draft.Identifier
		if draft.Flow != "" {
			proxy["flow"] = draft.Flow
		}
	case "vmess":
		proxy["uuid"] = draft.Identifier
		alterID, _ := strconv.Atoi(firstNonEmpty(draft.VMessAlterID, "0"))
		proxy["alterId"] = alterID
		proxy["cipher"] = firstNonEmpty(draft.VMessSecurity, "auto")
	case "trojan":
		proxy["password"] = draft.Identifier
	}
	security := strings.ToLower(strings.TrimSpace(draft.Security))
	if security == "tls" || security == "reality" {
		proxy["tls"] = true
		if draft.SNI != "" {
			proxy["servername"] = draft.SNI
		}
		if draft.Fingerprint != "" {
			proxy["client-fingerprint"] = draft.Fingerprint
		}
		proxy["skip-cert-verify"] = draft.AllowInsecure
	}
	if security == "reality" {
		reality := map[string]any{}
		if draft.PublicKey != "" {
			reality["public-key"] = draft.PublicKey
		}
		if draft.ShortID != "" {
			reality["short-id"] = draft.ShortID
		}
		if len(reality) > 0 {
			proxy["reality-opts"] = reality
		}
	}
	applyMihomoTransport(proxy, draft)
	return proxy
}

func singBoxLegacyOutbound(draft profileconfig.LinkConfigurationDraft, name string) map[string]any {
	outbound := map[string]any{"type": draft.Protocol, "tag": name, "server": draft.Server, "server_port": draft.Port}
	switch draft.Protocol {
	case "vless":
		outbound["uuid"] = draft.Identifier
		if draft.Flow != "" {
			outbound["flow"] = draft.Flow
		}
	case "vmess":
		outbound["uuid"] = draft.Identifier
		outbound["security"] = firstNonEmpty(draft.VMessSecurity, "auto")
		if alterID, err := strconv.Atoi(draft.VMessAlterID); err == nil && alterID > 0 {
			outbound["alter_id"] = alterID
		}
	case "trojan":
		outbound["password"] = draft.Identifier
	}
	security := strings.ToLower(strings.TrimSpace(draft.Security))
	if security == "tls" || security == "reality" {
		tlsOptions := map[string]any{"enabled": true, "insecure": draft.AllowInsecure}
		if draft.SNI != "" {
			tlsOptions["server_name"] = draft.SNI
		}
		if draft.Fingerprint != "" {
			tlsOptions["utls"] = map[string]any{"enabled": true, "fingerprint": draft.Fingerprint}
		}
		if security == "reality" {
			reality := map[string]any{"enabled": true}
			if draft.PublicKey != "" {
				reality["public_key"] = draft.PublicKey
			}
			if draft.ShortID != "" {
				reality["short_id"] = draft.ShortID
			}
			tlsOptions["reality"] = reality
		}
		outbound["tls"] = tlsOptions
	}
	switch strings.TrimSpace(draft.Network) {
	case "ws":
		transport := map[string]any{"type": "ws"}
		if draft.Path != "" {
			transport["path"] = draft.Path
		}
		if draft.Host != "" {
			transport["headers"] = map[string]string{"Host": draft.Host}
		}
		outbound["transport"] = transport
	case "grpc":
		transport := map[string]any{"type": "grpc"}
		if draft.GRPCServiceName != "" {
			transport["service_name"] = draft.GRPCServiceName
		}
		outbound["transport"] = transport
	}
	return outbound
}

var mihomoShadowsocksMethods = map[string]struct{}{
	"aes-128-gcm": {}, "aes-192-gcm": {}, "aes-256-gcm": {},
	"chacha20-ietf-poly1305": {}, "xchacha20-ietf-poly1305": {},
	"2022-blake3-aes-128-gcm": {}, "2022-blake3-aes-256-gcm": {},
	"2022-blake3-chacha20-poly1305": {},
}

var singBoxShadowsocksMethods = map[string]struct{}{
	"aes-128-gcm": {}, "aes-192-gcm": {}, "aes-256-gcm": {},
	"chacha20-ietf-poly1305": {}, "xchacha20-ietf-poly1305": {},
	"2022-blake3-aes-128-gcm": {}, "2022-blake3-aes-256-gcm": {},
	"2022-blake3-chacha20-poly1305": {},
}

var xrayShadowsocksMethods = map[string]struct{}{
	"aes-128-gcm": {}, "aes-256-gcm": {},
	"chacha20-poly1305": {}, "chacha20-ietf-poly1305": {},
	"xchacha20-poly1305": {}, "xchacha20-ietf-poly1305": {},
	"2022-blake3-aes-128-gcm": {}, "2022-blake3-aes-256-gcm": {},
	"2022-blake3-chacha20-poly1305": {},
}

func mihomoProxy(profile *profiles.Profile, name string) (map[string]any, string) {
	switch data := profile.Data.(type) {
	case profiles.ShadowsocksData:
		if _, ok := mihomoShadowsocksMethods[strings.ToLower(data.Method)]; !ok {
			return nil, ReasonClientVersion
		}
		proxy := map[string]any{"name": name, "type": "ss", "server": profile.Server, "port": portNumber(profile.Port), "cipher": data.Method, "password": data.Password.Reveal(), "udp": true}
		if data.Plugin != nil {
			plugin, options, ok := mihomoShadowsocksPlugin(data.Plugin)
			if !ok {
				return nil, ReasonPlugin
			}
			proxy["plugin"] = plugin
			if len(options) > 0 {
				proxy["plugin-opts"] = options
			}
		}
		return proxy, ""
	case profiles.Hysteria2Data:
		proxy := map[string]any{"name": name, "type": "hysteria2", "server": profile.Server, "password": data.Authentication.Reveal(), "sni": data.SNI, "skip-cert-verify": data.Insecure}
		if profile.Port.Kind == profiles.PortExpression {
			proxy["ports"] = profile.Port.Expression
		} else {
			proxy["port"] = portNumber(profile.Port)
		}
		if data.CertificateSHA256 != "" {
			proxy["fingerprint"] = data.CertificateSHA256
		}
		if data.ObfuscationType != "" {
			proxy["obfs"] = data.ObfuscationType
			proxy["obfs-password"] = data.ObfuscationPassword.Reveal()
		}
		return proxy, ""
	case profiles.TUICData:
		if data.Generation != 5 {
			return nil, ReasonCompatibility
		}
		if tuicHasFieldClass(data, profiles.TUICProvenanceSingBox) {
			return nil, ReasonUnrepresentable
		}
		proxy := map[string]any{"name": name, "type": "tuic", "server": profile.Server, "port": portNumber(profile.Port), "uuid": data.UUID.Reveal(), "password": data.Password.Reveal(), "congestion-controller": data.CongestionController, "udp-relay-mode": data.UDPRelayMode}
		if data.SNI != "" {
			proxy["sni"] = data.SNI
		}
		if len(data.ALPN) > 0 {
			proxy["alpn"] = append([]string(nil), data.ALPN...)
		}
		if data.SkipCertificateVerification {
			proxy["skip-cert-verify"] = true
		}
		if data.DisableSNI {
			proxy["disable-sni"] = true
		}
		if data.ZeroRTT {
			proxy["reduce-rtt"] = true
		}
		if data.Heartbeat != "" {
			value, ok := durationMilliseconds(data.Heartbeat)
			if !ok {
				return nil, ReasonUnrepresentable
			}
			proxy["heartbeat-interval"] = value
		}
		if data.RequestTimeout != "" {
			value, ok := durationMilliseconds(data.RequestTimeout)
			if !ok {
				return nil, ReasonUnrepresentable
			}
			proxy["request-timeout"] = value
		}
		if data.FastOpen {
			proxy["fast-open"] = true
		}
		if data.MaxOpenStreams > 0 {
			proxy["max-open-streams"] = data.MaxOpenStreams
		}
		if data.MaxUDPRelayPacketSize > 0 {
			proxy["max-udp-relay-packet-size"] = data.MaxUDPRelayPacketSize
		}
		return proxy, ""
	default:
		return nil, ReasonUnsupportedProtocol
	}
}

func singBoxOutbound(profile *profiles.Profile, name string) (map[string]any, string) {
	switch data := profile.Data.(type) {
	case profiles.ShadowsocksData:
		if _, ok := singBoxShadowsocksMethods[strings.ToLower(data.Method)]; !ok {
			return nil, ReasonClientVersion
		}
		outbound := map[string]any{"type": "shadowsocks", "tag": name, "server": profile.Server, "server_port": portNumber(profile.Port), "method": data.Method, "password": data.Password.Reveal()}
		if data.Plugin != nil {
			plugin, ok := singBoxShadowsocksPlugin(data.Plugin)
			if !ok {
				return nil, ReasonPlugin
			}
			outbound["plugin"] = plugin
			if data.Plugin.Options.IsSet() {
				outbound["plugin_opts"] = data.Plugin.Options.Reveal()
			}
		}
		return outbound, ""
	case profiles.Hysteria2Data:
		if data.ObfuscationType == "gecko" {
			return nil, ReasonClientVersion
		}
		if data.CertificateSHA256 != "" {
			return nil, ReasonUnrepresentable
		}
		outbound := map[string]any{"type": "hysteria2", "tag": name, "server": profile.Server, "password": data.Authentication.Reveal(), "tls": map[string]any{"enabled": true, "server_name": data.SNI, "insecure": data.Insecure}}
		if profile.Port.Kind == profiles.PortExpression {
			outbound["server_ports"] = singBoxPortRanges(profile.Port)
		} else {
			outbound["server_port"] = portNumber(profile.Port)
		}
		if data.ObfuscationType != "" {
			outbound["obfs"] = map[string]any{"type": data.ObfuscationType, "password": data.ObfuscationPassword.Reveal()}
		}
		return outbound, ""
	case profiles.TUICData:
		if data.Generation != 5 {
			return nil, ReasonCompatibility
		}
		if tuicHasFieldClass(data, profiles.TUICProvenanceMihomo) {
			return nil, ReasonUnrepresentable
		}
		outbound := map[string]any{"type": "tuic", "tag": name, "server": profile.Server, "server_port": portNumber(profile.Port), "uuid": data.UUID.Reveal(), "password": data.Password.Reveal(), "congestion_control": data.CongestionController, "tls": map[string]any{"enabled": true, "server_name": data.SNI, "insecure": data.SkipCertificateVerification}}
		if data.UDPOverStream {
			outbound["udp_over_stream"] = true
		} else {
			outbound["udp_relay_mode"] = data.UDPRelayMode
		}
		if data.ZeroRTT {
			outbound["zero_rtt_handshake"] = true
		}
		if data.Heartbeat != "" {
			outbound["heartbeat"] = data.Heartbeat
		}
		tlsOptions := outbound["tls"].(map[string]any)
		if len(data.ALPN) > 0 {
			tlsOptions["alpn"] = append([]string(nil), data.ALPN...)
		}
		return outbound, ""
	default:
		return nil, ReasonUnsupportedProtocol
	}
}

func portNumber(port profiles.PortSpec) int {
	if len(port.Ranges) == 0 {
		return 0
	}
	return int(port.Ranges[0].Start)
}

func singBoxPortRanges(port profiles.PortSpec) []string {
	result := make([]string, 0, len(port.Ranges))
	for _, item := range port.Ranges {
		if item.Start == item.End {
			value := strconv.Itoa(int(item.Start))
			result = append(result, value+":"+value)
		} else {
			result = append(result, fmt.Sprintf("%d:%d", item.Start, item.End))
		}
	}
	return result
}

func singBoxShadowsocksPlugin(plugin *profiles.ShadowsocksPlugin) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(plugin.Name))
	options, ok := parsePluginOptions(plugin.Options.Reveal())
	if !ok {
		return "", false
	}
	switch name {
	case "obfs-local":
		for key := range options {
			if key != "obfs" && key != "obfs-host" {
				return "", false
			}
		}
	case "v2ray-plugin":
		for key, value := range options {
			switch key {
			case "mode", "host", "path":
			case "tls":
				if _, err := strconv.ParseBool(value); err != nil {
					return "", false
				}
			case "mux":
				mux, err := strconv.Atoi(value)
				if err != nil || mux < 1 {
					return "", false
				}
			default:
				return "", false
			}
		}
	default:
		return "", false
	}
	return name, true
}

func durationMilliseconds(raw string) (int64, bool) {
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 || duration%time.Millisecond != 0 {
		return 0, false
	}
	return duration.Milliseconds(), true
}

func tuicHasFieldClass(data profiles.TUICData, class profiles.TUICFieldProvenance) bool {
	for _, observation := range data.FieldObservations {
		if observation.FieldClass == class {
			return true
		}
	}
	return false
}

func parsePluginOptions(raw string) (map[string]string, bool) {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return result, true
	}
	for _, part := range strings.Split(raw, ";") {
		if part == "" {
			continue
		}
		key, value, hasValue := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" || !hasValue || strings.TrimSpace(value) == "" {
			return nil, false
		}
		if _, duplicate := result[key]; duplicate {
			return nil, false
		}
		result[key] = strings.TrimSpace(value)
	}
	return result, true
}

func mihomoShadowsocksPlugin(plugin *profiles.ShadowsocksPlugin) (string, map[string]any, bool) {
	name := strings.ToLower(strings.TrimSpace(plugin.Name))
	options, ok := parsePluginOptions(plugin.Options.Reveal())
	if !ok {
		return "", nil, false
	}
	output := make(map[string]any)
	switch name {
	case "obfs-local", "simple-obfs", "obfs":
		allowed := map[string]string{"obfs": "mode", "mode": "mode", "obfs-host": "host", "host": "host"}
		for key, value := range options {
			target, found := allowed[key]
			if !found {
				return "", nil, false
			}
			output[target] = value
		}
		return "obfs", output, true
	case "v2ray-plugin":
		for key, value := range options {
			switch key {
			case "mode", "host", "path":
				output[key] = value
			case "tls", "mux":
				parsed, err := strconv.ParseBool(value)
				if err != nil {
					return "", nil, false
				}
				output[key] = parsed
			default:
				return "", nil, false
			}
		}
		return "v2ray-plugin", output, true
	default:
		return "", nil, false
	}
}

func xrayJSONForEntryWithNames(entry Entry, names *UniqueNames) ([]string, *Exclusion) {
	scheme := profileconfig.SupportedConfigScheme(entry.Raw)
	if scheme == "ss" {
		profile, exclusion := structuredProfile(entry, "xray-json")
		if exclusion != nil {
			return nil, exclusion
		}
		data := profile.Data.(profiles.ShadowsocksData)
		if data.Plugin != nil {
			return nil, &Exclusion{entry.SafeRef(), "shadowsocks", "xray-json", ReasonPlugin}
		}
		if _, ok := xrayShadowsocksMethods[strings.ToLower(data.Method)]; !ok {
			return nil, &Exclusion{entry.SafeRef(), "shadowsocks", "xray-json", ReasonClientVersion}
		}
		tag := profileName(entry, profile, names)
		payload, err := json.Marshal(map[string]any{"outbounds": []any{map[string]any{
			"tag":      tag,
			"protocol": "shadowsocks",
			"settings": map[string]any{
				"address":  profile.Server,
				"port":     portNumber(profile.Port),
				"method":   data.Method,
				"password": data.Password.Reveal(),
			},
		}}})
		if err != nil {
			return nil, &Exclusion{entry.SafeRef(), "shadowsocks", "xray-json", ReasonSerialization}
		}
		return []string{string(payload)}, nil
	}
	if scheme == "hysteria2" || scheme == "hy2" {
		profile, exclusion := structuredProfile(entry, "xray-json")
		if exclusion != nil {
			return nil, exclusion
		}
		data := profile.Data.(profiles.Hysteria2Data)
		if data.ObfuscationType == "gecko" {
			// Xray enables Gecko through FinalMask's packetSize. The Hysteria URI
			// model does not carry that required value, so guessing one would
			// change wire behavior.
			return nil, &Exclusion{entry.SafeRef(), "hysteria2", "xray-json", ReasonUnrepresentable}
		}
		if data.Insecure {
			// Xray v26.3.27 and v26.7.28 reject the removed allowInsecure
			// setting. A URI that disables certificate verification cannot be
			// safely translated into a certificate pin or peer-name constraint.
			return nil, &Exclusion{entry.SafeRef(), "hysteria2", "xray-json", ReasonUnrepresentable}
		}
		stream := map[string]any{
			"method":   "hysteria",
			"security": "tls",
			"hysteriaSettings": map[string]any{
				"version": 2,
				"auth":    data.Authentication.Reveal(),
			},
		}
		tlsSettings := map[string]any{}
		if data.SNI != "" {
			tlsSettings["serverName"] = data.SNI
		}
		if data.CertificateSHA256 != "" {
			tlsSettings["pinnedPeerCertSha256"] = data.CertificateSHA256
		}
		if len(tlsSettings) > 0 {
			stream["tlsSettings"] = tlsSettings
		}
		finalMask := map[string]any{}
		if profile.Port.Kind == profiles.PortExpression {
			finalMask["quicParams"] = map[string]any{"udpHop": map[string]any{
				"ports": profile.Port.Expression,
			}}
		}
		if data.ObfuscationType == "salamander" {
			finalMask["udp"] = []any{map[string]any{
				"type": "salamander",
				"settings": map[string]any{
					"password": data.ObfuscationPassword.Reveal(),
				},
			}}
		}
		if len(finalMask) > 0 {
			stream["finalmask"] = finalMask
		}
		payload, err := json.Marshal(map[string]any{"outbounds": []any{map[string]any{
			"tag":      profileName(entry, profile, names),
			"protocol": "hysteria",
			"settings": map[string]any{
				"version": 2,
				"address": profile.Server,
				"port":    portNumber(profile.Port),
			},
			"streamSettings": stream,
		}}})
		if err != nil {
			return nil, &Exclusion{entry.SafeRef(), "hysteria2", "xray-json", ReasonSerialization}
		}
		return []string{string(payload)}, nil
	}
	if scheme == "tuic" {
		profile, err := profiles.Parse(entry.Raw)
		protocol := SafeProtocolName(entry.Raw, entry.StoredProtocol)
		if err == nil {
			protocol = string(profile.Protocol)
			if data, ok := profile.Data.(profiles.TUICData); ok && data.Generation == 4 {
				return nil, &Exclusion{entry.SafeRef(), protocol, "xray-json", ReasonCompatibility}
			}
		}
		return nil, &Exclusion{entry.SafeRef(), protocol, "xray-json", ReasonUnsupportedProtocol}
	}
	if scheme == model.SubscriptionFormatXrayJSON {
		if !json.Valid([]byte(entry.Raw)) {
			return nil, &Exclusion{entry.SafeRef(), "xray-json", "xray-json", ReasonInvalidStored}
		}
		var value any
		if err := json.Unmarshal([]byte(entry.Raw), &value); err != nil {
			return nil, &Exclusion{entry.SafeRef(), "xray-json", "xray-json", ReasonInvalidStored}
		}
		normalized, err := json.Marshal(value)
		if err != nil {
			return nil, &Exclusion{entry.SafeRef(), "xray-json", "xray-json", ReasonSerialization}
		}
		return []string{string(normalized)}, nil
	}
	converted, err := NormalizeForOutput(entry.Raw, model.SubscriptionFormatXrayJSON, firstNonEmpty(entry.ClientDisplayName, entry.Label))
	if err != nil {
		return nil, &Exclusion{entry.SafeRef(), SafeProtocolName(entry.Raw, entry.StoredProtocol), "xray-json", ReasonInvalidStored}
	}
	return []string{converted}, nil
}

func ApplyExclusionHeaders(headers interface{ Set(string, string) }, exclusions []Exclusion) {
	if len(exclusions) == 0 {
		return
	}
	counts := ExclusionCounts(exclusions)
	ordered := make([]string, 0, len(counts))
	for code := range counts {
		ordered = append(ordered, code)
	}
	sortStrings(ordered)
	summary := make([]string, 0, len(ordered))
	for _, code := range ordered {
		summary = append(summary, code+"="+strconv.Itoa(counts[code]))
	}
	headers.Set("SubShare-Excluded-Count", strconv.Itoa(len(exclusions)))
	headers.Set("SubShare-Exclusion-Codes", strings.Join(ordered, ","))
	headers.Set("SubShare-Exclusion-Counts", strings.Join(summary, ","))
}

func ExclusionCounts(exclusions []Exclusion) map[string]int {
	allowed := make(map[string]struct{})
	for _, code := range ExclusionReasonCodes() {
		allowed[code] = struct{}{}
	}
	counts := make(map[string]int)
	for _, exclusion := range exclusions {
		if _, ok := allowed[exclusion.Reason]; ok {
			counts[exclusion.Reason]++
		}
	}
	return counts
}

func FailurePayload(generated Generated) Failure {
	return Failure{
		ErrorCode:       ReasonAllExcluded,
		OutputFormat:    generated.OutputFormat,
		EligibleCount:   generated.EligibleCount,
		ExcludedCount:   len(generated.Exclusions),
		ExclusionCounts: ExclusionCounts(generated.Exclusions),
	}
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for cursor := index; cursor > 0 && values[cursor] < values[cursor-1]; cursor-- {
			values[cursor], values[cursor-1] = values[cursor-1], values[cursor]
		}
	}
}

func ValidateStructuredBody(format, body string) error {
	if hasUnsafeStructuredControl(body) {
		return errors.New(ReasonSerialization)
	}
	switch format {
	case "xray-json":
		var documents []map[string]any
		if err := json.Unmarshal([]byte(body), &documents); err != nil || len(documents) == 0 {
			return errors.New(ReasonSerialization)
		}
		for _, document := range documents {
			outbounds, ok := document["outbounds"].([]any)
			if !ok || !validJSONOutboundList(outbounds, "protocol") {
				return errors.New(ReasonSerialization)
			}
		}
	case "sing-box":
		var document map[string]any
		if err := json.Unmarshal([]byte(body), &document); err != nil {
			return errors.New(ReasonSerialization)
		}
		outbounds, ok := document["outbounds"].([]any)
		if !ok || !validJSONOutboundList(outbounds, "type") {
			return errors.New(ReasonSerialization)
		}
	case "mihomo":
		var document yaml.Node
		if err := yaml.Unmarshal([]byte(body), &document); err != nil || yamlHasDuplicateMappingKey(&document) || !validMihomoDocument(&document) {
			return errors.New(ReasonSerialization)
		}
	}
	return nil
}

func hasUnsafeStructuredControl(body string) bool {
	for _, char := range body {
		if char == '\t' || char == '\n' || char == '\r' {
			continue
		}
		if char == 0x7f || char < 0x20 {
			return true
		}
	}
	return false
}

func validJSONOutboundList(values []any, protocolField string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		outbound, ok := value.(map[string]any)
		protocol, protocolOK := outbound[protocolField].(string)
		if !ok || !protocolOK || strings.TrimSpace(protocol) == "" {
			return false
		}
	}
	return true
}

func yamlHasDuplicateMappingKey(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index+1 < len(node.Content); index += 2 {
			key := node.Content[index].Value
			if _, duplicate := seen[key]; duplicate {
				return true
			}
			seen[key] = struct{}{}
		}
	}
	for _, child := range node.Content {
		if yamlHasDuplicateMappingKey(child) {
			return true
		}
	}
	return false
}

func validMihomoDocument(document *yaml.Node) bool {
	if document == nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return false
	}
	root := document.Content[0]
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value != "proxies" {
			continue
		}
		proxies := root.Content[index+1]
		if proxies.Kind != yaml.SequenceNode || len(proxies.Content) == 0 {
			return false
		}
		for _, proxy := range proxies.Content {
			if proxy.Kind != yaml.MappingNode || !yamlMappingHasNonEmptyString(proxy, "name") || !yamlMappingHasNonEmptyString(proxy, "type") {
				return false
			}
		}
		return true
	}
	return false
}

func yamlMappingHasNonEmptyString(mapping *yaml.Node, field string) bool {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != field {
			continue
		}
		value := mapping.Content[index+1]
		return value.Kind == yaml.ScalarNode && value.Tag == "!!str" && strings.TrimSpace(value.Value) != ""
	}
	return false
}

func EncodeBase64(body string) string {
	return base64.StdEncoding.EncodeToString([]byte(body))
}

type TemplateData struct {
	UserName       string
	Telegram       string
	SubscriptionID string
	ExpiryDate     string
	ExpiryDateTime string
	RealKeysCount  int
}

func RenderInfoTemplate(template string, data TemplateData) string {
	text := strings.TrimSpace(template)
	if text == "" {
		return ""
	}
	replacements := map[string]string{
		"{user_name}":       data.UserName,
		"{telegram}":        data.Telegram,
		"{subscription_id}": data.SubscriptionID,
		"{expires_date}":    data.ExpiryDate,
		"{expires_at}":      data.ExpiryDateTime,
		"{real_keys_count}": fmt.Sprintf("%d", data.RealKeysCount),
	}
	for key, value := range replacements {
		text = strings.ReplaceAll(text, key, strings.TrimSpace(value))
	}
	return strings.TrimSpace(text)
}

func InformationalVLESSURL(displayText string) string {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		displayText = "Info"
	}
	return "vless://00000000-0000-0000-0000-000000000000@info.invalid:443?type=tcp&security=none#" + url.QueryEscape(displayText)
}

func InformationalXrayJSON(displayText string) string {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		displayText = "Info"
	}

	description := displayText
	if newline := strings.Index(description, "\n"); newline >= 0 {
		description = strings.TrimSpace(description[:newline])
	}
	if description == "" {
		description = "Informational key"
	}

	payload := map[string]any{
		"remarks": displayText,
		"meta": map[string]any{
			"serverDescription": description,
			"informational":     true,
		},
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []any{},
		"outbounds": []any{
			map[string]any{
				"tag":      "proxy",
				"protocol": "vless",
				"settings": map[string]any{
					"vnext": []any{
						map[string]any{
							"address": "info.invalid",
							"port":    443,
							"users": []any{
								map[string]any{
									"id":         "00000000-0000-0000-0000-000000000000",
									"encryption": "none",
								},
							},
						},
					},
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "none",
				},
			},
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func applyMihomoTransport(proxy map[string]any, draft profileconfig.LinkConfigurationDraft) {
	network := strings.TrimSpace(draft.Network)
	if network == "" {
		network = "tcp"
	}
	proxy["network"] = network
	switch network {
	case "ws":
		options := map[string]any{}
		if draft.Path != "" {
			options["path"] = draft.Path
		}
		if draft.Host != "" {
			options["headers"] = map[string]string{"Host": draft.Host}
		}
		if len(options) > 0 {
			proxy["ws-opts"] = options
		}
	case "grpc":
		if draft.GRPCServiceName != "" {
			proxy["grpc-opts"] = map[string]string{"grpc-service-name": draft.GRPCServiceName}
		}
	}
}

func NormalizeForOutput(raw string, format string, fallbackRemark string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("configuration is empty")
	}
	if strings.TrimSpace(format) != "xray-json" {
		return trimmed, nil
	}

	scheme := profileconfig.SupportedConfigScheme(trimmed)
	switch scheme {
	case "xray-json":
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "", fmt.Errorf("invalid XRAY-JSON syntax")
		}
		normalized, err := json.Marshal(parsed)
		if err != nil {
			return "", err
		}
		return string(normalized), nil
	case "vless", "vmess", "trojan":
		return profileconfig.BuildXrayJSONFromLink(trimmed, fallbackRemark)
	default:
		return "", fmt.Errorf("unsupported configuration scheme")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func splitSubscriptionEntries(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	entries := make([]string, 0, len(lines))
	var jsonBuffer strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if jsonBuffer.Len() > 0 || strings.HasPrefix(trimmed, "{") {
			if jsonBuffer.Len() > 0 {
				jsonBuffer.WriteByte('\n')
			}
			jsonBuffer.WriteString(trimmed)
			if json.Valid([]byte(jsonBuffer.String())) {
				entries = append(entries, jsonBuffer.String())
				jsonBuffer.Reset()
			}
			continue
		}
		entries = append(entries, trimmed)
	}
	if jsonBuffer.Len() > 0 {
		entries = append(entries, jsonBuffer.String())
	}
	return entries
}

func draftDisplayName(draft profileconfig.LinkConfigurationDraft, index int) string {
	if name := strings.TrimSpace(draft.Remark); name != "" {
		return name
	}
	if name := strings.TrimSpace(draft.ServerDescription); name != "" {
		return name
	}
	return fmt.Sprintf("%s-%d", draft.Server, index+1)
}
