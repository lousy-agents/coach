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

func validateAgainstSchema(path string, value any, sch *argsSchemaDoc) error {
	if sch == nil {
		return nil
	}
	switch sch.Type {
	case "", "object":
		return validateObjectAgainstSchema(path, value, sch)
	case "array":
		return validateArrayItems(path, value, sch.Items)
	default:
		return nil
	}
}

func validateObjectAgainstSchema(path string, value any, sch *argsSchemaDoc) error {
	obj, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: %s must be a JSON object", ErrInvalidArgs, rootLabel(path))
	}
	for _, req := range sch.Required {
		if _, present := obj[req]; !present {
			return fmt.Errorf("%w: missing required property %q", ErrInvalidArgs, qualifiedLabel(path, req))
		}
	}
	for name, prop := range sch.Properties {
		raw, present := obj[name]
		if !present {
			continue
		}
		propPath := qualifiedLabel(path, name)
		if err := checkPropType(propPath, raw, prop.types); err != nil {
			return err
		}
		if err := validateArrayItemsIfPresent(propPath, raw, prop.Items); err != nil {
			return err
		}
	}
	return nil
}

func validateArrayItemsIfPresent(propPath string, raw any, items *argsSchemaDoc) error {
	if items == nil {
		return nil
	}
	return validateArrayItems(propPath, raw, items)
}
