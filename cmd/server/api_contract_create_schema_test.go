package main

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAPIContractCreateSchemaSeparatesInformationalProtocol(t *testing.T) {
	var document map[string]any
	requireRepositorySuccess(t, yaml.Unmarshal(openAPISpec, &document))
	protocols := apiContractCreateProtocolSchemas(t, document)
	for _, test := range []struct {
		kind, protocol string
		admitted       bool
	}{
		{"real", "", false}, {"informational", "", true}, {"real", "vless", true},
	} {
		t.Run(test.kind+"/"+test.protocol, func(t *testing.T) {
			choices := protocols[test.kind]["enum"].([]any)
			requireRepositoryEqual(t, "documented create protocol admission", slices.Contains(choices, any(test.protocol)), test.admitted)
		})
	}
}

func apiContractCreateProtocolSchemas(t *testing.T, document map[string]any) map[string]map[string]any {
	t.Helper()
	root := document["components"].(map[string]any)["schemas"].(map[string]any)["CreateKeyProfile"].(map[string]any)
	discriminator, ok := root["discriminator"].(map[string]any)
	if !ok {
		t.Fatal("missing discriminated create request variants")
	}
	requireRepositoryEqual(t, "create discriminator", discriminator["propertyName"], "kind")
	mapping := discriminator["mapping"].(map[string]any)
	protocols := map[string]map[string]any{}
	for _, entry := range root["oneOf"].([]any) {
		variant := resolveAPIContractSchema(t, document, entry)
		parts := variant["allOf"].([]any)
		constraint := resolveAPIContractSchema(t, document, parts[len(parts)-1])
		properties := constraint["properties"].(map[string]any)
		kind := properties["kind"].(map[string]any)["enum"].([]any)[0].(string)
		requireRepositoryEqual(t, "discriminator mapped variant", mapping[kind], entry.(map[string]any)["$ref"])
		protocols[kind] = resolveAPIContractSchema(t, document, properties["protocol"])
	}
	return protocols
}
