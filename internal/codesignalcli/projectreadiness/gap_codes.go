package projectreadiness

const (
	GapUnsupportedRepositoryShape        = "unsupported_repository_shape"
	GapNodeMissing                       = "node_missing"
	GapNodeUnsupported                   = "node_unsupported"
	GapNodeUnverifiable                  = "node_unverifiable"
	GapTypescriptCompilerMissing         = "typescript_compiler_missing"
	GapTypescriptVersionMismatch         = "typescript_version_mismatch"
	GapTypescriptVersionConflict         = "typescript_version_conflict"
	GapPackageManagerAmbiguous           = "package_manager_ambiguous"
	GapPackageManagerConfigUnverifiable  = "package_manager_config_unverifiable"
	GapPackageManagerVersionUnverifiable = "package_manager_version_unverifiable"
	GapPackageManagerVersionUnsupported  = "package_manager_version_unsupported"
	GapPolicyMissing                     = "policy_missing"
	GapPolicyInvalid                     = "policy_invalid"
)

func StatusForGapCode(code string) Status {
	if info, ok := gapCodeTable[code]; ok {
		return info.status
	}
	return StatusReady
}

func StatusRank(status Status) int {
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

// NextActionInstallSupportedRuntime and NextActionRepairRuntimeProbe
// are permanently instruction-only (see NextActionExecutable): Coach never
// installs or repairs a host Node runtime itself.
const (
	NextActionInstallSupportedRuntime = "install_supported_runtime"
	NextActionRepairRuntimeProbe      = "repair_runtime_probe"
	NextActionPrepareCompiler         = "prepare_compiler"
	NextActionResolvePackageManager   = "resolve_package_manager"
)

func NextActionForGapCode(code string) (string, bool) {
	info, ok := gapCodeTable[code]
	if !ok {
		return "", false
	}
	return info.nextActionKind, true
}

func NextActionExecutable(kind string) bool {
	return kind == NextActionPrepareCompiler
}

// IsPackageManagerGapCode reports whether code is one of the four
// setup-scoped package_manager_* codes: reported as a gap only while
// checks.Compiler has not passed, and independently withholdable per
// installation choice (project adapter or a specific mise origin) without
// disturbing any other verified choice.
func IsPackageManagerGapCode(code string) bool {
	return gapCodeTable[code].isPackageManagerGap
}

// gapCodeTable is the single source of truth StatusForGapCode,
// NextActionForGapCode, and IsPackageManagerGapCode each look up: previously
// three separate switches enumerated overlapping subsets of the same 13 gap
// codes, which TestGapCodeMappings could only catch drifting apart after the
// fact rather than prevent by construction.
type gapCodeInfo struct {
	status              Status
	nextActionKind      string
	isPackageManagerGap bool
}

var gapCodeTable = map[string]gapCodeInfo{
	GapUnsupportedRepositoryShape:        {status: StatusOutsideSupport, nextActionKind: "confirm_repository_shape"},
	GapNodeMissing:                       {status: StatusNeedsPrerequisite, nextActionKind: NextActionInstallSupportedRuntime},
	GapNodeUnsupported:                   {status: StatusNeedsPrerequisite, nextActionKind: NextActionInstallSupportedRuntime},
	GapNodeUnverifiable:                  {status: StatusNeedsPrerequisite, nextActionKind: NextActionRepairRuntimeProbe},
	GapTypescriptCompilerMissing:         {status: StatusNeedsPrerequisite, nextActionKind: NextActionPrepareCompiler},
	GapTypescriptVersionMismatch:         {status: StatusNeedsPrerequisite, nextActionKind: NextActionPrepareCompiler},
	GapTypescriptVersionConflict:         {status: StatusNeedsPrerequisite, nextActionKind: NextActionPrepareCompiler},
	GapPackageManagerAmbiguous:           {status: StatusNeedsPrerequisite, nextActionKind: NextActionResolvePackageManager, isPackageManagerGap: true},
	GapPackageManagerConfigUnverifiable:  {status: StatusNeedsPrerequisite, nextActionKind: NextActionResolvePackageManager, isPackageManagerGap: true},
	GapPackageManagerVersionUnverifiable: {status: StatusNeedsPrerequisite, nextActionKind: NextActionResolvePackageManager, isPackageManagerGap: true},
	GapPackageManagerVersionUnsupported:  {status: StatusNeedsPrerequisite, nextActionKind: NextActionResolvePackageManager, isPackageManagerGap: true},
	GapPolicyMissing:                     {status: StatusNeedsPolicy, nextActionKind: "author_policy"},
	GapPolicyInvalid:                     {status: StatusNeedsPolicy, nextActionKind: "author_policy"},
}

// KnownGapCodes lists every gap code readiness can report, in no particular
// order.
func KnownGapCodes() []string {
	codes := make([]string, 0, len(gapCodeTable))
	for code := range gapCodeTable {
		codes = append(codes, code)
	}
	return codes
}
