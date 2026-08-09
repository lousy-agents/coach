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

// goToctouStatCallNames is the CWE-367 "check" call name set (Story 3):
// os.Stat and os.Lstat both return (FileInfo, error) and both observe a
// path's current state that can change before an "act" call on the same
// path runs.
var goToctouStatCallNames = map[string]bool{
	"Stat":  true,
	"Lstat": true,
}

// goToctouActCallNames is the CWE-367 "act" call name set (Story 3): os
// calls whose target path can change between a preceding os.Stat/os.Lstat
// gate observing it and this call acting on it.
var goToctouActCallNames = map[string]bool{
	"Open":      true,
	"OpenFile":  true,
	"Remove":    true,
	"RemoveAll": true,
	"ReadFile":  true,
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

// goStatInitializerCall extracts an os.Stat/os.Lstat call from ifStmt's own
// "initializer" field and reports the identifier bound to its second
// (error) result value -- Stat/Lstat's signature is (FileInfo, error), so
// the second bound identifier is always the error result regardless of its
// name. Go's grammar uses "short_var_declaration" for `:=` and
// "assignment_statement" for `=`; both share the same "left"/"right" field
// shapes, so both are handled identically here. It reports call == nil for
// any other initializer shape, a left side that doesn't bind exactly two
// values, or a right side that isn't exactly one os.Stat/os.Lstat
// call_expression.
func goStatInitializerCall(ifStmt engine.Node, source []byte) (call engine.Node, errName string) {
	init := ifStmt.ChildByFieldName("initializer")
	if init == nil {
		return nil, ""
	}
	switch init.Kind() {
	case "short_var_declaration", "assignment_statement":
	default:
		return nil, ""
	}

	left := init.ChildByFieldName("left")
	right := init.ChildByFieldName("right")
	if left == nil || right == nil {
		return nil, ""
	}

	targets := goExpressionListValues(left)
	if len(targets) != 2 || targets[1].Kind() != "identifier" {
		return nil, ""
	}

	values := goExpressionListValues(right)
	if len(values) != 1 {
		return nil, ""
	}
	rhs := values[0]
	pkg, name, ok := goSelectorCallInfo(rhs, source)
	if !ok || pkg != "os" || !goToctouStatCallNames[name] {
		return nil, ""
	}

	return rhs, targets[1].Utf8Text(source)
}

// goExpressionListValues returns list's non-punctuation children in source
// order (filtering out "," and any other literal tokens), i.e. the actual
// expression nodes an expression_list holds.
func goExpressionListValues(list engine.Node) []engine.Node {
	var out []engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		if child.Kind() == "," {
			continue
		}
		out = append(out, child)
	}
	return out
}

// isGoErrNilGate reports whether cond is a direct nil-comparison
// binary_expression on the identifier named errName -- `err == nil` or `nil
// == err`, either operand order -- the only valid success gate for v1. Any
// other condition shape (`err != nil`, a sentinel check like
// errors.Is(err, fs.ErrNotExist) or os.IsNotExist(err), a boolean
// combination, etc.) reports false.
func isGoErrNilGate(cond engine.Node, errName string, source []byte) bool {
	if cond == nil || errName == "" || cond.Kind() != "binary_expression" {
		return false
	}
	if goBinaryOp(cond, source) != "==" {
		return false
	}
	left := cond.ChildByFieldName("left")
	right := cond.ChildByFieldName("right")
	if left == nil || right == nil {
		return false
	}
	if left.Kind() == "nil" && right.Kind() == "identifier" && right.Utf8Text(source) == errName {
		return true
	}
	if right.Kind() == "nil" && left.Kind() == "identifier" && left.Utf8Text(source) == errName {
		return true
	}
	return false
}

// findGoToctouActCall searches n's subtree (including inside any nested
// statements/if -- this detector does no scope resolution, only syntactic
// call/argument-text matching) for a call_expression named in
// goToctouActCallNames on the "os" package whose first argument's source
// text equals pathText, returning the first one found or nil.
func findGoToctouActCall(n engine.Node, source []byte, pathText string) engine.Node {
	if n == nil {
		return nil
	}
	if n.Kind() == "call_expression" {
		if pkg, name, ok := goSelectorCallInfo(n, source); ok && pkg == "os" && goToctouActCallNames[name] {
			if arg := goCallFirstArgument(n); arg != nil && arg.Utf8Text(source) == pathText {
				return n
			}
		}
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if found := findGoToctouActCall(n.Child(i), source, pathText); found != nil {
			return found
		}
	}
	return nil
}

// goSelectorCallInfo reports call's package and function name if call is a
// call_expression whose callee is a selector_expression on a bare
// identifier operand (`pkg.Name(...)`), e.g. ("os", "Stat", true) for
// `os.Stat(path)`. It reports ok == false for any other callee shape (a
// bare identifier callee, a chained/method selector, a call result, etc.)
// or if call is not a call_expression.
func goSelectorCallInfo(call engine.Node, source []byte) (pkg, name string, ok bool) {
	if call == nil || call.Kind() != "call_expression" {
		return "", "", false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "selector_expression" {
		return "", "", false
	}
	operand := fn.ChildByFieldName("operand")
	field := fn.ChildByFieldName("field")
	if operand == nil || operand.Kind() != "identifier" || field == nil {
		return "", "", false
	}
	return operand.Utf8Text(source), field.Utf8Text(source), true
}

// goCallFirstArgument returns call's "arguments" field's first non-
// punctuation child (the first argument expression), or nil if call has no
// arguments.
func goCallFirstArgument(call engine.Node) engine.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return nil
	}
	count := args.ChildCount()
	for i := 0; i < count; i++ {
		child := args.Child(i)
		switch child.Kind() {
		case "(", ")", ",":
			continue
		}
		return child
	}
	return nil
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
