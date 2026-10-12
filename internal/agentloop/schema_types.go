package agentloop

import (
	"fmt"
	"strings"
)

func checkPropType(name string, value any, types []string) error {
	if len(types) == 0 {
		return nil
	}
	actual := jsonTypeOf(value)
	for _, t := range types {
		if strings.EqualFold(t, actual) {
			return nil
		}
		if strings.EqualFold(t, "integer") && actual == "number" && wholeJSONNumber(value) {
			return nil
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

func wholeJSONNumber(value any) bool {
	f, ok := value.(float64)
	return ok && f == float64(int64(f))
}
