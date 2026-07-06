// Package semantics extracts deterministic structural facts from raw source
// bytes (syntax validity, imports, branching metrics, constructor-like
// patterns) using Tree-sitter grammars, without depending on GitHub or any
// pkg/githubingest code.
//
// CGO requirement: this package binds to Tree-sitter's C runtime via
// github.com/tree-sitter/go-tree-sitter, so it cannot be built with
// CGO_ENABLED=0. A C toolchain must be available at build time.
//
// Concurrency: an *Analyzer holds no C-backed resources between calls —
// AnalyzeBytes creates and closes its own Parser, Tree, Query, and
// QueryCursor per call — so a single *Analyzer is safe for concurrent use by
// multiple goroutines.
package semantics
