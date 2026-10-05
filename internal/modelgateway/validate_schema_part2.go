package modelgateway

import (
	"encoding/json"
	"fmt"
)

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
