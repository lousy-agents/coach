package modelgateway

import (
	"encoding/json"
	"fmt"
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
