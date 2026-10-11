package main

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"gopkg.in/yaml.v3"
)

func TestAPIContractPanelSchemaMatchesActualBrandingFields(t *testing.T) {
	var document map[string]any
	requireRepositorySuccess(t, yaml.Unmarshal(openAPISpec, &document))
	properties := document["components"].(map[string]any)["schemas"].(map[string]any)["PanelSettings"].(map[string]any)["properties"].(map[string]any)
	encoded, err := json.Marshal(model.PanelSettings{PanelTitle: "fixture", SubscriptionPageConfig: "{}"})
	requireRepositorySuccess(t, err)
	var actual map[string]any
	requireRepositorySuccess(t, json.Unmarshal(encoded, &actual))
	requireRepositoryEqual(t, "documented branding wire fields", apiContractFieldNames(properties), apiContractFieldNames(actual))
}

func apiContractFieldNames(object map[string]any) []string {
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
