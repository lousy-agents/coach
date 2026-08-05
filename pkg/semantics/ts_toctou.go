package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsLocationKey dedupes findings by source span alone (StartByte/EndByte),
// for detectors like toctou_check_then_act where a given resolved node
// (e.g. the "act" call) can only ever belong to one canonical Finding, so
// there is no separate "owning construct" half to key on the way
// tsMutatesInputKey has for mutates_input.
type tsLocationKey struct {
	startByte uint
	endByte   uint
}

// tsToctouActCallNames is the CWE-367 "act" file-operation call name set
// (Story 1): synchronous Node fs calls whose target path can change between
// a preceding existsSync gate observing it and this call acting on it.
var tsToctouActCallNames = map[string]bool{
	"readFileSync":   true,
	"writeFileSync":  true,
	"appendFileSync": true,
	"unlinkSync":     true,
	"rmSync":         true,
}

// checkTOCTOUCheckThenAct emits a "toctou_check_then_act" Finding (Story 1,
// CWE-367) if n (an if_statement or while_statement) gates its guarded body
// -- the if's "consequence" field or the while's "body" field, never the
// if's "alternative" (else) branch -- behind a bare `existsSync(path)` /
// `<obj>.existsSync(path)` condition, and that body's subtree contains a
// matching fs act call (tsToctouActCallNames) whose first argument has
// identical source text to the check call's first argument. A condition
// combined with &&, ||, a ternary, or a `!` negation is never a bare
// call_expression once unwrapTSParen has stripped the grammar's mandatory
// parenthesization of an if/while condition, so those out-of-scope forms
// are excluded without any special-casing.
//
// A nested existsSync guard on the same path (`if (existsSync(p)) { if
// (existsSync(p)) { readFileSync(p); } }`) makes both the outer and inner
// if's own call to this method resolve to the identical act call node
// independently, since findTSToctouActCall searches the whole guarded
// body's subtree, including any nested if. Deduping by the act call's
// Location (toctouActSeen) -- mirroring mutatesInputSeen's dedup rule for
// mutates_input -- ensures that resolves to exactly one Finding, not one
// per enclosing guard.
func (c *tsFeatureCollector) checkTOCTOUCheckThenAct(n engine.Node, source []byte) {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	checkArg := tsToctouCallArg(cond, source, "existsSync")
	if checkArg == nil {
		return
	}

	var body engine.Node
	if n.Kind() == "if_statement" {
		body = n.ChildByFieldName("consequence")
	} else {
		body = n.ChildByFieldName("body")
	}
	if body == nil {
		return
	}

	act := findTSToctouActCall(body, source, checkArg.Utf8Text(source))
	if act == nil {
		return
	}

	loc := locationFromNode(act)
	key := tsLocationKey{startByte: loc.StartByte, endByte: loc.EndByte}
	if c.toctouActSeen == nil {
		c.toctouActSeen = map[tsLocationKey]bool{}
	}
	if c.toctouActSeen[key] {
		return
	}
	c.toctouActSeen[key] = true

	c.findings = append(c.findings, newTOCTOUCheckThenActFinding(cond, act, checkArg.Utf8Text(source), source))
}

// findTSToctouActCall searches n's subtree (including inside any nested
// function-like constructs -- this detector does no scope resolution, only
// syntactic call/argument-text matching) for a call_expression named in
// tsToctouActCallNames whose first argument's source text equals pathText,
// returning the first one found or nil.
func findTSToctouActCall(n engine.Node, source []byte, pathText string) engine.Node {
	if n == nil {
		return nil
	}
	if n.Kind() == "call_expression" {
		if name, ok := tsCallFunctionName(n, source); ok && tsToctouActCallNames[name] {
			if arg := tsCallFirstArgument(n); arg != nil && arg.Utf8Text(source) == pathText {
				return n
			}
		}
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if found := findTSToctouActCall(n.Child(i), source, pathText); found != nil {
			return found
		}
	}
	return nil
}

// tsToctouCallArg returns call's first argument node if call is a
// call_expression named wantName -- a bare identifier callee (`wantName(...)`)
// or a member_expression callee whose "property" field's text is wantName
// (`<obj>.wantName(...)`), for any object -- or nil otherwise.
func tsToctouCallArg(call engine.Node, source []byte, wantName string) engine.Node {
	name, ok := tsCallFunctionName(call, source)
	if !ok || name != wantName {
		return nil
	}
	return tsCallFirstArgument(call)
}

// tsCallFunctionName resolves call's callee name: the bare identifier
// (`existsSync`) or the "property" field's text for a member_expression
// callee (`fs.existsSync`), regardless of the object. It reports ok == false
// for any other callee shape (e.g. a call result: `f()()`) or if call is
// not a call_expression.
func tsCallFunctionName(call engine.Node, source []byte) (name string, ok bool) {
	if call == nil || call.Kind() != "call_expression" {
		return "", false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return "", false
	}
	switch fn.Kind() {
	case "identifier":
		return fn.Utf8Text(source), true
	case "member_expression":
		property := fn.ChildByFieldName("property")
		if property == nil {
			return "", false
		}
		return property.Utf8Text(source), true
	default:
		return "", false
	}
}

// tsCallFirstArgument returns call's "arguments" field's first non-
// punctuation child (the first argument expression), or nil if call has no
// arguments.
func tsCallFirstArgument(call engine.Node) engine.Node {
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

// newTOCTOUCheckThenActFinding builds a "toctou_check_then_act" Finding
// (Story 1, CWE-367) for an existsSync-style checkCall guarding actCall (a
// matching fs act call on the identical path text pathText). Location is
// actCall's own span, not checkCall's, so tooling points directly at the
// racy operation; checkTOCTOUCheckThenAct only ever calls this once actCall
// has already been resolved non-nil.
func newTOCTOUCheckThenActFinding(checkCall, actCall engine.Node, pathText string, source []byte) Finding {
	return Finding{
		Kind:           "toctou_check_then_act",
		Name:           pathText,
		Location:       locationFromNode(actCall),
		Confidence:     "medium",
		Evidence:       checkCall.Utf8Text(source) + " ... " + actCall.Utf8Text(source),
		Recommendation: "Don't gate a filesystem operation behind an existsSync check on the same path -- the file can be created, removed, or replaced between the check and the act (CWE-367/TOCTOU). Call the operation directly and handle its ENOENT (or equivalent) error instead (EAFP-style); fs.promises.access does not close this race either, since it is still a separate check before the act.",
		SuggestedSkill: "find-bugs",
	}
}
