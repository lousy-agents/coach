package codesignal

import (
	"reflect"
	"strings"
	"testing"
)

var frozenReportJSONFieldNames = map[string]struct{}{
	"schema_version": {}, "scope": {}, "summary": {}, "signals": {},
	"diagnostics": {}, "coverage": {}, "project_changes": {}, "project_facts": {},
	"project_summary": {}, "project_coverage": {},
	"repository": {}, "revision": {}, "base": {}, "applied_scope": {}, "baseline": {},
	"files_analyzed": {}, "files_with_diagnostics": {}, "files_unanalyzed": {}, "active_signals": {},
	"introduced_signals": {}, "existing_signals": {}, "resolved_signals": {},
	"baseline_signals": {}, "unknown_signals": {},
	"id": {}, "fingerprint": {}, "rule_id": {}, "rule_version": {}, "kind": {},
	"category": {}, "severity": {}, "confidence": {}, "lifecycle": {}, "changed": {},
	"path": {}, "source_scope": {}, "subject": {}, "location": {}, "evidence": {},
	"why_it_matters": {}, "recommendation": {}, "suggested_skill": {}, "provenance": {},
	"machine_evidence": {}, "related_locations": {}, "path_steps": {}, "coverage_refs": {},
	"producer": {}, "finding_kind": {}, "language": {},
	"message":                  {},
	"tracked_files_discovered": {}, "files_unanalyzable": {}, "unsupported": {}, "excluded": {},
	"reason": {}, "count": {},
	"semantic_key": {}, "backend_version": {}, "algorithm_version": {}, "config_digest": {},
	"causal_evidence_digest": {}, "primary_anchor": {},
	"node_id": {}, "display_name": {}, "resolution": {}, "source_locations": {},
	"active_changes": {}, "introduced_changes": {}, "existing_changes": {},
	"resolved_changes": {}, "baseline_changes": {},
	"start_byte": {}, "end_byte": {}, "start_row": {}, "start_col": {}, "end_row": {}, "end_col": {},
	"phase": {}, "complete": {}, "counts": {}, "budgets": {},
	"code": {},

	"signals_withheld": {}, "min_severity": {}, "below_min_severity": {}, "top": {}, "beyond_top": {},

	"project_provenance": {}, "project_scope": {}, "project_next_actions": {},
	"selected_roots": {}, "analyzer": {}, "runtime": {}, "package_manager": {}, "head": {},
	"version": {}, "digest": {}, "protocol_version": {},
	"origin": {}, "compiler_version": {}, "compiler_origin": {}, "declared_version": {},
	"model": {}, "bypass": {}, "reachability": {},
	"inclusion_rule": {}, "pattern_set": {},
	"roots": {}, "matched_layers": {}, "unmatched_layers": {},
	"root": {}, "candidate_files": {}, "analyzed_files": {},
}

// jsonFieldWalker walks a type's json struct tags, following pointers,
// slices, arrays, and map values (never map keys, which are caller data, not
// schema) across package boundaries, and records every field's tag name (the
// part before any comma) in out. It collects a flat set rather than a
// per-type map: the frozen-name assertion below only needs to know which
// names can ever appear in Report's JSON output, not which struct owns each
// one, so a struct-tag rename is caught even for a field that happens to be
// zero/empty (and therefore invisible via omitempty) in every golden
// fixture. The walker owns the traversal state so field-name collection
// writes the walker's maps rather than caller-owned parameters.
type jsonFieldWalker struct {
	seen map[reflect.Type]bool
	out  map[string]struct{}
}

func (w *jsonFieldWalker) walk(t reflect.Type) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		w.walk(t.Elem())
		return
	case reflect.Struct:
	default:
		return
	}

	if w.seen[t] {
		return
	}
	w.seen[t] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		tag, ok := field.Tag.Lookup("json")
		if !ok {
			w.out[field.Name] = struct{}{}
			w.walk(field.Type)
			continue
		}

		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}

		w.out[name] = struct{}{}
		w.walk(field.Type)
	}
}

// TestFrozenSchema_FieldNames locks Report's JSON field names (across every
// nested type it can reach) against accidental rename, independent of which
// fields any golden fixture happens to populate. It walks Go struct
// tags via reflection rather than any single marshaled Report, so the same
// assertion holds regardless of whether a field is present-but-empty or
// omitted in a given fixture. Issue #269 (#308) made Report's own
// slice/map/pointer fields always-present rather than `omitempty`; this
// test needed no changes when that landed, exactly as designed -- a
// presence/absence change is not a rename.
//
// This test freezes JSON field names only, for Report's JSON rendering path.
// It says nothing about text-format output: internal/codesignalcli/render/report_text.go's
// ReportText is pinned byte-for-byte only for the single scenario in
// internal/codesignalcli/render/signals_test.go's
// TestRenderTextSignalsPresentRenderingIsPinnedExactly; there is no
// exhaustive, field-name-level freeze of the text schema equivalent to this
// test. That narrower gap is not something this test (or #218/T7, scoped to
// golden_test.go and README.md) covers -- do not read this test's presence as
// evidence that the "text" half of AC-3 is satisfied.
func TestFrozenSchema_FieldNames(t *testing.T) {
	got := map[string]struct{}{}
	(&jsonFieldWalker{seen: map[reflect.Type]bool{}, out: got}).walk(reflect.TypeOf(Report{}))

	t.Run("NoFrozenNameMissing", func(t *testing.T) {
		expectEveryFrozenNameReachable(t, got)
	})
	t.Run("NoUnexpectedNameAppeared", func(t *testing.T) {
		expectOnlyFrozenNamesReachable(t, got)
	})
}

func expectEveryFrozenNameReachable(t *testing.T, got map[string]struct{}) {
	for name := range frozenReportJSONFieldNames {
		if _, ok := got[name]; !ok {
			t.Errorf("expected JSON field %q not found among Report's reachable struct tags (renamed or removed?)", name)
		}
	}
}

func expectOnlyFrozenNamesReachable(t *testing.T, got map[string]struct{}) {
	for name := range got {
		if _, ok := frozenReportJSONFieldNames[name]; !ok {
			t.Errorf("unexpected JSON field %q found among Report's reachable struct tags (new field, or a rename, not reflected in frozenReportJSONFieldNames)", name)
		}
	}
}
