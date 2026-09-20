package delivery

import (
	"encoding/json"
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

func ValidateStructuredBody(format, body string) error {
	if hasUnsafeStructuredControl(body) {
		return errors.New(ReasonSerialization)
	}
	if !validStructuredBody(format, body) {
		return errors.New(ReasonSerialization)
	}
	return nil
}

func validStructuredBody(format, body string) bool {
	switch format {
	case "xray-json":
		return validXrayJSONBody(body)
	case "sing-box":
		return validSingBoxBody(body)
	case "mihomo":
		return validMihomoBody(body)
	}
	return true
}

func validXrayJSONBody(body string) bool {
	var documents []map[string]any
	if err := json.Unmarshal([]byte(body), &documents); err != nil || len(documents) == 0 {
		return false
	}
	for _, document := range documents {
		if !validJSONOutbounds(document, "protocol") {
			return false
		}
	}
	return true
}

func validSingBoxBody(body string) bool {
	var document map[string]any
	if err := json.Unmarshal([]byte(body), &document); err != nil {
		return false
	}
	return validJSONOutbounds(document, "type")
}

func validJSONOutbounds(document map[string]any, protocolField string) bool {
	outbounds, ok := document["outbounds"].([]any)
	return ok && validJSONOutboundList(outbounds, protocolField)
}

func validJSONOutboundList(values []any, protocolField string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !hasNonEmptyStringField(value, protocolField) {
			return false
		}
	}
	return true
}

func hasNonEmptyStringField(value any, field string) bool {
	outbound, ok := value.(map[string]any)
	if !ok {
		return false
	}
	text, ok := outbound[field].(string)
	return ok && strings.TrimSpace(text) != ""
}

func validMihomoBody(body string) bool {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(body), &document); err != nil {
		return false
	}
	if yamlHasDuplicateMappingKey(&document) {
		return false
	}
	return validMihomoDocument(&document)
}

func yamlHasDuplicateMappingKey(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.MappingNode && yamlMappingHasDuplicateKey(node) {
		return true
	}
	for _, child := range node.Content {
		if yamlHasDuplicateMappingKey(child) {
			return true
		}
	}
	return false
}

func yamlMappingHasDuplicateKey(mapping *yaml.Node) bool {
	seen := make(map[string]struct{}, len(mapping.Content)/2)
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key := mapping.Content[index].Value
		if _, duplicate := seen[key]; duplicate {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

func validMihomoDocument(document *yaml.Node) bool {
	root := yamlDocumentRoot(document)
	if root == nil {
		return false
	}
	proxies := yamlMappingValue(root, "proxies")
	if proxies == nil || proxies.Kind != yaml.SequenceNode {
		return false
	}
	if len(proxies.Content) == 0 {
		return false
	}
	for _, proxy := range proxies.Content {
		if !validMihomoProxy(proxy) {
			return false
		}
	}
	return true
}

// yamlDocumentRoot returns the single mapping at the top of document, or nil.
func yamlDocumentRoot(document *yaml.Node) *yaml.Node {
	if document == nil || len(document.Content) != 1 {
		return nil
	}
	if root := document.Content[0]; root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

// yamlMappingValue returns the first value stored under field, or nil.
func yamlMappingValue(mapping *yaml.Node, field string) *yaml.Node {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == field {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func validMihomoProxy(proxy *yaml.Node) bool {
	if proxy.Kind != yaml.MappingNode {
		return false
	}
	return isNonEmptyYAMLString(yamlMappingValue(proxy, "name")) && isNonEmptyYAMLString(yamlMappingValue(proxy, "type"))
}

func isNonEmptyYAMLString(value *yaml.Node) bool {
	if value == nil || value.Kind != yaml.ScalarNode {
		return false
	}
	return value.Tag == "!!str" && strings.TrimSpace(value.Value) != ""
}
