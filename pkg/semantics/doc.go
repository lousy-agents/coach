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
// The "toctou_check_then_act" finding (CWE-367) flags a filesystem
// existence/state check whose success path later acts on the identical path
// expression — a non-atomic check-then-act window a concurrent process can
// race, adapted from a Semgrep TOCTOU rule referenced by GitHub issue #162.
// TS/TSX (ts_toctou.go) detects a bare existsSync/fs.existsSync call as an
// if/while condition guarding a same-path act call (readFileSync,
// writeFileSync, appendFileSync, unlinkSync, rmSync, bare or fs. form); Go
// (toctou_go.go) detects an os.Stat/os.Lstat call bound by the if
// statement's own initializer (if _, err := os.Stat(p); err == nil) whose
// error result is gated by a direct err == nil comparison guarding a
// same-path act call (os.Open, os.OpenFile, os.Remove, os.RemoveAll,
// os.ReadFile); a Stat/Lstat call in a preceding statement is never
// considered. Both detectors match path identity by Tree-sitter source-text
// equality only — no alias analysis and no path.join/path.resolve
// reconstruction — so several conservative forms are known, intentional
// non-detections rather than bugs: a bare existsSync with no following act
// in the guarded body; an existsSync combined via && or ||, a ternary, or a
// for-loop condition; a negated if whose act sits only in the else branch
// (TS/TSX); Go's inverse err != nil early-return form; a Go sentinel gate
// such as errors.Is(err, fs.ErrNotExist) or os.IsNotExist(err) used as the
// sole success-path check instead of a direct nil comparison; a Go
// Stat/Lstat call whose results are discarded rather than bound and gated;
// and a Go act call reached only through a walk callback with no
// Stat/Lstat gate of its own. Async-only pairs (e.g. fs.promises.access
// followed by an awaited read) are out of scope for v1 in both languages.
//
// Concurrency: an *Analyzer holds no backend-held resources between calls —
// AnalyzeBytes creates and closes its own Parser, Tree, Query, and
// QueryCursor per call — so a single *Analyzer is safe for concurrent use by
// multiple goroutines.
package semantics
