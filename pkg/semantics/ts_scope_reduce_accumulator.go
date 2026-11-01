package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

var tsReduceMethodNames = map[string]bool{"reduce": true, "reduceRight": true}

// tsFreshReduceAccumulatorName returns the accumulator parameter of decl when
// decl is an anonymous callback of a `x.reduce(...)` or `x.reduceRight(...)`
// member call whose initial value is an empty literal or a constructor call
// with no arguments, the callback returns nothing but that accumulator, and
// nothing in it reassigns the accumulator. Such an accumulator is created by
// the call, so mutating it is not a mutation of anything the caller owns.
//
// Each condition closes a way for caller-owned data to reach the accumulator.
// Reduce passes each return value in as the accumulator of the next call, so
// a callback that returns anything else, or reassigns the accumulator
// directly or from a closure, turns the parameter into that value from the
// second call on. A named callback can call itself with other data. A
// non-empty literal can hold references to caller-owned data (`{ items }`),
// and a reduce with no initial value uses the array's first element. A bare
// `reduce(...)` is a free function whose handling of its arguments is
// unknown. The method name is matched, not the receiver type, so a non-array
// receiver with a reduce method of the same shape is treated alike. Aliasing
// through property assignment, `arguments[0]` in sloppy-mode scripts, and an
// assertion wrapped around one element of a destructuring target
// (`({ a: acc! } = o)`) are not tracked.
func tsFreshReduceAccumulatorName(decl engine.Node, source []byte) (string, bool) {
	args := decl.Parent()
	if args == nil || args.Kind() != "arguments" || decl.ChildByFieldName("name") != nil {
		return "", false
	}
	if !tsIsMethodCallNamed(args.Parent(), tsReduceMethodNames, source) {
		return "", false
	}
	callArgs := tsArgumentNodes(args)
	if len(callArgs) < 2 || !sameNodeSpan(callArgs[0], decl) || !tsIsEmptyFreshValue(callArgs[1]) {
		return "", false
	}
	name, ok := tsFirstIdentifierParamName(decl, source)
	if !ok || !tsCallbackReturnsOnly(decl, name, source) || tsAssignsToName(decl.ChildByFieldName("body"), name, source) {
		return "", false
	}
	return name, true
}

// tsAssignsToName reports whether node rebinds name, including inside the
// right-hand side of another assignment and inside a closure: by assignment,
// destructuring, a for-in or for-of target, or a `var` declaration with an
// initializer. A nested function is searched too, unless its own parameters
// bind name, because a closure can reassign the outer variable. A callback
// that does so can return the accumulator while it holds something else, so
// the return check alone cannot establish that it is still the fresh value.
func tsAssignsToName(node engine.Node, name string, source []byte) bool {
	if node == nil {
		return false
	}
	if (tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition") && tsIdentifierParams(node, source)[name] {
		return false
	}
	switch node.Kind() {
	case "assignment_expression", "augmented_assignment_expression", "for_in_statement":
		if tsRebindTargetsInclude(node.ChildByFieldName("left"), name, source) {
			return true
		}
	case "variable_declarator":
		if node.Parent() != nil && node.Parent().Kind() == "variable_declaration" && node.ChildByFieldName("value") != nil {
			if target := node.ChildByFieldName("name"); target != nil && tsRebindTargetsInclude(target, name, source) {
				return true
			}
		}
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		if tsAssignsToName(node.Child(i), name, source) {
			return true
		}
	}
	return false
}

func tsRebindTargetsInclude(target engine.Node, name string, source []byte) bool {
	target = tsValueExpression(target)
	if target == nil {
		return false
	}
	targets := nameSet{}
	targets.collectReboundTargetNames(target, source)
	return targets[name]
}

// tsIsMethodCallNamed reports whether call is a member call such as
// `x.reduce(...)` whose method name is in names. A bare call `reduce(...)`
// is not: nothing is known about how a free function treats its arguments.
func tsIsMethodCallNamed(call engine.Node, names map[string]bool, source []byte) bool {
	name, ok := tsCallFunctionName(call, source)
	if !ok || !names[name] {
		return false
	}
	callee := call.ChildByFieldName("function")
	return callee != nil && callee.Kind() == "member_expression"
}

// tsFirstIdentifierParamName reports decl's first parameter when it is a
// plain identifier binding (tsFormalParameterIdentifierName's definition).
func tsFirstIdentifierParamName(decl engine.Node, source []byte) (string, bool) {
	if bare := decl.ChildByFieldName("parameter"); bare != nil && decl.Kind() == "arrow_function" {
		if bare.Kind() != "identifier" {
			return "", false
		}
		return bare.Utf8Text(source), true
	}
	formal := decl.ChildByFieldName("parameters")
	if formal == nil {
		return "", false
	}
	params := tsArgumentNodes(formal)
	if len(params) == 0 {
		return "", false
	}
	return tsFormalParameterIdentifierName(params[0], source)
}

// tsCallbackReturnsOnly reports whether every value decl returns is the
// identifier name. A callback that returns nothing, or a bare `return;`,
// does not qualify.
func tsCallbackReturnsOnly(decl engine.Node, name string, source []byte) bool {
	body := decl.ChildByFieldName("body")
	if body == nil {
		return false
	}
	if body.Kind() != "statement_block" {
		return tsIsIdentifierNamed(body, name, source)
	}
	var returned []engine.Node
	collectTSReturnedExpressions(body, &returned)
	for _, expression := range returned {
		if expression == nil || !tsIsIdentifierNamed(expression, name, source) {
			return false
		}
	}
	return len(returned) > 0
}

// collectTSReturnedExpressions appends the returned expression of every
// return statement under node, or nil for a bare `return;`, without entering
// nested functions.
func collectTSReturnedExpressions(node engine.Node, returned *[]engine.Node) {
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		child := node.Child(i)
		switch {
		case child.Kind() == "return_statement":
			*returned = append(*returned, tsReturnStatementExpression(child))
		case tsFunctionLikeKinds[child.Kind()] || child.Kind() == "method_definition":
		default:
			collectTSReturnedExpressions(child, returned)
		}
	}
}

func tsReturnStatementExpression(statement engine.Node) engine.Node {
	count := statement.ChildCount()
	for i := 0; i < count; i++ {
		child := statement.Child(i)
		switch child.Kind() {
		case "return", ";", "comment":
		default:
			return child
		}
	}
	return nil
}

func tsIsIdentifierNamed(n engine.Node, name string, source []byte) bool {
	value := tsValueExpression(n)
	return value != nil && value.Kind() == "identifier" && value.Utf8Text(source) == name
}

// tsIsEmptyFreshValue reports whether n evaluates to a value created in
// place with nothing inside it: `{}`, `[]`, or `new X()`.
func tsIsEmptyFreshValue(n engine.Node) bool {
	value := tsValueExpression(n)
	if value == nil {
		return false
	}
	switch value.Kind() {
	case "object", "array":
		return len(tsArgumentNodes(value)) == 0
	case "new_expression":
		args := value.ChildByFieldName("arguments")
		return args == nil || len(tsArgumentNodes(args)) == 0
	}
	return false
}

// tsValueExpression returns the expression n evaluates to, seen through
// parentheses, `as`, `satisfies`, angle-bracket and non-null assertions, and
// comma expressions (whose value is their last operand).
func tsValueExpression(n engine.Node) engine.Node {
	for n != nil {
		switch n.Kind() {
		case "parenthesized_expression":
			n = n.Child(1)
		case "as_expression", "satisfies_expression", "non_null_expression":
			n = n.Child(0)
		case "type_assertion", "sequence_expression":
			n = n.Child(n.ChildCount() - 1)
		default:
			return n
		}
	}
	return nil
}

// tsArgumentNodes returns the children of a delimited list node (call
// arguments, formal parameters, an object or array literal) that are not
// delimiters or comments.
func tsArgumentNodes(list engine.Node) []engine.Node {
	var nodes []engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		switch child.Kind() {
		case "(", ")", "[", "]", "{", "}", ",", "comment":
		default:
			nodes = append(nodes, child)
		}
	}
	return nodes
}
