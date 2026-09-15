package codesignalcli

import "errors"

// ProjectConfigErrorWithReadiness enriches a ProjectConfigError with the
// full TypeScript project-readiness snapshot computed for the same
// dir/revision/configPath. prepareProjectAnalysis's loadProjectConfig short
// circuit means only the policy failure would otherwise ever reach
// classifyAnalysisError, masking a simultaneous compiler gap (AC-SET-13).
// Unwrap returns the original *ProjectConfigError so
// errors.As(err, &plainTarget) still matches through this wrapper exactly as
// it did before wrapping existed.
type ProjectConfigErrorWithReadiness struct {
	*ProjectConfigError
	Readiness  *ReadinessResult
	ConfigPath string
}

func (e *ProjectConfigErrorWithReadiness) Unwrap() error { return e.ProjectConfigError }

// WrapProjectConfigErrorWithReadiness recomputes readiness for
// dir/revision/configPath and wraps err with it, for AC-SET-13's
// report-all-gaps requirement. It returns err unchanged when err is not a
// *ProjectConfigError, or when readiness itself cannot be computed: a masked
// compiler gap is a strictly smaller problem than losing the original
// diagnostic entirely.
func WrapProjectConfigErrorWithReadiness(err error, dir, revision, configPath string) error {
	var configErr *ProjectConfigError
	if !errors.As(err, &configErr) {
		return err
	}
	readiness, readinessErr := CheckProjectReadiness(dir, revision, configPath)
	if readinessErr != nil {
		return err
	}
	return &ProjectConfigErrorWithReadiness{ProjectConfigError: configErr, Readiness: readiness, ConfigPath: configPath}
}

// AlsoFailingCompilerGapLine names readiness's compiler gap alongside a
// masking policy failure (AC-SET-13's "report all gaps"). It withholds the
// line unless the repository is genuinely TypeScript-shaped
// (checks.project_shape passing): otherwise the compiler check's result is
// not a real, independent finding, only a byproduct of the shape gap the
// policy failure already coexists with (see the
// unsupported_repository_shape case in
// cmd/coach/project_ts_scan_preflight_acceptance_test.go). It never
// suggests --prepare-compiler: AC-SET-13 requires a reviewed, committed
// policy before compiler setup is offered at all, so this line is
// informational rather than an offered command like PrepareCompilerRemediation.
func AlsoFailingCompilerGapLine(checks ReadinessChecks, configPath string) string {
	if checks.ProjectShape.State != ReadinessPass || checks.Compiler.State != ReadinessFail {
		return ""
	}
	return checks.Compiler.Code + ": also failing; run " + typescriptInvocation("--check-project", configPath) + " once the policy above is authored and committed"
}

// PrepareCompilerRemediation names the interactive, consented mise
// TypeScript compiler-setup command that resolves a CompilerUnresolvedError
// gap, for AC-SET-9's appended no-controlling-terminal remediation line. It
// returns "" for a gap code whose next action is not the executable
// prepare-compiler kind (e.g. node_missing, node_unsupported): Coach has no
// setup command that fixes a runtime-boundary gap, so appending one would
// name a command that either does nothing or targets the wrong problem.
func PrepareCompilerRemediation(gapCode, configPath string) string {
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	return typescriptInvocation("--prepare-compiler", configPath)
}

// SuggestProjectConfigRemediation names the interactive, guided TypeScript
// policy-authoring command that resolves a ProjectConfigError gap, for
// AC-SET-9's appended no-controlling-terminal remediation line. It never
// appends a --project-config suffix: validateSuggestProjectConfigFlags
// rejects --suggest-project-config combined with --project-config.
func SuggestProjectConfigRemediation() string {
	return typescriptInvocation("--suggest-project-config", "")
}

// AppendedRemediationLine withholds line whenever hasControllingTerminal is
// true: the interactive setup offer itself (#330 Task 7) owns that case, so
// AC-SET-9's appended command is printed only when no controlling terminal
// is available to run it.
func AppendedRemediationLine(hasControllingTerminal bool, line string) string {
	if hasControllingTerminal {
		return ""
	}
	return line
}

// gapCodeIsExecutablePrepareCompiler reports whether gapCode's next action,
// per the authoritative gapCodeTable, is the executable prepare-compiler
// kind -- the only kind Coach can actually run a command for
// (nextActionExecutable).
func gapCodeIsExecutablePrepareCompiler(gapCode string) bool {
	kind, ok := nextActionForGapCode(gapCode)
	return ok && nextActionExecutable(kind)
}

func typescriptInvocation(flag, projectConfigPath string) string {
	invocation := "coach codesignal --baseline " + flag + " --project-language typescript"
	if projectConfigPath != "" {
		invocation += " --project-config " + projectConfigPath
	}
	return invocation
}
