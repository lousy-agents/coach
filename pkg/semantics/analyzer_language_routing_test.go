package semantics

import (
	"context"
	"testing"
)

// AC-R2.1, AC-R2.2: NewAnalyzer must accept LanguageTypeScript, and
// AnalyzeBytes on valid TS source must route to the TypeScript grammar and
// extractors, reporting ParseStatus "ok" and Result.Language "typescript".
func TestAnalyzeBytes_RoutesTypeScriptLanguageToTSGrammar(t *testing.T) {
	a, err := NewAnalyzer(AnalyzerOptions{Languages: []Language{LanguageTypeScript}})
	if err != nil {
		t.Fatalf("NewAnalyzer with Languages: []Language{LanguageTypeScript}: got err %v, want nil", err)
	}

	source := []byte(`import { A } from "./a";

function f(x: number) {
	if (x > 0) {
	}
}
`)
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "f.ts",
		Language: LanguageTypeScript,
		Content:  source,
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes for valid TS source %q: got err %v, want nil", source, err)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("AnalyzeBytes for valid TS source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "ok")
	}
	if result.Language != LanguageTypeScript {
		t.Errorf("AnalyzeBytes for valid TS source %q: Language = %q, want %q", source, result.Language, LanguageTypeScript)
	}
	if len(result.Imports) != 1 || result.Imports[0].Path != "./a" {
		t.Errorf("AnalyzeBytes for valid TS source %q: Imports = %+v, want one import with Path %q", source, result.Imports, "./a")
	}
	if result.Metrics.Ifs != 1 || result.Metrics.Functions != 1 {
		t.Errorf("AnalyzeBytes for valid TS source %q: Metrics = %+v, want Ifs=1 Functions=1", source, result.Metrics)
	}
}

// AC-R2.3: AnalyzeBytes with Language "tsx" on valid TSX source (containing
// at least one import and one metric-bearing construct) must report
// ParseStatus "ok", Result.Language "tsx", and the same import/metric
// extraction the TS path produces -- verifying the shared extractors work
// across both grammars end-to-end.
func TestAnalyzeBytes_RoutesTSXLanguageToTSXGrammar(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte(`import React from "react";

const App = () => {
	if (true) {
		return <div>hi</div>;
	}
	return null;
};
`)
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "App.tsx",
		Language: LanguageTSX,
		Content:  source,
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes for valid TSX source %q: got err %v, want nil", source, err)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("AnalyzeBytes for valid TSX source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "ok")
	}
	if result.Language != LanguageTSX {
		t.Errorf("AnalyzeBytes for valid TSX source %q: Language = %q, want %q", source, result.Language, LanguageTSX)
	}
	if len(result.Imports) != 1 || result.Imports[0].Path != "react" {
		t.Errorf("AnalyzeBytes for valid TSX source %q: Imports = %+v, want one import with Path %q", source, result.Imports, "react")
	}
	if result.Metrics.Functions != 1 || result.Metrics.Ifs != 1 {
		t.Errorf("AnalyzeBytes for valid TSX source %q: Metrics = %+v, want Functions=1 Ifs=1", source, result.Metrics)
	}
}
