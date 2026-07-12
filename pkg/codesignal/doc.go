// Package codesignal turns pkg/semantics analysis results into prioritized,
// change-aware coaching signals.
//
// Signal derivation is deterministic and rule-based. v0.1 defines one rule,
// "state.hidden_input_mutation", mapped from semantics.Finding values of
// Kind "mutates_input". Unrecognized finding kinds are ignored.
//
// A Signal's Lifecycle classifies it relative to an optional Base result.
// Signal.Changed is independent of Lifecycle: a pre-existing signal may sit
// on a changed line, and a new signal may sit outside any changed range.
//
// Fingerprints are stable across line moves within a file, but adding or
// removing an earlier duplicate with the same key can shift occurrence
// ordinals and change later fingerprints.
//
// pkg/codesignal itself does no I/O or LLM calls: Build is a pure function
// of its Input.
package codesignal
