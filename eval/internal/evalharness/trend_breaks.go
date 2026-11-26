package evalharness

import "strings"

// breakReason names why a complete run cannot continue the open segment, or returns "" when it can.
func breakReason(open bool, previous, run map[string]any, metric string) string {
	if open {
		if fields := differingFields(previous, run); len(fields) > 0 {
			return differReason(fields)
		}
	}
	if _, ok := metricValue(run, metric); !ok {
		return "metric absent"
	}
	return ""
}

func differReason(fields []string) string {
	if len(fields) == 1 {
		return fields[0] + " differs"
	}
	return strings.Join(fields, ", ") + " differ"
}
