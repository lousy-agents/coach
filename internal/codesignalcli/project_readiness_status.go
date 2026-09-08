package codesignalcli

func statusForGapCode(code string) ReadinessStatus {
	switch code {
	case GapUnsupportedRepositoryShape:
		return StatusOutsideSupport
	case GapNodeMissing, GapNodeBelowMinimum, GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict, GapPackageManagerAmbiguous, GapPackageManagerConfigUnverifiable:
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

func nextActionForGapCode(code string) (string, bool) {
	switch code {
	case GapUnsupportedRepositoryShape:
		return "confirm_repository_shape", true
	case GapNodeMissing, GapNodeBelowMinimum:
		return "install_node", true
	case GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict:
		return "prepare_compiler", true
	case GapPackageManagerAmbiguous, GapPackageManagerConfigUnverifiable:
		return "resolve_package_manager", true
	case GapPolicyMissing, GapPolicyInvalid:
		return "author_policy", true
	default:
		return "", false
	}
}
