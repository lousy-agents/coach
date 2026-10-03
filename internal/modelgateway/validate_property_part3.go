package modelgateway

import (
	"strings"
)

func checkProperty(name string, raw any, prop propSchema) error {
	if hasType(prop.types, "array") {
		return checkArrayProperty(name, raw, prop)
	}
	if len(prop.Enum) > 0 {
		return checkEnumProperty(name, raw, prop.Enum)
	}
	if len(prop.types) == 0 {
		return nil
	}
	return checkTypedProperty(name, raw, prop.types)
}
func validationDetail(err error) string {
	if ve, ok := err.(*ValidationError); ok && ve != nil && ve.Detail != "" {
		return ve.Detail
	}
	if err == nil {
		return ""
	}
	return err.Error()
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
