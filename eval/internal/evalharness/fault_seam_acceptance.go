//go:build coach_eval_fault

package evalharness

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// faultVariable names a validation step and the seeds it rejects, as "<step>:<seed>,<seed>". It exists so the
// acceptance specs can drive rejection, replacement and exit 8 through the real binary.
const faultVariable = "COACH_EVAL_FAULT"

func faultsFromEnvironment() (faults, error) {
	value := os.Getenv(faultVariable)
	if value == "" {
		return nil, nil
	}
	step, seedList, _ := strings.Cut(value, ":")
	if !slices.Contains(evalgen.ValidationSteps, step) {
		return nil, fmt.Errorf("%s names %q, which is not a validation step (%s)", faultVariable, step, strings.Join(evalgen.ValidationSteps, ", "))
	}
	rejected := map[int64]bool{}
	for _, text := range strings.Split(seedList, ",") {
		seed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s has a seed %q that is not a number", faultVariable, text)
		}
		rejected[seed] = true
	}
	return faults{step: rejected}, nil
}
