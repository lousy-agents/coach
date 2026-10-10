package codesignal

import "slices"

// severityOrder is the single source of the severity vocabulary, lowest
// priority first: a severity missing here ranks as "low" and is not accepted
// as a floor.
var severityOrder = []Severity{"low", "advisory", "medium", "high"}

// severityRank maps a Severity to a sort priority (higher sorts first): its
// 1-based position in severityOrder.
// "advisory" ranks above "low" (2 vs 1): it is emitted only by
// user-declared, confidence:high architecture rules (layer_violation,
// layer_bypass), so it must not sort below heuristic low-confidence
// structural findings (issue #259). It intentionally does not outrank
// "medium"/"high" -- no acceptance criterion requires that, and doing so
// would let an advisory finding eclipse a genuinely higher-severity one.
// Any severity value outside this known set -- including future additions
// not yet added to severityOrder -- ranks the same as "low" (1) rather
// than falling to a bottom bucket below every known severity; this keeps
// unknown values deterministic without silently burying them last.
func severityRank(s Severity) int {
	if i := slices.Index(severityOrder, s); i >= 0 {
		return i + 1
	}
	return 1
}

// SeverityFloors lists the severities a floor may name, highest first. The
// slice is a copy the caller may modify.
func SeverityFloors() []Severity {
	floors := slices.Clone(severityOrder)
	slices.Reverse(floors)
	return floors
}

// ParseSeverityFloor accepts exactly the severities the report can emit.
func ParseSeverityFloor(value string) (Severity, bool) {
	if floor := Severity(value); slices.Contains(severityOrder, floor) {
		return floor, true
	}
	return "", false
}
