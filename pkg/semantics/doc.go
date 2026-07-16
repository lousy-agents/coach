// Package semantics extracts deterministic structural facts from raw source
// bytes (syntax validity, imports, branching metrics, constructor-like
// patterns, mutates_input) using Tree-sitter grammars.
//
// This package never imports pkg/githubingest, go-github, or ghinstallation:
// consumers that only analyze raw source bytes never need to build or
// vendor a GitHub client. GitHub App-authenticated file fetching, if
// needed, lives in the separate, optional pkg/githubingest package.
//
// Engine: parsing is pure Go, via github.com/odvcencio/gotreesitter
// (pkg/semantics/internal/engine/gotreesitter.go). No CGO or C toolchain is
// involved at build or run time.
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
// multiple goroutines.
package semantics
