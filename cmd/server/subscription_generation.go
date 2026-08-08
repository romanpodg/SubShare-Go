package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"gopkg.in/yaml.v3"
)

const (
	targetMihomoVersion      = "1.19.28"
	targetSingBoxVersion     = "1.13.12"
	targetXrayMinimumVersion = "26.3.27"
	targetXrayCurrentVersion = "26.7.28"
)

func generationExclusionReasonCodes() []string {
	return []string{
		generationReasonAmbiguous,
		generationReasonClientVersion,
		generationReasonCompatibility,
		generationReasonUnrepresentable,
		generationReasonInvalidStored,
		generationReasonSerialization,
		generationReasonUnsupportedProtocol,
		generationReasonPlugin,
		generationReasonUnsafeControl,
	}
}

const (
	generationReasonUnsupportedProtocol = "protocol_unsupported_by_format"
	generationReasonClientVersion       = "client_version_unsupported"
	generationReasonPlugin              = "required_plugin_unsupported"
	generationReasonCompatibility       = "compatibility_only_profile"
	generationReasonInvalidStored       = "invalid_stored_raw_uri"
	generationReasonUnsafeControl       = "unsafe_control_character"
	generationReasonUnrepresentable     = "field_not_representable"
	generationReasonAmbiguous           = "ambiguous_profile"
	generationReasonSerialization       = "generator_serialization_failure"
	generationReasonAllExcluded         = "all_profiles_excluded"
)

type capabilitySupport string

const (
	capabilitySupported     capabilitySupport = "supported"
	capabilityUnsupported   capabilitySupport = "unsupported"
	capabilityConditional   capabilitySupport = "conditionally_supported"
	capabilityCompatibility capabilitySupport = "compatibility_only"
)

type outputCapability struct {
	Status                  capabilitySupport `json:"status"`
	ReasonCode              string            `json:"reason_code,omitempty"`
	TargetVersion           string            `json:"target_version,omitempty"`
	MinimumVersion          string            `json:"minimum_version,omitempty"`
	SyntaxValidation        string            `json:"syntax_validation,omitempty"`
	RuntimeInteroperability string            `json:"runtime_interoperability,omitempty"`
}

type protocolCapability struct {
	Protocol   string                      `json:"protocol"`
	Generation string                      `json:"generation"`
	Outputs    map[string]outputCapability `json:"outputs"`
}

// subscriptionCapabilityMatrix is the single backend source of truth for
// delivery, editing, and probing claims. Structured generators are pinned to
// explicit client schemas; an absent generator is never interpreted as support.
func subscriptionCapabilityMatrix() []protocolCapability {
	structural := func(version string) outputCapability {
		return outputCapability{Status: capabilitySupported, TargetVersion: version, SyntaxValidation: "structurally_generated", RuntimeInteroperability: "not_tested"}
	}
	clientSupported := func(version, minimum string) outputCapability {
		return outputCapability{Status: capabilitySupported, TargetVersion: version, MinimumVersion: minimum, SyntaxValidation: "official_binary", RuntimeInteroperability: "not_tested"}
	}
	conditional := func(reason, version, minimum, syntax string) outputCapability {
		return outputCapability{Status: capabilityConditional, ReasonCode: reason, TargetVersion: version, MinimumVersion: minimum, SyntaxValidation: syntax, RuntimeInteroperability: "not_tested"}
	}
	unsupported := func(reason, version string) outputCapability {
		return outputCapability{Status: capabilityUnsupported, ReasonCode: reason, TargetVersion: version}
	}
	compatibility := func(reason string) outputCapability {
		return outputCapability{Status: capabilityCompatibility, ReasonCode: reason}
	}
	legacy := func(protocol string) protocolCapability {
		return protocolCapability{Protocol: protocol, Generation: "current", Outputs: map[string]outputCapability{
			"plain": structural("sip-uri"), "base64": structural("whole-body-standard-base64"),
			"mihomo": clientSupported(targetMihomoVersion, targetMihomoVersion), "sing-box": clientSupported(targetSingBoxVersion, targetSingBoxVersion),
			"xray-json": clientSupported(targetXrayCurrentVersion, targetXrayMinimumVersion), "structured-editing": structural("subshare-current"),
			"connectivity-probe": conditional("tcp_reachability_only", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(generationReasonUnsupportedProtocol, ""),
		}}
	}
	matrix := []protocolCapability{legacy("vless"), legacy("vmess"), legacy("trojan")}
	matrix = append(matrix,
		protocolCapability{Protocol: "shadowsocks", Generation: "sip002/sip022", Outputs: map[string]outputCapability{
			"plain": structural("sip002/sip022"), "base64": structural("whole-body-standard-base64"),
			"mihomo":             conditional("method_or_plugin_dependent", targetMihomoVersion, targetMihomoVersion, "official_binary"),
			"sing-box":           conditional("method_or_plugin_dependent", targetSingBoxVersion, targetSingBoxVersion, "official_binary"),
			"xray-json":          conditional("plugin_free_only", targetXrayCurrentVersion, targetXrayMinimumVersion, "official_binary"),
			"structured-editing": unsupported("frontend_editor_deferred", ""),
			"connectivity-probe": conditional("tcp_reachability_only", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(generationReasonUnsupportedProtocol, ""),
		}},
		protocolCapability{Protocol: "hysteria2", Generation: "2", Outputs: map[string]outputCapability{
			"plain": structural("hysteria2-uri"), "base64": structural("whole-body-standard-base64"),
			"mihomo":             conditional("field_and_version_dependent", targetMihomoVersion, targetMihomoVersion, "official_binary"),
			"sing-box":           conditional("field_and_version_dependent", targetSingBoxVersion, targetSingBoxVersion, "official_binary"),
			"xray-json":          conditional("obfuscation_or_extension_dependent", targetXrayCurrentVersion, targetXrayMinimumVersion, "official_binary"),
			"structured-editing": unsupported("frontend_editor_deferred", ""),
			"connectivity-probe": conditional("dns_only_udp_quic_probe_unavailable", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(generationReasonUnsupportedProtocol, ""),
		}},
		protocolCapability{Protocol: "tuic", Generation: "5", Outputs: map[string]outputCapability{
			"plain": structural("compatibility-uri"), "base64": structural("whole-body-standard-base64"),
			"mihomo":             conditional("provenance_and_field_dependent", targetMihomoVersion, targetMihomoVersion, "official_binary"),
			"sing-box":           conditional("provenance_and_field_dependent", targetSingBoxVersion, targetSingBoxVersion, "official_binary"),
			"xray-json":          unsupported(generationReasonUnsupportedProtocol, targetXrayCurrentVersion),
			"structured-editing": unsupported("frontend_editor_deferred", ""),
			"connectivity-probe": conditional("dns_only_udp_quic_probe_unavailable", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  unsupported(generationReasonUnsupportedProtocol, ""),
		}},
		protocolCapability{Protocol: "tuic", Generation: "4", Outputs: map[string]outputCapability{
			"plain": compatibility("raw_delivery_only"), "base64": compatibility("raw_delivery_only"),
			"mihomo":             unsupported(generationReasonCompatibility, targetMihomoVersion),
			"sing-box":           unsupported(generationReasonCompatibility, targetSingBoxVersion),
			"xray-json":          unsupported(generationReasonCompatibility, targetXrayCurrentVersion),
			"structured-editing": unsupported(generationReasonCompatibility, ""),
			"connectivity-probe": conditional("dns_only_udp_quic_probe_unavailable", "subshare-current", "", "address_probe_only"),
			"compatibility-raw":  compatibility("raw_delivery_only"),
		}},
	)
	return matrix
}

type generationExclusion struct {
	RecordRef string `json:"record_ref"`
	Protocol  string `json:"protocol"`
	Format    string `json:"format"`
	Reason    string `json:"reason_code"`
}

func (item generationExclusion) Error() string {
	return "subscription_generation_exclusion{record_ref=" + item.RecordRef + ", protocol=" + item.Protocol + ", format=" + item.Format + ", reason=" + item.Reason + "}"
}

type deliveryEntry struct {
	ID             int64
	SourceID       sql.NullInt64
	Raw            string
	Kind           string
	TemplateText   string
	Label          string
	StoredProtocol string
	Compatibility  string
}

func (entry deliveryEntry) safeRef() string { return "key:" + strconv.FormatInt(entry.ID, 10) }

type deliverySelection struct {
	Entries       []deliveryEntry
	Exclusions    []generationExclusion
	Settings      model.SubscriptionSettings
	EligibleCount int
}

type generatedSubscription struct {
	Body           string
	Exclusions     []generationExclusion
	OutputFormat   string
	EligibleCount  int
	GeneratedCount int
}

type subscriptionGenerationFailure struct {
	ErrorCode       string         `json:"error_code"`
	OutputFormat    string         `json:"output_format"`
	EligibleCount   int            `json:"eligible_count"`
	ExcludedCount   int            `json:"excluded_count"`
	ExclusionCounts map[string]int `json:"exclusion_counts"`
}

func hasUnsafeSubscriptionControl(raw string) bool {
	for _, char := range raw {
		if char == 0x7f || char < 0x20 {
			return true
		}
	}
	return false
}

func safeProtocolName(raw, fallback string) string {
	scheme := supportedConfigScheme(raw)
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

func (a *App) deliveryIdentity(raw string) (string, error) {
	if len(a.profileFingerprintKey) == 0 {
		return "", fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if profile, err := profiles.Parse(raw); err == nil {
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, a.profileFingerprintKey)
		if fingerprintErr != nil {
			return "", fmt.Errorf("profile_fingerprint_failed")
		}
		return "profile:" + fingerprint, nil
	}
	digest := hmac.New(sha256.New, a.profileFingerprintKey)
	_, _ = digest.Write([]byte("subshare-delivery-raw-v1\x00"))
	_, _ = digest.Write([]byte(raw))
	return "raw:" + hex.EncodeToString(digest.Sum(nil)), nil
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

	rows, err := a.db.Query(`
		SELECT k.id, k.external_source_id, s.encrypted_url, k.key_kind, COALESCE(k.template_text, ''),
		       COALESCE(k.label, ''), COALESCE(k.protocol, 'legacy'), COALESCE(k.profile_compatibility, 'legacy')
		FROM users u
		JOIN user_keys uk ON uk.user_id = u.id
		JOIN vless_keys k ON k.id = uk.key_id
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		LEFT JOIN key_categories kc ON kc.id = k.category_id
		WHERE u.subscription_id = ?
		  AND k.status = 'active'
		  AND (k.key_kind = 'informational' OR COALESCE(k.health_failure_count, 0) < 3)
		ORDER BY
		  CASE WHEN k.category_id IS NULL THEN 0 ELSE 1 END,
		  COALESCE(kc.sort_order, 2147483647),
		  k.sort_order,
		  COALESCE(k.external_source_id, 0),
		  k.id
	`, subscriptionID)
	if err != nil {
		return deliverySelection{}, 0, "", err
	}
	defer rows.Close()

	selection := deliverySelection{Settings: settings, Entries: []deliveryEntry{}, Exclusions: []generationExclusion{}}
	seen := make(map[string]struct{})
	for rows.Next() {
		var entry deliveryEntry
		var encURL sql.NullString
		if err := rows.Scan(&entry.ID, &entry.SourceID, &encURL, &entry.Kind, &entry.TemplateText, &entry.Label, &entry.StoredProtocol, &entry.Compatibility); err != nil {
			return deliverySelection{}, 0, "", err
		}
		kind, _ := model.NormalizeKeyKind(entry.Kind)
		entry.Kind = kind
		selection.EligibleCount++
		if kind == model.KeyKindInformational {
			selection.Entries = append(selection.Entries, entry)
			continue
		}
		if !encURL.Valid || encURL.String == "" {
			log.Printf("operator_event: row_id=%d source_id=%v reason=profile_storage_integrity_error error=missing_secret", entry.ID, entry.SourceID)
			selection.Exclusions = append(selection.Exclusions, generationExclusion{entry.safeRef(), "unknown", responseType, "profile_storage_integrity_error"})
			continue
		}
		sec, err := profilestorage.Decrypt(encURL.String, a.profileKeyring, entry.ID)
		if err != nil {
			log.Printf("operator_event: row_id=%d source_id=%v reason=profile_storage_integrity_error error=decryption_failed", entry.ID, entry.SourceID)
			selection.Exclusions = append(selection.Exclusions, generationExclusion{entry.safeRef(), "unknown", responseType, "profile_storage_integrity_error"})
			continue
		}
		entry.Raw = sec.Reveal()
		protocol := safeProtocolName(entry.Raw, entry.StoredProtocol)
		if hasUnsafeSubscriptionControl(entry.Raw) {
			selection.Exclusions = append(selection.Exclusions, generationExclusion{entry.safeRef(), protocol, responseType, generationReasonUnsafeControl})
			continue
		}
		if err := validateStoredDeliveryEntry(entry.Raw); err != nil {
			selection.Exclusions = append(selection.Exclusions, generationExclusion{entry.safeRef(), protocol, responseType, generationReasonInvalidStored})
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
	if err := rows.Err(); err != nil {
		return deliverySelection{}, 0, "", err
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

func validateStoredDeliveryEntry(raw string) error {
	switch supportedConfigScheme(raw) {
	case "vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic":
		if _, err := profiles.Parse(raw); err == nil {
			return nil
		}
		if _, err := parseLinkConfiguration(raw); err == nil {
			return nil
		}
		return errors.New(generationReasonInvalidStored)
	case model.SubscriptionFormatXrayJSON:
		_, err := parseXrayJSONDrafts(raw)
		return err
	default:
		return errors.New(generationReasonInvalidStored)
	}
}

func (a *App) generateSelectedSubscription(subscriptionID, responseType string) (generatedSubscription, model.SubscriptionSettings, int, string, error) {
	selection, denyCode, denyReason, err := a.selectSubscriptionEntries(subscriptionID, responseType)
	if err != nil || denyCode != 0 {
		return generatedSubscription{Exclusions: selection.Exclusions}, selection.Settings, denyCode, denyReason, err
	}
	var generated generatedSubscription
	switch responseType {
	case "mihomo":
		entries, materializeErr := a.materializeInformationalEntries(subscriptionID, selection.Settings, selection.Entries, false)
		if materializeErr != nil {
			return generatedSubscription{}, selection.Settings, 0, "", materializeErr
		}
		generated, err = renderMihomoEntries(entries)
	case "sing-box":
		entries, materializeErr := a.materializeInformationalEntries(subscriptionID, selection.Settings, selection.Entries, false)
		if materializeErr != nil {
			return generatedSubscription{}, selection.Settings, 0, "", materializeErr
		}
		generated, err = renderSingBoxEntries(entries)
	case "xray-json":
		entries, materializeErr := a.materializeInformationalEntries(subscriptionID, selection.Settings, selection.Entries, true)
		if materializeErr != nil {
			return generatedSubscription{}, selection.Settings, 0, "", materializeErr
		}
		generated, err = renderXrayEntries(entries)
	default:
		generated, err = renderPlainEntries(selection.Entries, selection.Settings, subscriptionID, a)
	}
	generated.Exclusions = append(selection.Exclusions, generated.Exclusions...)
	generated.EligibleCount = selection.EligibleCount
	generated.OutputFormat = effectiveGenerationFormat(responseType, selection.Settings, a.subscriptionBodyEncoding)
	if err != nil {
		return generated, selection.Settings, 0, "", err
	}
	if generated.GeneratedCount == 0 {
		if isStructuredGenerationFormat(generated.OutputFormat) && len(generated.Exclusions) > 0 {
			generated.Body = ""
			return generated, selection.Settings, http.StatusUnprocessableEntity, generationReasonAllExcluded, nil
		}
		return generated, selection.Settings, 503, "subscription has no available keys", nil
	}
	return generated, selection.Settings, 0, "", nil
}

func effectiveGenerationFormat(responseType string, settings model.SubscriptionSettings, defaultEncoding string) string {
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

func isStructuredGenerationFormat(format string) bool {
	switch format {
	case "mihomo", "sing-box", "xray-json":
		return true
	default:
		return false
	}
}

func (a *App) materializeInformationalEntries(subscriptionID string, settings model.SubscriptionSettings, entries []deliveryEntry, xray bool) ([]deliveryEntry, error) {
	format := model.SubscriptionFormatLinks
	if xray {
		format = model.SubscriptionFormatXrayJSON
	}
	templateData, err := a.buildSubscriptionTemplateData(subscriptionID, format)
	if err != nil {
		return nil, err
	}
	result := make([]deliveryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != model.KeyKindInformational {
			result = append(result, entry)
			continue
		}
		textTemplate := strings.TrimSpace(entry.TemplateText)
		if textTemplate == "" {
			textTemplate = strings.TrimSpace(entry.Label)
		}
		rendered := renderInfoTemplate(textTemplate, templateData)
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		entry.Kind = model.KeyKindReal
		if xray {
			entry.Raw = buildInformationalXrayJSON(rendered)
			entry.StoredProtocol = "xray-json"
		} else {
			entry.Raw = buildInformationalVLESSURL(rendered)
			entry.StoredProtocol = "vless"
		}
		result = append(result, entry)
	}
	return result, nil
}

func renderPlainEntries(entries []deliveryEntry, settings model.SubscriptionSettings, subscriptionID string, app *App) (generatedSubscription, error) {
	templateData, err := app.buildSubscriptionTemplateData(subscriptionID, settings.SubscriptionFormat)
	if err != nil {
		return generatedSubscription{}, err
	}
	result := generatedSubscription{Exclusions: []generationExclusion{}}
	lines := make([]string, 0, len(entries))
	names := &uniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			textTemplate := strings.TrimSpace(entry.TemplateText)
			if textTemplate == "" {
				textTemplate = strings.TrimSpace(entry.Label)
			}
			rendered := renderInfoTemplate(textTemplate, templateData)
			if strings.TrimSpace(rendered) == "" {
				continue
			}
			if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
				if info := buildInformationalXrayJSON(rendered); strings.TrimSpace(info) != "" {
					lines = append(lines, info)
				}
			} else {
				lines = append(lines, buildInformationalVLESSURL(rendered))
			}
			continue
		}
		if settings.SubscriptionFormat == model.SubscriptionFormatLinks && supportedConfigScheme(entry.Raw) == model.SubscriptionFormatXrayJSON {
			drafts, parseErr := parseXrayJSONDrafts(entry.Raw)
			if parseErr != nil {
				result.Exclusions = append(result.Exclusions, generationExclusion{entry.safeRef(), "xray-json", "plain", generationReasonInvalidStored})
				continue
			}
			for _, draft := range drafts {
				if draft.Remark == "" {
					draft.Remark = entry.Label
				}
				link, buildErr := buildShareLinkFromDraft(draft)
				if buildErr != nil || hasUnsafeSubscriptionControl(link) {
					result.Exclusions = append(result.Exclusions, generationExclusion{entry.safeRef(), draft.Protocol, "plain", generationReasonSerialization})
					continue
				}
				lines = append(lines, link)
			}
			continue
		}
		if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
			converted, exclusion := xrayJSONForEntryWithNames(entry, names)
			if exclusion != nil {
				result.Exclusions = append(result.Exclusions, *exclusion)
				continue
			}
			lines = append(lines, converted...)
			continue
		}
		// Exact source bytes are delivered for URI formats. Safety validation
		// above guarantees one stored record cannot inject another line.
		lines = append(lines, entry.Raw)
	}
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		items := make([]json.RawMessage, 0, len(lines))
		for _, line := range lines {
			if !json.Valid([]byte(line)) {
				return generatedSubscription{}, errors.New(generationReasonSerialization)
			}
			items = append(items, json.RawMessage(line))
		}
		payload, marshalErr := json.Marshal(items)
		if marshalErr != nil {
			return generatedSubscription{}, errors.New(generationReasonSerialization)
		}
		result.Body = string(payload)
		result.GeneratedCount = len(items)
		return result, nil
	}
	result.Body = strings.Join(lines, "\n")
	result.GeneratedCount = len(lines)
	return result, nil
}

func structuredProfile(entry deliveryEntry, format string) (*profiles.Profile, *generationExclusion) {
	profile, err := profiles.Parse(entry.Raw)
	if err != nil {
		return nil, &generationExclusion{entry.safeRef(), safeProtocolName(entry.Raw, entry.StoredProtocol), format, generationReasonInvalidStored}
	}
	if len(profile.UnknownQueryParameters) > 0 {
		return nil, &generationExclusion{entry.safeRef(), string(profile.Protocol), format, generationReasonUnrepresentable}
	}
	for _, warning := range profile.Warnings {
		if warning.Code == profiles.WarningDuplicateParameter || warning.Code == profiles.WarningAmbiguousParameter || warning.Code == profiles.WarningConflictingPreference {
			return nil, &generationExclusion{entry.safeRef(), string(profile.Protocol), format, generationReasonAmbiguous}
		}
	}
	return profile, nil
}

type uniqueNames struct{ counts map[string]int }

func (names *uniqueNames) next(raw, fallback string) string {
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

func profileName(entry deliveryEntry, profile *profiles.Profile, names *uniqueNames) string {
	return names.next(profile.DisplayName, firstNonEmpty(entry.Label, string(profile.Protocol)+"-"+profile.Server))
}

func syntheticDeliveryEntries(raw string) []deliveryEntry {
	parts := splitSubscriptionEntries(raw)
	entries := make([]deliveryEntry, 0, len(parts))
	for index, part := range parts {
		entries = append(entries, deliveryEntry{ID: int64(index + 1), Raw: part, Kind: model.KeyKindReal, Label: fmt.Sprintf("proxy-%d", index+1)})
	}
	return entries
}

func renderMihomoEntries(entries []deliveryEntry) (generatedSubscription, error) {
	result := generatedSubscription{Exclusions: []generationExclusion{}}
	proxiesOut := make([]map[string]any, 0, len(entries))
	names := &uniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		scheme := supportedConfigScheme(entry.Raw)
		if scheme == "ss" || scheme == "hysteria2" || scheme == "hy2" || scheme == "tuic" {
			profile, exclusion := structuredProfile(entry, "mihomo")
			if exclusion != nil {
				result.Exclusions = append(result.Exclusions, *exclusion)
				continue
			}
			proxy, reason := mihomoProxy(profile, profileName(entry, profile, names))
			if reason != "" {
				result.Exclusions = append(result.Exclusions, generationExclusion{entry.safeRef(), string(profile.Protocol), "mihomo", reason})
				continue
			}
			proxiesOut = append(proxiesOut, proxy)
			continue
		}
		drafts, err := entryDrafts(entry.Raw)
		if err != nil {
			result.Exclusions = append(result.Exclusions, generationExclusion{entry.safeRef(), safeProtocolName(entry.Raw, entry.StoredProtocol), "mihomo", generationReasonInvalidStored})
			continue
		}
		for index, draft := range drafts {
			name := names.next(draftDisplayName(draft, index), entry.Label)
			proxiesOut = append(proxiesOut, mihomoLegacyProxy(draft, name))
		}
	}
	payload, err := json.MarshalIndent(map[string]any{"proxies": proxiesOut}, "", "  ")
	if err != nil {
		return result, errors.New(generationReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(proxiesOut)
	return result, nil
}

func renderSingBoxEntries(entries []deliveryEntry) (generatedSubscription, error) {
	result := generatedSubscription{Exclusions: []generationExclusion{}}
	outbounds := make([]map[string]any, 0, len(entries))
	names := &uniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		scheme := supportedConfigScheme(entry.Raw)
		if scheme == "ss" || scheme == "hysteria2" || scheme == "hy2" || scheme == "tuic" {
			profile, exclusion := structuredProfile(entry, "sing-box")
			if exclusion != nil {
				result.Exclusions = append(result.Exclusions, *exclusion)
				continue
			}
			outbound, reason := singBoxOutbound(profile, profileName(entry, profile, names))
			if reason != "" {
				result.Exclusions = append(result.Exclusions, generationExclusion{entry.safeRef(), string(profile.Protocol), "sing-box", reason})
				continue
			}
			outbounds = append(outbounds, outbound)
			continue
		}
		drafts, err := entryDrafts(entry.Raw)
		if err != nil {
			result.Exclusions = append(result.Exclusions, generationExclusion{entry.safeRef(), safeProtocolName(entry.Raw, entry.StoredProtocol), "sing-box", generationReasonInvalidStored})
			continue
		}
		for index, draft := range drafts {
			name := names.next(draftDisplayName(draft, index), entry.Label)
			outbounds = append(outbounds, singBoxLegacyOutbound(draft, name))
		}
	}
	payload, err := json.MarshalIndent(map[string]any{"outbounds": outbounds}, "", "  ")
	if err != nil {
		return result, errors.New(generationReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(outbounds)
	return result, nil
}

func renderXrayEntries(entries []deliveryEntry) (generatedSubscription, error) {
	result := generatedSubscription{Exclusions: []generationExclusion{}}
	items := make([]json.RawMessage, 0, len(entries))
	names := &uniqueNames{}
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
				return result, errors.New(generationReasonSerialization)
			}
			items = append(items, json.RawMessage(item))
		}
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return result, errors.New(generationReasonSerialization)
	}
	result.Body = string(payload)
	result.GeneratedCount = len(items)
	return result, nil
}

func entryDrafts(raw string) ([]linkConfigurationDraft, error) {
	if supportedConfigScheme(raw) == model.SubscriptionFormatXrayJSON {
		return parseXrayJSONDrafts(raw)
	}
	draft, err := parseLinkConfiguration(raw)
	if err != nil {
		return nil, err
	}
	return []linkConfigurationDraft{draft}, nil
}

func mihomoLegacyProxy(draft linkConfigurationDraft, name string) map[string]any {
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

func singBoxLegacyOutbound(draft linkConfigurationDraft, name string) map[string]any {
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
			return nil, generationReasonClientVersion
		}
		proxy := map[string]any{"name": name, "type": "ss", "server": profile.Server, "port": portNumber(profile.Port), "cipher": data.Method, "password": data.Password.Reveal(), "udp": true}
		if data.Plugin != nil {
			plugin, options, ok := mihomoShadowsocksPlugin(data.Plugin)
			if !ok {
				return nil, generationReasonPlugin
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
			return nil, generationReasonCompatibility
		}
		if tuicHasFieldClass(data, profiles.TUICProvenanceSingBox) {
			return nil, generationReasonUnrepresentable
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
				return nil, generationReasonUnrepresentable
			}
			proxy["heartbeat-interval"] = value
		}
		if data.RequestTimeout != "" {
			value, ok := durationMilliseconds(data.RequestTimeout)
			if !ok {
				return nil, generationReasonUnrepresentable
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
		return nil, generationReasonUnsupportedProtocol
	}
}

func singBoxOutbound(profile *profiles.Profile, name string) (map[string]any, string) {
	switch data := profile.Data.(type) {
	case profiles.ShadowsocksData:
		if _, ok := singBoxShadowsocksMethods[strings.ToLower(data.Method)]; !ok {
			return nil, generationReasonClientVersion
		}
		outbound := map[string]any{"type": "shadowsocks", "tag": name, "server": profile.Server, "server_port": portNumber(profile.Port), "method": data.Method, "password": data.Password.Reveal()}
		if data.Plugin != nil {
			plugin, ok := singBoxShadowsocksPlugin(data.Plugin)
			if !ok {
				return nil, generationReasonPlugin
			}
			outbound["plugin"] = plugin
			if data.Plugin.Options.IsSet() {
				outbound["plugin_opts"] = data.Plugin.Options.Reveal()
			}
		}
		return outbound, ""
	case profiles.Hysteria2Data:
		if data.ObfuscationType == "gecko" {
			return nil, generationReasonClientVersion
		}
		if data.CertificateSHA256 != "" {
			return nil, generationReasonUnrepresentable
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
			return nil, generationReasonCompatibility
		}
		if tuicHasFieldClass(data, profiles.TUICProvenanceMihomo) {
			return nil, generationReasonUnrepresentable
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
		return nil, generationReasonUnsupportedProtocol
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

func xrayJSONForEntryWithNames(entry deliveryEntry, names *uniqueNames) ([]string, *generationExclusion) {
	scheme := supportedConfigScheme(entry.Raw)
	if scheme == "ss" {
		profile, exclusion := structuredProfile(entry, "xray-json")
		if exclusion != nil {
			return nil, exclusion
		}
		data := profile.Data.(profiles.ShadowsocksData)
		if data.Plugin != nil {
			return nil, &generationExclusion{entry.safeRef(), "shadowsocks", "xray-json", generationReasonPlugin}
		}
		if _, ok := xrayShadowsocksMethods[strings.ToLower(data.Method)]; !ok {
			return nil, &generationExclusion{entry.safeRef(), "shadowsocks", "xray-json", generationReasonClientVersion}
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
			return nil, &generationExclusion{entry.safeRef(), "shadowsocks", "xray-json", generationReasonSerialization}
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
			return nil, &generationExclusion{entry.safeRef(), "hysteria2", "xray-json", generationReasonUnrepresentable}
		}
		if data.Insecure {
			// Xray v26.3.27 and v26.7.28 reject the removed allowInsecure
			// setting. A URI that disables certificate verification cannot be
			// safely translated into a certificate pin or peer-name constraint.
			return nil, &generationExclusion{entry.safeRef(), "hysteria2", "xray-json", generationReasonUnrepresentable}
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
			return nil, &generationExclusion{entry.safeRef(), "hysteria2", "xray-json", generationReasonSerialization}
		}
		return []string{string(payload)}, nil
	}
	if scheme == "tuic" {
		profile, err := profiles.Parse(entry.Raw)
		protocol := safeProtocolName(entry.Raw, entry.StoredProtocol)
		if err == nil {
			protocol = string(profile.Protocol)
			if data, ok := profile.Data.(profiles.TUICData); ok && data.Generation == 4 {
				return nil, &generationExclusion{entry.safeRef(), protocol, "xray-json", generationReasonCompatibility}
			}
		}
		return nil, &generationExclusion{entry.safeRef(), protocol, "xray-json", generationReasonUnsupportedProtocol}
	}
	if scheme == model.SubscriptionFormatXrayJSON {
		if !json.Valid([]byte(entry.Raw)) {
			return nil, &generationExclusion{entry.safeRef(), "xray-json", "xray-json", generationReasonInvalidStored}
		}
		var value any
		if err := json.Unmarshal([]byte(entry.Raw), &value); err != nil {
			return nil, &generationExclusion{entry.safeRef(), "xray-json", "xray-json", generationReasonInvalidStored}
		}
		normalized, err := json.Marshal(value)
		if err != nil {
			return nil, &generationExclusion{entry.safeRef(), "xray-json", "xray-json", generationReasonSerialization}
		}
		return []string{string(normalized)}, nil
	}
	converted, err := normalizeConfigurationForSubscriptionOutput(entry.Raw, model.SubscriptionFormatXrayJSON, entry.Label)
	if err != nil {
		return nil, &generationExclusion{entry.safeRef(), safeProtocolName(entry.Raw, entry.StoredProtocol), "xray-json", generationReasonInvalidStored}
	}
	return []string{converted}, nil
}

func applyGenerationExclusionHeaders(headers interface{ Set(string, string) }, exclusions []generationExclusion) {
	if len(exclusions) == 0 {
		return
	}
	counts := generationExclusionCounts(exclusions)
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

func generationExclusionCounts(exclusions []generationExclusion) map[string]int {
	allowed := make(map[string]struct{})
	for _, code := range generationExclusionReasonCodes() {
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

func generationFailurePayload(generated generatedSubscription) subscriptionGenerationFailure {
	return subscriptionGenerationFailure{
		ErrorCode:       generationReasonAllExcluded,
		OutputFormat:    generated.OutputFormat,
		EligibleCount:   generated.EligibleCount,
		ExcludedCount:   len(generated.Exclusions),
		ExclusionCounts: generationExclusionCounts(generated.Exclusions),
	}
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for cursor := index; cursor > 0 && values[cursor] < values[cursor-1]; cursor-- {
			values[cursor], values[cursor-1] = values[cursor-1], values[cursor]
		}
	}
}

func validateGeneratedStructuredBody(format, body string) error {
	if hasUnsafeStructuredControl(body) {
		return errors.New(generationReasonSerialization)
	}
	switch format {
	case "xray-json":
		var documents []map[string]any
		if err := json.Unmarshal([]byte(body), &documents); err != nil || len(documents) == 0 {
			return errors.New(generationReasonSerialization)
		}
		for _, document := range documents {
			outbounds, ok := document["outbounds"].([]any)
			if !ok || !validJSONOutboundList(outbounds, "protocol") {
				return errors.New(generationReasonSerialization)
			}
		}
	case "sing-box":
		var document map[string]any
		if err := json.Unmarshal([]byte(body), &document); err != nil {
			return errors.New(generationReasonSerialization)
		}
		outbounds, ok := document["outbounds"].([]any)
		if !ok || !validJSONOutboundList(outbounds, "type") {
			return errors.New(generationReasonSerialization)
		}
	case "mihomo":
		var document yaml.Node
		if err := yaml.Unmarshal([]byte(body), &document); err != nil || yamlHasDuplicateMappingKey(&document) || !validMihomoDocument(&document) {
			return errors.New(generationReasonSerialization)
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

func encodeBase64Subscription(body string) string {
	return base64.StdEncoding.EncodeToString([]byte(body))
}
