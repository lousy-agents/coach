//go:build !coach_eval_fault

package evalharness

import (
	"time"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// faultsFromEnvironment is empty in every build but the acceptance one: the release binary cannot be made to
// reject a program by an environment variable.
func faultsFromEnvironment() (faults, error) { return faults{}, nil }

// gremlinsOverride is empty in every build but the acceptance one: the release build always uses the pinned tool.
func gremlinsOverride() (binary, version string) { return "", "" }

// detectorVersion is the pinned detector's version in every build but the acceptance one.
func detectorVersion() string { return evaldetect.PinnedGocognitVersion }

// validationSteps and validationVersion are the step list and the version a cached program is keyed by, in every
// build but the acceptance one.
func validationSteps() []string { return evalgen.ValidationSteps }

func validationVersion() string { return ValidationVersion }

// gremlinsLimit is how long a Gremlins run may take, in every build but the acceptance one.
func gremlinsLimit() time.Duration { return DefaultGremlinsLimit }
