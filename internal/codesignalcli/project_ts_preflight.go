package codesignalcli

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
