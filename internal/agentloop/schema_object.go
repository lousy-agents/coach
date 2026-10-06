package agentloop

import (
	"fmt"
)

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

func qualifiedLabel(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func rootLabel(path string) string {
	if path == "" {
		return "args"
	}
	return path
}
