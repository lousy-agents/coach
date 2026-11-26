//go:build !coach_eval_fault

package evalharness

import "github.com/lousy-agents/coach/eval/internal/evaldetect"

// faultsFromEnvironment is empty in every build but the acceptance one: the release binary cannot be made to
// reject a program by an environment variable.
func faultsFromEnvironment() (faults, error) { return faults{}, nil }

// detectorVersion is the pinned detector's version in every build but the acceptance one.
func detectorVersion() string { return evaldetect.PinnedGocognitVersion }
