package evalharness

import "strings"

// breakReason names why a complete run cannot continue the open segment, and the comparable fields
// that differ when the reason is a field mismatch. It returns "" when the run can continue.
func breakReason(open bool, previous, run map[string]any, metric string) (string, []string) {
	if open {
		if fields := differingFields(previous, run); len(fields) > 0 {
			return differReason(fields), fields
		}
	}
	if _, ok := metricValue(run, metric); !ok {
		return "metric absent", nil
	}
	return "", nil
}

func differReason(fields []string) string {
	if len(fields) == 1 {
		return fields[0] + " differs"
	}
	return strings.Join(fields, ", ") + " differ"
}
