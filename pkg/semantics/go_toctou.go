package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// goToctouLocationKey dedupes findings by source span alone (StartByte/
// EndByte), mirroring tsLocationKey's role for TS's toctou_check_then_act --
// a given resolved act call can only ever belong to one canonical Finding,
// so there is no separate "owning construct" half to key on. It has its own
// name (rather than reusing tsLocationKey) because it keys dedup state on
// featureCollector, a distinct struct from tsFeatureCollector.
type goToctouLocationKey struct {
	startByte uint
	endByte   uint
}

// checkGoTOCTOUCheckThenAct emits a "toctou_check_then_act" Finding (Story
// 3, CWE-367) if n (an if_statement) has an initializer that binds an
// os.Stat/os.Lstat call's (FileInfo, error) results, a condition that is a
// direct `err == nil` / `nil == err` comparison on that same error
// identifier -- the ONLY valid success gate for v1, per issue #179's gate
// clarification (#190): `err != nil` early-return and sentinel checks like
// errors.Is(err, fs.ErrNotExist) are both out of scope -- and a consequence
// subtree containing a matching os act call (goToctouActCallNames) whose
// first argument has identical source text to the Stat/Lstat call's first
// argument. Because goStatInitializerCall only ever reads n's own
// "initializer" field, a Stat call in a preceding sibling statement (with
// the if only checking a pre-existing variable) is never considered: this
// falls out for free without any special-case logic.
//
// A nested Stat-gated if on the same path (`if _, err := os.Stat(p); err ==
// nil { if _, err := os.Stat(p); err == nil { os.Open(p) } }`) makes both
// the outer and inner if's own call to this method resolve to the identical
// act call node independently, since findGoToctouActCall searches the whole
// consequence subtree, including any nested if. Deduping by the act call's
// Location (goToctouActSeen) ensures that resolves to exactly one Finding,
// not one per enclosing guard.
func (c *featureCollector) checkGoTOCTOUCheckThenAct(n engine.Node, source []byte) {
	statCall, errName := goStatInitializerCall(n, source)
	if statCall == nil {
		return
	}
	checkArg := goCallFirstArgument(statCall)
	if checkArg == nil {
		return
	}

	cond := n.ChildByFieldName("condition")
	if !isGoErrNilGate(cond, errName, source) {
		return
	}

	consequence := n.ChildByFieldName("consequence")
	if consequence == nil {
		return
	}

	act := findGoToctouActCall(consequence, source, checkArg.Utf8Text(source))
	if act == nil {
		return
	}

	loc := locationFromNode(act)
	key := goToctouLocationKey{startByte: loc.StartByte, endByte: loc.EndByte}
	if c.goToctouActSeen == nil {
		c.goToctouActSeen = map[goToctouLocationKey]bool{}
	}
	if c.goToctouActSeen[key] {
		return
	}
	c.goToctouActSeen[key] = true

	c.findings = append(c.findings, newGoTOCTOUCheckThenActFinding(statCall, act, checkArg.Utf8Text(source), source))
}

// newGoTOCTOUCheckThenActFinding builds a "toctou_check_then_act" Finding
// (Story 3, CWE-367) for an os.Stat/os.Lstat checkCall guarding actCall (a
// matching os act call on the identical path text pathText). Location is
// actCall's own span, not checkCall's, so tooling points directly at the
// racy operation; checkGoTOCTOUCheckThenAct only ever calls this once
// actCall has already been resolved non-nil.
func newGoTOCTOUCheckThenActFinding(checkCall, actCall engine.Node, pathText string, source []byte) Finding {
	return Finding{
		Kind:           "toctou_check_then_act",
		Name:           pathText,
		Location:       locationFromNode(actCall),
		Confidence:     "medium",
		Evidence:       checkCall.Utf8Text(source) + " ... " + actCall.Utf8Text(source),
		Recommendation: "Don't gate a filesystem operation behind an os.Stat/os.Lstat check on the same path -- the file can be created, removed, or replaced between the check and the act (CWE-367/TOCTOU). Call the operation directly and handle its fs.ErrNotExist (or equivalent) error instead (EAFP-style); for stronger hardening, prefer errors.Is(err, fs.ErrNotExist) over string/type sniffing, and consider os.Root to scope filesystem access and further shrink this race.",
		SuggestedSkill: "find-bugs",
	}
}
