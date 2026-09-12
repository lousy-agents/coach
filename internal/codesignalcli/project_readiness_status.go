package codesignalcli

func statusForGapCode(code string) ReadinessStatus {
	switch code {
	case GapUnsupportedRepositoryShape:
		return StatusOutsideSupport
	case GapNodeMissing, GapNodeUnsupported, GapNodeUnverifiable, GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict, GapPackageManagerAmbiguous, GapPackageManagerVersionUnverifiable, GapPackageManagerVersionUnsupported, GapPackageManagerConfigUnverifiable:
		return StatusNeedsPrerequisite
	case GapPolicyMissing, GapPolicyInvalid:
		return StatusNeedsPolicy
	default:
		return StatusReady
	}
}

func statusRank(status ReadinessStatus) int {
	switch status {
	case StatusOutsideSupport:
		return 4
	case StatusNeedsPrerequisite:
		return 3
	case StatusNeedsPolicy:
		return 2
	case StatusReadyWithLimits:
		return 1
	default:
		return 0
	}
}

// nextActionKindInstallSupportedRuntime and nextActionKindRepairRuntimeProbe
// are permanently instruction-only (see nextActionExecutable): Coach never
// installs or repairs a host Node runtime itself.
const (
	nextActionKindInstallSupportedRuntime = "install_supported_runtime"
	nextActionKindRepairRuntimeProbe      = "repair_runtime_probe"
	nextActionKindPrepareCompiler         = "prepare_compiler"
	nextActionKindResolvePackageManager   = "resolve_package_manager"
)

func nextActionForGapCode(code string) (string, bool) {
	switch code {
	case GapUnsupportedRepositoryShape:
		return "confirm_repository_shape", true
	case GapNodeMissing, GapNodeUnsupported:
		return nextActionKindInstallSupportedRuntime, true
	case GapNodeUnverifiable:
		return nextActionKindRepairRuntimeProbe, true
	case GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict:
		return nextActionKindPrepareCompiler, true
	case GapPackageManagerAmbiguous, GapPackageManagerVersionUnverifiable, GapPackageManagerVersionUnsupported, GapPackageManagerConfigUnverifiable:
		return nextActionKindResolvePackageManager, true
	case GapPolicyMissing, GapPolicyInvalid:
		return "author_policy", true
	default:
		return "", false
	}
}

func nextActionExecutable(kind string) bool {
	return kind == nextActionKindPrepareCompiler
}

// isPackageManagerGapCode reports whether code is one of the four
// setup-scoped package_manager_* codes (SA-280-045): reported as a gap only
// while checks.Compiler has not passed, and independently withholdable per
// installation choice (project adapter or a specific mise origin) without
// disturbing any other verified choice.
func isPackageManagerGapCode(code string) bool {
	switch code {
	case GapPackageManagerAmbiguous, GapPackageManagerVersionUnverifiable, GapPackageManagerVersionUnsupported, GapPackageManagerConfigUnverifiable:
		return true
	default:
		return false
	}
}
