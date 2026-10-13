package keymanagement

import (
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"net"
	"net/url"
)

// BuildURIFromStructuredCreate builds an escaped share URI and lets the adapter
// canonicalize valid profiles. Invalid legacy-compatible inputs retain their bytes.
func BuildURIFromStructuredCreate(proto, label string, patch *model.StructuredProfilePatch) (string, error) {
	if patch == nil {
		return "", fmt.Errorf("missing structured patch payload")
	}
	built, query, err := structuredConnection(proto, patch)
	if err != nil {
		return "", err
	}
	built.Host = net.JoinHostPort(setString(patch.Server, ""), setString(patch.Port, "443"))
	built.RawQuery = query.Encode()
	built.Fragment = setString(patch.DisplayName, label)
	return canonicalCreateURI(built.String()), nil
}

func canonicalCreateURI(raw string) string {
	parsed, err := profiles.Parse(raw)
	if err != nil {
		return raw
	}
	canonical, err := profiles.Serialize(parsed, profiles.CanonicalSerialization)
	if err != nil {
		return raw
	}
	return canonical.URI.Reveal()
}

func structuredConnection(proto string, patch *model.StructuredProfilePatch) (url.URL, url.Values, error) {
	switch proto {
	case "shadowsocks":
		return shadowsocksConnection(patch.Shadowsocks)
	case "hysteria2", "hy2":
		return hysteria2Connection(patch.Hysteria2)
	case "tuic":
		return tuicConnection(patch.TUIC)
	default:
		return url.URL{}, nil, fmt.Errorf("unsupported protocol for structured creation: %s", proto)
	}
}

func setField[T any](field *model.TriStatePatch[T], fallback T) T {
	if field == nil {
		return fallback
	}
	if !field.Set {
		return fallback
	}
	if field.Operation != model.TriStateSet {
		return fallback
	}
	return field.Value
}

func setString(field *model.TriStatePatch[string], fallback string) string {
	return setField(field, fallback)
}

func setBool(field *model.TriStatePatch[bool]) bool {
	return setField(field, false)
}

func setIf(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}
