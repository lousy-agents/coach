package antipatternfixtures

import (
	"fmt"
	"strconv"
)

// observation is the statistic a template is scaled to: the most frequent
// value at or over Coach's threshold, ties to the larger, and the value it
// was capped from when it ran over the cap.
type observation struct {
	measure          string
	value, from      int
	threshold, limit int
	// basis replaces the default account of how the value was chosen.
	basis string
}

// typical is the observation of a distribution, or false when no value
// reaches the threshold.
func typical(distribution map[string]int, measure string, threshold, limit int) (observation, bool) {
	best, bestCount := -1, 0
	for bucket, count := range distribution {
		value, err := strconv.Atoi(bucket)
		if err != nil || value < threshold || count == 0 {
			continue
		}
		if count > bestCount || count == bestCount && value > best {
			best, bestCount = value, count
		}
	}
	if best < 0 {
		return observation{}, false
	}
	return observation{measure: measure, value: min(best, limit), from: best, threshold: threshold, limit: limit}, true
}

// note says where the fixture's number came from.
func (o observation) note(identity string) string {
	capped := ""
	if o.from != o.value {
		capped = fmt.Sprintf("; capped at %d from %d, so the fixture stays readable", o.value, o.from)
	}
	basis := o.basis
	if basis == "" {
		basis = fmt.Sprintf("the most frequent value at or over Coach's threshold of %d in the profile", o.threshold)
	}
	return fmt.Sprintf("Derived from pattern profile %s: %s %d, %s%s.", identity, o.measure, o.value, basis, capped)
}
