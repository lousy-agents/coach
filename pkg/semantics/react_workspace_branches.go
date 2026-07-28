package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactDiscriminantOps is the set of equality operators a workspace-branch
// discriminant condition may use.
var reactDiscriminantOps = map[string]bool{"===": true, "!==": true, "==": true, "!=": true}

// reactExtractWorkspaceBranches collects ReactWorkspaceBranch entries from
// two disjoint constructs: discriminant ternary/if-else-if chains gated at
// >=3 JSX-bearing branches per chain, and role="tabpanel" JSX elements
// (ungated). Results are de-duplicated by the branch's own Location.StartByte
// and ordered by that same start_byte.
func reactExtractWorkspaceBranches(body engine.Node, source []byte) []ReactWorkspaceBranch {
	var branches []ReactWorkspaceBranch
	seen := map[uint]struct{}{}
	addBranch := func(b ReactWorkspaceBranch) {
		if _, dup := seen[b.Location.StartByte]; dup {
			return
		}
		seen[b.Location.StartByte] = struct{}{}
		branches = append(branches, b)
	}

	reactCollectDiscriminantBranches(body, source, addBranch)
	reactCollectTabpanelBranches(body, source, addBranch)

	sort.SliceStable(branches, func(i, j int) bool {
		return branches[i].Location.StartByte < branches[j].Location.StartByte
	})
	return branches
}

func reactCollectDiscriminantBranches(body engine.Node, source []byte, add func(ReactWorkspaceBranch)) {
	consumed := map[[2]uint]struct{}{}
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "ternary_expression", "if_statement":
		default:
			return
		}
		key := [2]uint{n.StartByte(), n.EndByte()}
		if _, done := consumed[key]; done {
			return
		}
		base, ok := reactDiscriminantConditionBase(n, source)
		if !ok {
			return
		}
		chainBranches, chainNodes := reactCollectDiscriminantChain(n, base, source)
		for _, cn := range chainNodes {
			consumed[[2]uint{cn.StartByte(), cn.EndByte()}] = struct{}{}
		}
		if len(chainBranches) < 3 {
			return
		}
		for _, b := range chainBranches {
			add(b)
		}
	})
}

func reactCollectTabpanelBranches(body engine.Node, source []byte, add func(ReactWorkspaceBranch)) {
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "jsx_opening_element", "jsx_self_closing_element":
		default:
			return
		}
		if b, ok := reactTabpanelBranch(n, source); ok {
			add(b)
		}
	})
}

// reactDiscriminantConditionBase reports whether n's "condition" field is a
// binary_expression testing equality/inequality between a discriminant base
// (a plain identifier, or a member_expression -- whose full text, not just
// its property, is used as the base so structurally distinct discriminants
// like props.v and other.v never compare equal) and a string/number
// literal, and returns that base text.
func reactDiscriminantConditionBase(n engine.Node, source []byte) (string, bool) {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return "", false
	}
	if !reactDiscriminantOps[tsBinaryOp(cond)] {
		return "", false
	}
	left := cond.ChildByFieldName("left")
	right := cond.ChildByFieldName("right")
	if left == nil || right == nil {
		return "", false
	}
	if base, ok := reactDiscriminantBase(left, right, source); ok {
		return base, true
	}
	return reactDiscriminantBase(right, left, source)
}

func reactDiscriminantBase(baseSide, literalSide engine.Node, source []byte) (string, bool) {
	if literalSide == nil || (literalSide.Kind() != "string" && literalSide.Kind() != "number") {
		return "", false
	}
	switch baseSide.Kind() {
	case "identifier":
		return baseSide.Utf8Text(source), true
	case "member_expression":
		return baseSide.Utf8Text(source), true
	}
	return "", false
}

// reactCollectDiscriminantChain walks n forward through same-base chained
// ternary/if-else-if branches and returns the JSX-bearing branches found
// plus every chain node visited (regardless of JSX-bearing outcome), so the
// caller can mark the whole chain consumed even when the gate fails it.
func reactCollectDiscriminantChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	switch n.Kind() {
	case "ternary_expression":
		return reactCollectTernaryChain(n, base, source)
	case "if_statement":
		return reactCollectIfChain(n, base, source)
	default:
		return nil, nil
	}
}

func reactCollectTernaryChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	var branches []ReactWorkspaceBranch
	var nodes []engine.Node
	cur := n
	for cur != nil && cur.Kind() == "ternary_expression" {
		curBase, ok := reactDiscriminantConditionBase(cur, source)
		if !ok || curBase != base {
			break
		}
		nodes = append(nodes, cur)
		if b, ok := reactTernaryBranch(cur, source); ok {
			branches = append(branches, b)
		}
		alt := unwrapTSParen(cur.ChildByFieldName("alternative"))
		if alt != nil && alt.Kind() == "ternary_expression" {
			cur = alt
			continue
		}
		// Terminal residual alternative of the same-base chain (e.g. the
		// final `: <DefaultPanel />` arm). null/undefined/non-JSX yield no branch.
		if b, ok := reactResidualBranch(alt, source); ok {
			branches = append(branches, b)
		}
		break
	}
	return branches, nodes
}

func reactCollectIfChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	var branches []ReactWorkspaceBranch
	var nodes []engine.Node
	cur := n
	for cur != nil && cur.Kind() == "if_statement" {
		curBase, ok := reactDiscriminantConditionBase(cur, source)
		if !ok || curBase != base {
			break
		}
		nodes = append(nodes, cur)
		if b, ok := reactIfBranch(cur, source); ok {
			branches = append(branches, b)
		}
		next, terminal := reactIfChainContinue(cur, source)
		if next != nil {
			cur = next
			continue
		}
		if terminal != nil {
			branches = append(branches, *terminal)
		}
		break
	}
	return branches, nodes
}

// reactIfChainContinue returns the next if_statement in an else-if chain, or
// a terminal residual branch when the chain ends in a final else body.
func reactIfChainContinue(cur engine.Node, source []byte) (next engine.Node, terminal *ReactWorkspaceBranch) {
	alt := cur.ChildByFieldName("alternative")
	if alt != nil && alt.Kind() == "else_clause" {
		alt = tsElseClauseInner(alt)
	}
	if alt != nil && alt.Kind() == "if_statement" {
		return alt, nil
	}
	if b, ok := reactResidualBranch(alt, source); ok {
		return nil, &b
	}
	return nil, nil
}

func reactTernaryBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	cons := unwrapTSParen(n.ChildByFieldName("consequence"))
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(n, cons, source),
		Location: locationFromNode(cons),
	}, true
}

func reactIfBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	cons := n.ChildByFieldName("consequence")
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(n, cons, source),
		Location: locationFromNode(cons),
	}, true
}

// reactResidualBranch emits a workspace branch for a chain's terminal
// residual alternative / final else body when it contains JSX. Label uses
// Design precedence steps 2–3 only (no equality literal on residual arms).
func reactResidualBranch(cons engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(nil, cons, source),
		Location: locationFromNode(cons),
	}, true
}

// reactWorkspaceBranchLabel applies the label precedence rule: the
// discriminant condition's literal text first, else a capitalized primary
// JSX child's tag name, else the "<branch>" sentinel.
func reactWorkspaceBranchLabel(condHolder, cons engine.Node, source []byte) string {
	if condHolder != nil {
		if lbl := reactDiscriminantLiteralLabel(condHolder, source); lbl != "" {
			return lbl
		}
	}
	if lbl := reactCapitalizedJSXChildLabel(cons, source); lbl != "" {
		return lbl
	}
	return "<branch>"
}

func reactDiscriminantLiteralLabel(n engine.Node, source []byte) string {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return ""
	}
	if lbl, ok := reactLiteralLabelText(cond.ChildByFieldName("left"), source); ok {
		return lbl
	}
	if lbl, ok := reactLiteralLabelText(cond.ChildByFieldName("right"), source); ok {
		return lbl
	}
	return ""
}

func reactLiteralLabelText(n engine.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case "string":
		return reactBareStringText(n, source)
	case "number":
		return n.Utf8Text(source), true
	default:
		return "", false
	}
}

func reactCapitalizedJSXChildLabel(cons engine.Node, source []byte) string {
	el := reactPrimaryJSXElement(cons)
	if el == nil {
		return ""
	}
	name := reactJSXElementTagName(el, source)
	if name == "" || !isPascalCaseName(name) {
		return ""
	}
	return name
}

func reactPrimaryJSXElement(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element":
		return n
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if el := reactPrimaryJSXElement(n.Child(i)); el != nil {
			return el
		}
	}
	return nil
}

func reactJSXElementTagName(n engine.Node, source []byte) string {
	switch n.Kind() {
	case "jsx_element":
		open := n.ChildByFieldName("open_tag")
		if open == nil {
			return ""
		}
		if name := open.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return ""
	case "jsx_self_closing_element":
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return ""
	default:
		return ""
	}
}

func reactContainsJSX(n engine.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
		return true
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if reactContainsJSX(n.Child(i)) {
			return true
		}
	}
	return false
}

// reactTabpanelBranch reports whether n is a JSX opening/self-closing
// element carrying role="tabpanel", and if so returns its branch: label is
// its aria-label attribute's string value, else its id attribute's string
// value, else the literal "tabpanel".
func reactTabpanelBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	attrs := reactJSXAttributes(n)
	if !reactHasStringJSXAttr(attrs, source, "role", "tabpanel") {
		return ReactWorkspaceBranch{}, false
	}
	label := reactFirstStringJSXAttr(attrs, source, "aria-label")
	if label == "" {
		label = reactFirstStringJSXAttr(attrs, source, "id")
	}
	if label == "" {
		label = "tabpanel"
	}
	return ReactWorkspaceBranch{Label: label, Location: locationFromNode(n)}, true
}

func reactHasStringJSXAttr(attrs []engine.Node, source []byte, name, want string) bool {
	for _, attr := range attrs {
		if reactJSXAttributeNameText(attr, source) != name {
			continue
		}
		if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok && s == want {
			return true
		}
	}
	return false
}

func reactFirstStringJSXAttr(attrs []engine.Node, source []byte, name string) string {
	for _, attr := range attrs {
		if reactJSXAttributeNameText(attr, source) != name {
			continue
		}
		if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok {
			return s
		}
	}
	return ""
}
