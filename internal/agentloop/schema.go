package agentloop

import (
	"encoding/json"
	"fmt"
	"strings"
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

func validateAgainstSchema(path string, value any, sch *argsSchemaDoc) error {
	if sch == nil {
		return nil
	}
	if sch.Type != "" && sch.Type != "object" && sch.Type != "array" {
		return nil
	}

	switch sch.Type {
	case "", "object":
		obj, ok := value.(map[string]any)
		if !ok {
			label := path
			if label == "" {
				label = "args"
			}
			return fmt.Errorf("%w: %s must be a JSON object", ErrInvalidArgs, label)
		}
		for _, req := range sch.Required {
			if _, present := obj[req]; !present {
				label := req
				if path != "" {
					label = path + "." + req
				}
				return fmt.Errorf("%w: missing required property %q", ErrInvalidArgs, label)
			}
		}
		for name, prop := range sch.Properties {
			raw, present := obj[name]
			if !present {
				continue
			}
			propPath := name
			if path != "" {
				propPath = path + "." + name
			}
			if err := checkPropType(propPath, raw, prop.types); err != nil {
				return err
			}
			if prop.Items != nil {
				if err := validateArrayItems(propPath, raw, prop.Items); err != nil {
					return err
				}
			}
		}
		return nil
	case "array":
		return validateArrayItems(path, value, sch.Items)
	default:
		return nil
	}
}

func validateArrayItems(path string, value any, itemSchema *argsSchemaDoc) error {
	if itemSchema == nil {
		return nil
	}
	arr, ok := value.([]any)
	if !ok {
		label := path
		if label == "" {
			label = "args"
		}
		return fmt.Errorf("%w: %s must be a JSON array", ErrInvalidArgs, label)
	}
	for i, item := range arr {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if itemSchema.Type == "object" || itemSchema.Type == "" || len(itemSchema.Required) > 0 || len(itemSchema.Properties) > 0 {
			if err := validateAgainstSchema(itemPath, item, itemSchema); err != nil {
				return err
			}
			continue
		}
		if err := checkPropType(itemPath, item, []string{itemSchema.Type}); err != nil {
			return err
		}
	}
	return nil
}

func checkPropType(name string, value any, types []string) error {
	if len(types) == 0 {
		return nil
	}
	actual := jsonTypeOf(value)
	for _, t := range types {
		if strings.EqualFold(t, actual) {
			return nil
		}
		if strings.EqualFold(t, "integer") && actual == "number" {
			if f, ok := value.(float64); ok && f == float64(int64(f)) {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: property %q has type %s, expected %s", ErrInvalidArgs, name, actual, strings.Join(types, "|"))
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func cloneRawMessage(m json.RawMessage) json.RawMessage {
	if m == nil {
		return nil
	}
	out := make(json.RawMessage, len(m))
	copy(out, m)
	return out
}
