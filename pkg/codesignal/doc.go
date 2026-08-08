// Package codesignal turns pkg/semantics analysis results into prioritized,
// change-aware coaching signals.
//
// Signal derivation is deterministic and rule-based. A shared registry
// (registry.go) dispatches each semantics.Finding.Kind to its rule
// constructor: "state.hidden_input_mutation" ("mutates_input"),
// "coupling.tight_constructor_init" ("tight_coupling"),
// "structure.constructor_density" ("constructor_func"),
// "structure.pointer_return_density" ("pointer_return"), and
// "security.toctou_check_then_act" ("toctou_check_then_act"). The two
// structure rules are density-gated: a file only surfaces them once its own
// per-side count of that finding kind reaches a threshold. Unrecognized
// finding kinds are ignored. A third dispatch path, signalsFromImports,
// derives "coupling.deep_relative_import" directly from a TypeScript/TSX
// file's semantics.ImportFeature list rather than from Findings or Metrics.
// A fourth path, signalsFromCognitiveComplexity, derives
// "complexity.cognitive_complexity" from per-function
// semantics.FunctionCognitiveComplexity records (threshold-gated in
// codesignal, not in semantics). A fifth path, signalsFromReactOrchestration,
// derives "structure.react_component_orchestration_density" from per-component
// semantics.ReactComponentFacts records via a state-domain classification and
// Emission Rule that live entirely in this package.
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
