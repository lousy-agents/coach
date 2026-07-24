package modelgateway

import "strings"

func requireProperties(obj map[string]any, required []string) error {
	for _, key := range required {
		if _, present := obj[key]; !present {
			return NewValidationError("missing required property: " + key)
		}
	}
	return nil
}

func checkPresentProperties(obj map[string]any, properties map[string]propSchema) error {
	for name, prop := range properties {
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

func checkProperty(name string, raw any, prop propSchema) error {
	if len(prop.Enum) > 0 {
		return checkEnumProperty(name, raw, prop.Enum)
	}
	if len(prop.types) == 0 {
		return nil
	}
	return checkTypedProperty(name, raw, prop.types)
}

func checkEnumProperty(name string, raw any, allowed []string) error {
	s, ok := raw.(string)
	if !ok {
		return NewValidationError(name + " must be a string enum value")
	}
	for _, v := range allowed {
		if s == v {
			return nil
		}
	}
	return NewValidationError(name + " value not in enum")
}

func checkTypedProperty(name string, raw any, types []string) error {
	if raw == nil {
		if hasType(types, "null") {
			return nil
		}
		return NewValidationError(name + " must not be null")
	}
	kind := jsonValueKind(raw)
	if kind == "" {
		return NewValidationError(name + " has unsupported JSON type")
	}
	if kind == "string" {
		if hasType(types, "string") {
			return nil
		}
		return NewValidationError(name + " must not be a string")
	}
	return NewValidationError(name + " must not be a " + kind)
}

func jsonValueKind(raw any) string {
	switch raw.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return ""
	}
}

func hasType(types []string, want string) bool {
	for _, t := range types {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}
