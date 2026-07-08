package semantics

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// AC-3.3: computeGoFeatures must count exactly one StructuralMetrics field per
// tracked Tree-sitter node kind, in a single traversal. The fixture below
// hand-counts to 2 ifs, 1 for, 1 expression switch, 1 type switch, 1
// select, 2 functions (Foo, Bar), and 1 method (on *T) -- every field must
// match that exact count, not merely be non-zero.
func TestMetrics_CountsEachNodeKindExactly(t *testing.T) {
	source := []byte(`package main

type T struct{}

func Foo(x int) {
	if x > 0 {
	}
	if x < 0 {
	}
	for i := 0; i < x; i++ {
	}
	switch x {
	case 1:
	}
	var v interface{} = x
	switch v.(type) {
	case int:
	}
	ch := make(chan int)
	select {
	case <-ch:
	}
}

func Bar() {
}

func (t *T) Method() {
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	want := StructuralMetrics{
		Ifs:          2,
		Fors:         1,
		ExprSwitches: 1,
		TypeSwitches: 1,
		Selects:      1,
		Functions:    2,
		Methods:      1,
	}
	if metrics.Ifs != want.Ifs {
		t.Errorf("computeGoFeatures Ifs: got %d, want %d", metrics.Ifs, want.Ifs)
	}
	if metrics.Fors != want.Fors {
		t.Errorf("computeGoFeatures Fors: got %d, want %d", metrics.Fors, want.Fors)
	}
	if metrics.ExprSwitches != want.ExprSwitches {
		t.Errorf("computeGoFeatures ExprSwitches: got %d, want %d", metrics.ExprSwitches, want.ExprSwitches)
	}
	if metrics.TypeSwitches != want.TypeSwitches {
		t.Errorf("computeGoFeatures TypeSwitches: got %d, want %d", metrics.TypeSwitches, want.TypeSwitches)
	}
	if metrics.Selects != want.Selects {
		t.Errorf("computeGoFeatures Selects: got %d, want %d", metrics.Selects, want.Selects)
	}
	if metrics.Functions != want.Functions {
		t.Errorf("computeGoFeatures Functions: got %d, want %d", metrics.Functions, want.Functions)
	}
	if metrics.Methods != want.Methods {
		t.Errorf("computeGoFeatures Methods: got %d, want %d", metrics.Methods, want.Methods)
	}
}

// AC-3.4: max_nesting_depth is the maximum depth of nested block nodes
// within any single function/method body, where the body's own top-level
// block counts as depth 1. This fixture nests: func body (block, depth 1)
// -> if consequence (block, depth 2) -> for body (block, depth 3) -> if
// consequence (block, depth 4). The innermost if body is empty, so it adds
// no further depth.
func TestMetrics_MaxNestingDepthWithinFunctionBody(t *testing.T) {
	source := []byte(`package main

func f() {
	if true {
		for {
			if true {
			}
		}
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	if metrics.MaxNestingDepth != 4 {
		t.Errorf("computeGoFeatures MaxNestingDepth for 4-deep nested blocks %q: got %d, want %d", source, metrics.MaxNestingDepth, 4)
	}
}

// AC-3.4: a file with no function/method declarations at all must report
// MaxNestingDepth == 0, since there is no function body to measure nesting
// within.
func TestMetrics_ZeroNestingDepthWhenFileHasNoFunctions(t *testing.T) {
	source := []byte(`package main

var x int

type T struct {
	Field int
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	if metrics.MaxNestingDepth != 0 {
		t.Errorf("computeGoFeatures MaxNestingDepth for a file with no functions %q: got %d, want 0", source, metrics.MaxNestingDepth)
	}
}

// AC-3.5: a function_declaration whose name matches ^New([A-Z0-9_]|$) must
// emit a "constructor_func" Finding named after the function. NewFoo and the
// bare New both match; Newton must not, since 't' is neither uppercase,
// a digit, underscore, nor end-of-string.
func TestFindings_ConstructorFuncMatchesNewPrefixButNotNewton(t *testing.T) {
	source := []byte(`package main

func NewFoo() {}

func New() {}

func Newton() {}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	constructorNames := map[string]bool{}
	for _, f := range findings {
		if f.Kind == "constructor_func" {
			constructorNames[f.Name] = true
		}
	}

	if !constructorNames["NewFoo"] {
		t.Errorf("computeGoFeatures findings for %q: want a constructor_func finding named %q, got %+v", source, "NewFoo", findings)
	}
	if !constructorNames["New"] {
		t.Errorf("computeGoFeatures findings for %q: want a constructor_func finding named %q, got %+v", source, "New", findings)
	}
	if constructorNames["Newton"] {
		t.Errorf("computeGoFeatures findings for %q: want no constructor_func finding named %q (does not match ^New([A-Z0-9_]|$)), got %+v", source, "Newton", findings)
	}
}

// hasFinding reports whether findings contains a Finding with the given
// kind and name.
func hasFinding(findings []Finding, kind, name string) bool {
	for _, f := range findings {
		if f.Kind == kind && f.Name == name {
			return true
		}
	}
	return false
}

// AC-3.6: a function_declaration whose result includes a pointer_type (here,
// the single unnamed return value `*Foo`) must emit a "pointer_return"
// Finding named after the function.
func TestFindings_PointerReturnOnFunction(t *testing.T) {
	source := []byte(`package main

type Foo struct{}

func f() *Foo {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeGoFeatures findings for pointer-returning function %q: want a pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// AC-3.6: the same pointer_type-in-result rule applies to method_declaration,
// using the method's field_identifier name.
func TestFindings_PointerReturnOnMethod(t *testing.T) {
	source := []byte(`package main

type T struct{}
type Foo struct{}

func (t *T) Method() *Foo {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "Method") {
		t.Errorf("computeGoFeatures findings for pointer-returning method %q: want a pointer_return finding named %q, got %+v", source, "Method", findings)
	}
}

// AC-3.6: a function returning a plain value type (no pointer_type anywhere
// in the result) must not emit a pointer_return finding.
func TestFindings_ValueReturnEmitsNoPointerFinding(t *testing.T) {
	source := []byte(`package main

type Foo struct{}

func f() Foo {
	return Foo{}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeGoFeatures findings for value-returning function %q: want no pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// Regression guard raised by review: AC-3.6 requires a finding when the
// result list contains at least one pointer_type anywhere in it, not just
// as the direct result node or a parameter_declaration's direct type field.
// A slice-of-pointer result (func f() []*T -> result: (slice_type element:
// (pointer_type ...))) previously produced no finding at all.
func TestFindings_PointerReturnOnSliceOfPointerResult(t *testing.T) {
	source := []byte(`package main

type T struct{}

func f() []*T {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeGoFeatures findings for []*T-returning function %q: want a pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// Regression guard: a map-value-of-pointer result among multiple named
// return values (func g() (map[string]*T, error) -> the first
// parameter_declaration's type field is map_type, not pointer_type
// directly) previously produced no finding.
func TestFindings_PointerReturnOnMapValuePointerAmongMultipleResults(t *testing.T) {
	source := []byte(`package main

type T struct{}

func g() (map[string]*T, error) {
	return nil, nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "g") {
		t.Errorf("computeGoFeatures findings for map[string]*T-returning function %q: want a pointer_return finding named %q, got %+v", source, "g", findings)
	}
}

// Regression guard: a channel-of-pointer result (func h() chan *T ->
// result: (channel_type value: (pointer_type ...))) previously produced no
// finding.
func TestFindings_PointerReturnOnChannelOfPointerResult(t *testing.T) {
	source := []byte(`package main

type T struct{}

func h() chan *T {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "h") {
		t.Errorf("computeGoFeatures findings for chan *T-returning function %q: want a pointer_return finding named %q, got %+v", source, "h", findings)
	}
}

// Regression guard: the same composite-result rule must apply to methods,
// not just functions, since checkPointerReturn is shared between them.
func TestFindings_PointerReturnOnMethodWithSliceOfPointerResult(t *testing.T) {
	source := []byte(`package main

type T struct{}
type Recv struct{}

func (r *Recv) Items() []*T {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "Items") {
		t.Errorf("computeGoFeatures findings for []*T-returning method %q: want a pointer_return finding named %q, got %+v", source, "Items", findings)
	}
}

// mutatesInputFinding returns a pointer to the first "mutates_input" Finding
// in findings matching name, or nil if there is none.
func mutatesInputFinding(findings []Finding, name string) *Finding {
	for i := range findings {
		if findings[i].Kind == "mutates_input" && findings[i].Name == name {
			return &findings[i]
		}
	}
	return nil
}

// countMutatesInputFindings reports how many "mutates_input" findings with
// the given name are present in findings.
func countMutatesInputFindings(findings []Finding, name string) int {
	n := 0
	for _, f := range findings {
		if f.Kind == "mutates_input" && f.Name == name {
			n++
		}
	}
	return n
}

// Story 1/3: a selector write through a syntactically pointer-typed
// parameter (cfg.Name = "x", cfg *Config) must emit exactly one
// "mutates_input" Finding, with Name "<func>:<param>", Location pointing at
// the mutation expression (not the function declaration), and the coaching
// metadata fields set as specified.
func TestGoMutatesInput_PointerSelectorWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	cfg.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Name" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Name")
	}
	if got.Confidence != "medium" {
		t.Errorf("mutates_input Confidence: got %q, want %q", got.Confidence, "medium")
	}
	if got.Recommendation == "" {
		t.Errorf("mutates_input Recommendation: got empty, want a non-empty recommendation")
	}
	if got.SuggestedSkill != "refactor-hidden-mutation" {
		t.Errorf("mutates_input SuggestedSkill: got %q, want %q", got.SuggestedSkill, "refactor-hidden-mutation")
	}
	wantExprStart := uint(strings.Index(string(source), `cfg.Name = "x"`))
	if got.Location.StartByte != wantExprStart {
		t.Errorf("mutates_input Location.StartByte: got %d, want %d (start of mutation expression, not the function declaration)", got.Location.StartByte, wantExprStart)
	}
}

// Story 1: a dereference write through a pointer-typed parameter
// ((*cfg).Name = "x") must also emit a mutates_input finding.
func TestGoMutatesInput_DereferenceWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(*cfg).Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for dereference write %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// Story 1: an index write on a map-typed parameter (values[key] = x,
// values map[string]int) must emit a mutates_input finding.
func TestGoMutatesInput_MapIndexWrite(t *testing.T) {
	source := []byte(`package main

func f(values map[string]int) {
	values["k"] = 1
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:values") {
		t.Errorf("computeGoFeatures findings for map index write %q: want a mutates_input finding named %q, got %+v", source, "f:values", findings)
	}
}

// Story 1: an index write on a slice-typed parameter (items[i] = x,
// items []int) must emit a mutates_input finding.
func TestGoMutatesInput_SliceIndexWrite(t *testing.T) {
	source := []byte(`package main

func f(items []int) {
	items[0] = 2
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:items") {
		t.Errorf("computeGoFeatures findings for slice index write %q: want a mutates_input finding named %q, got %+v", source, "f:items", findings)
	}
}

// AC-5: multiple writes through the same parameter at distinct source
// locations must produce one finding per distinct location, and writing to
// the exact same expression location must never be double-counted.
func TestGoMutatesInput_DuplicateWritesDeduped(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
	Age  int
}

func f(cfg *Config) {
	cfg.Name = "x"
	cfg.Age = 1
	cfg.Name = "y"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := countMutatesInputFindings(findings, "f:cfg")
	if got != 3 {
		t.Errorf("computeGoFeatures findings for %q: got %d mutates_input findings named %q (one per distinct mutation-expression location), want 3; findings=%+v", source, got, "f:cfg", findings)
	}

	seenLocations := map[uint]bool{}
	for _, f := range findings {
		if f.Kind != "mutates_input" || f.Name != "f:cfg" {
			continue
		}
		if seenLocations[f.Location.StartByte] {
			t.Errorf("computeGoFeatures findings for %q: duplicate mutates_input finding at the same Location.StartByte %d", source, f.Location.StartByte)
		}
		seenLocations[f.Location.StartByte] = true
	}
}

// AC-4: a selector write on a plain (non-pointer/map/slice) value parameter
// mutates only a local copy, not caller-visible state, and must not emit a
// finding.
func TestGoMutatesInput_ValueParameterSelectorWriteNoFinding(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg Config) {
	cfg.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for value-parameter selector write %q: want no mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// AC-3: reassigning the parameter variable itself (cfg = other) rebinds the
// local variable rather than writing through it, and must not emit a
// finding, even though cfg's declared type is a pointer.
func TestGoMutatesInput_PlainParameterReassignmentNoFinding(t *testing.T) {
	source := []byte(`package main

type Config struct{}

func f(cfg *Config, other *Config) {
	cfg = other
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for plain parameter reassignment %q: want no mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// A pointer-typed receiver mutated through a selector is not a declared
// parameter (the issue explicitly scopes this feature to parameters, not
// receivers), so a method mutating only its receiver must not emit a
// mutates_input finding.
func TestGoMutatesInput_ReceiverMutationIsNotAParameter(t *testing.T) {
	source := []byte(`package main

type T struct {
	Name string
}

func (t *T) Method() {
	t.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Errorf("computeGoFeatures findings for receiver-only mutation %q: want no mutates_input findings (receiver is not a parameter), got %+v", source, findings)
		}
	}
}

// Copilot review fix: a nested selector write (cfg.Sub.Name = "x", where
// cfg is a pointer parameter) is still a caller-visible write through cfg
// one field deeper, and must resolve to the root identifier cfg rather than
// being silently missed because the selector's own operand is itself a
// selector_expression rather than a bare identifier.
func TestGoMutatesInput_NestedSelectorWrite(t *testing.T) {
	source := []byte(`package main

type Sub struct {
	Name string
}

type Config struct {
	Sub Sub
}

func f(cfg *Config) {
	cfg.Sub.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for nested selector write %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Sub.Name" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Sub.Name")
	}
}

// Copilot review fix: a func_literal that declares its own parameter with
// the same name as an outer function's mutable parameter (both named cfg,
// both *Config) introduces a distinct binding per normal Go scoping. The
// closure's own mutation of its cfg must not be misattributed to the outer
// function f's cfg.
func TestGoMutatesInput_FuncLiteralShadowsOuterParameter(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	g := func(cfg *Config) {
		cfg.Name = "shadowed"
	}
	g(cfg)
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for closure-shadowed parameter %q: want no mutates_input finding named %q (closure's cfg is a distinct binding), got %+v", source, "f:cfg", findings)
	}
}

// AC-6: computeGoFeatures is only reached from AnalyzeBytes on a clean parse
// (analyzer.go returns a partial Result on root.HasError() before ever
// calling spec.computeFeatures), so a file with syntax errors can never
// produce mutates_input findings. Verified at the AnalyzeBytes level,
// matching the existing syntax-error test pattern used elsewhere in this
// package.
func TestGoMutatesInput_SyntaxErrorEmitsNoFindings(t *testing.T) {
	source := []byte(`package main

func f(cfg *Config) {
	cfg.Name =
}
`)
	a := mustNewAnalyzer(t)
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "f.go",
		Language: LanguageGo,
		Content:  source,
	})
	if err == nil {
		t.Fatalf("AnalyzeBytes for syntactically invalid source %q: got nil err, want a syntax error", source)
	}
	if result == nil {
		t.Fatalf("AnalyzeBytes for syntactically invalid source %q: got nil result, want a partial Result", source)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes for syntactically invalid source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "syntax_errors")
	}
	if len(result.Findings) != 0 {
		t.Errorf("AnalyzeBytes for syntactically invalid source %q: Findings = %+v, want empty", source, result.Findings)
	}
}

// Copilot review fix: an index_expression target's operand can itself be a
// nested selector/index chain (cfg.Items[0], not just a bare identifier
// like items[0]), and must resolve to its root identifier the same way
// selector_expression targets already do, so a caller-visible index write
// reached through a pointer-typed parameter's field is still detected.
func TestGoMutatesInput_NestedIndexWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Items []int
}

func f(cfg *Config) {
	cfg.Items[0] = 1
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for nested index write %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Items[0]" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Items[0]")
	}
}

// Copilot review fix: mutableParamTypes previously collapsed pointer/map/
// slice parameters into a single bool, so a DIRECT index write on a
// pointer parameter (cfg[0] = x, valid Go only for a pointer-to-array) was
// treated the same as a direct index write on a map/slice parameter, even
// though the acceptance criteria scope index writes to map/slice
// parameters specifically. A direct index target's root kind must now
// match exactly.
func TestGoMutatesInput_DirectIndexWriteOnPointerParamNoFinding(t *testing.T) {
	source := []byte(`package main

func f(cfg *[5]int) {
	cfg[0] = 1
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for direct index write on pointer parameter %q: want no mutates_input finding, got %+v", source, findings)
	}
}

// Copilot review fix: the same collapse also let a DIRECT selector write
// on a map/slice parameter pass the (bool) mutability check even though
// only pointer parameters are in scope for selector/dereference writes.
// This is not valid Go (a slice type has no fields) but this detector
// never type-checks the file, only its own syntax shape -- Tree-sitter
// parses "items.Foo = x" as a selector_expression assignment target
// regardless of whether Foo is a real field, so the fix must reject it at
// the syntax level rather than relying on the parameter merely being
// "mutable at all".
func TestGoMutatesInput_DirectSelectorWriteOnSliceParamNoFinding(t *testing.T) {
	source := []byte(`package main

func f(items []int) {
	items.Foo = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:items") {
		t.Errorf("computeGoFeatures findings for direct selector write on slice parameter %q: want no mutates_input finding, got %+v", source, findings)
	}
}

// Copilot review fix: derefOperand previously unwrapped any
// unary_expression inside parentheses without checking its operator, so a
// parenthesized non-dereference unary expression like (&cfg) could be
// mis-resolved as if it were (*cfg) and misattribute the write to cfg
// (Tree-sitter parses this syntactically even though it would not
// type-check: &cfg is **Config, which does not have a Name field the way
// *Config's (*cfg) does -- this detector never type-checks the file, only
// its own syntax shape, so the fix must reject this at the syntax level).
func TestGoMutatesInput_ParenthesizedAddressOfIsNotADereference(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(&cfg).Name = "y"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for parenthesized address-of (not a dereference) %q: want no mutates_input finding, got %+v", source, findings)
	}
}

// AC-3.7: Finding's grammar-node facts (Kind, Name, Location) are required
// and must stay first, in this order; the remaining fields are optional
// coaching metadata (Confidence, Evidence, Recommendation, SuggestedSkill)
// used by findings like "mutates_input" and omitted via omitempty for
// findings that don't set them. Checked structurally via reflection rather
// than by code review, so a future field addition fails this test loudly.
func TestFinding_StructCarriesOnlyDataFields(t *testing.T) {
	typ := reflect.TypeOf(Finding{})

	wantFields := []string{"Kind", "Name", "Location", "Confidence", "Evidence", "Recommendation", "SuggestedSkill"}
	if typ.NumField() != len(wantFields) {
		t.Fatalf("Finding field count: got %d fields, want exactly %d (%v)", typ.NumField(), len(wantFields), wantFields)
	}
	for i, want := range wantFields {
		if got := typ.Field(i).Name; got != want {
			t.Errorf("Finding field %d: got %q, want %q", i, got, want)
		}
	}
}
