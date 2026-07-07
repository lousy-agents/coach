package semantics_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// ExampleAnalyzer_AnalyzeBytes shows analyzing valid Go source and reading
// the resulting structural findings.
func ExampleAnalyzer_AnalyzeBytes() {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	source := []byte(`package greet

func NewGreeter() *Greeter {
	return &Greeter{}
}

type Greeter struct{}
`)

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "greeter.go",
		Language: semantics.LanguageGo,
		Content:  source,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(result.ParseStatus)
	for _, f := range result.Findings {
		fmt.Println(f.Kind, f.Name)
	}
	// Output:
	// ok
	// constructor_func NewGreeter
	// pointer_return NewGreeter
}

// ExampleAnalyzer_AnalyzeBytes_syntaxError shows the error-matching pattern
// for source containing syntax errors: AnalyzeBytes returns both a partial
// *Result and an error satisfying errors.Is(err, semantics.ErrSyntax), from
// which a *semantics.SyntaxError can be recovered via errors.As.
func ExampleAnalyzer_AnalyzeBytes_syntaxError() {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Language: semantics.LanguageGo,
		Content:  []byte("package main\nfunc {"),
	})

	fmt.Println(result.ParseStatus)
	fmt.Println(errors.Is(err, semantics.ErrSyntax))

	var syntaxErr *semantics.SyntaxError
	if errors.As(err, &syntaxErr) {
		fmt.Println(len(syntaxErr.Issues) == len(result.SyntaxErrors))
	}
	// Output:
	// syntax_errors
	// true
	// true
}

// ExampleAnalyzer_AnalyzeBytes_typeScript shows analyzing valid TypeScript
// source, producing exactly one deterministic tight_coupling finding
// (AC-R6.3).
func ExampleAnalyzer_AnalyzeBytes_typeScript() {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	source := []byte(`import { HttpClient } from "./http";

class Greeter {
	constructor() {
		this.client = new HttpClient();
	}
}
`)

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "greeter.ts",
		Language: semantics.LanguageTypeScript,
		Content:  source,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(result.ParseStatus)
	for _, f := range result.Findings {
		fmt.Println(f.Kind, f.Name)
	}
	// Output:
	// ok
	// tight_coupling HttpClient
}
