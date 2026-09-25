package delivery

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

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

type Exclusion struct {
	RecordRef string `json:"record_ref"`
	Protocol  string `json:"protocol"`
	Format    string `json:"format"`
	Reason    string `json:"reason_code"`
}

func (item Exclusion) Error() string {
	return "subscription_generation_exclusion{record_ref=" + item.RecordRef + ", protocol=" + item.Protocol + ", format=" + item.Format + ", reason=" + item.Reason + "}"
}

// exclusionList adapts an optional exclusion to the slice form renderers
// accumulate.
func exclusionList(exclusion *Exclusion) []Exclusion {
	if exclusion == nil {
		return nil
	}
	return []Exclusion{*exclusion}
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

// exclude records why this entry was left out of an output format.
func (entry Entry) exclude(protocol, format, reason string) *Exclusion {
	return &Exclusion{RecordRef: entry.SafeRef(), Protocol: protocol, Format: format, Reason: reason}
}

// protocolName is the safe protocol label for exclusions when the raw URI
// may not parse.
func (entry Entry) protocolName() string {
	return SafeProtocolName(entry.Raw, entry.StoredProtocol)
}

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

func draftDisplayName(draft profileconfig.LinkConfigurationDraft, index int) string {
	if name := strings.TrimSpace(draft.Remark); name != "" {
		return name
	}
	if name := strings.TrimSpace(draft.ServerDescription); name != "" {
		return name
	}
	return fmt.Sprintf("%s-%d", draft.Server, index+1)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// setIfNotEmpty writes value under key only when it carries content.
func setIfNotEmpty(target map[string]any, key, value string) {
	if value != "" {
		target[key] = value
	}
}

// setIfTrue writes a literal true under key only when the flag is on.
func setIfTrue(target map[string]any, key string, on bool) {
	if on {
		target[key] = true
	}
}
