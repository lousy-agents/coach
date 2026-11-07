package tssetup

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// ProjectConfigErrorWithReadiness enriches a projectconfig.ConfigError with the
// full TypeScript project-readiness snapshot computed for the same
// dir/revision/policy. prepareProjectAnalysis's policy-load short
// circuit means only the policy failure would otherwise ever reach
// classifyAnalysisError, masking a simultaneous compiler gap (AC-SET-13).
// Unwrap returns the original *projectconfig.ConfigError so
// errors.As(err, &plainTarget) still matches through this wrapper exactly as
// it did before wrapping existed.
type ProjectConfigErrorWithReadiness struct {
	*projectconfig.ConfigError
	Readiness *projectreadiness.Result
	// Policy is the policy the readiness snapshot was computed for.
	Policy policyref.Ref
}

func (e *ProjectConfigErrorWithReadiness) Unwrap() error { return e.ConfigError }

// AlsoFailingLines are the lines for the gaps readiness reports beside this policy failure. A policy refused
// for the case of its name is refused again by any rerun that names it as typed, so its other gaps are listed
// without a command that repeats the spelling just refused.
func (e *ProjectConfigErrorWithReadiness) AlsoFailingLines() []string {
	return alsoFailingGapLines(e.Readiness, e.Policy, e.PolicyCause != projectreadiness.CauseNameCaseDiffers)
}

// CompilerUnresolvedErrorWithReadiness enriches a tstoolchain.CompilerUnresolvedError
// with the full TypeScript project-readiness snapshot computed for the same
// dir/revision/policy, so a real scan's controlling-terminal branch can
// drive the interactive compiler-setup offer from the same facts the gap
// itself was raised from, without a second, possibly racing readiness
// computation. Unwrap returns the original
// *tstoolchain.CompilerUnresolvedError so errors.As(err, &plainTarget) still matches
// through this wrapper exactly as it did before wrapping existed -- in
// particular, classifyAnalysisError's own no-controlling-terminal fallback
// needs no change at all.
// Policy is deliberately absent: *tstoolchain.CompilerUnresolvedError already
// carries the policy RemediationLine() renders, and a second copy here
// could drift from it, pointing the printed remediation and the setup
// session it precedes at two different policies.
type CompilerUnresolvedErrorWithReadiness struct {
	*tstoolchain.CompilerUnresolvedError
	Readiness *projectreadiness.Result
	Revision  string
}

func (e *CompilerUnresolvedErrorWithReadiness) Unwrap() error { return e.CompilerUnresolvedError }

// WrapCompilerUnresolvedErrorWithReadiness recomputes readiness for
// dir/revision/policy and wraps err with it. It returns err unchanged
// when err is not a *tstoolchain.CompilerUnresolvedError, or when readiness itself
// cannot be computed: an unofferable setup session is a strictly smaller
// problem than losing the original diagnostic entirely. tstoolchain.CompilerUnresolvedError
// is only ever constructed deep inside the TypeScript project backend, so
// recomputing readiness here -- rather than threading a precomputed snapshot
// down through that backend -- is what lets this wrapping live entirely in
// this file.
func WrapCompilerUnresolvedErrorWithReadiness(err error, dir, revision string, policy policyref.Ref) error {
	var unresolved *tstoolchain.CompilerUnresolvedError
	if !errors.As(err, &unresolved) {
		return err
	}
	readiness, readinessErr := projectcheck.Run(dir, revision, policy)
	if readinessErr != nil {
		return err
	}
	return &CompilerUnresolvedErrorWithReadiness{CompilerUnresolvedError: unresolved, Readiness: readiness, Revision: revision}
}

// WrapProjectConfigErrorWithReadiness computes readiness for
// dir/revision/policy, taking the working tree's verdict from err instead of
// reading the working tree again, and wraps err with it, for AC-SET-13's
// report-all-gaps requirement. It returns err unchanged when err is not a
// *projectconfig.ConfigError, or when readiness itself cannot be computed: a masked
// compiler gap is a strictly smaller problem than losing the original
// diagnostic entirely.
func WrapProjectConfigErrorWithReadiness(err error, dir, revision string, policy policyref.Ref) error {
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) {
		return err
	}
	readiness, readinessErr := projectcheck.RunAfterPolicyError(dir, revision, policy, configErr)
	if readinessErr != nil {
		return err
	}
	return &ProjectConfigErrorWithReadiness{ConfigError: configErr, Readiness: readiness, Policy: policy}
}
