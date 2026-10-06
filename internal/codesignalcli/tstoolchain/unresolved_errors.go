package tstoolchain

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// CompilerUnresolvedError is the typed scan-time failure the CLI maps to
// exit 2 with one stderr remediation line. Host-runtime clean misses are
// RuntimeUnresolvedError, not this type. Other host-runtime probe failures
// stay operational errors. RootFindings entries are repository-relative
// policy roots, never filesystem paths.
type CompilerUnresolvedError struct {
	Code         string
	ConfigPath   string
	RootFindings []projectreadiness.RootFinding
}

func (e *CompilerUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *CompilerUnresolvedError) RemediationLine() string {
	return gapRemediationLine(e.Code, e.ConfigPath, e.RootFindings)
}

// RuntimeUnresolvedError is a host-runtime miss (node_missing / node_unsupported).
// It is not a CompilerUnresolvedError: the compiler-setup offer must not
// errors.As it, because Coach has no runtime installer.
type RuntimeUnresolvedError struct {
	Code       string
	ConfigPath string
}

func (e *RuntimeUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *RuntimeUnresolvedError) RemediationLine() string {
	return gapRemediationLine(e.Code, e.ConfigPath, nil)
}

func gapRemediationLine(code, configPath string, findings []projectreadiness.RootFinding) string {
	invocation := "coach codesignal --baseline --check-project --project-language typescript"
	if configPath != "" {
		invocation += " --project-config " + configPath
	}
	return code + formatRemediationRootFindings(findings) + ": run " + invocation
}

func formatRemediationRootFindings(findings []projectreadiness.RootFinding) string {
	formatted := projectreadiness.FormatRootFindings(findings)
	if formatted == "" {
		return ""
	}
	return " (" + formatted + ")"
}
