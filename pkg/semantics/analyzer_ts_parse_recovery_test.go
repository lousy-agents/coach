package semantics

import (
	"context"
	"errors"
	"testing"
)

type tsSourceCase struct {
	name string
	lang Language
	src  string
}

// TestAnalyzeBytes_TSMissingInitializerFalseNegative covers the
// declaration/assignment-missing-RHS-expression shape gotreesitter's error
// recovery leaves root.HasError() == false for (issue #33): const/let/var
// with no initializer, and a bare assignment, each at top level, inside a
// function body, and inside a class body (proving detectTSBareStatementTokens's
// class_body guard still fires on a real bug there, not just skip it
// wholesale).
func TestAnalyzeBytes_TSMissingInitializerFalseNegative(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []tsSourceCase{
		{name: "const at top level", lang: LanguageTypeScript, src: "const x = ;\n"},
		{name: "let at top level", lang: LanguageTypeScript, src: "let y = ;\n"},
		{name: "var at top level", lang: LanguageTypeScript, src: "var z = ;\n"},
		{name: "bare assignment at top level", lang: LanguageTypeScript, src: "x = ;\n"},
		{
			name: "const inside function body",
			lang: LanguageTypeScript,
			src:  "function f() {\n  const x = ;\n}\n",
		},
		{
			name: "let inside class method body",
			lang: LanguageTypeScript,
			src:  "class C {\n  m() {\n    let x = ;\n  }\n}\n",
		},
		{
			name: "const inside TSX component",
			lang: LanguageTSX,
			src:  "function App() {\n  const x = ;\n  return <div />;\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectTSSyntaxErrorReported(t, a, tt)
		})
	}
}

func expectTSSyntaxErrorReported(t *testing.T, a *Analyzer, tt tsSourceCase) {
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "broken",
		Language: tt.lang,
		Content:  []byte(tt.src),
	})
	if result == nil {
		t.Fatalf("AnalyzeBytes(%q): got nil result, want a partial *Result", tt.src)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes(%q): ParseStatus = %q, want %q", tt.src, result.ParseStatus, "syntax_errors")
	}
	if len(result.SyntaxErrors) == 0 {
		t.Errorf("AnalyzeBytes(%q): SyntaxErrors is empty, want at least one issue", tt.src)
	}
	if !errors.Is(err, ErrSyntax) {
		t.Errorf("AnalyzeBytes(%q): errors.Is(err, ErrSyntax) = false, want true (err = %v)", tt.src, err)
	}
}

// TestAnalyzeBytes_TSPlainIdentifierDefaultParametersParseOK covers the
// specific false-positive gap (issue #33) WantsForest fixes:
// gotreesitter's plain (non-forest) parse path misparses plain-identifier
// default parameters and array-destructuring defaults as syntax errors, even
// though they are ordinary valid TS/TSX. Enabling forest routing for
// TypeScript/TSX (see GoTreeSitterLanguage in
// internal/engine/gotreesitter.go) fixes these without regressing genuinely
// malformed input -- see TestAnalyzeBytes_TSMissingInitializerFalseNegative
// and TestAnalyzeBytes_TSEndToEndSyntaxErrorContract, which are unaffected.
func TestAnalyzeBytes_TSPlainIdentifierDefaultParametersParseOK(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []tsSourceCase{
		{name: "plain identifier default parameter", lang: LanguageTypeScript, src: "function f(x = 1) {}\n"},
		{name: "arrow function plain identifier default parameter", lang: LanguageTypeScript, src: "const g = (a = 1) => {};\n"},
		{name: "array destructuring default", lang: LanguageTypeScript, src: "const [a = 2] = z;\n"},
		{
			name: "object destructuring default combined with JSX",
			lang: LanguageTSX,
			src:  "function App({name = 'world'}: {name?: string}) { return <div>{name}</div>; }\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectTSParsesCleanly(t, a, tt)
		})
	}
}

func expectTSParsesCleanly(t *testing.T, a *Analyzer, tt tsSourceCase) {
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "ok",
		Language: tt.lang,
		Content:  []byte(tt.src),
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes(%q): got err %v, want nil", tt.src, err)
	}
	if result == nil {
		t.Fatalf("AnalyzeBytes(%q): got nil result, want a valid *Result", tt.src)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("AnalyzeBytes(%q): ParseStatus = %q, want %q (SyntaxErrors = %+v)", tt.src, result.ParseStatus, "ok", result.SyntaxErrors)
	}
	if len(result.SyntaxErrors) != 0 {
		t.Errorf("AnalyzeBytes(%q): SyntaxErrors = %+v, want empty", tt.src, result.SyntaxErrors)
	}
}
