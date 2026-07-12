// Package codesignal turns pkg/semantics analysis results into prioritized,
// change-aware coaching signals -- severity, lifecycle, a recommendation,
// and a stable identity -- so reviewers and coaching agents can act on a
// pull request's structural findings without re-deriving that
// interpretation themselves from raw semantics.Result output.
//
// Signal derivation is deterministic and rule-based: there are no LLM
// calls and no subjective architectural judgment involved. Every Signal
// traces back to an explicit, versioned rule applied to pkg/semantics's
// deterministic output; v0.1 defines exactly one, "state.hidden_input_mutation"
// (rule_hidden_input_mutation.go), mapped from semantics.Finding values of
// Kind "mutates_input". This package is not a generic severity wrapper
// around every semantics.Finding -- finding kinds that no rule recognizes
// are silently ignored rather than defaulted to some generic signal.
//
// This package only consumes pkg/semantics's public API (Result, Finding,
// Location, SyntaxIssue, and friends) and is never imported back by
// pkg/semantics, mirroring the one-directional dependency rule already
// enforced between pkg/semantics and pkg/githubingest: a consumer that only
// needs raw structural analysis never has to pull in coaching-signal logic
// to get it.
//
// A Signal's Lifecycle -- "introduced", "existing", "resolved", or
// "unknown" -- classifies it by comparing a FileChange's Head result
// against its optional Base result; lifecycle.go documents the matching
// algorithm (grouping by rule/path/subject/evidence, then ordering
// same-key duplicates by location) in more depth. Lifecycle classification
// requires a non-nil FileChange.Base for the file in question: a file with
// no supplied Base gets "unknown" lifecycle for every one of its signals,
// since there is no baseline available to compare against. Signal.Changed
// and Signal.Lifecycle are independent dimensions rather than a single
// collapsed score -- a signal can be pre-existing and still sit on a line
// the change touched, or newly introduced and sit entirely outside any
// changed range.
//
// Each Signal also carries a Fingerprint intended to survive the finding's
// line moving within the same file across Base and Head. That
// location-independence has a documented limit, described fully in
// fingerprint.go: fingerprints incorporate an occurrence ordinal assigned
// by sorting same-key duplicates by location on every run, so adding or
// removing an earlier duplicate finding in the same file can shift the
// ordinals -- and therefore the fingerprints -- of later duplicates in
// that group, even though those later findings didn't themselves change.
//
// pkg/codesignal itself never touches the filesystem, the network, or an
// LLM: a Builder's Build is a pure function of the Input it's given.
// cmd/codesignal-report is a separate, thin adapter that does that I/O
// (reading files, invoking pkg/semantics, writing a report) and then calls
// into this package, which keeps this package's own purity intact.
package codesignal
