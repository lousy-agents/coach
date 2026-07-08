//go:build cgo

// Conformance tests comparing the CGO (go-tree-sitter) and pure-Go
// (gotreesitter) engine backends. Tagged cgo (not coach_gotreesitter):
// pkg/semantics/internal/engine/gotreesitter.go carries no build tag and
// is always compiled, so both backends are already available under plain
// cgo, and this file never touches the package-level languageRegistry (see
// analyzeWith below) -- it needs nothing from language_gotreesitter.go.
package semantics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

var cgoConformanceRegistry = map[Language]languageSpec{
	LanguageGo: {
		engineLang:      engine.CGOLanguage(tree_sitter_go.Language),
		extractImports:  extractGoImports,
		computeFeatures: computeGoFeatures,
	},
	LanguageTypeScript: {
		engineLang:      engine.CGOLanguage(tree_sitter_typescript.LanguageTypescript),
		extractImports:  extractTSImports,
		computeFeatures: computeTSFeatures,
	},
	LanguageTSX: {
		engineLang:      engine.CGOLanguage(tree_sitter_typescript.LanguageTSX),
		extractImports:  extractTSXImports,
		computeFeatures: computeTSFeatures,
	},
}

var gtsConformanceRegistry = map[Language]languageSpec{
	LanguageGo: {
		engineLang:      engine.GoTreeSitterLanguage("go"),
		extractImports:  extractGoImports,
		computeFeatures: computeGoFeatures,
	},
	LanguageTypeScript: {
		engineLang:      engine.GoTreeSitterLanguage("typescript"),
		extractImports:  extractTSImports,
		computeFeatures: computeTSFeatures,
	},
	LanguageTSX: {
		engineLang:      engine.GoTreeSitterLanguage("tsx"),
		extractImports:  extractTSXImports,
		computeFeatures: computeTSFeatures,
	},
}

// analyzeWith runs the AnalyzeBytes pipeline (validate -> parse ->
// syntax-error branch -> extract/compute) against an explicit registry
// instead of the package-level languageRegistry, so both backends can be
// exercised side by side in one test binary regardless of which
// language_*.go file happens to be compiled in. This intentionally
// duplicates validate (parser.go) and AnalyzeBytes (analyzer.go); keep it in
// sync with those if the pipeline shape changes.
func analyzeWith(reg map[Language]languageSpec, opts AnalyzerOptions, in FileInput) (*Result, error) {
	if opts.MaxFileBytes < 0 {
		return nil, fmt.Errorf("semantics: MaxFileBytes must be >= 0, got %d", opts.MaxFileBytes)
	}
	allowed := make(map[Language]bool, len(opts.Languages))
	for _, lang := range opts.Languages {
		if _, ok := reg[lang]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, lang)
		}
		allowed[lang] = true
	}

	if len(in.Content) == 0 {
		return nil, ErrEmptyContent
	}
	if _, ok := reg[in.Language]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, in.Language)
	}
	if len(allowed) > 0 && !allowed[in.Language] {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, in.Language)
	}
	max := opts.MaxFileBytes
	if max <= 0 {
		max = defaultMaxFileBytes
	}
	if len(in.Content) > max {
		return nil, fmt.Errorf("%w: content is %d bytes, exceeds max %d bytes", ErrFileTooLarge, len(in.Content), max)
	}
	if bytes.IndexByte(in.Content, 0x00) != -1 {
		return nil, ErrBinaryContent
	}

	spec := reg[in.Language]
	parser, err := spec.engineLang.NewParser()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailure, err)
	}
	tree, err := parser.Parse(in.Content)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailure, err)
	}
	if tree == nil {
		return nil, fmt.Errorf("%w: Parse returned a nil tree", ErrParseFailure)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		issues := collectSyntaxIssues(root)
		result := &Result{
			Path:         in.Path,
			Language:     in.Language,
			ParseStatus:  ParseStatus("syntax_errors"),
			SyntaxErrors: issues,
		}
		return result, &SyntaxError{Issues: issues}
	}

	imports, err := spec.extractImports(spec.engineLang, root, in.Content)
	if err != nil {
		return nil, err
	}
	metrics, findings := spec.computeFeatures(root, in.Content)

	return &Result{
		Path:        in.Path,
		Language:    in.Language,
		ParseStatus: ParseStatus("ok"),
		Imports:     imports,
		Metrics:     metrics,
		Findings:    findings,
	}, nil
}

// parityCase mirrors internal/jsbridge's manifest.json shape, so both
// backends can replay the exact fixtures the Go/JS bridge parity suite
// already locks byte-for-byte against pkg/semantics's real output.
type parityCase struct {
	Name     string `json:"name"`
	Src      string `json:"src"`
	Path     string `json:"path"`
	Language string `json:"language"`
	Options  struct {
		Languages    []string `json:"languages"`
		MaxFileBytes int      `json:"max_file_bytes"`
	} `json:"options"`
	Expected string `json:"expected"`
}

type parityExpectedResponse struct {
	Result *Result `json:"result"`
	Error  *struct {
		Kind string `json:"kind"`
	} `json:"error"`
}

const parityFixtureDir = "../../internal/jsbridge/testdata/parity"

func loadParityManifest(t *testing.T) []parityCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parityFixtureDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var cases []parityCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return cases
}

// TestBackendConformance_FixtureReplay runs every internal/jsbridge parity
// fixture through both engine backends and asserts each agrees with the
// other AND with the Go-locked .expected.json -- catching either backend
// drifting from frozen output, not just the backends disagreeing with each
// other.
func TestBackendConformance_FixtureReplay(t *testing.T) {
	for _, tc := range loadParityManifest(t) {
		t.Run(tc.Name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(parityFixtureDir, tc.Src))
			if err != nil {
				t.Fatalf("read src: %v", err)
			}
			expectedRaw, err := os.ReadFile(filepath.Join(parityFixtureDir, tc.Expected))
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			var expected parityExpectedResponse
			if err := json.Unmarshal(expectedRaw, &expected); err != nil {
				t.Fatalf("parse expected: %v", err)
			}

			opts := AnalyzerOptions{MaxFileBytes: tc.Options.MaxFileBytes}
			for _, l := range tc.Options.Languages {
				opts.Languages = append(opts.Languages, Language(l))
			}
			in := FileInput{Path: tc.Path, Language: Language(tc.Language), Content: content}

			cgoResult, cgoErr := analyzeWith(cgoConformanceRegistry, opts, in)
			gtsResult, gtsErr := analyzeWith(gtsConformanceRegistry, opts, in)

			// The CGO backend's output is the shipped default and the
			// frozen contract itself, so it must match the golden
			// .expected.json exactly in every case, including syntax_errors
			// diagnostic detail.
			assertMatchesExpected(t, "cgo", cgoResult, cgoErr, expected, true)
			// The pure-Go backend must match exactly for every outcome
			// EXCEPT the syntax_errors diagnostic detail: two independently
			// implemented parsers are expected to diverge in error-recovery
			// heuristics for malformed input (see
			// TestBackendConformance_GrammarParity's ts_syntax_errors case,
			// where gotreesitter's TS grammar promotes the whole root to an
			// ERROR node where go-tree-sitter keeps "program" with nested
			// ERROR/MISSING descendants). AnalyzeBytes never computes
			// Imports/Metrics/Findings for a HasError() tree, so this
			// divergence cannot reach the fields that drive real output --
			// only the SyntaxErrors position list can differ. Contract-level
			// agreement (parse_status, at least one issue, zero-valued
			// Metrics/Imports/Findings) is asserted instead.
			assertMatchesExpected(t, "gotreesitter", gtsResult, gtsErr, expected, false)
		})
	}
}

// assertMatchesExpected checks result/err against expected (one manifest
// fixture's golden .expected.json). strictSyntaxErrors controls whether the
// syntax_errors case requires byte-identical Result JSON (true) or only the
// weaker contract every backend must uphold (false) -- see the comment at
// this function's call site in TestBackendConformance_FixtureReplay for why
// the two backends are held to different bars here.
func assertMatchesExpected(t *testing.T, backend string, result *Result, err error, expected parityExpectedResponse, strictSyntaxErrors bool) {
	t.Helper()

	if expected.Error != nil && expected.Error.Kind == "syntax" {
		if _, ok := err.(*SyntaxError); !ok {
			t.Fatalf("%s: err = %v (%T), want *SyntaxError", backend, err, err)
		}
		if !strictSyntaxErrors {
			assertSyntaxErrorContract(t, backend, result)
			return
		}
	} else if expected.Error != nil {
		if err == nil {
			t.Fatalf("%s: err is nil, want a non-nil error (kind %q)", backend, expected.Error.Kind)
		}
		return
	} else if err != nil {
		t.Fatalf("%s: unexpected error: %v", backend, err)
	}

	if result == nil {
		t.Fatalf("%s: result is nil, want a Result", backend)
	}
	got, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("%s: marshal result: %v", backend, err)
	}
	want, err := json.MarshalIndent(expected.Result, "", "  ")
	if err != nil {
		t.Fatalf("marshal expected result: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s: Result diverges from golden expected.json\ngot:\n%s\nwant:\n%s", backend, got, want)
	}
}

// assertSyntaxErrorContract checks the observable guarantee every backend
// must uphold on malformed input, without requiring identical error-recovery
// diagnostic detail: parse_status is reported, at least one issue is found,
// and Imports/Metrics/Findings stay zero-valued (AnalyzeBytes never computes
// them for a HasError() tree, regardless of backend).
func assertSyntaxErrorContract(t *testing.T, backend string, result *Result) {
	t.Helper()
	if result == nil {
		t.Fatalf("%s: result is nil, want the partial Result alongside the syntax error", backend)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("%s: parse_status = %q, want %q", backend, result.ParseStatus, "syntax_errors")
	}
	if len(result.SyntaxErrors) == 0 {
		t.Errorf("%s: SyntaxErrors is empty, want at least one issue", backend)
	}
	if len(result.Imports) != 0 || len(result.Findings) != 0 || result.Metrics != (StructuralMetrics{}) {
		t.Errorf("%s: Imports/Findings/Metrics are non-zero on a syntax_errors Result: %+v", backend, result)
	}
}

// malformedFixture is a hand-picked malformed/partial source sample, not
// present in internal/jsbridge's parity manifest, specifically chosen to
// exercise recovery paths (unterminated strings, unbalanced braces,
// whitespace-only content) that both backends' Parse must survive without
// a hard error, per go-tree-sitter's "always returns a tree" contract.
type malformedFixture struct {
	name     string
	language Language
	content  []byte
}

var malformedFixtures = []malformedFixture{
	{name: "go_unterminated_string", language: LanguageGo, content: []byte("package main\n\nvar s = \"unterminated\n")},
	{name: "go_unbalanced_braces", language: LanguageGo, content: []byte("package main\n\nfunc oops( {\n\tif true {\n}\n")},
	{name: "ts_unterminated_string", language: LanguageTypeScript, content: []byte("const s = \"unterminated\n")},
	{name: "ts_unbalanced_braces", language: LanguageTypeScript, content: []byte("export function broken( {\n  if (true) {\n}\n")},
	{name: "tsx_unbalanced_jsx", language: LanguageTSX, content: []byte("const App = () => <div>hi;\n")},
}

// TestBackendConformance_MalformedSource proves both backends handle
// malformed input the same way: a partial Result (parse_status
// "syntax_errors") plus a *SyntaxError, never a hard parse-failure error.
// It does not require the two backends' SyntaxErrors detail to be
// byte-identical -- see assertMatchesExpected's comment on
// strictSyntaxErrors for why two independently implemented parsers'
// error-recovery output is allowed to diverge here.
func TestBackendConformance_MalformedSource(t *testing.T) {
	for _, tc := range malformedFixtures {
		t.Run(tc.name, func(t *testing.T) {
			in := FileInput{Path: tc.name, Language: tc.language, Content: tc.content}

			cgoResult, cgoErr := analyzeWith(cgoConformanceRegistry, AnalyzerOptions{}, in)
			gtsResult, gtsErr := analyzeWith(gtsConformanceRegistry, AnalyzerOptions{}, in)

			if _, ok := cgoErr.(*SyntaxError); !ok {
				t.Fatalf("cgo: err = %v (%T), want *SyntaxError (never a hard parse failure)", cgoErr, cgoErr)
			}
			assertSyntaxErrorContract(t, "cgo", cgoResult)

			if _, ok := gtsErr.(*SyntaxError); !ok {
				t.Fatalf("gotreesitter: err = %v (%T), want *SyntaxError (never a hard parse failure)", gtsErr, gtsErr)
			}
			assertSyntaxErrorContract(t, "gotreesitter", gtsResult)
		})
	}
}

// TestBackendConformance_GrammarParity walks both backends' trees for the
// same input in lockstep, asserting Kind() and ChildCount() agree
// node-for-node. This catches grammar-version drift between go-tree-sitter's
// pinned grammars and gotreesitter's bundled ones even in cases where the
// final Result happens to still match by coincidence.
//
// Restricted to clean-parse ("_ok") fixtures: malformed input is exactly
// where two independently implemented parsers' error-recovery heuristics
// are expected to diverge (e.g. gotreesitter's TypeScript grammar can
// promote the whole root to a single ERROR node where go-tree-sitter keeps
// "program" with nested ERROR/MISSING descendants), so a node-for-node walk
// isn't a meaningful check there. TestBackendConformance_MalformedSource
// covers that case at the contract level instead.
func TestBackendConformance_GrammarParity(t *testing.T) {
	sources := []struct {
		name     string
		language Language
		content  []byte
	}{
		{"go_ok", LanguageGo, mustReadFixture(t, "go_ok.src")},
		{"ts_ok", LanguageTypeScript, mustReadFixture(t, "ts_ok.src")},
		{"tsx_ok", LanguageTSX, mustReadFixture(t, "tsx_ok.src")},
	}

	for _, tc := range sources {
		t.Run(tc.name, func(t *testing.T) {
			cgoSpec := cgoConformanceRegistry[tc.language]
			gtsSpec := gtsConformanceRegistry[tc.language]

			cgoParser, err := cgoSpec.engineLang.NewParser()
			if err != nil {
				t.Fatalf("cgo NewParser: %v", err)
			}
			cgoTree, err := cgoParser.Parse(tc.content)
			if err != nil {
				t.Fatalf("cgo Parse: %v", err)
			}
			defer cgoTree.Close()

			gtsParser, err := gtsSpec.engineLang.NewParser()
			if err != nil {
				t.Fatalf("gotreesitter NewParser: %v", err)
			}
			gtsTree, err := gtsParser.Parse(tc.content)
			if err != nil {
				t.Fatalf("gotreesitter Parse: %v", err)
			}
			defer gtsTree.Close()

			walkBoth(t, cgoTree.RootNode(), gtsTree.RootNode(), "root")
		})
	}
}

func walkBoth(t *testing.T, a, b engine.Node, path string) {
	t.Helper()
	if a == nil || b == nil {
		if a != b {
			t.Errorf("%s: nilness mismatch: cgo=%v gotreesitter=%v", path, a == nil, b == nil)
		}
		return
	}
	if a.Kind() != b.Kind() {
		t.Errorf("%s: Kind mismatch: cgo=%q gotreesitter=%q", path, a.Kind(), b.Kind())
		return
	}
	if a.ChildCount() != b.ChildCount() {
		t.Errorf("%s (%s): ChildCount mismatch: cgo=%d gotreesitter=%d", path, a.Kind(), a.ChildCount(), b.ChildCount())
		return
	}
	for i := 0; i < a.ChildCount(); i++ {
		walkBoth(t, a.Child(i), b.Child(i), fmt.Sprintf("%s.child[%d]", path, i))
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(parityFixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return content
}

// TestBackendConformance_ParseNeverHardErrorsOnMalformedInput is the
// Milestone-2-spike assertion promoted to a permanent regression test: the
// gotreesitter backend's plain Parse (never ParseStrict) must return
// (tree, nil) with tree.RootNode().HasError() == true for malformed input,
// not (nil, err) -- confirmed empirically before this backend was wired in.
func TestBackendConformance_ParseNeverHardErrorsOnMalformedInput(t *testing.T) {
	for _, tc := range malformedFixtures {
		t.Run(tc.name, func(t *testing.T) {
			spec := gtsConformanceRegistry[tc.language]
			parser, err := spec.engineLang.NewParser()
			if err != nil {
				t.Fatalf("NewParser: %v", err)
			}
			tree, err := parser.Parse(tc.content)
			if err != nil {
				t.Fatalf("Parse returned a hard error for malformed input: %v", err)
			}
			if tree == nil {
				t.Fatal("Parse returned a nil tree for malformed input")
			}
			defer tree.Close()
			if !tree.RootNode().HasError() {
				t.Error("Parse succeeded without HasError() on malformed input; fixture may no longer be malformed")
			}
		})
	}
}
