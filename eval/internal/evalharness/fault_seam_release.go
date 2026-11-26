//go:build !coach_eval_fault

package evalharness

// faultsFromEnvironment is empty in every build but the acceptance one: the release binary cannot be made to
// reject a program by an environment variable.
func faultsFromEnvironment() (faults, error) { return faults{}, nil }

// gremlinsOverride is empty in every build but the acceptance one: the release build always uses the pinned tool.
func gremlinsOverride() (binary, version string) { return "", "" }
