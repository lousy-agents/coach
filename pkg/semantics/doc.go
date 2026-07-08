// Package semantics extracts deterministic structural facts from raw source
// bytes (syntax validity, imports, branching metrics, constructor-like
// patterns) using Tree-sitter grammars.
//
// This package never imports pkg/githubingest, go-github, or ghinstallation:
// consumers that only analyze raw source bytes never need to build or
// vendor a GitHub client. GitHub App-authenticated file fetching, if
// needed, lives in the separate, optional pkg/githubingest package.
//
// Engine backends: by default this package binds to Tree-sitter's C runtime
// via github.com/tree-sitter/go-tree-sitter, which requires CGO_ENABLED=1
// and a C toolchain at build time. When CGO is unavailable — CGO_ENABLED=0,
// or GOOS=js GOARCH=wasm, where CGO cannot be used at all — it automatically
// falls back to a pure-Go engine (github.com/odvcencio/gotreesitter,
// pkg/semantics/internal/engine/gotreesitter.go), with no code or build-flag
// changes required from callers. The pure-Go engine is newer than the CGO
// one and is verified only against the fixture corpus in
// pkg/semantics/backend_conformance_test.go: it is not proven to detect
// every syntax error the CGO engine detects on adversarial malformed input
// (see that file's comments for a known, tracked example), though the two
// engines agree exactly on every clean parse in the conformance suite. The
// coach_gotreesitter build tag forces the pure-Go engine on a native build
// (CGO available) for testing or comparison.
//
// Concurrency: an *Analyzer holds no backend-held resources between calls —
// AnalyzeBytes creates and closes its own Parser, Tree, Query, and
// QueryCursor per call — so a single *Analyzer is safe for concurrent use by
// multiple goroutines, regardless of which engine backend is compiled in.
package semantics
