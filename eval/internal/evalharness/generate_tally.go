package evalharness

import (
	"fmt"
	"sort"
	"strings"
)

// rejectionTally counts the attempts of one run and the step that rejected each failed one. The operators that
// would be named beside the steps arrive with #782; until then every program is a base program.
type rejectionTally struct {
	attempts int
	rejected map[string]int
}

func (t *rejectionTally) accept() { t.attempts++ }

func (t *rejectionTally) reject(step string) {
	if t.rejected == nil {
		t.rejected = map[string]int{}
	}
	t.attempts++
	t.rejected[step]++
}

func (t *rejectionTally) rejections() int {
	total := 0
	for _, count := range t.rejected {
		total += count
	}
	return total
}

// exceeded is true only above 50%: one rejection in two attempts passes (design §6, our choice).
func (t *rejectionTally) exceeded() bool {
	return t.rejections()*2 > t.attempts
}

func (t *rejectionTally) message() string {
	steps := make([]string, 0, len(t.rejected))
	for step, count := range t.rejected {
		steps = append(steps, fmt.Sprintf("%s=%d", step, count))
	}
	sort.Strings(steps)
	return fmt.Sprintf("rejection rate %d%% (%d of %d attempts) is above 50%%; rejected at: %s; operators: none, these are base programs",
		(t.rejections()*100+t.attempts/2)/t.attempts, t.rejections(), t.attempts, strings.Join(steps, ", "))
}
