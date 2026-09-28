package semantics

import (
	"regexp"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// constructorFuncNameRe matches function names that look like Go
// constructors: "New" followed by an uppercase letter, digit, or
// underscore (e.g. NewFoo, New2, New_thing), or "New" alone. It
// deliberately does not match names like "Newton", where the character
// after "New" is a lowercase letter.
var constructorFuncNameRe = regexp.MustCompile(`^New([A-Z0-9_]|$)`)

// computeGoFeatures walks root exactly once, producing both the structural
// metrics and the pattern findings for the file. A single traversal is used
// because nesting depth cannot be expressed as a Tree-sitter query -- doing
// metrics and findings together keeps results deterministic and cheap
// (avoids re-walking the tree per concern).
func computeGoFeatures(root engine.Node, source []byte) (StructuralMetrics, []Finding) {
	c := &featureCollector{}
	c.walk(root, source, 0, false)
	return c.metrics, c.findings
}

// featureCollector accumulates StructuralMetrics and Findings during a
// single pre-order walk of the tree.
type featureCollector struct {
	metrics  StructuralMetrics
	findings []Finding

	// goToctouActSeen dedupes toctou_check_then_act Findings by the act
	// call's own source span (see checkGoTOCTOUCheckThenAct), since a
	// nested Stat-gated if on the same path resolves the same act call
	// once per enclosing guard.
	goToctouActSeen map[goToctouLocationKey]bool
	mutatesSeen     map[mutatesInputKey]bool
}

// walk visits n and its descendants in pre-order, incrementing metrics
// counters for the node kinds AC-3.3 tracks. blockDepth counts nested
// "block" nodes seen so far within the current function/method body (0
// outside any body); inFunc reports whether the walk is currently inside a
// function_declaration or method_declaration, since nesting depth (AC-3.4)
// is only measured inside those bodies.

// checkConstructorFunc emits a "constructor_func" Finding (AC-3.5) if decl's
// name field matches constructorFuncNameRe.

// checkPointerReturn emits a "pointer_return" Finding (AC-3.6) if decl's
// result field contains a pointer_type, either directly (a single unnamed
// pointer return value) or among a parameter_list's parameter_declaration
// types (multiple and/or named return values).

// resultHasPointerType reports whether a function/method's result field
// contains a pointer_type anywhere in its subtree: as the result node
// itself (a single unnamed pointer return value), as a parameter_list's
// parameter_declaration type (multiple and/or named return values), or
// nested inside a composite type such as a slice, map value, or channel
// element (e.g. []*T, map[string]*T, chan *T). A full descendant search is
// used rather than checking only the direct result/type node, since Go
// permits pointer_type at any depth within a composite result type.

// checkMutatesInput emits one "mutates_input" Finding per distinct
// (parameter, mutation-expression location) pair where decl's body writes
// through a syntactically pointer/map/slice-typed parameter, either via a
// selector on the parameter or a dereference of it (cfg.Name = x,
// (*cfg).Name = x) or via index assignment on a map/slice parameter
// (values[k] = x, items[i] = x). Plain reassignment of the parameter
// variable itself (cfg = other) is a rebind, not a caller-visible mutation,
// and is deliberately excluded.

// mutatesInputKey dedupes findings by (parameter, mutation-expression
// location) per AC-5: repeated writes to the same parameter through the
// same source location must not produce duplicate findings.
type mutatesInputKey struct {
	paramName string
	startByte uint
	endByte   uint
}

// paramMutKind classifies a declared parameter's syntactic type for
// mutates_input purposes. Selector/dereference writes (cfg.Name = x,
// (*cfg).Name = x) are only caller-visible through a pointer, and index
// writes (values[k] = x, items[i] = x) are only caller-visible through a
// map or slice -- collapsing these into a single bool would let a
// selector write on a map/slice parameter, or an index write on a
// (non-array) pointer parameter, be misreported as mutates_input even
// though neither is a caller-visible write for that parameter's actual
// type.
type paramMutKind uint8

const (
	// paramNotMutable is also the zero value, so a parameter absent from
	// a paramMutKind map (or explicitly recorded as such) is never
	// mistaken for a pointer/collection parameter.
	paramNotMutable paramMutKind = iota
	paramMutPointer
	paramMutCollection
)

// mutableParamTypes maps each declared parameter identifier to its
// paramMutKind, derived from whether its syntactic type is a pointer_type
// (paramMutPointer) or a map_type/slice_type (paramMutCollection). Go
// allows a single parameter_declaration to bind multiple names to one
// shared type (func f(a, b *T)), so every identifier child of each
// parameter_declaration is collected, not just the first.

// findAssignments walks n's subtree looking for assignment_statement nodes
// whose left-hand-side targets write through a mutable parameter, and
// records a deduplicated "mutates_input" Finding for each. It does not
// descend specially into nested function_declaration/func_literal bodies:
// a closure's own mutation of an outer mutable parameter is still a
// caller-visible mutation of that parameter and is deliberately still
// reported.
func (c *featureCollector) findAssignments(n engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind) {
	if n == nil {
		return
	}

	if n.Kind() == "type_switch_statement" {
		// The type-switch alias (e.g. `cfg` in `switch cfg := v.(type)`) is
		// scoped only to this switch statement's own subtree, so it must
		// shadow mutableParams for the recursive descent below but must not
		// leak into the shadowing applied to the enclosing block's later
		// siblings at the bottom of this function.
		mutableParams = shadowNames(mutableParams, identifiersInNodeField(n, "alias", source))
	}

	if n.Kind() == "assignment_statement" {
		left := n.ChildByFieldName("left")
		if left != nil {
			targetCount := left.ChildCount()
			for i := 0; i < targetCount; i++ {
				c.checkAssignmentTarget(left.Child(i), source, funcName, mutableParams)
			}
		}
	}
	if n.Kind() == "inc_statement" || n.Kind() == "dec_statement" {
		c.checkAssignmentTarget(updateStatementTarget(n), source, funcName, mutableParams)
	}
	if n.Kind() == "func_literal" {
		if params := n.ChildByFieldName("parameters"); params != nil {
			mutableParams = shadowParamTypes(mutableParams, params, source)
		}
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		c.findAssignments(child, source, funcName, mutableParams)
		mutableParams = shadowLocalDeclarations(mutableParams, child, source)
	}
}

// shadowLocalDeclarations returns a copy of outer with any entries shadowed by
// local declarations or direct parameter rebindings in n removed. It is
// applied while walking child nodes in source order so a short variable
// declaration such as `cfg := &Config{}` or a parameter rebind such as
// `cfg = other` shadows an outer parameter only for subsequent source nodes.

// shadowNames returns a copy of outer with any entries whose name appears in
// names removed.

type identSet map[string]bool

// shadowParamTypes returns a copy of outer with any entries shadowed by
// literalParams (a func_literal's own "parameters" field) removed, so that
// a closure declaring its own parameter with the same name as an outer
// function's parameter (e.g. an outer `cfg` shadowed by a closure's own
// `func(cfg *Config){ ... }`) is never misattributed to the outer
// parameter: the closure's binding is a distinct variable per normal Go
// scoping, and this single-pass walk has no separate owner name to
// attribute the closure's own mutations to, so those are simply not
// reported rather than being reported against the wrong (outer) name.
// Outer parameters not redeclared by the literal are left untouched, since
// the walk is still inside their scope.

// checkAssignmentTarget inspects a single assignment target (one of an
// assignment_statement's possibly-multiple left-hand-side expressions) and
// emits a "mutates_input" Finding if it writes through a mutable parameter.

// requiredKind is the only paramMutKind that makes a DIRECT selector or
// index target (the parameter itself, optionally through one "(*p)"
// dereference hop -- exactly the shapes the acceptance criteria name:
// cfg.Name, (*cfg).Name, values[k], items[i]) a caller-visible write:
// selector/dereference is only caller-visible through a pointer
// parameter, and index is only caller-visible through a map/slice
// parameter. A map/slice parameter's direct selector write, or a
// pointer parameter's direct index write, is not detected.
//
// A NESTED target (reached through at least one additional
// selector/index hop beyond the direct base, e.g. cfg.Sub.Name or
// cfg.Items[0]) is checked more permissively -- any mutable root kind
// is accepted regardless of the target's own shape -- because the
// intermediate field/element's own type (e.g. that Items is a slice)
// is not visible without resolving another type's declaration
// elsewhere in the file, which is out of scope for this syntax-only
// detector; the root parameter being a pointer (or map/slice) at all
// already establishes that state reachable through it is
// caller-visible.

// Plain identifier targets (cfg = other) rebind the local
// parameter variable rather than writing through it, and any
// other target kind is out of scope for this detector.

func (c *featureCollector) recordMutatesInput(funcName, paramName string, target engine.Node, source []byte) {
	loc := locationFromNode(target)
	key := mutatesInputKey{paramName: paramName, startByte: loc.StartByte, endByte: loc.EndByte}
	if c.mutatesSeen[key] {
		return
	}
	c.mutatesSeen[key] = true
	c.findings = append(c.findings, newMutatesInputFinding(funcName, paramName, target, source))
}

// selectorBaseIdentifier resolves a selector_expression's operand down to
// the root identifier it ultimately reads, walking through any chain of
// nested selector_expression/index_expression operands (cfg.Sub.Name,
// cfg.Items[0].Name) and the parenthesized-unary-dereference case
// ((*cfg).Name, (*cfg.Sub).Name), iterating until it reaches a plain
// identifier -- the root -- or determines there is no such root (e.g. the
// chain bottoms out in a function call like f().Name), in which case base
// is nil.
//
// nested reports whether reaching that root required stepping through at
// least one selector_expression/index_expression hop beyond the "direct"
// shapes the acceptance criteria name explicitly: a bare identifier
// (cfg.Name, where operand IS the parameter) or a single parenthesized
// dereference of one ((*cfg).Name). Those two direct shapes report
// nested == false; anything requiring an additional hop (cfg.Sub.Name,
// cfg.Items[0]) reports nested == true. checkAssignmentTarget uses this to
// require an exact selector<->pointer / index<->collection kind match only
// for direct targets, where the acceptance criteria pin down the required
// parameter kind precisely -- a nested target's intermediate field/element
// type is not visible to this syntax-only detector, so it is checked more
// permissively (see checkAssignmentTarget's comment).

// derefOperand returns the operand of the parenthesized unary "*"
// (dereference) expression nested directly inside a parenthesized_expression
// ((*cfg) -> cfg, (*cfg.Sub) -> cfg.Sub), or nil if paren does not wrap
// exactly that shape. A unary_expression's "operator" field must be checked
// by source text, not just by the presence of a unary_expression node: Go's
// grammar uses the same unary_expression node for "*", "&", "!", "-", "+",
// "^", and "<-", so a parenthesized non-dereference unary expression like
// (-x), (!x), or (&x) must not be mis-resolved as if it were (*x).

// newMutatesInputFinding builds a "mutates_input" Finding (Story 1/3) for a
// mutation of parameter paramName within function/method funcName, located
// at evidence's own source span so tooling can point directly at the
// mutating expression rather than the enclosing declaration.
func newMutatesInputFinding(funcName, paramName string, evidence engine.Node, source []byte) Finding {
	return Finding{
		Kind:           "mutates_input",
		Name:           funcName + ":" + paramName,
		Location:       locationFromNode(evidence),
		Confidence:     "medium",
		Evidence:       evidence.Utf8Text(source),
		Recommendation: "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
		SuggestedSkill: "refactor-hidden-mutation",
	}
}
