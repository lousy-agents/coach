package modelgateway

import (
	"encoding/json"
	"fmt"
	"strings"
)

// validateJudgmentJSON ensures judgment is a JSON object. When schema is
// non-empty, it enforces a minimal JSON Schema subset used by seed rubrics:
// object type, required properties, string enums, and string|null types.
// Other JSON Schema types (integer, number, boolean, object, array) are
// rejected so incomplete checks cannot silently accept invalid values.
func validateJudgmentJSON(judgment, schema json.RawMessage) error {
	var value any
	if err := json.Unmarshal(judgment, &value); err != nil {
		return NewValidationError("judgment is not valid JSON")
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return NewValidationError("judgment must be a JSON object")
	}
	if len(schema) == 0 {
		return nil
	}

	var sch schemaDoc
	if err := json.Unmarshal(schema, &sch); err != nil {
		return NewValidationError("output schema is not valid JSON")
	}
	if sch.Type != "" && sch.Type != "object" {
		return NewValidationError("output schema type must be object")
	}
	for _, key := range sch.Required {
		if _, present := obj[key]; !present {
			return NewValidationError("missing required property: " + key)
		}
	}
	for name, prop := range sch.Properties {
		if err := ensureSupportedPropSchema(name, prop); err != nil {
			return err
		}
		raw, present := obj[name]
		if !present {
			continue
		}
		if err := checkProperty(name, raw, prop); err != nil {
			return err
		}
	}
	return nil
}

type schemaDoc struct {
	Type       string                `json:"type"`
	Required   []string              `json:"required"`
	Properties map[string]propSchema `json:"properties"`
}

type propSchema struct {
	Type  json.RawMessage `json:"type"`
	Enum  []string        `json:"enum"`
	types []string
}

func (p *propSchema) UnmarshalJSON(data []byte) error {
	type alias propSchema
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*p = propSchema(a)
	if len(p.Type) == 0 {
		return nil
	}
	var single string
	if err := json.Unmarshal(p.Type, &single); err == nil {
		p.types = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(p.Type, &multi); err != nil {
		return fmt.Errorf("property type: %w", err)
	}
	p.types = multi
	return nil
}

func ensureSupportedPropSchema(name string, prop propSchema) error {
	for _, t := range prop.types {
		if !supportedPropType(t) {
			return NewValidationError(name + " has unsupported schema type: " + t)
		}
	}
	return nil
}

func supportedPropType(t string) bool {
	switch strings.ToLower(t) {
	case "string", "null":
		return true
	default:
		return false
	}
}

func checkProperty(name string, raw any, prop propSchema) error {
	if len(prop.Enum) > 0 {
		s, ok := raw.(string)
		if !ok {
			return NewValidationError(name + " must be a string enum value")
		}
		for _, allowed := range prop.Enum {
			if s == allowed {
				return nil
			}
		}
		return NewValidationError(name + " value not in enum")
	}
	if len(prop.types) == 0 {
		return nil
	}
	if raw == nil {
		if hasType(prop.types, "null") {
			return nil
		}
		return NewValidationError(name + " must not be null")
	}
	switch raw.(type) {
	case string:
		if !hasType(prop.types, "string") {
			return NewValidationError(name + " must not be a string")
		}
	case float64:
		return NewValidationError(name + " must not be a number")
	case bool:
		return NewValidationError(name + " must not be a boolean")
	case map[string]any:
		return NewValidationError(name + " must not be an object")
	case []any:
		return NewValidationError(name + " must not be an array")
	default:
		return NewValidationError(name + " has unsupported JSON type")
	}
	return nil
}

func hasType(types []string, want string) bool {
	for _, t := range types {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}
