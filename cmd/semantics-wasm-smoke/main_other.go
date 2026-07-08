//go:build !(js && wasm)

// Stub entry point so `go build ./...` / `go vet ./...` on native
// platforms don't fail with "build constraints exclude all Go files".
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "semantics-wasm-smoke is a js/wasm-only binary; build with GOOS=js GOARCH=wasm")
	os.Exit(1)
}
