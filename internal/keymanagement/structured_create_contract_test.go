package keymanagement

import (
	"net/url"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestStructuredCreateTUICALPNPreservesExplicitEmptyAndOmission(t *testing.T) {
	for _, test := range []struct {
		name string
		alpn *model.TriStatePatch[[]string]
		present bool
	}{
		{name: "omitted"},
		{name: "empty list", alpn: &model.TriStatePatch[[]string]{Set: true, Operation: model.TriStateSet}},
		{name: "explicit empty entry", alpn: &model.TriStatePatch[[]string]{Set: true, Operation: model.TriStateSet, Value: []string{""}}, present: true},
		{name: "clear", alpn: &model.TriStatePatch[[]string]{Set: true, Operation: model.TriStateClear, Value: []string{"h3"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Missing credentials deliberately exercise the legacy-compatible fallback.
			uri, err := BuildURIFromStructuredCreate("tuic", "fallback", &model.StructuredProfilePatch{
				Server: set("edge.example"), TUIC: &model.TUICStructuredPatch{ALPN: test.alpn},
			})
			requireFixturef(t, err == nil, "build fallback: %v", err)
			parsed, err := url.Parse(uri)
			requireFixturef(t, err == nil, "parse escaped URI: %v", err)
			requireFixturef(t, parsed.Query().Has("alpn") == test.present, "ALPN presence changed: %s", uri)
			requireFixturef(t, parsed.Query().Get("alpn") == "", "unexpected ALPN value: %s", uri)
		})
	}
}
