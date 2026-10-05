package modelgateway

import (
	"strings"
)

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

func hasType(types []string, want string) bool {
	for _, t := range types {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
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
