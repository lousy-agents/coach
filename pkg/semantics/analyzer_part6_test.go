package semantics

import (
	"errors"

	"testing"
)

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

// thenErrorIs fails the test unless errors.Is(err, target) holds.
func thenErrorIs(t *testing.T, err, target error, why string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s: got err %v, want errors.Is(err, %v)", why, err, target)
	}
}
