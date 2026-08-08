package profiles

import (
	"encoding/json"
	"fmt"
	"io"
)

// Protocol is a canonical protocol identifier independent of URI aliases.
type Protocol string

const (
	ProtocolVLESS       Protocol = "vless"
	ProtocolVMess       Protocol = "vmess"
	ProtocolTrojan      Protocol = "trojan"
	ProtocolShadowsocks Protocol = "shadowsocks"
	ProtocolHysteria2   Protocol = "hysteria2"
	ProtocolTUIC        Protocol = "tuic"
)

// SensitiveValue forces callers to opt in before accessing secret-bearing
// values. Formatting and JSON marshaling are redacted by design.
type SensitiveValue struct {
	value string
}

// NewSensitiveValue marks value as secret-bearing.
func NewSensitiveValue(value string) SensitiveValue {
	return SensitiveValue{value: value}
}

// Reveal explicitly returns the wrapped value.
func (value SensitiveValue) Reveal() string {
	return value.value
}

// IsSet reports whether the wrapped value is non-empty.
func (value SensitiveValue) IsSet() bool {
	return value.value != ""
}

func (value SensitiveValue) String() string {
	if !value.IsSet() {
		return ""
	}
	return "[redacted]"
}

func (value SensitiveValue) GoString() string {
	return value.String()
}

// Format prevents fmt verbs such as %#v from exposing the wrapped value.
func (value SensitiveValue) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, value.String())
}

// MarshalJSON emits a redaction marker rather than secret material.
func (value SensitiveValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(value.String())
}

type PortKind string

const (
	PortSingle     PortKind = "single"
	PortExpression PortKind = "expression"
)

// PortRange is an inclusive port interval. A single port has Start == End.
type PortRange struct {
	Start uint16
	End   uint16
}

// PortSpec retains a canonical port expression without collapsing Hysteria 2
// port hopping into one selected port.
type PortSpec struct {
	Expression string
	Kind       PortKind
	Ranges     []PortRange
	// Explicit distinguishes an omitted protocol default from a port that was
	// present in the parsed authority. It does not change connectivity and is
	// therefore excluded from fingerprints.
	Explicit bool
}

func (port PortSpec) String() string {
	return port.Expression
}

// QueryParameter retains duplicate entries in source order. Typed adapters use
// the first known value and emit a warning for duplicates. Decoded and raw
// keys/values are sensitive because unrecognized extensions can contain tokens.
type QueryParameter struct {
	Key      string
	Value    SensitiveValue
	HasValue bool
	rawKey   string
	rawValue SensitiveValue
}

func (parameter QueryParameter) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "query_parameter{value=[redacted]}")
}

func (parameter QueryParameter) String() string   { return "query_parameter{value=[redacted]}" }
func (parameter QueryParameter) GoString() string { return parameter.String() }

func (parameter QueryParameter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		HasValue bool   `json:"has_value"`
	}{Key: "[redacted]", Value: "[redacted]", HasValue: parameter.HasValue})
}

type Warning struct {
	Code    string
	Message string
}

const (
	WarningLegacyInput            = "legacy_input"
	WarningDuplicateParameter     = "duplicate_parameter"
	WarningAmbiguousParameter     = "ambiguous_parameter"
	WarningCompatibilityOnly      = "compatibility_only"
	WarningNonCanonicalScheme     = "non_canonical_scheme"
	WarningConflictingPreference  = "conflicting_preference"
	WarningClientVersionSensitive = "client_version_sensitive"
)

type CapabilityStatus string

const (
	CapabilityFull     CapabilityStatus = "full"
	CapabilityReadOnly CapabilityStatus = "read_only"
)

// Capabilities describes what can safely be done with a parsed profile.
type Capabilities struct {
	Status              CapabilityStatus
	Generation          string
	Parse               bool
	Validate            bool
	CanonicalSerialize  bool
	Generate            bool
	Fingerprint         bool
	ExactOriginalOutput bool
}

// Profile is the common, loss-aware representation shared by all adapters.
// Data contains one of the protocol-specific data structs below.
type Profile struct {
	Protocol               Protocol
	Server                 string
	Port                   PortSpec
	DisplayName            string
	OriginalURI            SensitiveValue
	Data                   any
	QueryParameters        []QueryParameter
	UnknownQueryParameters []QueryParameter
	Warnings               []Warning
	Capabilities           Capabilities
}

// Format makes accidental logging of a complete Profile safe. Protocol data,
// query values, and both original and canonical URIs are intentionally absent.
func (profile Profile) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "profile{protocol="+string(profile.Protocol)+", server="+profile.Server+", port="+profile.Port.Expression+", display_name="+profile.DisplayName+"}")
}

func (profile Profile) String() string {
	return fmt.Sprintf("%v", profile.SafeMetadata())
}

func (profile Profile) GoString() string {
	return profile.String()
}

// MarshalJSON intentionally exposes only SafeMetadata. The full Profile is
// an in-process model and is not a JSON serialization contract: it contains
// credentials, raw URI bytes, extension values, and canonical secret material.
func (profile Profile) MarshalJSON() ([]byte, error) {
	return json.Marshal(profile.SafeMetadata())
}

// SafeMetadata contains no credentials, query values, or complete URIs.
type SafeMetadata struct {
	Protocol     Protocol
	Server       string
	Port         string
	DisplayName  string
	Capabilities Capabilities
	Warnings     []Warning
}

func (profile *Profile) SafeMetadata() SafeMetadata {
	if profile == nil {
		return SafeMetadata{}
	}
	return SafeMetadata{
		Protocol:     profile.Protocol,
		Server:       profile.Server,
		Port:         profile.Port.Expression,
		DisplayName:  profile.DisplayName,
		Capabilities: profile.Capabilities,
		Warnings:     append([]Warning(nil), profile.Warnings...),
	}
}

type SerializationMode string

const (
	// OriginalSerialization returns the exact input bytes retained by Parse.
	// It is the only mode that may report Exact=true.
	OriginalSerialization SerializationMode = "original"
	// CanonicalSerialization returns a deterministic semantic normalization.
	// It never claims byte-exact reproduction, including for unambiguous input.
	CanonicalSerialization SerializationMode = "canonical"
)

// SerializationResult distinguishes exact source reproduction from a
// normalized semantic representation. URI is always sensitive. Exact describes
// byte equality with the original input; SemanticallyStable means a successful
// canonical output is intended to reparse to the same connectivity identity.
type SerializationResult struct {
	URI                SensitiveValue
	Mode               SerializationMode
	Exact              bool
	SemanticallyStable bool
	Warnings           []Warning
}

func (result SerializationResult) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "serialization{mode="+string(result.Mode)+", exact="+fmt.Sprint(result.Exact)+", uri=[redacted]}")
}

func (result SerializationResult) String() string {
	return "serialization{mode=" + string(result.Mode) + ", exact=" + fmt.Sprint(result.Exact) + ", uri=[redacted]}"
}
func (result SerializationResult) GoString() string { return result.String() }

type ShadowsocksUserInfoStyle string

const (
	ShadowsocksUserInfoBase64 ShadowsocksUserInfoStyle = "base64url"
	ShadowsocksUserInfoPlain  ShadowsocksUserInfoStyle = "plain"
	ShadowsocksUserInfoLegacy ShadowsocksUserInfoStyle = "legacy_full_base64"
)

type ShadowsocksPlugin struct {
	Name    string
	Options SensitiveValue
}

func (plugin ShadowsocksPlugin) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "shadowsocks_plugin{options=[redacted]}")
}

type ShadowsocksData struct {
	Method        string
	Password      SensitiveValue
	Plugin        *ShadowsocksPlugin
	UserInfoStyle ShadowsocksUserInfoStyle
	LegacyInput   bool
}

func (data ShadowsocksData) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "shadowsocks_data{credentials=[redacted]}")
}

type Hysteria2Data struct {
	Authentication      SensitiveValue
	SNI                 string
	Insecure            bool
	CertificateSHA256   string
	ObfuscationType     string
	ObfuscationPassword SensitiveValue
}

func (data Hysteria2Data) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "hysteria2_data{credentials=[redacted]}")
}

type TUICData struct {
	Generation                  int
	UUID                        SensitiveValue
	Password                    SensitiveValue
	Token                       SensitiveValue
	SNI                         string
	ALPN                        []string
	SkipCertificateVerification bool
	DisableSNI                  bool
	CongestionController        string
	UDPRelayMode                string
	UDPOverStream               bool
	ZeroRTT                     bool
	Heartbeat                   string
	RequestTimeout              string
	FastOpen                    bool
	MaxOpenStreams              int
	MaxUDPRelayPacketSize       int
	FieldObservations           []TUICFieldObservation
}

func (data TUICData) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "tuic_data{credentials=[redacted]}")
}

// TUICFieldProvenance records where a compatibility URI spelling comes from.
// TUIC v5 specifies the wire authentication model, but not a universal share
// URI, so preserving this distinction is required for later generator policy.
type TUICFieldProvenance string

const (
	TUICProvenanceProtocolNative TUICFieldProvenance = "protocol_native_v5"
	TUICProvenanceCommonClient   TUICFieldProvenance = "common_client"
	TUICProvenanceMihomo         TUICFieldProvenance = "mihomo_extension"
	TUICProvenanceSingBox        TUICFieldProvenance = "sing_box_extension"
	TUICProvenanceUnknown        TUICFieldProvenance = "unknown_extension"
	TUICProvenanceV4             TUICFieldProvenance = "tuic_v4_compatibility"
)

// TUICFieldObservation preserves both the normalized field support class and
// the dialect that supplied its spelling. SourceName is redacting because even
// unknown query keys cannot safely be assumed public.
type TUICFieldObservation struct {
	Field            string
	SourceName       SensitiveValue
	FieldClass       TUICFieldProvenance
	SourceProvenance TUICFieldProvenance
}

func (observation TUICFieldObservation) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "tuic_field_observation{source=[redacted]}")
}

type VLESSData struct {
	UUID SensitiveValue
}

func (data VLESSData) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "vless_data{credential=[redacted]}")
}

type TrojanData struct {
	Password SensitiveValue
}

func (data TrojanData) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "trojan_data{credential=[redacted]}")
}

type VMessData struct {
	UUID       SensitiveValue
	RawPayload SensitiveValue
}

func (data VMessData) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "vmess_data{credentials=[redacted]}")
}
