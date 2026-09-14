package codesignalcli

func statusForGapCode(code string) ReadinessStatus {
	if info, ok := gapCodeTable[code]; ok {
		return info.status
	}
	return StatusReady
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
	info, ok := gapCodeTable[code]
	if !ok {
		return "", false
	}
	return info.nextActionKind, true
}

func nextActionExecutable(kind string) bool {
	return kind == nextActionKindPrepareCompiler
}

// isPackageManagerGapCode reports whether code is one of the four
// setup-scoped package_manager_* codes: reported as a gap only while
// checks.Compiler has not passed, and independently withholdable per
// installation choice (project adapter or a specific mise origin) without
// disturbing any other verified choice.
func isPackageManagerGapCode(code string) bool {
	return gapCodeTable[code].isPackageManagerGap
}

// gapCodeTable is the single source of truth statusForGapCode,
// nextActionForGapCode, and isPackageManagerGapCode each look up: previously
// three separate switches enumerated overlapping subsets of the same 13 gap
// codes, which TestGapCodeMappings could only catch drifting apart after the
// fact rather than prevent by construction.
type gapCodeInfo struct {
	status              ReadinessStatus
	nextActionKind      string
	isPackageManagerGap bool
}

var gapCodeTable = map[string]gapCodeInfo{
	GapUnsupportedRepositoryShape:        {status: StatusOutsideSupport, nextActionKind: "confirm_repository_shape"},
	GapNodeMissing:                       {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindInstallSupportedRuntime},
	GapNodeUnsupported:                   {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindInstallSupportedRuntime},
	GapNodeUnverifiable:                  {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindRepairRuntimeProbe},
	GapTypescriptCompilerMissing:         {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindPrepareCompiler},
	GapTypescriptVersionMismatch:         {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindPrepareCompiler},
	GapTypescriptVersionConflict:         {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindPrepareCompiler},
	GapPackageManagerAmbiguous:           {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindResolvePackageManager, isPackageManagerGap: true},
	GapPackageManagerConfigUnverifiable:  {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindResolvePackageManager, isPackageManagerGap: true},
	GapPackageManagerVersionUnverifiable: {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindResolvePackageManager, isPackageManagerGap: true},
	GapPackageManagerVersionUnsupported:  {status: StatusNeedsPrerequisite, nextActionKind: nextActionKindResolvePackageManager, isPackageManagerGap: true},
	GapPolicyMissing:                     {status: StatusNeedsPolicy, nextActionKind: "author_policy"},
	GapPolicyInvalid:                     {status: StatusNeedsPolicy, nextActionKind: "author_policy"},
}
