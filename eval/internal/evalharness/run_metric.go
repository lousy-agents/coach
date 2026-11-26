package evalharness

import (
	"encoding/json"
	"strings"
)

// metricValue reads the number at a dot-separated path of a run record.
func metricValue(run map[string]any, path string) (float64, bool) {
	var current any = run
	for _, key := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		if current, ok = object[key]; !ok {
			return 0, false
		}
	}
	switch number := current.(type) {
	case float64:
		return number, true
	case json.Number:
		value, err := number.Float64()
		return value, err == nil
	default:
		return 0, false
	}
}
