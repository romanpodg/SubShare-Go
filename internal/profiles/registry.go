package profiles

import (
	"strings"
	"sync"
)

const maxProfileURIBytes = 64 * 1024

// Adapter is an explicit protocol boundary. Implementations own parsing,
// validation, canonicalization, serialization, and secret-bearing fingerprint
// input for one protocol.
type Adapter interface {
	Protocol() Protocol
	Schemes() []string
	Parse(raw string) (*Profile, error)
	Validate(profile *Profile) error
	Canonicalize(profile *Profile) (*Profile, error)
	SerializeCanonical(profile *Profile) (SerializationResult, error)
	FingerprintInput(profile *Profile) (SensitiveValue, error)
}

// Registry routes URI schemes and profiles without handler-level switches.
type Registry struct {
	mu         sync.RWMutex
	byScheme   map[string]Adapter
	byProtocol map[Protocol]Adapter
}

func NewRegistry() *Registry {
	return &Registry{
		byScheme:   make(map[string]Adapter),
		byProtocol: make(map[Protocol]Adapter),
	}
}

func (registry *Registry) Register(adapter Adapter) error {
	if registry == nil || adapter == nil || adapter.Protocol() == "" || len(adapter.Schemes()) == 0 {
		return newError(ErrorInvalidProfile, "", "adapter")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.byProtocol[adapter.Protocol()]; exists {
		return newError(ErrorInvalidProfile, adapter.Protocol(), "adapter")
	}
	for _, scheme := range adapter.Schemes() {
		normalized := strings.ToLower(strings.TrimSpace(scheme))
		if normalized == "" {
			return newError(ErrorInvalidProfile, adapter.Protocol(), "scheme")
		}
		if _, exists := registry.byScheme[normalized]; exists {
			return newError(ErrorInvalidProfile, adapter.Protocol(), "scheme")
		}
	}
	registry.byProtocol[adapter.Protocol()] = adapter
	for _, scheme := range adapter.Schemes() {
		registry.byScheme[strings.ToLower(strings.TrimSpace(scheme))] = adapter
	}
	return nil
}

func (registry *Registry) Parse(raw string) (*Profile, error) {
	if len(raw) > maxProfileURIBytes {
		return nil, newError(ErrorInvalidProfile, "", "uri_size")
	}
	scheme, err := detectScheme(raw)
	if err != nil {
		return nil, err
	}
	registry.mu.RLock()
	adapter := registry.byScheme[scheme]
	registry.mu.RUnlock()
	if adapter == nil {
		return nil, newError(ErrorUnsupportedScheme, "", "scheme")
	}
	return adapter.Parse(raw)
}

func (registry *Registry) Validate(profile *Profile) error {
	adapter, err := registry.adapterForProfile(profile)
	if err != nil {
		return err
	}
	return adapter.Validate(profile)
}

func (registry *Registry) Canonicalize(profile *Profile) (*Profile, error) {
	adapter, err := registry.adapterForProfile(profile)
	if err != nil {
		return nil, err
	}
	return adapter.Canonicalize(profile)
}

func (registry *Registry) Serialize(profile *Profile, mode SerializationMode) (SerializationResult, error) {
	if profile == nil {
		return SerializationResult{}, newError(ErrorInvalidProfile, "", "profile")
	}
	if mode == OriginalSerialization {
		if !profile.OriginalURI.IsSet() {
			return SerializationResult{}, newError(ErrorInvalidProfile, profile.Protocol, "original_uri")
		}
		return SerializationResult{
			URI:                profile.OriginalURI,
			Mode:               OriginalSerialization,
			Exact:              true,
			SemanticallyStable: true,
			Warnings:           append([]Warning(nil), profile.Warnings...),
		}, nil
	}
	if mode != CanonicalSerialization {
		return SerializationResult{}, newError(ErrorInvalidProfile, profile.Protocol, "serialization_mode")
	}
	adapter, err := registry.adapterForProfile(profile)
	if err != nil {
		return SerializationResult{}, err
	}
	if !profile.Capabilities.CanonicalSerialize {
		return SerializationResult{}, newError(ErrorCompatibilityOnlyInput, profile.Protocol, "generation")
	}
	canonical, err := adapter.Canonicalize(profile)
	if err != nil {
		return SerializationResult{}, err
	}
	return adapter.SerializeCanonical(canonical)
}

func (registry *Registry) fingerprintInput(profile *Profile) (SensitiveValue, error) {
	adapter, err := registry.adapterForProfile(profile)
	if err != nil {
		return SensitiveValue{}, err
	}
	if !profile.Capabilities.Fingerprint {
		return SensitiveValue{}, newError(ErrorCompatibilityOnlyInput, profile.Protocol, "fingerprint")
	}
	canonical, err := adapter.Canonicalize(profile)
	if err != nil {
		return SensitiveValue{}, err
	}
	return adapter.FingerprintInput(canonical)
}

func (registry *Registry) adapterForProfile(profile *Profile) (Adapter, error) {
	if registry == nil || profile == nil {
		return nil, newError(ErrorInvalidProfile, "", "profile")
	}
	registry.mu.RLock()
	adapter := registry.byProtocol[profile.Protocol]
	registry.mu.RUnlock()
	if adapter == nil {
		return nil, newError(ErrorUnsupportedScheme, profile.Protocol, "protocol")
	}
	return adapter, nil
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	for _, adapter := range []Adapter{
		vlessAdapter{},
		vmessAdapter{},
		trojanAdapter{},
		shadowsocksAdapter{},
		hysteria2Adapter{},
		tuicAdapter{},
	} {
		if err := registry.Register(adapter); err != nil {
			panic(err)
		}
	}
	return registry
}

var defaultRegistry = NewDefaultRegistry()

func Parse(raw string) (*Profile, error) {
	return defaultRegistry.Parse(raw)
}

func Validate(profile *Profile) error {
	return defaultRegistry.Validate(profile)
}

// Canonicalize returns a normalized structured clone while retaining the exact
// OriginalURI and source-ordered QueryParameters. It does not produce bytes;
// use Serialize with an explicit mode to choose raw or canonical output.
func Canonicalize(profile *Profile) (*Profile, error) {
	return defaultRegistry.Canonicalize(profile)
}

func Serialize(profile *Profile, mode SerializationMode) (SerializationResult, error) {
	return defaultRegistry.Serialize(profile, mode)
}
