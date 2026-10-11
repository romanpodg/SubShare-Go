package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAPIContractOpenAPIResponses(t *testing.T) {
	var document map[string]any
	requireRepositorySuccess(t, yaml.Unmarshal(openAPISpec, &document))
	for name, c := range loadAPIContracts(t) {
		if c.OpenAPIPath == "" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			contract := apiSchemaContract{t, document}
			var value any
			requireRepositorySuccess(t, json.Unmarshal(c.Response, &value))
			contract.assert(contract.responseSchema(c), value, "response")
		})
	}
}

func TestAPIContractOpenAPIPatchAdmission(t *testing.T) {
	var document map[string]any
	requireRepositorySuccess(t, yaml.Unmarshal(openAPISpec, &document))
	operation := document["paths"].(map[string]any)["/users/{id}/subscription"].(map[string]any)["patch"].(map[string]any)
	content := operation["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	schema := resolveAPIContractSchema(t, document, content["schema"])
	for name, test := range loadAPIContracts(t) {
		if test.Method != "PATCH" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			apiSchemaContract{t, document}.assertPatchAdmission(schema, test)
		})
	}
}

func resolveAPIContractSchema(t *testing.T, document map[string]any, raw any) map[string]any {
	t.Helper()
	schema, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("missing schema: %T", raw)
	}
	ref, ok := schema["$ref"].(string)
	if !ok {
		return schema
	}
	var current any = document
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		current = current.(map[string]any)[part]
	}
	return resolveAPIContractSchema(t, document, current)
}

// Validate the response shape subset used by these fixtures: refs, allOf,
// required properties, primitive/null types, arrays and enumerated strings.
// This is not a general OpenAPI validation or schema-generation framework.
type apiSchemaContract struct {
	t        *testing.T
	document map[string]any
}

func (contract apiSchemaContract) responseSchema(c apiContractCase) any {
	contract.t.Helper()
	paths := contract.document["paths"].(map[string]any)
	operation := paths[c.OpenAPIPath].(map[string]any)[strings.ToLower(c.Method)].(map[string]any)
	responses := operation["responses"].(map[string]any)
	response := responses[strconv.Itoa(c.Status)]
	if response == nil {
		response = responses["default"]
	}
	if response == nil {
		contract.t.Fatalf("missing documented response %d", c.Status)
	}
	resolved := resolveAPIContractSchema(contract.t, contract.document, response)
	content, ok := resolved["content"].(map[string]any)
	if !ok {
		contract.t.Fatal("missing response content schema")
	}
	return content["application/json"].(map[string]any)["schema"]
}

func (c apiSchemaContract) assertPatchAdmission(schema map[string]any, test apiContractCase) {
	c.t.Helper()
	var body map[string]any
	requireRepositorySuccess(c.t, json.Unmarshal(test.Body, &body))
	if test.Status == 200 {
		c.assert(schema, body, "request")
		return
	}
	for field, value := range body {
		property := schema["properties"].(map[string]any)[field].(map[string]any)
		admitted := apiContractMatchesType(property["type"], value) && apiContractMatchesBounds(property, value)
		requireRepositoryEqual(c.t, "documented rejected field", admitted, false)
	}
}

func (c apiSchemaContract) assert(raw, value any, location string) {
	c.t.Helper()
	schema := resolveAPIContractSchema(c.t, c.document, raw)
	if parts, ok := schema["allOf"].([]any); ok {
		for _, part := range parts {
			c.assert(part, value, location)
		}
	}
	if kind := schema["type"]; kind != nil && !apiContractMatchesType(kind, value) {
		c.t.Fatalf("%s type %T contradicts documented %v", location, value, kind)
	}
	c.assertEnum(schema, value, location)
	if !apiContractMatchesBounds(schema, value) {
		c.t.Fatalf("%s value %v contradicts documented numeric bounds", location, value)
	}
	if object, ok := value.(map[string]any); ok {
		c.assertObject(schema, object, location)
	}
	c.assertArray(schema, value, location)
}

func (c apiSchemaContract) assertEnum(schema map[string]any, value any, location string) {
	c.t.Helper()
	if choices, ok := schema["enum"].([]any); ok {
		found := false
		for _, choice := range choices {
			found = found || reflect.DeepEqual(choice, value)
		}
		if !found {
			c.t.Fatalf("%s value %v contradicts documented enum %v", location, value, choices)
		}
	}
}

func (c apiSchemaContract) assertArray(schema map[string]any, value any, location string) {
	c.t.Helper()
	if items, ok := value.([]any); ok && schema["items"] != nil {
		for i, item := range items {
			c.assert(schema["items"], item, fmt.Sprintf("%s[%d]", location, i))
		}
	}
}

func (c apiSchemaContract) assertRequired(schema, object map[string]any, location string) {
	c.t.Helper()
	if required, ok := schema["required"].([]any); ok {
		for _, name := range required {
			if _, exists := object[name.(string)]; !exists {
				c.t.Fatalf("%s missing required %s", location, name)
			}
		}
	}
}

func (c apiSchemaContract) assertObject(schema, object map[string]any, location string) {
	c.t.Helper()
	c.assertRequired(schema, object, location)
	properties, _ := schema["properties"].(map[string]any)
	for name, child := range properties {
		if value, exists := object[name]; exists {
			c.assert(child, value, location+"."+name)
		}
	}
}

func apiContractMatchesType(kind, value any) bool {
	if alternatives, ok := kind.([]any); ok {
		for _, item := range alternatives {
			if apiContractMatchesType(item, value) {
				return true
			}
		}
		return false
	}
	if kind == "integer" {
		number, ok := value.(float64)
		return ok && math.Trunc(number) == number
	}
	primitiveTypes := map[any]reflect.Type{"null": nil, "string": reflect.TypeOf(""), "boolean": reflect.TypeOf(false), "object": reflect.TypeOf(map[string]any{}), "array": reflect.TypeOf([]any{}), "number": reflect.TypeOf(float64(0))}
	expected, known := primitiveTypes[kind]
	return known && reflect.TypeOf(value) == expected
}

func apiContractMatchesBounds(schema map[string]any, value any) bool {
	number, numeric := value.(float64)
	if !numeric {
		return true
	}
	minimum, hasMinimum := schema["minimum"].(int)
	maximum, hasMaximum := schema["maximum"].(int)
	return (!hasMinimum || number >= float64(minimum)) && (!hasMaximum || number <= float64(maximum))
}
