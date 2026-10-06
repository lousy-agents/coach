package agentloop

import (
	"encoding/json"
	"fmt"
)

type argsSchemaDoc struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]argsPropSchema `json:"properties"`
	Items      *argsSchemaDoc            `json:"items"`
}

type argsPropSchema struct {
	Type  json.RawMessage `json:"type"`
	Items *argsSchemaDoc  `json:"items"`
	types []string
}

func (p *argsPropSchema) UnmarshalJSON(data []byte) error {
	type alias struct {
		Type  json.RawMessage `json:"type"`
		Items *argsSchemaDoc  `json:"items"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	p.Type = a.Type
	p.Items = a.Items
	types, err := parseJSONTypes(p.Type)
	if err != nil {
		return err
	}
	p.types = types
	return nil
}

func parseJSONTypes(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var multi []string
	if err := json.Unmarshal(raw, &multi); err != nil {
		return nil, err
	}
	return multi, nil
}

func validateToolArgs(schema json.RawMessage, args json.RawMessage) error {
	if len(schema) == 0 {
		if len(args) == 0 {
			return nil
		}
		var v any
		if err := json.Unmarshal(args, &v); err != nil {
			return fmt.Errorf("%w: args are not valid JSON", ErrInvalidArgs)
		}
		return nil
	}

	var sch argsSchemaDoc
	if err := json.Unmarshal(schema, &sch); err != nil {
		return fmt.Errorf("%w: tool schema is not valid JSON", ErrInvalidArgs)
	}

	if len(args) == 0 {
		return fmt.Errorf("%w: args must be a JSON object", ErrInvalidArgs)
	}
	var value any
	if err := json.Unmarshal(args, &value); err != nil {
		return fmt.Errorf("%w: args are not valid JSON", ErrInvalidArgs)
	}
	return validateAgainstSchema("", value, &sch)
}
