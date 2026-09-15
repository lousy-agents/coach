package codesignalcli

// PreflightOutcome is what PreflightScanPreparation determined about a
// normal (non --check-project, non --prepare-compiler) TypeScript-language
// scan before analysis begins.
type PreflightOutcome struct {
	// Blocked is true when readiness at revision shows guided policy
	// authoring or compiler setup would be required to proceed, but
	// hasControllingTerminal was false: the caller must perform no setup
	// mutation, leave stdout untouched, and report RemediationLines on
	// stderr before returning exit 2 (AC-SET-9).
	Blocked bool
	// Readiness is the readiness result this determination was based on.
	Readiness *ReadinessResult
	// RemediationLines names the supported commands that resolve the
	// detected gap(s) from a controlling terminal, one per still-needed
	// interactive flow.
	RemediationLines []string
}

// PreflightScanPreparation computes readiness at revision and reports
// whether a normal TypeScript-language scan can proceed. Readiness
// computation is read-only, so this performs no setup mutation of its own
// regardless of the outcome.
func PreflightScanPreparation(dir, revision, projectConfigPath string, hasControllingTerminal bool) (PreflightOutcome, error) {
	readiness, err := CheckProjectReadiness(dir, revision, projectConfigPath)
	if err != nil {
		return PreflightOutcome{}, err
	}

	lines := interactiveSetupRemediationLines(readiness, projectConfigPath)
	if len(lines) == 0 || hasControllingTerminal {
		return PreflightOutcome{Readiness: readiness}, nil
	}

	return PreflightOutcome{Blocked: true, Readiness: readiness, RemediationLines: lines}, nil
}

// interactiveSetupRemediationLines names the supported commands for an
// interactive, Coach-run remediation flow, gated on readiness's *overall*
// Status rather than on individual gaps in isolation: a StatusOutsideSupport
// repository shape outranks (statusRank) any simultaneous policy or
// compiler gap, and a repository Coach cannot analyze at all must never be
// reported as needing guided setup. Only the two statuses whose sole
// possible gaps are ones Coach can remediate interactively -- StatusNeedsPolicy
// (GapPolicyMissing/GapPolicyInvalid; guided policy authoring) and
// StatusNeedsPrerequisite carrying a prepare_compiler next action (mise
// compiler setup, the only currently executable next-action kind) --
// contribute a line. Host Node runtime and package-manager resolution gaps
// (also StatusNeedsPrerequisite) have no interactive flow of their own yet.
func interactiveSetupRemediationLines(readiness *ReadinessResult, projectConfigPath string) []string {
	switch readiness.Status {
	case StatusNeedsPolicy:
		return []string{suggestProjectConfigTypescriptInvocation()}
	case StatusNeedsPrerequisite:
		for _, action := range readiness.NextActions {
			if nextActionExecutable(action.Kind) {
				return []string{prepareCompilerTypescriptInvocation(projectConfigPath)}
			}
		}
	}
	return nil
}

// suggestProjectConfigTypescriptInvocation never appends a --project-config
// suffix: validateSuggestProjectConfigFlags rejects --suggest-project-config
// combined with --project-config, so naming one here would suggest a
// command Coach itself refuses.
func suggestProjectConfigTypescriptInvocation() string {
	return typescriptInvocation("--suggest-project-config", "")
}

func prepareCompilerTypescriptInvocation(projectConfigPath string) string {
	return typescriptInvocation("--prepare-compiler", projectConfigPath)
}

func typescriptInvocation(flag, projectConfigPath string) string {
	invocation := "coach codesignal --baseline " + flag + " --project-language typescript"
	if projectConfigPath != "" {
		invocation += " --project-config " + projectConfigPath
	}
	return invocation
}
