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

// faultVariable names a validation step and the seeds it rejects, as "<step>:<seed>,<seed>", optionally in one
// cell only, as "<step>:<seed>,<seed>@<cell index>". It exists so the acceptance specs can drive rejection,
// replacement and exit 8 through the real binary.
const faultVariable = "COACH_EVAL_FAULT"

func faultsFromEnvironment() (faults, error) {
	value := os.Getenv(faultVariable)
	if value == "" {
		return faults{}, nil
	}
	step, rest, _ := strings.Cut(value, ":")
	if !slices.Contains(evalgen.ValidationSteps, step) {
		return faults{}, fmt.Errorf("%s names %q, which is not a validation step (%s)", faultVariable, step, strings.Join(evalgen.ValidationSteps, ", "))
	}
	seedList, cellText, scoped := strings.Cut(rest, "@")
	rejected := map[int64]bool{}
	for _, text := range strings.Split(seedList, ",") {
		seed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return faults{}, fmt.Errorf("%s has a seed %q that is not a number", faultVariable, text)
		}
		rejected[seed] = true
	}
	fault := faults{step: step, seeds: rejected, allCell: !scoped}
	if scoped {
		cell, err := strconv.Atoi(cellText)
		if err != nil {
			return faults{}, fmt.Errorf("%s has a cell %q that is not an index", faultVariable, cellText)
		}
		fault.cell = cell
	}
	return fault, nil
}

// gremlinsVariable names a stand-in for Gremlins, so the acceptance specs can drive the thresholds with chosen
// reports. The release build never reads it.
const gremlinsVariable = "COACH_EVAL_GREMLINS"

const gremlinsVersionVariable = "COACH_EVAL_GREMLINS_VERSION"

func gremlinsOverride() (binary, version string) {
	version = os.Getenv(gremlinsVersionVariable)
	if version == "" {
		version = PinnedGremlinsVersion
	}
	return os.Getenv(gremlinsVariable), version
}
