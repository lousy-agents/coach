package evalgen

import (
	"fmt"
	"os/exec"
	"time"
)

// MinimumNodeMajor is the oldest Node that runs a .ts file by stripping its types, without a flag.
const MinimumNodeMajor = 24

// PinnedTypeScript is the compiler version a TypeScript program is validated with: the native port that
// js/semantics pins.
const PinnedTypeScript = "7.0.2"

// DefaultValidationLimit bounds each tsc or node run that validates a generated program.
const DefaultValidationLimit = 60 * time.Second

// TypeScriptTools are the programs that validate a TypeScript program, and the time each run of them may take.
type TypeScriptTools struct {
	Node, TSC string
	// Limit is zero for DefaultValidationLimit.
	Limit time.Duration
}

// ResolveTypeScriptTools finds node on PATH and the pinned compiler: EVAL_TSC if it is set, otherwise the one
// installed in js/semantics of the checkout that holds workingDir. Global compilers are never used, because they
// differ between machines.
func ResolveTypeScriptTools(workingDir string) (TypeScriptTools, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return TypeScriptTools{}, fmt.Errorf("node is not on PATH: TypeScript programs are validated with node --test")
	}
	if err := requireNodeMajor(node); err != nil {
		return TypeScriptTools{}, err
	}
	tsc, err := locateTSC(workingDir)
	if err != nil {
		return TypeScriptTools{}, err
	}
	if err := requirePinnedVersion(tsc); err != nil {
		return TypeScriptTools{}, err
	}
	return TypeScriptTools{Node: node, TSC: tsc}, nil
}
