package agentloop

import (
	"encoding/json"
	"fmt"
)

func validateArrayItems(path string, value any, itemSchema *argsSchemaDoc) error {
	if itemSchema == nil {
		return nil
	}
	arr, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: %s must be a JSON array", ErrInvalidArgs, rootLabel(path))
	}
	for i, item := range arr {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if err := validateSchemaItem(itemPath, item, itemSchema); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaItem(itemPath string, item any, itemSchema *argsSchemaDoc) error {
	if itemSchema.Type == "object" || itemSchema.Type == "" || len(itemSchema.Required) > 0 || len(itemSchema.Properties) > 0 {
		return validateAgainstSchema(itemPath, item, itemSchema)
	}
	return checkPropType(itemPath, item, []string{itemSchema.Type})
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
func rootLabel(path string) string {
	if path == "" {
		return "args"
	}
	return path
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
