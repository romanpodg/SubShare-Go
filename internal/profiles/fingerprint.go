package profiles

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

const fingerprintVersionPrefix = "pf1_"

// CanonicalFingerprintInput returns deterministic secret-bearing semantic
// material in a redacting wrapper. Display names and original URI bytes are
// excluded. Callers should normally use Fingerprint instead.
func CanonicalFingerprintInput(profile *Profile) (SensitiveValue, error) {
	return defaultRegistry.CanonicalFingerprintInput(profile)
}

func (registry *Registry) CanonicalFingerprintInput(profile *Profile) (SensitiveValue, error) {
	return registry.fingerprintInput(profile)
}

// Fingerprint returns a versioned HMAC-SHA-256 digest. A caller-managed key is
// mandatory because canonical material contains credentials.
func Fingerprint(profile *Profile, key []byte) (string, error) {
	return defaultRegistry.Fingerprint(profile, key)
}

func (registry *Registry) Fingerprint(profile *Profile, key []byte) (string, error) {
	if len(key) == 0 {
		return "", newError(ErrorMissingFingerprintKey, "", "key")
	}
	input, err := registry.fingerprintInput(profile)
	if err != nil {
		return "", err
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte(input.Reveal()))
	return fingerprintVersionPrefix + hex.EncodeToString(digest.Sum(nil)), nil
}

func semanticFingerprintInput(profile *Profile, fields []canonicalParameter, extras []string) SensitiveValue {
	lines := []string{
		"version=1",
		"protocol=" + string(profile.Protocol),
		"server=" + strings.ToLower(profile.Server),
		"port=" + profile.Port.Expression,
	}
	sort.SliceStable(fields, func(left, right int) bool {
		if fields[left].key != fields[right].key {
			return fields[left].key < fields[right].key
		}
		return fields[left].value < fields[right].value
	})
	for _, field := range fields {
		lines = append(lines, "field="+field.key+"\x00"+field.value)
	}
	sort.Strings(extras)
	lines = append(lines, extras...)
	return NewSensitiveValue(strings.Join(lines, "\n"))
}

// fingerprintExtraParameterLines removes the first occurrence of every known
// semantic parameter because adapters already emit its normalized value.
// Duplicate known parameters and every unknown extension remain part of the
// connectivity identity. Ordering between different keys is ignored, while
// the order of repeated values for the same key is retained because clients
// can apply first- or last-value-wins semantics.
func fingerprintExtraParameterLines(parameters []QueryParameter, aliases map[string]string) []string {
	type value struct {
		marker string
		value  string
	}
	seenKnown := make(map[string]int)
	groups := make(map[string][]value)
	for _, parameter := range parameters {
		key := parameter.Key
		if canonical, known := aliases[strings.ToLower(parameter.Key)]; known {
			seenKnown[canonical]++
			if seenKnown[canonical] == 1 {
				continue
			}
			key = canonical
		}
		marker := "0"
		if parameter.HasValue {
			marker = "1"
		}
		groups[key] = append(groups[key], value{marker: marker, value: parameter.Value.Reveal()})
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(parameters))
	for _, key := range keys {
		for index, item := range groups[key] {
			lines = append(lines, "query="+key+"\x00"+item.marker+"\x00"+strconv.Itoa(index)+"\x00"+item.value)
		}
	}
	return lines
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
