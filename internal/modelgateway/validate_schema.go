package modelgateway

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	types, err := parsePropTypes(p.Type)
	if err != nil {
		return err
	}
	p.types = types
	return nil
}

func parsePropTypes(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var multi []string
	if err := json.Unmarshal(raw, &multi); err != nil {
		return nil, fmt.Errorf("property type: %w", err)
	}
	return multi, nil
}

func parseSchemaDoc(schema json.RawMessage) (schemaDoc, error) {
	var sch schemaDoc
	if err := json.Unmarshal(schema, &sch); err != nil {
		return schemaDoc{}, NewValidationError("output schema is not valid JSON")
	}
	if sch.Type != "" && sch.Type != "object" {
		return schemaDoc{}, NewValidationError("output schema type must be object")
	}
	return sch, nil
}

func parseJudgmentObject(judgment json.RawMessage) (map[string]any, error) {
	var value any
	if err := json.Unmarshal(judgment, &value); err != nil {
		return nil, NewValidationError("judgment is not valid JSON")
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, NewValidationError("judgment must be a JSON object")
	}
	return obj, nil
}

func ensureSupportedProperties(sch schemaDoc) error {
	for name, prop := range sch.Properties {
		if err := ensureSupportedPropSchema(name, prop); err != nil {
			return err
		}
	}
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
