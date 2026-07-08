//go:build js && wasm

// Command semantics-wasm-smoke is a minimal proof that pkg/semantics's
// pure-Go engine (see pkg/semantics/internal/engine/gotreesitter.go) works
// end-to-end when compiled for GOOS=js GOARCH=wasm and run in a real JS
// host -- not just that it compiles. It deliberately does not wire into
// js/semantics: a production cmd/semantics-wasm + backend-wasm.ts
// implementing js/semantics's Backend interface (see
// js/semantics/src/backend-default.ts) is a separate, later decision.
package main

import (
	"context"
	"encoding/json"
	"syscall/js"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// smokeSource is real Go source with a syntax error, a constructor-shaped
// function, and a pointer return, chosen so the smoke result exercises the
// syntax-error, import, metrics, and finding paths in one call rather than
// just proving the analyzer doesn't crash on trivial input.
const smokeSource = `package widget

import "fmt"

func NewWidget() *int {
	if true {
		fmt.Println("ok")
		return nil
	}
	return nil
`

func smokeAnalyze(this js.Value, args []js.Value) any {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return js.ValueOf(map[string]any{"ok": false, "error": err.Error()})
	}

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "smoke.go",
		Language: semantics.LanguageGo,
		Content:  []byte(smokeSource),
	})
	// smokeSource is missing its closing brace, so this must be the
	// syntax_errors double return, not a clean parse.
	if result == nil {
		return js.ValueOf(map[string]any{"ok": false, "error": "AnalyzeBytes returned a nil result: " + errString(err)})
	}

	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return js.ValueOf(map[string]any{"ok": false, "error": marshalErr.Error()})
	}
	return js.ValueOf(map[string]any{
		"ok":               true,
		"result":           string(encoded),
		"had_syntax_error": err != nil,
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func main() {
	js.Global().Set("__coachSemanticsSmokeAnalyze", js.FuncOf(smokeAnalyze))
	if onReady := js.Global().Get("__coachSemanticsSmokeOnReady"); onReady.Type() == js.TypeFunction {
		onReady.Invoke()
	}
	select {}
}
