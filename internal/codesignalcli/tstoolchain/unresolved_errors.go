package tstoolchain

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/invocation"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// CompilerUnresolvedError is the typed scan-time failure the CLI maps to
// exit 2 with one stderr remediation line. Host-runtime clean misses are
// RuntimeUnresolvedError, not this type. Other host-runtime probe failures
// stay operational errors. RootFindings entries are repository-relative
// policy roots, never filesystem paths.
type CompilerUnresolvedError struct {
	Code string
	// Policy is the policy the failed run read. The remediation line names it
	// with --project-config only when the user named it.
	Policy       policyref.Ref
	RootFindings []projectreadiness.RootFinding
}

func (e *CompilerUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *CompilerUnresolvedError) RemediationLine() string {
	return gapRemediationLine(e.Code, e.Policy, e.RootFindings)
}

// RuntimeUnresolvedError is a host-runtime miss (node_missing / node_unsupported).
// It is not a CompilerUnresolvedError: the compiler-setup offer must not
// errors.As it, because Coach has no runtime installer.
type RuntimeUnresolvedError struct {
	Code string
	// Policy is the policy the failed run read. The remediation line names it
	// with --project-config only when the user named it.
	Policy policyref.Ref
}

func (e *RuntimeUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *RuntimeUnresolvedError) RemediationLine() string {
	return gapRemediationLine(e.Code, e.Policy, nil)
}

func gapRemediationLine(code string, policy policyref.Ref, findings []projectreadiness.RootFinding) string {
	return code + formatRemediationRootFindings(findings) + ": " + invocation.Command{Mode: invocation.CheckProject, Policy: policy}.Phrase("run")
}

func formatRemediationRootFindings(findings []projectreadiness.RootFinding) string {
	formatted := projectreadiness.FormatRootFindings(findings)
	if formatted == "" {
		return ""
	}
	return " (" + formatted + ")"
}
