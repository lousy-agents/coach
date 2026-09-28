package modelgateway

import (
	"encoding/json"

	"strings"
)

type schemaDoc struct {
	Type       string                `json:"type"`
	Required   []string              `json:"required"`
	Properties map[string]propSchema `json:"properties"`
}

// propSchema is the supported JSON Schema subset for judgment OutputSchema
// properties: string|null leaves (optionally enum), or a single array of
// objects whose leaves are the same string|null subset (batch envelope).
type propSchema struct {
	Type  json.RawMessage `json:"type"`
	Enum  []string        `json:"enum"`
	Items *schemaDoc      `json:"items"`
	types []string
}

func ensureSupportedPropSchema(name string, prop propSchema) error {
	for _, t := range prop.types {
		switch strings.ToLower(t) {
		case "string", "null":
			// leaf types used by singular seed schemas
		case "array":
			if err := ensureSupportedArrayProp(name, prop); err != nil {
				return err
			}
		default:
			return NewValidationError(name + " has unsupported schema type: " + t)
		}
	}
	return nil
}

// ensureSupportedArrayProp allows only array-of-object envelopes whose element
// properties are the same string|null leaf subset as singular judgments.
// Nested arrays, array-of-string, and non-object items fail closed.
func ensureSupportedArrayProp(name string, prop propSchema) error {
	if prop.Items == nil {
		return NewValidationError(name + " array schema requires object items")
	}
	if prop.Items.Type != "" && !strings.EqualFold(prop.Items.Type, "object") {
		return NewValidationError(name + " array items must be object")
	}
	for elemName, elemProp := range prop.Items.Properties {
		if t, bad := unsupportedLeafType(elemProp.types); bad {
			return NewValidationError(name + " items." + elemName + " has unsupported schema type: " + t)
		}
		if elemProp.Items != nil {
			return NewValidationError(name + " items." + elemName + " has unsupported nested array")
		}
	}
	return nil
}

func unsupportedLeafType(types []string) (string, bool) {
	for _, t := range types {
		if !supportedLeafPropType(t) {
			return t, true
		}
	}
	return "", false
}

func supportedLeafPropType(t string) bool {
	switch strings.ToLower(t) {
	case "string", "null":
		return true
	default:
		return false
	}
}
