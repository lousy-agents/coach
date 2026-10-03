package codesignalcli

// readinessHasBlockingRuntimeGap reports whether readiness.Checks.Runtime
// independently fails with a gap code that is not the executable
// prepare-compiler kind, through the same gapCodeIsExecutablePrepareCompiler
// predicate the real scan's own compiler-setup gate uses
// (project_ts_preflight.go). readinessFromGapChecks (project_readiness_
// aggregate.go) derives Runtime's and Compiler's next actions independently
// of one another, so a failing runtime check can coexist in NextActions with
// a genuinely executable prepare_compiler entry; without this check,
// RunPrepareCompilerMiseSetup would offer to install a compiler while the
// host Node runtime that would run it is still missing or unsupported --
// exactly the gap a real scan's own PrepareTSRuntime never reaches, since it
// resolves host Node first and fails fast there.
func readinessHasBlockingRuntimeGap(readiness *ReadinessResult) (string, bool) {
	if readiness == nil {
		return "", false
	}
	code := readiness.Checks.Runtime.Code
	if readiness.Checks.Runtime.State != ReadinessFail || code == "" {
		return "", false
	}
	if gapCodeIsExecutablePrepareCompiler(code) {
		return "", false
	}
	return code, true
}

// filterMiseChoiceKinds keeps only the mise origins this flow handles.
// action.Choices may also name a project-package-manager choice from a
// sibling task; that choice is a different installation-choice kind
// entirely, not this flow's concern.
func filterMiseChoiceKinds(choices []string) []string {
	var mise []string
	for _, choice := range choices {
		if choice == compilerOriginMiseProject || choice == compilerOriginMiseGlobal {
			mise = append(mise, choice)
		}
	}
	return mise
}

// miseSetupChoicesForReadiness reuses checkPolicy exactly the way
// CheckProjectReadiness itself derives roots -- never a second, independent
// notion of "roots".
func miseSetupChoicesForReadiness(dir, revision, configPath string) []ReadinessMiseChoice {
	policyPath := configPath
	if policyPath == "" {
		policyPath = defaultProjectConfigPath
	}
	_, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return nil
	}
	return evaluateMiseSetupChoices(dir, roots)
}
