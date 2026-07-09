// Package semantics extracts deterministic structural facts from raw source
// bytes (syntax validity, imports, branching metrics, constructor-like
// patterns, mutates_input) using Tree-sitter grammars.
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
// The "mutates_input" finding is a syntax-based, conservative first-slice
// detector: it flags caller-visible writes through a function/method's own
// parameters without any whole-program alias analysis or type inference
// beyond that parameter's own syntactic declaration. Go detects assignment
// and update writes rooted at pointer/map/slice parameters. TS/TSX fires on
// the same underlying idea — a parameter mutated in place is a hidden side
// effect on the caller's value — but each language's detection is
// necessarily different: Go parameter types are explicit in the source
// (pointer_type/map_type/slice_type), so mutableParamTypes reads them
// directly (features.go), while TS/TSX has no required type annotations, so
// tsParamScope instead tracks which identifiers are bound to (non-
// destructured, non-rest, non-defaulted) parameters and matches property/
// index assignments, update expressions, deletes, and a fixed list of known
// mutating collection methods (including bracket notation such as arr["push"])
// (ts_features.go). Neither detector tracks aliases assigned to local
// variables or follows values across function calls.
//
// Concurrency: an *Analyzer holds no backend-held resources between calls —
// AnalyzeBytes creates and closes its own Parser, Tree, Query, and
// QueryCursor per call — so a single *Analyzer is safe for concurrent use by
// multiple goroutines, regardless of which engine backend is compiled in.
package semantics
