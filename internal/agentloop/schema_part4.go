package agentloop

import (
	"encoding/json"
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

func wholeJSONNumber(value any) bool {
	f, ok := value.(float64)
	return ok && f == float64(int64(f))
}
func cloneRawMessage(m json.RawMessage) json.RawMessage {
	if m == nil {
		return nil
	}
	out := make(json.RawMessage, len(m))
	copy(out, m)
	return out
}
